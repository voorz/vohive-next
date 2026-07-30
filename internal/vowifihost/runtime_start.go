package vowifihost

import (
	"context"
	"fmt"
	"strings"
	"time"

	swusim "github.com/voorz/vowifi-core/engine/sim"
	"github.com/voorz/vowifi-core/runtimehost"
	"github.com/voorz/vowifi-core/runtimehost/eventhost"
	"github.com/voorz/vowifi-core/runtimehost/messaging"
	"github.com/voorz/vowifi-core/runtimehost/voiceclient"
	"github.com/voorz/vowifi-core/runtimehost/voicehost"

	"github.com/voorz/vohive/pkg/logger"
)

type runtimeStartFunc func(context.Context, runtimehost.StartRequest) (*runtimehost.Instance, error)

type missingSIMProvider struct{}

func (m missingSIMProvider) GetIMSI() (string, error) {
	return "", fmt.Errorf("missing SIM provider")
}
func (m missingSIMProvider) CalculateAKA(rand, autn []byte) (swusim.AKAResult, error) {
	return swusim.AKAResult{}, fmt.Errorf("missing SIM provider")
}
func (m missingSIMProvider) Close() error { return nil }

// buildVoWiFiSIMAdapter prefers an injected SIM adapter (e.g. MBIM Auth AKA for
// modems without SIM logical-channel APDU); otherwise derives one from the
// modem's APDU path (AT/QMI).
func buildVoWiFiSIMAdapter(override runtimehost.SIMAdapter, modem runtimehost.Modem, imsi string) runtimehost.SIMAdapter {
	if override != nil {
		return override
	}
	// 所有后端的 AKA 现由 vohive 注入；缺失说明编排未设置，属调用错误。
	return runtimehost.NewReaderSIMAdapter(missingSIMProvider{})
}

type RuntimeStartRequest struct {
	DeviceID      string
	TraceID       string
	Epoch         uint64
	Prepared      PreparedStart
	Modem         runtimehost.Modem
	Dataplane     runtimehost.DataplanePolicy
	VoiceGateway  *voicehost.Gateway
	DeliveryStore messaging.DeliveryStore
	Dispatch      eventhost.Dispatcher
	BeforeStart   func(context.Context, runtimehost.SessionConfig) error
}

type RuntimeStartResult struct {
	Instance *runtimehost.Instance
	Stale    bool
}

func (m *Manager) SetRuntimeStartForTest(fn runtimeStartFunc) {
	if m == nil {
		return
	}
	m.runtimeStart = fn
}

func (m *Manager) runtimeStarter() runtimeStartFunc {
	if m != nil && m.runtimeStart != nil {
		return m.runtimeStart
	}
	return runtimehost.Start
}

func (m *Manager) StartRuntime(ctx context.Context, req RuntimeStartRequest) (RuntimeStartResult, error) {
	if m == nil {
		return RuntimeStartResult{}, fmt.Errorf("vowifi host manager is nil")
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return RuntimeStartResult{}, fmt.Errorf("vowifi runtime start device_id is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	prepared := req.Prepared.Prepared
	profile := prepared.Profile
	if strings.TrimSpace(profile.IMSI) == "" {
		profile = req.Prepared.Profile
	}
	networkMode := strings.TrimSpace(req.Prepared.StartupState.NetworkMode)
	if networkMode == "" {
		networkMode = strings.TrimSpace(req.Prepared.NetworkMode)
	}

	inst, err := m.runtimeStarter()(ctx, runtimehost.StartRequest{
		Mode:          runtimehost.StartModeMain,
		DeviceID:      deviceID,
		TraceID:       strings.TrimSpace(req.TraceID),
		Profile:       profile,
		Prepared:      &prepared,
		NetworkMode:   networkMode,
		VoiceGateway:  req.VoiceGateway,
		SIM:           buildVoWiFiSIMAdapter(req.Prepared.SIM, req.Modem, prepared.Profile.IMSI),
		Access:        runtimehost.NewModemAccessAdapter(req.Modem),
		Dataplane:     req.Dataplane,
		Proxy:         req.Prepared.Proxy,
		DeliveryStore: req.DeliveryStore,
		Dispatch:      req.Dispatch,
		BeforeStart:   req.BeforeStart,
		ShouldRun: func() bool {
			return ctx.Err() == nil && m.ShouldRun(deviceID, req.Epoch)
		},
		OnTunnelDown: func(downDeviceID string) {
			go func() {
				// Wait for the pipeline goroutine to exit and RuntimeStore
				// to transition out of Active/Starting before attempting
				// recovery. Exponential backoff avoids tight retry loops.
				backoff := 3 * time.Second
				maxBackoff := 30 * time.Second
				for attempt := 0; attempt < 10; attempt++ {
					select {
					case <-time.After(backoff):
					case <-ctx.Done():
						return
					}
					if m.DesiredRecoverable(downDeviceID) {
						m.ScheduleDesiredRecover(context.Background(), DesiredRecoverRequest{
							DeviceID: downDeviceID,
							Reason:   "tunnel_down_auto_recover",
						})
						return
					}
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				}
				logger.Warn("VoWiFi 隧道自动重连放弃：RuntimeStore 状态未变为可恢复",
					"event", "VOWIFI_AUTO_RECOVER_GIVEUP",
					"device", downDeviceID)
			}()
		},
		OnIMSReady: func(vc *voiceclient.Client, imsDeviceID string) {
			agent := &voicehost.IMSOutboundAgent{
				Transport: vc.SIPClient(),
				UA:        vc.SIPUA(),
				Profile: voicehost.IMSProfile{
					IMPI:      vc.PrivateID(),
					IMPU:      vc.PublicURI(),
					Domain:    vc.HomeDomain(),
					LocalIP:   vc.LocalIP().String(),
					UserAgent: "vowifi-core",
				},
				Domain:     vc.HomeDomain(),
				UserAgent:  "vowifi-core",
				LocalTag:   "vowifi-core",
			}
			if m.voiceGateway != nil {
				m.voiceGateway.RegisterAgent(imsDeviceID, agent)
				logger.Info("VoWiFi 语音 Agent 已注册",
					"event", "VOWIFI_VOICE_AGENT_REGISTERED",
					"device", imsDeviceID)
			}
		},
		OnInboundCall: func(ctx context.Context, callReq runtimehost.InboundCallRequest) (runtimehost.InboundCallResponse, error) {
			return m.handleInboundCall(ctx, callReq)
		},
	})
	if err != nil {
		return RuntimeStartResult{}, err
	}

	inst.AddObserver(runtimehost.ObserverFunc(func(_ context.Context, ev runtimehost.Event) {
		if m.IsCurrentInstance(deviceID, inst) {
			m.BroadcastState(deviceID)
			return
		}
		m.RecordStartupState(deviceID, ev.State)
	}))

	if !m.ClaimStarted(deviceID, req.Epoch, inst) {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = inst.Stop(stopCtx)
		cancel()
		m.ClearStartupStateAndBroadcast(deviceID)
		return RuntimeStartResult{Instance: inst, Stale: true}, nil
	}

	return RuntimeStartResult{Instance: inst}, nil
}
