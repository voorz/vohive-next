package vowifihost

import (
	"strings"

	"github.com/voorz/ims-go/ims"
)

func (m *Manager) Instance(deviceID string) *ims.Client {
	if m == nil {
		return nil
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}
	return m.RuntimeStore().Instance(deviceID)
}

func (m *Manager) Instances() map[string]*ims.Client {
	if m == nil {
		return nil
	}
	return m.RuntimeStore().Instances()
}

func (m *Manager) InstanceIDs() []string {
	if m == nil {
		return nil
	}
	return m.RuntimeStore().InstanceIDs()
}

func (m *Manager) Active(deviceID string) bool {
	if m == nil {
		return false
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false
	}
	return m.RuntimeStore().Active(deviceID)
}

func (m *Manager) Starting(deviceID string) bool {
	if m == nil {
		return false
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false
	}
	return m.RuntimeStore().Starting(deviceID)
}

func (m *Manager) State(deviceID string) (DeviceStartupState, bool) {
	if m == nil {
		return DeviceStartupState{}, false
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DeviceStartupState{}, false
	}
	return m.RuntimeStore().State(deviceID)
}
