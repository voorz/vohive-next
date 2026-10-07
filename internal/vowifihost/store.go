package vowifihost

import (
	"strings"
	"sync"
	"time"

	"github.com/voorz/ims-go/ims"
	"github.com/voorz/vohive/pkg/logger"
)

type RuntimeStore interface {
	BeginStart(deviceID string) StartClaim
	ClaimStarted(deviceID string, epoch uint64, inst *ims.Client) bool
	FailStart(deviceID string, epoch uint64, state DeviceStartupState, err error)
	RecordStartupState(deviceID string, state DeviceStartupState) bool
	UpdateReadiness(deviceID string, update func(*DeviceStartupState)) bool
	ClearStartupState(deviceID string) bool
	Invalidate(deviceID string) (uint64, bool)
	CurrentEpoch(deviceID string) uint64
	Active(deviceID string) bool
	Starting(deviceID string) bool
	Instance(deviceID string) *ims.Client
	SetInstance(deviceID string, inst *ims.Client)
	DeleteInstance(deviceID string, inst *ims.Client) bool
	State(deviceID string) (DeviceStartupState, bool)
	Instances() map[string]*ims.Client
	InstanceIDs() []string
}

type Store struct {
	mu    sync.RWMutex
	slots map[string]*runtimeSlot
}

type runtimeSlot struct {
	instance  *ims.Client
	starting  bool
	epoch     uint64
	state     DeviceStartupState
	lastErr   string
	updatedAt time.Time
}

type StartClaim struct {
	Epoch    uint64
	Accepted bool
	Active   bool
	Starting bool
}

func NewRuntimeStore() *Store {
	return &Store{slots: make(map[string]*runtimeSlot)}
}

func (s *Store) ensureSlotLocked(deviceID string) *runtimeSlot {
	if s.slots == nil {
		s.slots = make(map[string]*runtimeSlot)
	}
	slot := s.slots[deviceID]
	if slot == nil {
		slot = &runtimeSlot{}
		s.slots[deviceID] = slot
	}
	return slot
}

func (s *Store) BeginStart(deviceID string) StartClaim {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return StartClaim{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.ensureSlotLocked(deviceID)
	// 存活检查：instance 非 nil 但已停止的是僵尸实例，视为不存在。
	// （2026-10-07：僵尸实例导致 BeginStart 误判 Active，enableRuntime 直接返回 nil
	//  却啥也没启动，前端表现为"点重连后快速走完前置流程但没进 VoWiFi"。）
	if slot.instance != nil {
		if slot.instance.IsRunning() {
			return StartClaim{Epoch: slot.epoch, Active: true}
		}
		// 僵尸实例：清理掉，继续正常启动流程
		slot.instance = nil
		logger.Warn("检测到 VoWiFi 僵尸实例，已清理", "device", deviceID)
	}
	if slot.starting {
		return StartClaim{Epoch: slot.epoch, Starting: true}
	}
	slot.starting = true
	slot.lastErr = ""
	slot.updatedAt = time.Now()
	return StartClaim{Epoch: slot.epoch, Accepted: true}
}

func (s *Store) ClaimStarted(deviceID string, epoch uint64, inst *ims.Client) bool {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" || inst == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.ensureSlotLocked(deviceID)
	if slot.epoch != epoch {
		return false
	}
	if slot.instance != nil {
		return false
	}
	slot.instance = inst
	slot.starting = false
	slot.state = DeviceStartupState{}
	slot.lastErr = ""
	slot.updatedAt = time.Now()
	return true
}

func (s *Store) FailStart(deviceID string, epoch uint64, state DeviceStartupState, err error) {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.ensureSlotLocked(deviceID)
	if slot.epoch != epoch {
		return
	}
	slot.starting = false
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now()
	}
	slot.state = state
	if err != nil {
		slot.lastErr = err.Error()
	}
	slot.updatedAt = time.Now()
}

func (s *Store) RecordStartupState(deviceID string, state DeviceStartupState) bool {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return false
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.ensureSlotLocked(deviceID)
	if slot.instance != nil {
		return false
	}
	if !slot.state.UpdatedAt.IsZero() && state.UpdatedAt.Before(slot.state.UpdatedAt) {
		return false
	}
	slot.state = state
	slot.updatedAt = state.UpdatedAt
	return true
}

// UpdateReadiness 原子更新设备的就绪布尔值（运行时状态上报）。
// 与 RecordStartupState 不同：允许在实例已活跃后更新（用于 ims-go 事件驱动的实时状态同步）。
func (s *Store) UpdateReadiness(deviceID string, update func(*DeviceStartupState)) bool {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" || update == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.slots[deviceID]
	if slot == nil {
		return false
	}
	update(&slot.state)
	slot.state.UpdatedAt = time.Now()
	slot.updatedAt = slot.state.UpdatedAt
	return true
}

func (s *Store) ClearStartupState(deviceID string) bool {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.slots[deviceID]
	if slot == nil || slot.state.UpdatedAt.IsZero() {
		return false
	}
	// 保留 6 个就绪布尔值（运行时状态），只将会话阶段标记为 running。
	// 之前直接清零导致前端 6 步骤全灭。
	preserved := slot.state
	slot.state = DeviceStartupState{
		DeviceID:    deviceID,
		Phase:       "running",
		SIMReady:    preserved.SIMReady,
		AccessReady: preserved.AccessReady,
		TunnelReady: preserved.TunnelReady,
		IMSReady:    preserved.IMSReady,
		SMSReady:    preserved.SMSReady,
		CallReady:   preserved.CallReady,
		UpdatedAt:   time.Now(),
	}
	slot.updatedAt = time.Now()
	if slot.instance == nil && !slot.starting && slot.lastErr == "" {
		delete(s.slots, deviceID)
	}
	return true
}

func (s *Store) Invalidate(deviceID string) (uint64, bool) {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.ensureSlotLocked(deviceID)
	slot.epoch++
	slot.starting = false
	hadState := !slot.state.UpdatedAt.IsZero()
	slot.state = DeviceStartupState{}
	slot.lastErr = ""
	slot.updatedAt = time.Now()
	return slot.epoch, hadState
}

func (s *Store) CurrentEpoch(deviceID string) uint64 {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if slot := s.slots[deviceID]; slot != nil {
		return slot.epoch
	}
	return 0
}

func (s *Store) Active(deviceID string) bool {
	return s.Instance(deviceID) != nil
}

func (s *Store) Starting(deviceID string) bool {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if slot := s.slots[deviceID]; slot != nil {
		return slot.starting
	}
	return false
}

func (s *Store) Instance(deviceID string) *ims.Client {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if slot := s.slots[deviceID]; slot != nil {
		return slot.instance
	}
	return nil
}

func (s *Store) SetInstance(deviceID string, inst *ims.Client) {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.ensureSlotLocked(deviceID)
	slot.instance = inst
	slot.starting = false
	slot.state = DeviceStartupState{}
	slot.lastErr = ""
	slot.updatedAt = time.Now()
}

func (s *Store) DeleteInstance(deviceID string, inst *ims.Client) bool {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.slots[deviceID]
	if slot == nil || slot.instance == nil {
		return false
	}
	if inst != nil && slot.instance != inst {
		return false
	}
	slot.instance = nil
	slot.starting = false
	slot.state = DeviceStartupState{}
	slot.lastErr = ""
	slot.updatedAt = time.Now()
	if slot.epoch == 0 {
		delete(s.slots, deviceID)
	}
	return true
}

func (s *Store) State(deviceID string) (DeviceStartupState, bool) {
	deviceID = strings.TrimSpace(deviceID)
	if s == nil || deviceID == "" {
		return DeviceStartupState{}, false
	}
	s.mu.RLock()
	slot := s.slots[deviceID]
	if slot == nil {
		s.mu.RUnlock()
		return DeviceStartupState{}, false
	}
	state := slot.state
	hasState := !state.UpdatedAt.IsZero() || state.Phase != "" || slot.starting
	s.mu.RUnlock()
	// ims-go 迁移：Client 状态由 vowifihost 的 DeviceStartupState 跟踪，不再从 Instance 获取。
	// 如果有活跃实例但无 tracked state，返回运行中占位。
	if hasState {
		return state, true
	}
	if slot.instance != nil {
		return DeviceStartupState{DeviceID: deviceID, Phase: "running", UpdatedAt: time.Now()}, true
	}
	return DeviceStartupState{}, false
}

func (s *Store) Instances() map[string]*ims.Client {
	out := make(map[string]*ims.Client)
	if s == nil {
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for deviceID, slot := range s.slots {
		if slot != nil && slot.instance != nil {
			out[deviceID] = slot.instance
		}
	}
	return out
}

func (s *Store) InstanceIDs() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.slots))
	for deviceID, slot := range s.slots {
		if slot != nil && slot.instance != nil {
			ids = append(ids, deviceID)
		}
	}
	return ids
}
