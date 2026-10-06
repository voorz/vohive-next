package vowifihost

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DeviceStartupState 是设备启动状态（本地定义，替代 vowifi-core DeviceStartupState）。
// 用于跟踪每个设备的启动阶段，供前端展示和内部协调。
type DeviceStartupState struct {
	DeviceID      string
	Phase         string
	LastReason    string
	LastError     string
	LastErrorClass string
	UpdatedAt     time.Time
	NetworkMode   string
	DataplaneMode string
	// Ready 标志（IMS/SMS 状态展示）
	SIMReady    bool
	AccessReady bool
	TunnelReady bool
	IMSReady    bool
	SMSReady    bool
	CallReady   bool
	// 注册状态
	RegStatus     int
	RegStatusText string
	IMSI          string
	PhoneNumber   string
	// 实时进度
	Generation     uint64
	Stage          string
	StageLabel     string
	StageStartedAt time.Time
	AttemptIndex   int
	MaxAttempts    int
}

type PreparedStart struct {
	Profile      IdentityProfile
	Prepared     PreparedSession
	Modem        Modem
	SIM          SIMAdapter // optional override; when nil, derived from Modem APDU
	Proxy        *ProxyConfig
	NetworkMode  string
	StartupState DeviceStartupState
}

type Adapter interface {
	Context() context.Context
	IsSwitching(deviceID string) bool
	WorkerExists(deviceID string) bool
	IsVoWiFiDesired(deviceID string) bool
	WaitQMICoreReady(deviceID string, timeout time.Duration) error
	WaitWorkerReady(deviceID string, timeout time.Duration) error
	// EUICCAvailable 返回 eUICC 可用状态的三态指针：
	// - nil: overview 缓存未加载（尚未扫描），不阻止启动
	// - &true: eUICC 可用，不阻止启动
	// - &false: eUICC 不可用（UIM 状态不佳），阻止 VoWiFi 启动
	EUICCAvailable(deviceID string) *bool
	PrepareStart(deviceID, traceID, runtimeEPDGOverride string) (PreparedStart, error)
	BeforeStart(deviceID string, modem Modem, proxy *ProxyConfig) func(context.Context, SessionConfig) error
	HandleStartupError(req StartupErrorRequest) error
	MarkRuntimeStarted(req RuntimeStartedRequest)
	RestoreSMSMode(deviceID string)
	RestoreRadioAfterVoWiFi(deviceID string) error
}

type StartupErrorRequest struct {
	TraceID             string
	DeviceID            string
	RuntimeEPDGOverride string
	Generation          uint64
	StartedAt           time.Time
	State               DeviceStartupState
	Err                 error
}

type RuntimeStartedRequest struct {
	TraceID     string
	DeviceID    string
	ActiveCount int
	Elapsed     time.Duration
}

func (m *Manager) ConfigureAdapter(adapter Adapter) {
	if m == nil {
		return
	}
	m.adapter = adapter
}

func (m *Manager) PrepareStart(deviceID, traceID, runtimeEPDGOverride string) (PreparedStart, error) {
	adapter := m.hostAdapter()
	if adapter == nil {
		return PreparedStart{}, fmt.Errorf("vowifi host adapter is not configured")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return PreparedStart{}, fmt.Errorf("vowifi prepare start device_id is empty")
	}
	return adapter.PrepareStart(deviceID, strings.TrimSpace(traceID), strings.TrimSpace(runtimeEPDGOverride))
}

func (m *Manager) BeforeStart(deviceID string, modem Modem, proxy *ProxyConfig) func(context.Context, SessionConfig) error {
	adapter := m.hostAdapter()
	if adapter == nil {
		return nil
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}
	return adapter.BeforeStart(deviceID, modem, proxy)
}

func (m *Manager) hostAdapter() Adapter {
	if m == nil {
		return nil
	}
	return m.adapter
}
