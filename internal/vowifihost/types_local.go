package vowifihost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/voorz/ims-go/ims"
)

// 本文件定义 vowifihost 本地类型（替代 vowifi-core runtimehost 类型）。
// 迁移策略：先类型解耦（本地定义），再逻辑迁移到 ims.Client。

// Modem 是调制解调器硬件抽象（替代 Modem）。
// vohive-next 的 modem 适配器（qmi/pcsc）实现此接口。
type Modem interface {
	DeviceID() string
	IsHealthy() bool
	IsSimInserted() bool
	QuerySIMInserted() (bool, error)
	GetRegStatus() (int, string)
	ExecuteATSilent(cmd string, timeout time.Duration) (string, error)
	OpenLogicalChannel(aid string) (int, error)
	ResolveLogicalChannelAID(app string, fallbackAID string) (string, string, error)
	CloseLogicalChannel(channel int) error
	TransmitAPDU(channel int, hexAPDU string) (string, error)
}

// SIMAdapter 是 SIM 卡适配器（替代 SIMAdapter）。
type SIMAdapter interface {
	GetIMSI() (string, error)
	CalculateAKA(rand16, autn16 []byte) (ims.AKAResult, error)
	Close() error
}

// ProxyConfig 是代理配置（直接使用 ims.ProxyConfig，A7 已定义）。
type ProxyConfig = ims.ProxyConfig

// SessionConfig 是会话配置（替代 SessionConfig）。
// 迁移到 ims.Config 的过渡类型，字段按需映射。
type SessionConfig struct {
	DeviceID  string
	IMSI      string
	MCC       string
	MNC       string
	PCSCFAddr string
	Proxy     *ProxyConfig
}

// IdentityProfile 是身份配置（替代 vowifi-core IdentityProfile）。
type IdentityProfile struct {
	IMSI string
	IMPI string
	IMPU string
	MCC  string
	MNC  string
	SPN  string
	GID1 string
	GID2 string
}

// PreparedSession 是准备好的会话（替代 vowifi-core PreparedSession）。
type PreparedSession struct {
	Profile IdentityProfile
}

// IdentityProfile 扩展字段（MCC/MNC/SPN/GID，用于运营商匹配）。
type traceIDKey struct{}

// NewTraceID 生成追踪 ID（替代 runtimehost.NewTraceID）。
func NewTraceID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("vowifi-%d", time.Now().UnixNano())
	}
	return "vowifi-" + hex.EncodeToString(b[:])
}

// WithTraceID 将追踪 ID 注入 context（替代 runtimehost.WithTraceID）。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		traceID = NewTraceID()
	}
	return context.WithValue(ctx, traceIDKey{}, traceID)
}
