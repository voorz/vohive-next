package device

import "sync"

// euiccStateSubscribers 管理按设备 ID 订阅的 eUICC 状态变化通知。
// 设计为可复用：任何需要感知 eUICC 可用性的组件（SSE 流、VoWiFi 恢复逻辑等）
// 都可以通过 SubscribeEUICCState 订阅，eSIM 扫描完成/失败后通过
// broadcastEUICCStateChange 通知所有订阅者。
//
// 实现方式仿照 SubscribeVoWiFiState：返回 <-chan struct{} + func() 取消订阅。
// channel 使用 buffer=1 的非阻塞模式，避免广播时阻塞调用方。

// SubscribeEUICCState 订阅指定设备的 eUICC 状态变化通知。
// 返回的 channel 在 eUICC overview 缓存更新后收到信号。
// 返回的取消订阅函数应在调用方生命周期结束时调用。
func (p *Pool) SubscribeEUICCState(deviceID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	p.euiccStateMu.Lock()
	defer p.euiccStateMu.Unlock()
	if p.euiccStateSubscribers == nil {
		p.euiccStateSubscribers = make(map[string][]chan struct{})
	}
	p.euiccStateSubscribers[deviceID] = append(p.euiccStateSubscribers[deviceID], ch)

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			p.euiccStateMu.Lock()
			defer p.euiccStateMu.Unlock()
			subs := p.euiccStateSubscribers[deviceID]
			for i, sub := range subs {
				if sub == ch {
					p.euiccStateSubscribers[deviceID] = append(subs[:i], subs[i+1:]...)
					break
				}
			}
			if len(p.euiccStateSubscribers[deviceID]) == 0 {
				delete(p.euiccStateSubscribers, deviceID)
			}
		})
	}
	return ch, unsub
}

// broadcastEUICCStateChange 通知指定设备的所有 eUICC 状态订阅者。
// 非阻塞：如果 channel 已有信号则跳过（合并重复通知）。
func (p *Pool) broadcastEUICCStateChange(deviceID string) {
	p.euiccStateMu.RLock()
	subs := p.euiccStateSubscribers[deviceID]
	p.euiccStateMu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default: // channel 已满，跳过（订阅者会处理上一次信号）
		}
	}
}

// onEUICCOverviewUpdated 是注入到 eSIM Manager 的回调，
// 在 overview 缓存更新后被调用，触发 SSE 推送。
func (p *Pool) onEUICCOverviewUpdated(deviceID string, available *bool) {
	p.broadcastEUICCStateChange(deviceID)
}
