package device

import (
	"strings"

	"github.com/voorz/vohive/internal/vowifihost"
	"github.com/voorz/ims-go/ims"
)

func (p *Pool) voWiFiRuntimeStore() vowifihost.RuntimeStore {
	if p == nil {
		return vowifihost.NewRuntimeStore()
	}
	return p.voWiFiHost().RuntimeStore()
}

func (p *Pool) currentVoWiFiRuntimeEpoch(deviceID string) uint64 {
	if p == nil {
		return 0
	}
	return p.voWiFiHost().CurrentEpoch(deviceID)
}

func (p *Pool) invalidateVoWiFiRuntime(deviceID, reason string) uint64 {
	if p == nil {
		return 0
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0
	}
	return p.voWiFiHost().InvalidateRuntime(deviceID, reason)
}

func (p *Pool) claimStartedVoWiFiApp(deviceID string, runtime any, startupEpoch uint64) bool {
	if p == nil || runtime == nil {
		return false
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false
	}

	switch v := runtime.(type) {
	case *ims.Client:
		return p.voWiFiHost().ClaimStarted(deviceID, startupEpoch, v)
	default:
		return false
	}
}
