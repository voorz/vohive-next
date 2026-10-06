package e911

import (
	"strings"

	"github.com/voorz/vohive/internal/modem"
)

func SetupAvailable(status modem.DeviceStatus) bool {
	// e911 暂不处理（IMS 注册/SMS 优先），返回 false
	return false
}

func nativePLMN(status modem.DeviceStatus) (string, string) {
	mcc := strings.TrimSpace(status.NativeMCC)
	mnc := strings.TrimSpace(status.NativeMNC)
	if mcc != "" && mnc != "" {
		return mcc, mnc
	}
	imsi := strings.TrimSpace(status.IMSI)
	if len(imsi) >= 6 {
		return imsi[:3], imsi[3:6]
	}
	return "", ""
}
