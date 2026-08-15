package voice

import (
	"sync"
	"time"
)

// CallState 通话状态
type CallState string

const (
	CallStateDialing  CallState = "dialing"
	CallStateRinging  CallState = "ringing"
	CallStateConnected CallState = "connected"
	CallStateEnded    CallState = "ended"
)

// CallDirection 通话方向
type CallDirection string

const (
	CallDirectionIncoming CallDirection = "incoming"
	CallDirectionOutgoing CallDirection = "outgoing"
)

// CallType 通话类型（用于入库）
type CallType string

const (
	CallTypeIncoming CallType = "incoming"
	CallTypeOutgoing CallType = "outgoing"
	CallTypeMissed  CallType = "missed"
)

// CallEvent 通话状态变更事件
type CallEvent struct {
	DeviceID  string      `json:"device_id"`
	CallID    string      `json:"call_id"`
	State     CallState   `json:"state"`
	Direction CallDirection `json:"direction"`
	Number    string      `json:"number"`
	// 通话开始时间（Established 时设置）
	StartedAt *time.Time `json:"started_at,omitempty"`
	// 通话结束时间（Ended 时设置）
	EndedAt *time.Time `json:"ended_at,omitempty"`
	// 通话时长（秒，Ended 时设置）
	Duration int `json:"duration,omitempty"`
	// 通话类型（Ended 时设置，用于入库）
	Type CallType `json:"type,omitempty"`
}

// CallEventHandler 通话事件处理器
type CallEventHandler func(event CallEvent)

// activeCall 活跃通话跟踪
type activeCall struct {
	callID    string
	deviceID  string
	direction CallDirection
	number    string
	startedAt time.Time
	connected bool
	// 是否已接听（用于判断 missed）
	answered bool
}

// Bus 通话事件总线
//
// 从 sipgw 的 onInvite/onBye/onCancel 回调中发布状态变更，
// 通过 SSE 推送给前端，通话结束时自动写入 voice_history。
type Bus struct {
	mu          sync.RWMutex
	activeCalls map[string]*activeCall // callID → activeCall
	handlers    []CallEventHandler
}

// NewBus 创建通话事件总线
func NewBus() *Bus {
	return &Bus{
		activeCalls: make(map[string]*activeCall),
	}
}

// OnEvent 注册事件处理器
func (b *Bus) OnEvent(handler CallEventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
}

// emit 发布事件给所有处理器
func (b *Bus) emit(event CallEvent) {
	b.mu.RLock()
	handlers := make([]CallEventHandler, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()
	for _, h := range handlers {
		h(event)
	}
}

// OnOutboundInvite 呼出 INVITE（Linphone/浏览器发起拨号）
func (b *Bus) OnOutboundInvite(deviceID, callID, number string) {
	now := time.Now()
	call := &activeCall{
		callID:    callID,
		deviceID:  deviceID,
		direction: CallDirectionOutgoing,
		number:    number,
		startedAt: now,
	}
	b.mu.Lock()
	b.activeCalls[callID] = call
	b.mu.Unlock()

	b.emit(CallEvent{
		DeviceID:  deviceID,
		CallID:    callID,
		State:     CallStateDialing,
		Direction: CallDirectionOutgoing,
		Number:    number,
	})
}

// OnInboundInvite 来电 INVITE（IMS/CS 来电转发到 Linphone/浏览器）
func (b *Bus) OnInboundInvite(deviceID, callID, callerNumber string) {
	now := time.Now()
	call := &activeCall{
		callID:    callID,
		deviceID:  deviceID,
		direction: CallDirectionIncoming,
		number:    callerNumber,
		startedAt: now,
	}
	b.mu.Lock()
	b.activeCalls[callID] = call
	b.mu.Unlock()

	b.emit(CallEvent{
		DeviceID:  deviceID,
		CallID:    callID,
		State:     CallStateRinging,
		Direction: CallDirectionIncoming,
		Number:    callerNumber,
	})
}

// OnCallConnected 通话接通（200 OK）
func (b *Bus) OnCallConnected(deviceID, callID string) {
	b.mu.Lock()
	call, ok := b.activeCalls[callID]
	if ok {
		call.connected = true
		call.answered = true
		call.startedAt = time.Now() // 重置开始时间为接通时间
	}
	b.mu.Unlock()

	if !ok {
		return
	}

	b.emit(CallEvent{
		DeviceID:  deviceID,
		CallID:    callID,
		State:     CallStateConnected,
		Direction: call.direction,
		Number:    call.number,
		StartedAt: &call.startedAt,
	})
}

// OnCallEnded 通话结束（BYE/CANCEL/超时）
//
// connected=false 且 direction=incoming 时为 missed
// connected=false 且 direction=outgoing 时不算 missed（取消拨号）
func (b *Bus) OnCallEnded(deviceID, callID string) {
	b.mu.Lock()
	call, ok := b.activeCalls[callID]
	if ok {
		delete(b.activeCalls, callID)
	}
	b.mu.Unlock()

	if !ok {
		return
	}

	now := time.Now()
	duration := 0
	callType := CallTypeOutgoing

	if call.connected {
		// 已接通的通话，计算时长
		duration = int(now.Sub(call.startedAt).Seconds())
		if duration < 0 {
			duration = 0
		}
		if call.direction == CallDirectionIncoming {
			callType = CallTypeIncoming
		} else {
			callType = CallTypeOutgoing
		}
	} else {
		// 未接通
		if call.direction == CallDirectionIncoming {
			callType = CallTypeMissed
		} else {
			// 去电取消，不记录
			b.emit(CallEvent{
				DeviceID:  deviceID,
				CallID:    callID,
				State:     CallStateEnded,
				Direction: call.direction,
				Number:    call.number,
				EndedAt:   &now,
				Type:      CallTypeOutgoing,
				Duration:  0,
			})
			return
		}
	}

	b.emit(CallEvent{
		DeviceID:  deviceID,
		CallID:    callID,
		State:     CallStateEnded,
		Direction: call.direction,
		Number:    call.number,
		EndedAt:   &now,
		Duration:  duration,
		Type:      callType,
	})
}

// GetActiveCall 获取活跃通话信息
func (b *Bus) GetActiveCall(callID string) (direction CallDirection, number string, ok bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	call, exists := b.activeCalls[callID]
	if !exists {
		return "", "", false
	}
	return call.direction, call.number, true
}
