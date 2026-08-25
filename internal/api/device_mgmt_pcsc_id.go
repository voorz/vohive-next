package api

import (
	"fmt"
	"hash/crc32"

	"github.com/voorz/vohive/internal/device"
)

// pcscSuggestedID 根据 PC/SC 读卡器对应的 USB 路径生成建议设备ID。
// 使用 CRC32(IEEE) 计算完整 sysfs 路径的校验和，格式为 pcsc-{8位十六进制}。
// 例如 /sys/bus/usb/devices/1-1 → pcsc-a9b6e347
func pcscSuggestedID(detail *device.USBIdentity) string {
	if detail == nil || detail.SysPath == "" {
		return ""
	}
	checksum := crc32.ChecksumIEEE([]byte(detail.SysPath))
	return fmt.Sprintf("pcsc-%08x", checksum)
}
