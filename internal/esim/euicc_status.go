package esim

// EUICCAvailable 返回 eUICC 可用状态的三态指针：
// - nil: overview 缓存未加载（尚未扫描），不显示"重载"按钮
// - &true: 缓存非空且检测到至少一个 eUICC（ChipInfo.EIDs 非空），不显示"重载"按钮
// - &false: 缓存已加载但扫描失败（ErrNoEUCCFound 等），显示"重载"按钮
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
	// 扫描失败或 ChipInfo 为空
	result := false
	return &result
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
