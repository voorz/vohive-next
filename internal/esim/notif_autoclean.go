package esim

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// AutoCleanNotifications 对标 NekoKo _backgroundProcess：批量自动处理卡上通知。
// 按设备通知设置决定每条通知的 shouldSend / shouldRemove / shouldDeleteWithoutSending。
// 用于切卡后延迟 autoClean 链路（用户未打开通知页面时的自动清理）。
func (m *Manager) AutoCleanNotifications() error {
	targetAID := m.getWorkingAID()
	if targetAID == nil {
		aids := m.getEffectiveAIDs()
		if len(aids) == 0 {
			return fmt.Errorf("no effective AID available for autoclean")
		}
		targetAID = aids[0]
	}

	unlock, err := m.lockOperation("autoclean_notifications")
	if err != nil {
		return err
	}
	defer unlock()

	if err := m.waitForAPDUIdleForRead(); err != nil {
		return err
	}

	m.preCleanChannels()
	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		return fmt.Errorf("create LPA client failed: %w", err)
	}
	defer func() {
		_ = m.closeLPAClientForOperation("autoclean_notifications", client)
	}()

	aidHex := strings.ToUpper(hex.EncodeToString(targetAID))
	eid := m.firstEID()

	// 获取卡上全量 PendingNotification
	pendingNotifications, err := safeRetrieveAllNotifications(client)
	if err != nil {
		return fmt.Errorf("retrieve notifications failed: %w", err)
	}

	if len(pendingNotifications) == 0 {
		return nil
	}

	// 执行 autoClean
	cleaned := m.autoCleanPendingNotifications(client, pendingNotifications, aidHex, eid)

	// 同步 DB（移除已清理的通知记录）
	if eid != "" && len(cleaned) > 0 {
		dbStatusMap := m.syncNotificationsWithDB(eid, pendingNotifications)
		_ = dbStatusMap
	}

	// autoClean 完成后刷新 overview 缓存
	m.InvalidateOverviewCache("autoclean_done")
	m.WarmOverviewAsync("autoclean_done")

	return nil
}
