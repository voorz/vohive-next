package vowifihost

import (
	"context"
	"sync"

	"github.com/voorz/vohive/internal/sipgw"
	"github.com/voorz/vowifi-core/runtimehost"
	"github.com/voorz/vowifi-core/runtimehost/eventhost"
	"github.com/voorz/vowifi-core/runtimehost/messaging"
	"github.com/voorz/vowifi-core/runtimehost/voicehost"
)

// inboundDialogInfo stores the Linphone-side dialog parameters for an
// active inbound VoWiFi call, used to forward BYE/CANCEL from IMS to
// Linphone when the remote party hangs up or cancels.
type inboundDialogInfo struct {
	CallID     string
	DeviceID   string
	RemoteTag  string // Linphone's tag from 200 OK To header
	LocalTag   string // our tag from From header
	ContactURI string // Linphone's Contact from 200 OK
	RouteSet   []string
	CSeq       int
}

type Manager struct {
	runtimeStore RuntimeStore
	stateHub     *StateHub
	recoverStore *DesiredRecoverStore
	lifecycle    *LifecycleController
	runtimeStart  runtimeStartFunc
	adapter       Adapter
	voiceGateway  *voicehost.Gateway
	sipRegistrar  *sipgw.Registrar
	deliveryStore messaging.DeliveryStore
	dispatcher    eventhost.Dispatcher

	inboundDialogsMu sync.Mutex
	inboundDialogs   map[string]*inboundDialogInfo

	inboundRelaysMu sync.Mutex
	inboundRelays   map[string]*voicehost.RTPRelaySession
}

func NewManager() *Manager {
	return NewManagerWithRuntimeStore(NewRuntimeStore())
}

func NewManagerWithRuntimeStore(store RuntimeStore) *Manager {
	if store == nil {
		store = NewRuntimeStore()
	}
	m := &Manager{
		runtimeStore: store,
		stateHub:     NewStateHub(),
		recoverStore: NewDesiredRecoverStore(),
	}
	m.lifecycle = NewLifecycleController(LifecycleControllerOptions{
		IsActive: m.Active,
		Run:      m.runLifecycleCommand,
	})
	return m
}

func (m *Manager) RuntimeStore() RuntimeStore {
	if m == nil || m.runtimeStore == nil {
		return NewRuntimeStore()
	}
	return m.runtimeStore
}

func (m *Manager) SubscribeState(deviceID string) (<-chan struct{}, func()) {
	return m.stateNotifications().Subscribe(deviceID)
}

func (m *Manager) BroadcastState(deviceID string) {
	m.stateNotifications().Broadcast(deviceID)
}

func (m *Manager) SubscriberCount(deviceID string) int {
	return m.stateNotifications().SubscriberCount(deviceID)
}

func (m *Manager) RecordStartupState(deviceID string, state runtimehost.State) bool {
	if !m.RuntimeStore().RecordStartupState(deviceID, state) {
		return false
	}
	m.BroadcastState(deviceID)
	return true
}

func (m *Manager) ClearStartupState(deviceID string) bool {
	return m.RuntimeStore().ClearStartupState(deviceID)
}

func (m *Manager) ConfigureRuntimeDependencies(vg *voicehost.Gateway, ds messaging.DeliveryStore, ed eventhost.Dispatcher) {
	if m == nil {
		return
	}
	m.voiceGateway = vg
	m.deliveryStore = ds
	m.dispatcher = ed
}

// SetSIPRegistrar injects the sipgw.Registrar so the OnInboundCall
// callback can forward incoming VoWiFi calls to Linphone.
func (m *Manager) SetSIPRegistrar(r *sipgw.Registrar) {
	if m == nil {
		return
	}
	m.sipRegistrar = r
}

// storeInboundDialog stores the Linphone-side dialog info for an active
// inbound call, keyed by Call-ID. Used to forward BYE/CANCEL from IMS.
func (m *Manager) storeInboundDialog(callID string, info *inboundDialogInfo) {
	if m == nil || callID == "" || info == nil {
		return
	}
	m.inboundDialogsMu.Lock()
	defer m.inboundDialogsMu.Unlock()
	if m.inboundDialogs == nil {
		m.inboundDialogs = make(map[string]*inboundDialogInfo)
	}
	m.inboundDialogs[callID] = info
}

func (m *Manager) loadInboundDialog(callID string) (*inboundDialogInfo, bool) {
	if m == nil {
		return nil, false
	}
	m.inboundDialogsMu.Lock()
	defer m.inboundDialogsMu.Unlock()
	info, ok := m.inboundDialogs[callID]
	return info, ok
}

func (m *Manager) deleteInboundDialog(callID string) {
	if m == nil {
		return
	}
	m.inboundDialogsMu.Lock()
	defer m.inboundDialogsMu.Unlock()
	delete(m.inboundDialogs, callID)
}

// storeInboundRelay stores the RTP relay for an active inbound call,
// keyed by Call-ID. Used to close the relay when the call ends.
func (m *Manager) storeInboundRelay(callID string, relay *voicehost.RTPRelaySession) {
	if m == nil || callID == "" || relay == nil {
		return
	}
	m.inboundRelaysMu.Lock()
	defer m.inboundRelaysMu.Unlock()
	if m.inboundRelays == nil {
		m.inboundRelays = make(map[string]*voicehost.RTPRelaySession)
	}
	m.inboundRelays[callID] = relay
}

// closeInboundRelay closes and removes the RTP relay for the given
// Call-ID. Safe to call when no relay exists (e.g. call rejected).
func (m *Manager) closeInboundRelay(callID string) {
	if m == nil {
		return
	}
	m.inboundRelaysMu.Lock()
	relay, ok := m.inboundRelays[callID]
	if ok {
		delete(m.inboundRelays, callID)
	}
	m.inboundRelaysMu.Unlock()
	if relay != nil {
		_ = relay.Close()
	}
}

func (m *Manager) ClearStartupStateAndBroadcast(deviceID string) {
	m.ClearStartupState(deviceID)
	m.BroadcastState(deviceID)
}

func (m *Manager) stateNotifications() *StateHub {
	if m == nil || m.stateHub == nil {
		return NewStateHub()
	}
	return m.stateHub
}

func (m *Manager) desiredRecoverStore() *DesiredRecoverStore {
	if m == nil || m.recoverStore == nil {
		return NewDesiredRecoverStore()
	}
	return m.recoverStore
}

func (m *Manager) ConfigureLifecycle(options LifecycleControllerOptions) {
	if m == nil {
		return
	}
	if options.IsActive == nil {
		options.IsActive = m.Active
	}
	if options.Run == nil {
		options.Run = m.runLifecycleCommand
	}
	m.lifecycle = NewLifecycleController(options)
}

func (m *Manager) SubmitLifecycle(ctx context.Context, cmd LifecycleCommand) error {
	return m.lifecycleController().Submit(ctx, cmd)
}

func (m *Manager) NextLifecycleGeneration(deviceID string) uint64 {
	return m.lifecycleController().NextGeneration(deviceID)
}

func (m *Manager) CurrentLifecycleGeneration(deviceID string) uint64 {
	return m.lifecycleController().CurrentGeneration(deviceID)
}

func (m *Manager) SetLifecycleRunForTest(fn func(context.Context, LifecycleCommand) error) {
	m.lifecycleController().SetRunForTest(fn)
}

func (m *Manager) SetLifecycleRecoverRunForTest(fn func(context.Context, string, string, string) error) {
	m.lifecycleController().SetRecoverRunForTest(fn)
}

func (m *Manager) LifecycleControllerForTest() *LifecycleController {
	return m.lifecycleController()
}

func (m *Manager) lifecycleController() *LifecycleController {
	if m == nil || m.lifecycle == nil {
		if m == nil {
			return NewLifecycleController()
		}
		m.ConfigureLifecycle(LifecycleControllerOptions{})
	}
	return m.lifecycle
}
