package vowifihost

import (
	"time"

	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vowifi-core/runtimehost"
)

// broadcastUIMGateCountdown 在 UIM 门控期间每秒广播状态更新，
// 让前端通过 SSE 实时获取 retry_in_seconds 倒计时。
//
// 退出条件（满足任一即停止）：
//   - 60s cooldown 到期
//   - UIM 恢复可用（euiccAvailable 变为 true）
//   - 设备启动态被清除/取代（不再是 uim_unavailable phase）
func (m *Manager) broadcastUIMGateCountdown(deviceID string) {
	if m == nil {
		return
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	deadline := time.Now().Add(uimGateCooldown)
	for {
		select {
		case <-ticker.C:
			// 检查是否仍然是 uim_unavailable 状态
			st, ok := m.State(deviceID)
			if !ok || st.Phase != "uim_unavailable" {
				return
			}
			// 检查 UIM 是否已恢复
			adapter := m.hostAdapter()
			if adapter != nil {
				if avail := adapter.EUICCAvailable(deviceID); avail == nil || *avail {
					// UIM 已恢复或状态未知，清除门控状态
					m.ClearStartupStateAndBroadcast(deviceID)
					return
				}
			}
			// 检查是否超时
			if time.Now().After(deadline) {
				logger.Debug("UIM 门控倒计时到期", "device", deviceID)
				return
			}
			// 更新 UpdatedAt 并广播，让前端获取最新 retry_in_seconds
			m.RecordStartupState(deviceID, runtimehost.State{
				DeviceID:   deviceID,
				Phase:      "uim_unavailable",
				LastReason: "USIM 逻辑通道状态异常",
				UpdatedAt:  time.Now(),
			})
		}
	}
}
