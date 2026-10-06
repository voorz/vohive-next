package e911

import (
	"context"
	"errors"

	"github.com/voorz/vohive/internal/device"
	"github.com/voorz/vohive/internal/websheet"
)

// e911 暂缓：Coordinator 为空 stub，所有方法返回不支持。

// ErrNotSupported means device status does not support e911 updates.
var ErrNotSupported = errors.New("e911 update not supported by current status")

var ErrProviderUnavailable = errors.New("e911 entitlement provider unavailable or unsupported")
var ErrChallengeIncomplete = errors.New("e911 websheet requires cellular authentication")
var ErrCarrierWebsheetAbsent = errors.New("e911 websheet url not provided by carrier")
var ErrIdentityUnavailable = errors.New("identity information unavailable")

// Coordinator 是 e911 协调器（暂缓，空实现）。
type Coordinator struct {
	deviceID  string
	pool      *device.Pool
	websheets *websheet.Broker
}

// NewCoordinator 创建 Coordinator。
func NewCoordinator(deviceID string, pool *device.Pool, websheets *websheet.Broker) *Coordinator {
	return &Coordinator{deviceID: deviceID, pool: pool, websheets: websheets}
}

// StartWebsheet 启动 e911 websheet（暂缓，直接返回不支持）。
func (c *Coordinator) StartWebsheet(ctx context.Context, deviceID string) (websheet.Info, error) {
	return websheet.Info{}, ErrNotSupported
}

// SetupAvailable 检查 e911 是否可用（暂缓，始终 false）。
func SetupAvailable(status interface{}) bool {
	return false
}
