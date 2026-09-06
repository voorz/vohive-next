package device

import (
	"context"
	"strings"
	"time"

	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/wwan-go/ccid"
)

// pcscReaderOnline 检查指定 USBPath 的 PC/SC 读卡器是否物理在线。
// 通过 ccid.ListReaderInfo 枚举当前所有读卡器，用 USBPath 匹配。
// 不依赖 QMI/MBIM 协议栈，纯 CCID USBFS 层探测。
func pcscReaderOnline(usbPath string) bool {
	usbPath = strings.TrimSpace(usbPath)
	if usbPath == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	readers, err := ccid.ListReaderInfo(ctx)
	if err != nil {
		logger.Debug("PC/SC 读卡器枚举失败", "usb_path", usbPath, "err", err)
		return false
	}
	for _, r := range readers {
		if matchUSBPath(r.USBPath, usbPath) {
			return true
		}
	}
	return false
}

// matchUSBPath 容错匹配两个 USB 路径。
// 支持完整路径（/sys/bus/usb/devices/1-2）和简短路径（1-2）之间的互相匹配。
// 与 esim 包中的 matchUSBPath 逻辑对齐。
func matchUSBPath(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	// 一方是完整路径时，检查另一方是否是其后缀
	if strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a) {
		return true
	}
	return false
}

// isPCSCDevice 判断 worker 是否为 PC/SC 读卡器设备。
// 与 pcsc_modem_adapter.go 中的 isPCSCDevice 对齐。
func isPCSCHealthDevice(w *Worker) bool {
	if w == nil {
		return false
	}
	return config.NormalizeESIMTransport(w.Config.ESIMTransport) == config.ESIMTransportPCSC
}
