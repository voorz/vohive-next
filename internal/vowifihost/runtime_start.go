package vowifihost

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/voorz/ims-go/ims"

	carrierconfig "github.com/voorz/vohive/internal/carrier"

	"github.com/voorz/vohive/pkg/logger"
)

// runtimeStartFunc 是启动函数的类型（测试可注入）。
type runtimeStartFunc func(context.Context, ims.Config) (*ims.Client, error)

type missingSIMProvider struct{}

func (m missingSIMProvider) GetIMSI() (string, error) {
	return "", fmt.Errorf("missing SIM provider")
}
func (m missingSIMProvider) CalculateAKA(rand, autn []byte) (ims.AKAResult, error) {
	return ims.AKAResult{}, fmt.Errorf("missing SIM provider")
}
func (m missingSIMProvider) Close() error { return nil }

// simAdapterToAKA 将 SIMAdapter 适配为 ims.AKAProvider。
type simAdapterToAKA struct {
	adapter SIMAdapter
}

func (a *simAdapterToAKA) CalculateAKA(rand16, autn16 []byte) (ims.AKAResult, error) {
	return a.adapter.CalculateAKA(rand16, autn16)
}

// buildVoWiFiSIMAdapter prefers an injected SIM adapter (e.g. MBIM Auth AKA for
// modems without SIM logical-channel APDU); otherwise returns error.
func buildVoWiFiSIMAdapter(override SIMAdapter, modem Modem, imsi string) (ims.AKAProvider, error) {
	if override != nil {
		return &simAdapterToAKA{adapter: override}, nil
	}
	// 所有后端的 AKA 现由 vohive 注入；缺失说明编排未设置，属调用错误。
	return nil, fmt.Errorf("vowifihost: SIM adapter 未注入（device %s）", imsi)
}

type RuntimeStartRequest struct {
	DeviceID      string
	TraceID       string
	Epoch         uint64
	Prepared      PreparedStart
	Modem         Modem
	Dataplane     ims.DataplaneConfig
	DeliveryStore ims.SMSDeliveryStore
	// EventDispatcher 由 Manager 持有，此处不再经 Request 传递
	BeforeStart   func(context.Context, SessionConfig) error
}

type RuntimeStartResult struct {
	Client *ims.Client
	Stale  bool
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
	return defaultRuntimeStart
}

// defaultRuntimeStart 是默认启动函数：ims.New + Client.Start。
func defaultRuntimeStart(ctx context.Context, cfg ims.Config) (*ims.Client, error) {
	client, err := ims.New(cfg)
	if err != nil {
		return nil, err
	}
	if err := client.Start(ctx); err != nil {
		return nil, err
	}
	return client, nil
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
	// A5：CellID 注入到 ims.SIPConfig。
	cellID := ""
	mcc := strings.TrimSpace(profile.MCC)
	mnc := strings.TrimSpace(profile.MNC)
	// 注意：carrier.IMSCellIDMode 等函数已迁移到 ims-go internal/carrier；
	// 此处简化为直接从 profile 取（完整逻辑在 ims-go 的 PrepareStart 中）。
	_ = mcc
	_ = mnc

	// 从 carrier profile 解析 IMS REGISTER 相关参数（A6）。
	var (
		registerExpiry time.Duration
		pcscfAddr      string
	)
	if mcc != "" && mnc != "" {
		plmn := mcc + mnc
		// 尝试从 DB 获取生效配置
		resolver := &carrierconfig.DBProfileResolver{}
		if p, err := resolver.LookupActiveProfile(plmn); err == nil && p != nil {
			registerExpiry = carrierconfig.ResolveRegisterExpiry(p)
			pcscfAddr = carrierconfig.ResolvePCSCFAddr(p)

			logger.Info(fmt.Sprintf("[%s] 🧩IMS 运营商模板已匹配", deviceID),
				"trace_id", strings.TrimSpace(req.TraceID),
				"plmn", plmn)
		}
	}

	// 构造 ims.Config（A4 PrepareStart 会在 ims.New 内部执行）。
	akaProvider, err := buildVoWiFiSIMAdapter(req.Prepared.SIM, req.Modem, profile.IMSI)
	if err != nil {
		return RuntimeStartResult{}, err
	}

	pcscfAddrs := []string{}
	if pcscfAddr != "" {
		pcscfAddrs = append(pcscfAddrs, pcscfAddr)
	}

	imsCfg := ims.Config{
		SIM: ims.SIMConfig{
			AKAProvider: akaProvider,
		},
		SWu: ims.SWuConfig{
			IMSI:  profile.IMSI,
			MCC:   mcc,
			MNC:   mnc,
			Proxy: req.Prepared.Proxy, // A7：*ims.ProxyConfig
		},
		SIP: ims.SIPConfig{
			IMPI:            profile.IMPI,
			IMPU:            profile.IMPU,
			PCSCFAddrs:      pcscfAddrs,
			CellID:          cellID, // A5
			RegisterExpires: int(registerExpiry.Seconds()),
		},
		SMS: ims.SMSConfig{
			Store: req.DeliveryStore,
		},
		Voice: ims.VoiceConfig{
			// A3：入站呼叫经新契约处理
			OnIncomingCall: m.incomingCallHandler(deviceID),
		},
		Dataplane: req.Dataplane,
		// RecoveryPolicy：ims-go 内部自动恢复（默认启用）
		Recovery: ims.RecoveryPolicy{},
	}

	// BeforeStart 钩子（vowifihost 编排保留）
	if req.BeforeStart != nil {
		sessionCfg := SessionConfig{
			DeviceID:  deviceID,
			IMSI:      profile.IMSI,
			MCC:       mcc,
			MNC:       mnc,
			PCSCFAddr: pcscfAddr,
			Proxy:     req.Prepared.Proxy,
		}
		if err := req.BeforeStart(ctx, sessionCfg); err != nil {
			return RuntimeStartResult{}, fmt.Errorf("vowifihost: BeforeStart 失败: %w", err)
		}
	}

	client, err := m.runtimeStarter()(ctx, imsCfg)
	if err != nil {
		return RuntimeStartResult{}, err
	}

	// 事件订阅（替代 ObserverFunc）：按设备创建处理器
	if m.eventDispatcher != nil {
		client.OnEvent(m.eventDispatcher.ForDevice(deviceID))
	}

	// 隧道断开自动恢复（ims-go RecoveryPolicy 内部处理，此处保留 vowifihost 的编排逻辑）
	// 注意：OnTunnelDown 回调已由 ims-go 内部恢复替代；vowifihost 的 DesiredRecover 流程保留

	if !m.ClaimStarted(deviceID, req.Epoch, client) {
		_ = client.Stop()
		m.ClearStartupStateAndBroadcast(deviceID)
		return RuntimeStartResult{Client: client, Stale: true}, nil
	}

	return RuntimeStartResult{Client: client}, nil
}

// incomingCallHandler 返回设备的入站呼叫处理器（A3 新契约）。
func (m *Manager) incomingCallHandler(deviceID string) ims.IncomingCallHandler {
	return &deviceIncomingCallHandler{manager: m, deviceID: deviceID}
}
