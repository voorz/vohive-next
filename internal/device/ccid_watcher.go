package device

import (
	"context"
	"fmt"
	"sync"

	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vohive/pkg/notify"
	"github.com/voorz/wwan-go/ccid"
)

// CCIDWatcher 监听 CCID 智能卡读卡器的热插拔事件，桥接到前端气泡通知。
// 使用 wwan-go/ccid.WatchReaders，底层与 modem.WatchDevices 相同的 netlink uevent 机制。
type CCIDWatcher struct {
	stop     chan struct{}
	stopOnce sync.Once
}

// NewCCIDWatcher 创建 CCID 读卡器热插拔监听器。
func NewCCIDWatcher() *CCIDWatcher {
	return &CCIDWatcher{
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
			w.broadcastReaderEvent("reader_added", &event.Reader)

		case ccid.ReaderRemoved:
			logger.Info("检测到读卡器拔出",
				"name", event.Reader.Name,
				"usb_path", event.Reader.USBPath)
			w.broadcastReaderEvent("reader_removed", &event.Reader)
		}
	}

	logger.Info("CCID 读卡器热插拔监听器已停止")
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
