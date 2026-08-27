package esim

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/voorz/vohive/pkg/logger"
)

// AutoCleanNotifications 对标 NekoKo _backgroundProcess：批量自动处理卡上通知。
// 按设备通知设置决定每条通知的 shouldSend / shouldRemove / shouldDeleteWithoutSending。
// 用于切卡后延迟 autoClean 链路（用户未打开通知页面时的自动清理）。
//
// AID 选择策略与 listNotificationsForCurrentCard 一致：
// 优先尝试 workingAID 缓存，失败后遍历 notificationCandidateAIDs，
// 命中第一个可用 AID 后停止。
func (m *Manager) AutoCleanNotifications() error {
	unlock, err := m.lockOperation("autoclean_notifications")
	if err != nil {
		return err
	}
	defer unlock()

	if err := m.waitForAPDUIdleForRead(); err != nil {
		return err
	}

	m.preCleanChannels()

	// 构建 AID 候选列表：workingAID 优先 + notificationCandidateAIDs
	candidates := m.notificationCandidateAIDs()
	if cached := m.getWorkingAID(); cached != nil {
		candidates = append([][]byte{cached}, candidates...)
	}

	eid := m.firstEID()

	var lastErr error
	for _, aid := range candidates {
		client, err := m.createLPAWithAID(aid)
		if err != nil {
			lastErr = err
			continue
		}

		aidHex := strings.ToUpper(hex.EncodeToString(aid))

		// 获取卡上全量 PendingNotification
		pendingNotifications, retrieveErr := safeRetrieveAllNotifications(client)
		if retrieveErr != nil {
			lastErr = retrieveErr
			_ = m.closeLPAClientForOperation("autoclean_notifications", client)
			continue
		}

		if len(pendingNotifications) == 0 {
			_ = m.closeLPAClientForOperation("autoclean_notifications", client)
			// 命中成功但无通知，缓存 AID 并退出
			m.setWorkingAID(aid)
			return nil
		}

		// 执行 autoClean
		cleaned := m.autoCleanPendingNotifications(client, pendingNotifications, aidHex, eid)

		// 同步 DB（移除已清理的通知记录）
		if eid != "" && len(cleaned) > 0 {
			dbStatusMap := m.syncNotificationsWithDB(eid, pendingNotifications)
			_ = dbStatusMap
		}

		// 缓存成功命中的 AID（对标 NekoKo setWorkingAid）
		m.setWorkingAID(aid)

		_ = m.closeLPAClientForOperation("autoclean_notifications", client)

		// autoClean 完成后刷新 overview 缓存
		m.InvalidateOverviewCache("autoclean_done")
		m.WarmOverviewAsync("autoclean_done")

		logger.Info("通知 autoClean 完成",
			"device", m.deviceID,
			"AID", aidHex,
			"cleaned_count", len(cleaned))

		return nil
	}

	if lastErr != nil {
		return fmt.Errorf("autoclean: all AID candidates failed: %w", lastErr)
	}
	return nil
}
