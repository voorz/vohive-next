package device

import (
	"context"
	"sync"

	"fmt"

	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vohive/pkg/notify"
	"github.com/voorz/wwan-go/modem"
	"github.com/voorz/wwan-go/modem/contract"
)

// ModemWatcher 包装 wwan-go/modem.WatchDevices，将内核热插拔事件桥接到 Pool。
//
// 与旧 udev.go 相比，WatchDevices 使用 SOCK_DGRAM + Groups=1 multicast 绑定，
// 在更多内核版本/环境下能正确接收 KOBJECT_UEVENT 事件（R0 Ubuntu 24.04 实测可用）。
// 同时 WatchDevices 提供初始快照（DevicePresent）和智能 diff（Added/Removed/Changed），
// 替代了 udev.go 的 3 秒 debounce + 全量 rescan 方案。
type ModemWatcher struct {
	pool     *Pool
	stop     chan struct{}
	stopOnce sync.Once
}

// NewModemWatcher 创建基于 wwan-go/modem 的设备热插拔监听器。
func NewModemWatcher(pool *Pool) *ModemWatcher {
	return &ModemWatcher{
		pool: pool,
		stop: make(chan struct{}),
	}
}

// Start 启动 WatchDevices 事件循环。
func (w *ModemWatcher) Start() {
	go w.loop()
}

// Stop 停止监听。
func (w *ModemWatcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.stop)
	})
}

func (w *ModemWatcher) loop() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听外部 stop 信号
	go func() {
		select {
		case <-w.stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	ch, err := modem.WatchDevices(ctx)
	if err != nil {
		logger.Warn("modem 设备热插拔监听器启动失败，热插拔功能不可用", "err", err)
		return
	}

	logger.Info("modem 设备热插拔监听器已启动（wwan-go/modem WatchDevices）")

	for result := range ch {
		if result.Err != nil {
			logger.Warn("modem 设备监听流错误", "err", result.Err)
			return
		}

		event := result.Value
		switch event.Type {
		case contract.DevicePresent:
			logger.Debug("modem 设备初始快照",
				"ports", len(event.Device.Ports),
				"physical_path", event.Device.PhysicalPath)
			w.triggerRescan("device_present")

		case contract.DeviceAdded:
			logger.Info("检测到设备插入",
				"physical_path", event.Device.PhysicalPath,
				"usb", event.Device.USB)
			w.broadcastDeviceEvent("device_added", &event.Device)
			w.triggerRescan("device_added")

		case contract.DeviceRemoved:
			logger.Info("检测到设备移除",
				"physical_path", event.Device.PhysicalPath,
				"usb", event.Device.USB)
			w.broadcastDeviceEvent("device_removed", &event.Device)
			w.triggerRescan("device_removed")

		case contract.DeviceChanged:
			logger.Info("检测到设备变化",
				"physical_path", event.Device.PhysicalPath)
			w.triggerRescan("device_changed")
		}
	}

	logger.Info("modem 设备热插拔监听器已停止")
}

// broadcastDeviceEvent 向前端推送设备热插拔气泡通知。
// 使用 pkg/notify.GlobalNotificationBroadcaster，与 logger 系统用法一致。
func (w *ModemWatcher) broadcastDeviceEvent(action string, dev *contract.Device) {
	if notify.GlobalNotificationBroadcaster.ClientCount() == 0 {
		return
	}

	event := "device_removed"
	title := "设备拔出"
	if action == "device_added" {
		event = "device_online"
		title = "发现新设备"
	}

	usbInfo := ""
	if dev.USB.VendorID != 0 || dev.USB.ProductID != 0 {
		usbInfo = fmt.Sprintf("USB %04x:%04x", dev.USB.VendorID, dev.USB.ProductID)
	}

	body := fmt.Sprintf("%s\n路径  %s", usbInfo, dev.PhysicalPath)

	notify.GlobalNotificationBroadcaster.Broadcast(notify.FrontendNotification{
		Level: "low",
		Event: event,
		Title: title,
		Body:  body,
	})
}
// 与旧 udev.go 行为一致：先尝试唤醒模组重启恢复流程，
// 如果没有恢复流程在等待，则执行全量 RescanAndReconnect。
func (w *ModemWatcher) triggerRescan(reason string) {
	if w.pool == nil {
		return
	}

	if woken := w.pool.WakeModemRebootRecoveries("modem_watcher:" + reason); woken > 0 {
		logger.Debug("modem 事件已唤醒模组重启恢复流程", "recoveries", woken, "reason", reason)
		return
	}

	if err := w.pool.RescanAndReconnect(); err != nil {
		logger.Warn("设备重新扫描失败", "err", err, "reason", reason)
	}
}
