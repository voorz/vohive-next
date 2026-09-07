package device

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vohive/pkg/notify"
	"github.com/voorz/wwan-go/ccid"
)

// CCIDWatcher 监听 CCID 智能卡读卡器的热插拔事件，桥接到 Pool 生命周期管理 + 前端气泡通知。
// 使用 wwan-go/ccid.WatchReaders，底层与 modem.WatchDevices 相同的 netlink uevent 机制。
//
// 与 ModemWatcher 对齐：
//   - ReaderRemoved → 查找匹配的 PC/SC Worker → 停止 VoWiFi + 标记离线 + 推送气泡
//   - ReaderAdded   → 查找匹配的 PC/SC 配置 → 预热恢复 + 推送气泡
type CCIDWatcher struct {
	pool     *Pool
	stop     chan struct{}
	stopOnce sync.Once
}

// NewCCIDWatcher 创建 CCID 读卡器热插拔监听器。
func NewCCIDWatcher(pool *Pool) *CCIDWatcher {
	return &CCIDWatcher{
		pool: pool,
		stop: make(chan struct{}),
	}
}

// Start 启动读卡器热插拔监听。
func (w *CCIDWatcher) Start() {
	go w.loop()
}

// Stop 停止监听。
func (w *CCIDWatcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.stop)
	})
}

func (w *CCIDWatcher) loop() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-w.stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	ch, err := ccid.WatchReaders(ctx)
	if err != nil {
		logger.Warn("CCID 读卡器热插拔监听器启动失败", "err", err)
		return
	}

	logger.Info("CCID 读卡器热插拔监听器已启动")

	for result := range ch {
		if result.Err != nil {
			logger.Warn("CCID 读卡器监听流错误", "err", result.Err)
			return
		}

		event := result.Value
		switch event.Type {
		case ccid.ReaderPresent:
			logger.Debug("CCID 读卡器初始快照",
				"name", event.Reader.Name,
				"usb_path", event.Reader.USBPath)

		case ccid.ReaderAdded:
			logger.Info("检测到读卡器插入",
				"name", event.Reader.Name,
				"usb_path", event.Reader.USBPath,
				"vendor", fmt.Sprintf("%04x", event.Reader.VendorID),
				"product", fmt.Sprintf("%04x", event.Reader.ProductID))
			w.onReaderAdded(&event.Reader)

		case ccid.ReaderRemoved:
			logger.Info("检测到读卡器拔出",
				"name", event.Reader.Name,
				"usb_path", event.Reader.USBPath)
			w.onReaderRemoved(&event.Reader)
		}
	}

	logger.Info("CCID 读卡器热插拔监听器已停止")
}

// onReaderAdded 读卡器插入：查找匹配的已添加 PC/SC 设备，触发恢复。
func (w *CCIDWatcher) onReaderAdded(info *ccid.ReaderInfo) {
	// 推送读卡器插入气泡（对所有读卡器）
	w.broadcastReaderEvent("reader_added", info)

	if w.pool == nil {
		return
	}

	// 查找匹配的已配置 PC/SC 设备
	worker := w.pool.findPCSCWorkerByUSBPath(info.USBPath)
	if worker == nil {
		// 未添加的读卡器，仅气泡通知已在上面完成
		return
	}

	// 已添加设备：触发预热恢复
	logger.Info("读卡器插入，触发设备恢复", "device", worker.ID, "usb_path", info.USBPath)
	w.pool.triggerPCSCDeviceRecovery(worker.ID, "ccid_reader_added")
}

// onReaderRemoved 读卡器拔出：查找匹配的已添加 PC/SC 设备，触发 VoWiFi 停止。
func (w *CCIDWatcher) onReaderRemoved(info *ccid.ReaderInfo) {
	// 推送读卡器拔出气泡（对所有读卡器）
	w.broadcastReaderEvent("reader_removed", info)

	if w.pool == nil {
		return
	}

	// 查找匹配的已配置 PC/SC 设备
	worker := w.pool.findPCSCWorkerByUSBPath(info.USBPath)
	if worker == nil {
		// 未添加的读卡器，仅气泡通知已在上面完成
		return
	}

	// 已添加设备：标记离线 + 停止 VoWiFi
	logger.Warn("读卡器拔出，触发设备离线处理", "device", worker.ID, "usb_path", info.USBPath)
	w.pool.triggerPCSCDeviceOffline(worker.ID, "ccid_reader_removed")
}

func (w *CCIDWatcher) broadcastReaderEvent(action string, info *ccid.ReaderInfo) {
	if notify.GlobalNotificationBroadcaster.ClientCount() == 0 {
		return
	}

	event := "reader_removed"
	title := "读卡器拔出"
	if action == "reader_added" {
		event = "reader_online"
		title = "读卡器插入"
	}

	body := info.Name
	if info.USBPath != "" {
		body = fmt.Sprintf("%s\n路径  %s", body, info.USBPath)
	}

	notify.GlobalNotificationBroadcaster.Broadcast(notify.FrontendNotification{
		Level: "low",
		Event: event,
		Title: title,
		Body:  body,
	})
}

// findPCSCWorkerByUSBPath 在 Pool 中查找匹配指定 USBPath 的 PC/SC Worker。
func (p *Pool) findPCSCWorkerByUSBPath(usbPath string) *Worker {
	if p == nil {
		return nil
	}
	usbPath = strings.TrimSpace(usbPath)
	if usbPath == "" {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, w := range p.workers {
		if w == nil {
			continue
		}
		if config.NormalizeESIMTransport(w.Config.ESIMTransport) != config.ESIMTransportPCSC {
			continue
		}
		if matchUSBPath(w.Config.PCSCUSBPath, usbPath) {
			return w
		}
	}
	return nil
}

// triggerPCSCDeviceOffline 读卡器物理离线：停止 VoWiFi + 移除 Worker + 生命周期标记 + 推送气泡。
// 与模组离线行为一致：移除 Worker，等读卡器重新插入时 CCIDWatcher 触发重建。
func (p *Pool) triggerPCSCDeviceOffline(deviceID, source string) {
	w := p.GetWorker(deviceID)
	if w == nil {
		return
	}

	deviceName := w.Config.Name

	// 停止 VoWiFi 实例
	if p.IsVoWiFiActive(deviceID) {
		logger.Warn("读卡器拔出，自动停止 VoWiFi", "device", deviceID, "source", source)
		if err := p.voWiFiHost().Disable(p.ctx, deviceID, source, false); err != nil {
			logger.Warn("读卡器拔出后停止 VoWiFi 失败", "device", deviceID, "source", source, "err", err)
		}
	}

	// 标记生命周期为"等待设备重新枚举"
	if p.lifecycle != nil {
		p.lifecycle.BeginRecovery(deviceID, LifecyclePhaseUSBWait, source, qmiLifecycleRecoveryTTL)
	}

	// 移除 Worker，与模组离线行为对齐
	_ = p.RemoveWorker(deviceID)

	// 推送设备离线气泡
	notify.GlobalNotificationBroadcaster.Broadcast(notify.FrontendNotification{
		Level:      "low",
		Event:      "device_offline",
		Title:      "设备已断开",
		Body:       deviceID,
		DeviceID:   deviceID,
		DeviceName: deviceName,
	})
}

// triggerPCSCDeviceRecovery 读卡器重新插入：重建 Worker + 推送气泡。
// 与模组恢复行为一致：通过 AddWorkerFromConfig 重建 Worker（含 EsimMgr + 预热）。
func (p *Pool) triggerPCSCDeviceRecovery(deviceID, source string) {
	// 离线时 Worker 已被移除，从配置读取设备信息后重建
	cfg, err := config.GetDeviceByID(deviceID)
	if err != nil || cfg == nil {
		logger.Warn("读卡器恢复：未找到设备配置", "device", deviceID, "err", err)
		return
	}

	deviceName := cfg.Name

	// 推送设备恢复中气泡
	notify.GlobalNotificationBroadcaster.Broadcast(notify.FrontendNotification{
		Level:      "low",
		Event:      "device_online",
		Title:      "设备已连接，恢复中",
		Body:       deviceID,
		DeviceID:   deviceID,
		DeviceName: deviceName,
	})

	// 异步重建 Worker（addPCSCWorker 内部已包含预热逻辑）
	go func() {
		if _, err := p.AddWorkerFromConfig(*cfg); err != nil {
			logger.Warn("读卡器恢复：重建 Worker 失败", "device", deviceID, "source", source, "err", err)
			return
		}
		// VoWiFi 自动恢复由 addPCSCWorker 预热完成后的 enableVoWiFiWhenReady 处理
	}()
}
