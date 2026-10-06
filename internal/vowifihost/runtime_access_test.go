package vowifihost

import (
	"testing"

	"github.com/voorz/ims-go/ims"
)

func TestManagerRuntimeAccessorsReadThroughRuntimeStore(t *testing.T) {
	manager := NewManager()
	deviceID := "dev-access"
	inst := &ims.Client{}
	manager.RuntimeStore().SetInstance(deviceID, inst)

	if manager.Instance(deviceID) != inst {
		t.Fatal("Instance() should return stored runtime instance")
	}
	if !manager.Active(deviceID) {
		t.Fatal("Active() = false, want true")
	}
	if manager.Starting(deviceID) {
		t.Fatal("Starting() = true for active instance, want false")
	}
	if got := manager.Instances(); got[deviceID] != inst {
		t.Fatalf("Instances()[%q] = %v, want stored instance", deviceID, got[deviceID])
	}
	ids := manager.InstanceIDs()
	if len(ids) != 1 || ids[0] != deviceID {
		t.Fatalf("InstanceIDs() = %v, want [%s]", ids, deviceID)
	}
	if _, ok := manager.State(deviceID); !ok {
		t.Fatal("State() should be visible for active instance")
	}
}
