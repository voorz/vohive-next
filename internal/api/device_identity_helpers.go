package api

import "strings"

// atCommandResidue 是已知的 AT 命令残留值，不应作为厂商名/型号显示。
var atCommandResidue = map[string]struct{}{
	"AT":    {},
	"ATI":   {},
	"AT+GMI": {},
	"AT+GMM": {},
	"AT+GMR": {},
	"OK":    {},
	"ERROR": {},
}

// isValidIdentity 判断字符串是否为有效的厂商名/型号。
// 过滤 AT 命令残留值（AT/ATI/ERROR/OK 等）和过短的值。
func isValidIdentity(v string) bool {
	s := strings.TrimSpace(v)
	if s == "" {
		return false
	}
	upper := strings.ToUpper(s)
	if _, ok := atCommandResidue[upper]; ok {
		return false
	}
	// 过短的值（≤2 字符）几乎不可能是有效厂商名/型号
	if len(s) <= 2 {
		return false
	}
	return true
}

// validOrFallback 返回 preferred 的有效值，否则返回 fallback。
// 用于厂商名/型号的 sysfs 优先 → ATI 回退链。
func validOrFallback(preferred, fallback string) string {
	if isValidIdentity(preferred) {
		return strings.TrimSpace(preferred)
	}
	return strings.TrimSpace(fallback)
}

// matchPCSCUSBPath 容错匹配两个 PC/SC USB 路径。
// 支持完整路径（/sys/bus/usb/devices/1-2）和简短路径（1-2）之间的互相匹配。
// 用于设备发现时配置匹配，兼容旧配置中可能存储的简短路径格式。
func matchPCSCUSBPath(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a) {
		return true
	}
	return false
}
