package vowifihost

import (
	"context"
	"fmt"
	"strings"
	"time"

	swusim "github.com/voorz/vowifi-core/engine/sim"
	"github.com/voorz/vowifi-core/runtimehost"
	"github.com/voorz/vowifi-core/runtimehost/carrier"
	"github.com/voorz/vowifi-core/runtimehost/eventhost"
	"github.com/voorz/vowifi-core/runtimehost/messaging"
	"github.com/voorz/vowifi-core/runtimehost/voiceclient"
	"github.com/voorz/vowifi-core/runtimehost/voicehost"

	carrierconfig "github.com/voorz/vohive/internal/carrier"

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

	// Carrier preset TAC/CellID fallback: when live QMI cell readings
	// are unavailable (flight mode), use the carrier preset's configured
	// TAC/CellID to avoid all-zero utran-cell-id-3gpp (causes 403 Forbidden).
	cellID := ""
	mcc := strings.TrimSpace(profile.MCC)
	mnc := strings.TrimSpace(profile.MNC)
	if mcc != "" {
		mode := carrier.IMSCellIDMode(mcc, mnc, profile.SPN)
		if mode != "none" {
			cellID = carrier.DefaultUTRANCellIDSuffix(mcc, mnc, profile.SPN)
		}
	}

	// 从 JSON carrier profile（含用户覆盖）解析 IMS REGISTER 相关参数。
	// RegisterProfile 必须在此注入，否则 Normalized() 的通用默认值会
	// 覆盖 carrier 特定的 REGISTER header 配置。
	var (
		registerProfile voiceclient.RegisterProfile
		sipInstanceURN  string
		registerExpiry  time.Duration
		pcscfAddr       string
	)
	if mcc != "" && mnc != "" {
		if p, err := carrier.LookupWithIdentity(mcc, mnc, profile.GID1, profile.GID2, profile.SPN); err == nil && p != nil {
			registerProfile = carrierconfig.ResolveRegisterProfile(p)
			sipInstanceURN = carrierconfig.ResolveSIPInstanceURN(p)
			registerExpiry = carrierconfig.ResolveRegisterExpiry(p)
			pcscfAddr = carrierconfig.ResolvePCSCFAddr(p)

			// 打印实际使用的 carrier 模板信息（1 条，带设备 ID）
			source := "系统默认"
			templateLevel := p.TemplateLevel
			if userP, _ := carrier.LookupWithSPN(mcc, mnc, profile.SPN); userP != nil {
				source = "用户自定义"
				templateLevel = userP.TemplateLevel
				if templateLevel == "" {
					templateLevel = "user"
				}
			}
			if templateLevel == "" {
				templateLevel = "default"
			}
			profileName := p.Name
			if profileName == "" {
				profileName = p.ID
			}
			logger.Info(fmt.Sprintf("[%s] 🧩IMS 运营商模板已匹配", deviceID),
				"trace_id", strings.TrimSpace(req.TraceID),
				"plmn", carrier.PlmnKey(mcc, mnc),
				"source", source,
				"template", profileName,
				"template_level", templateLevel,
				"gid1", profile.GID1,
				"gid2", profile.GID2)
		}
	}

	inst, err := m.runtimeStarter()(ctx, runtimehost.StartRequest{
		Mode:            runtimehost.StartModeMain,
		DeviceID:        deviceID,
		TraceID:         strings.TrimSpace(req.TraceID),
		Profile:         profile,
		CellID:          cellID,
		Prepared:        &prepared,
		NetworkMode:     networkMode,
		VoiceGateway:    req.VoiceGateway,
		SIM:             buildVoWiFiSIMAdapter(req.Prepared.SIM, req.Modem, prepared.Profile.IMSI),
		Access:          runtimehost.NewModemAccessAdapter(req.Modem),
		Dataplane:       req.Dataplane,
		Proxy:           req.Prepared.Proxy,
		PCSCFAddr:       pcscfAddr,
		RegisterProfile: registerProfile,
		SIPInstanceURN:  sipInstanceURN,
		RegisterExpiry:  registerExpiry,
		DeliveryStore:   req.DeliveryStore,
		Dispatch:        req.Dispatch,
		BeforeStart:     req.BeforeStart,
		IKERetryCount:   m.ikeRetryCount,
		ShouldRun: func() bool {
			return ctx.Err() == nil && m.ShouldRun(deviceID, req.Epoch)
		},
		OnTunnelDown: func(downDeviceID string) {
			go func() {
				// 使用配置的恢复间隔作为初始退避，覆盖全局设置中的 recover_interval_seconds。
				// 如果未配置（0），回退到默认 3s。
				backoff := m.DesiredRecoverDelay(0)
				if backoff <= 0 {
					backoff = 3 * time.Second
				}
				maxBackoff := 30 * time.Second
				for attempt := 0; attempt < 10; attempt++ {
					// 先获取上一个错误原因（stop 后会丢失）
					lastReason := "VoWiFi 隧道断开，等待自动恢复"
					if st, ok := m.State(downDeviceID); ok && st.LastReason != "" {
						lastReason = st.LastReason
					}
					// Stop and remove the old instance from the RuntimeStore
					// so DesiredRecoverable returns true. This handles both
					// initial tunnel connection failure and unexpected teardown.
					m.StopInstanceForTeardown(context.Background(), downDeviceID, "tunnel_down_auto_recover")
					// Check if VoWiFi has been disabled by the user (card policy).
					// If so, do not trigger auto-recovery.
					if adapter := m.hostAdapter(); adapter != nil && !adapter.IsVoWiFiDesired(downDeviceID) {
						logger.Info("VoWiFi 已被用户禁用，跳过隧道自动恢复",
							"event", "VOWIFI_AUTO_RECOVER_DISABLED",
							"device", downDeviceID)
						return
					}
					// 设置 cooldown + startup state 让前端看到倒计时和失败状态
					m.SetDesiredRecoverCooldown(downDeviceID, backoff)
					m.RecordStartupState(downDeviceID, runtimehost.State{
						DeviceID:   downDeviceID,
						Phase:      "recover_failed",
						LastReason: lastReason,
						UpdatedAt:  time.Now(),
					})
					// 在等待期间每秒广播状态更新，让前端实时获取倒计时
					countdownTicker := time.NewTicker(1 * time.Second)
					countdownDone := make(chan struct{})
					go func() {
						defer countdownTicker.Stop()
						for {
							select {
							case <-countdownTicker.C:
								// 检查是否还是 recover_failed 状态
								st, ok := m.State(downDeviceID)
								if !ok || st.Phase != "recover_failed" {
									return
								}
								m.RecordStartupState(downDeviceID, runtimehost.State{
									DeviceID:   downDeviceID,
									Phase:      "recover_failed",
									LastReason: lastReason,
									UpdatedAt:  time.Now(),
								})
							case <-countdownDone:
								return
							}
						}
					}()
					select {
					case <-time.After(backoff):
					case <-ctx.Done():
						return
					}
					close(countdownDone)
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
				Domain:    vc.HomeDomain(),
				UserAgent: "vowifi-core",
				LocalTag:  "vowifi-core",
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
		OnInboundBye: func(ctx context.Context, deviceID, callID string) error {
			return m.handleInboundBye(ctx, deviceID, callID)
		},
		OnInboundCancel: func(ctx context.Context, deviceID, callID string) error {
			return m.handleInboundCancel(ctx, deviceID, callID)
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
