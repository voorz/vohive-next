package esim

import "errors"

// EUICCAvailable 返回 eUICC 可用状态的三态指针：
// - nil: overview 缓存未加载（尚未扫描）、设备没有 eUICC（物理 SIM 卡）、
//        或扫描失败但属于临时性故障（操作进行中/传输层不可用），不阻止 VoWiFi
// - &true: 缓存非空且检测到至少一个 eUICC（ChipInfo.EIDs 非空），不阻止 VoWiFi
// - &false: 缓存已加载但扫描失败（eUICC 硬件故障），阻止 VoWiFi
func (m *Manager) EUICCAvailable() *bool {
	if m == nil {
		return nil
	}
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	// overviewCache 未加载（从未调用过 loadOverview）
	if m.overviewCache == nil && m.overviewLastErr == nil {
		return nil
	}
	// overview 缓存已加载
	if m.overviewCache != nil && m.overviewCache.ChipInfo != nil && len(m.overviewCache.ChipInfo.EIDs) > 0 {
		result := true
		return &result
	}
	// 扫描失败：根据错误类别决定是否阻止 VoWiFi
	if m.overviewLastErr != nil {
		// 卡不在位 或 物理SIM卡（所有AID返回6A82/6A88）：不阻止
		if IsScanCardAbsent(m.overviewLastErr) {
			return nil
		}
		// 临时性故障（操作进行中/传输层不可用）：不阻止，让 VoWiFi 尝试启动
		if IsScanTemporary(m.overviewLastErr) {
			return nil
		}
		// 兼容旧错误 ErrNoEUCCFound：不阻止
		if errors.Is(m.overviewLastErr, ErrNoEUCCFound) {
			return nil
		}
		// eUICC 硬件故障：阻止 VoWiFi 启动
		result := false
		return &result
	}
	// overviewCache 为空但无错误（边界情况）：不阻止
	return nil
}

// OverviewStateCallback 在 overview 缓存更新（扫描完成/失败/刷新）时被调用。
// Pool 通过 ManagerOptions.OnOverviewUpdated 注入实现，用于触发 SSE 推送。
type OverviewStateCallback func(deviceID string, available *bool)

// notifyOverviewStateChange 在 setOverviewCache 完成后调用，通知外部 overview 状态已变化。
// 由 Manager 的 onOverviewUpdated 字段驱动（可选）。
func (m *Manager) notifyOverviewStateChange() {
	if m == nil || m.onOverviewUpdated == nil {
		return
	}
	m.onOverviewUpdated(m.deviceID, m.EUICCAvailable())
}
