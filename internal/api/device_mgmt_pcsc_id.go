package api

import (
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/voorz/vohive/internal/device"
)

// pcscSuggestedID 根据 PC/SC 读卡器信息生成建议设备 ID。
//
// 策略（三段判断）：
//  1. 名称含 "estk" → SN 可信，直接用 SN（格式 pcsc-sn-{SN}）
//  2. 名称不含 "estk" 且 SN 含 "000000000001" → 山寨读卡器，用 USB 路径（格式 pcsc-{crc32}）
//  3. 名称不含 "estk" 但 SN 不含 "000000000001" → SN 可能为合法唯一值，回归用 SN
//
// 正规设备的 SN 唯一可靠，插拔换 USB 接口后 SN 不变，设备无需重新添加。
// 山寨读卡器 SN 重复（固定值），不可用作身份标识，必须用 USB 路径。
func pcscSuggestedID(detail *device.USBIdentity) string {
	if detail == nil {
		return ""
	}
	// SN 为空时只能用 USB 路径
	if detail.Serial == "" {
		return pcscIDFromUSBPath(detail.SysPath)
	}
	product := strings.ToLower(detail.Product)
	isESTK := strings.Contains(product, "estk")
	isFakeSN := strings.Contains(detail.Serial, "000000000001")

	// 1. 名称含 "estk" → SN 可信
	if isESTK {
		return fmt.Sprintf("pcsc-sn-%s", detail.Serial)
	}
	// 2. 名称不含 "estk" 且 SN 含固定值 → 山寨读卡器
	if isFakeSN {
		return pcscIDFromUSBPath(detail.SysPath)
	}
	// 3. 兜底：SN 不含固定值，回归用 SN
	return fmt.Sprintf("pcsc-sn-%s", detail.Serial)
}

// pcscIDFromUSBPath 用 USB 路径生成建议设备 ID（格式 pcsc-{crc32}）。
func pcscIDFromUSBPath(sysPath string) string {
	if sysPath == "" {
		return ""
	}
	checksum := crc32.ChecksumIEEE([]byte(sysPath))
	return fmt.Sprintf("pcsc-%08x", checksum)
}
