package notify

import (
	"sync"
	"time"
)

// FrontendNotification 是推送给前端的实时通知结构体，通过 SSE 传输。
type FrontendNotification struct {
	Level      string `json:"level"`               // "high" | "low"
	Event      string `json:"event"`               // "sms_received" | "incoming_call" | "device_online" | "device_offline" | "ip_rotated" | "raw"
	Title      string `json:"title"`               // 通知标题
	Body       string `json:"body"`                // 通知正文（纯文本）
	DeviceID   string `json:"device_id,omitempty"` // 关联设备 ID
	DeviceName string `json:"device_name,omitempty"`
	Timestamp  string `json:"timestamp"` 		// RFC3339
}

// NotificationBroadcaster 通知事件广播器，将 FrontendNotification 推送给所有订阅的 SSE 客户端。
// 模式仿 pkg/logger/log_broadcaster.go 的 Broadcaster。
type NotificationBroadcaster struct {
	clients map[chan FrontendNotification]struct{}
	mu      sync.RWMutex
	maxSize int
}

// GlobalNotificationBroadcaster 全局通知广播器实例
var GlobalNotificationBroadcaster = NewNotificationBroadcaster(64)

// NewNotificationBroadcaster 创建新的通知广播器
func NewNotificationBroadcaster(bufferSize int) *NotificationBroadcaster {
	return &NotificationBroadcaster{
		clients: make(map[chan FrontendNotification]struct{}),
		maxSize: bufferSize,
	}
}

// Subscribe 订阅通知流，返回接收通知的通道
func (b *NotificationBroadcaster) Subscribe() chan FrontendNotification {
	ch := make(chan FrontendNotification, b.maxSize)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (b *NotificationBroadcaster) Unsubscribe(ch chan FrontendNotification) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

// Broadcast 广播通知给所有订阅者（非阻塞，缓冲区满则丢弃）
func (b *NotificationBroadcaster) Broadcast(n FrontendNotification) {
	if n.Timestamp == "" {
		n.Timestamp = time.Now().Format(time.RFC3339)
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.clients {
		select {
		case ch <- n:
		default:
			// 缓冲区满，丢弃（非阻塞）
		}
	}
}

// ClientCount 返回当前订阅客户端数量
func (b *NotificationBroadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
