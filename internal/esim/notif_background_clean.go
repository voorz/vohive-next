package esim

import (
	"time"

	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/pkg/logger"
)

// backgroundAutoCleanDelay 是后台自动清理的延迟时间。
// 通知产生后（overview 刷新发现通知数 > 0），延迟 5 分钟执行 autoClean。
const backgroundAutoCleanDelay = 5 * time.Minute

// scheduleBackgroundAutoClean 在发现有待处理通知时启动/重置后台 5 分钟 autoClean 定时器。
// 如果已有定时器在运行，重置计时（对齐 NekoKo：操作产生通知后重新计时）。
// 如果通知数为 0，取消定时器。
func (m *Manager) scheduleBackgroundAutoClean(notificationCount int) {
	if m == nil {
		return
	}

	m.autoCleanTimerMu.Lock()
	defer m.autoCleanTimerMu.Unlock()

	// 通知数为 0，取消定时器
	if notificationCount <= 0 {
		if m.autoCleanTimer != nil {
			m.autoCleanTimer.Stop()
			m.autoCleanTimer = nil
			logger.Debug("后台 autoClean 定时器已取消（无待处理通知）",
				"device", m.deviceID)
		}
		return
	}

	// 已有定时器，重置（Stop + 重新设置）
	if m.autoCleanTimer != nil {
		m.autoCleanTimer.Stop()
	}

	m.autoCleanTimer = time.AfterFunc(backgroundAutoCleanDelay, func() {
		m.autoCleanTimerMu.Lock()
		m.autoCleanTimer = nil
		m.autoCleanTimerMu.Unlock()

		m.runBackgroundAutoClean()
	})

	logger.Info("后台 autoClean 定时器已启动/重置",
		"device", m.deviceID,
		"notification_count", notificationCount,
		"delay", backgroundAutoCleanDelay)
}

// runBackgroundAutoClean 执行后台 autoClean。
// 检查通知设置中的 ProcessAfterSwitch（复用此开关作为"后台自动清理"总开关），
// 然后调用 AutoCleanNotifications。
func (m *Manager) runBackgroundAutoClean() {
	// 检查通知设置：至少需要一个 autoSend 或 deleteWithoutSending 开启
	settings, err := db.GetEsimNotificationSettings(m.deviceID)
	if err != nil {
		logger.Warn("后台 autoClean: 读取通知设置失败",
			"device", m.deviceID,
			"err", err)
		return
	}

	// 检查是否有任何 autoSend 或 deleteWithoutSending 开启
	if !settings.AutoSendInstall && !settings.AutoSendEnable &&
		!settings.AutoSendDisable && !settings.AutoSendDelete &&
		!settings.DeleteWithoutSendingEnable && !settings.DeleteWithoutSendingDisable {
		logger.Debug("后台 autoClean: 所有 autoSend 开关均关闭，跳过",
			"device", m.deviceID)
		return
	}

	logger.Info("后台 autoClean: 开始执行",
		"device", m.deviceID)

	if err := m.AutoCleanNotifications(); err != nil {
		logger.Warn("后台 autoClean 执行失败",
			"device", m.deviceID,
			"err", err)
		return
	}

	logger.Info("后台 autoClean 执行完成",
		"device", m.deviceID)
}

// stopBackgroundAutoClean 停止后台 autoClean 定时器（Manager 销毁时调用）。
func (m *Manager) stopBackgroundAutoClean() {
	m.autoCleanTimerMu.Lock()
	defer m.autoCleanTimerMu.Unlock()

	if m.autoCleanTimer != nil {
		m.autoCleanTimer.Stop()
		m.autoCleanTimer = nil
	}
}
