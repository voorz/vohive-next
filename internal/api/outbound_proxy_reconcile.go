package api

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/internal/proxy/server"
	"github.com/voorz/vohive/pkg/logger"
)

// 出站代理联动逻辑（基座创建/销毁/禁用 + 暴露为前置代理）
//
// 章程锚点：
// - 出站代理 = 代理基座（本地 SOCKS5/HTTP，绑定模组网卡出站）
// - 前置代理 = 派生入口（将出站代理监听地址写入 upstream_proxies 表）
// - 自动创建 = 随机端口 + 固定凭据（vohive/vohive），端口从 10800 开始递增探测
// - 数据断开 = 禁用不销毁（仅设 Enabled=false）
// - ICCID 变化 = 销毁旧实例

const (
	outboundProxyStartPort  = 10800
	outboundProxyUsername   = "vohive"
	outboundProxyPassword   = "vohive"
	outboundProxyListenAddr = "0.0.0.0"
)

// ipCountryCache 缓存设备公网 IP 的归属地 ISO 代码。
// 在出站代理创建/启动时异步查询，供激活代理节点时绑定国家规则使用。
var (
	ipCountryCacheMu sync.RWMutex
	ipCountryCache   = make(map[string]ipCountryEntry)
)

type ipCountryEntry struct {
	iso      string
	ip       string
	fetchedAt time.Time
}

// getCachedCountryCode 返回设备缓存的 IP 归属地 ISO。
func getCachedCountryCode(deviceID string) string {
	ipCountryCacheMu.RLock()
	defer ipCountryCacheMu.RUnlock()
	entry, ok := ipCountryCache[deviceID]
	if !ok {
		return ""
	}
	// 缓存有效期 1 小时
	if time.Since(entry.fetchedAt) > time.Hour {
		return ""
	}
	return entry.iso
}

// setCachedCountryCode 设置设备缓存的 IP 归属地 ISO。
func setCachedCountryCode(deviceID, iso, ip string) {
	ipCountryCacheMu.Lock()
	defer ipCountryCacheMu.Unlock()
	ipCountryCache[deviceID] = ipCountryEntry{iso: iso, ip: ip, fetchedAt: time.Now()}
}

// lookupAndCacheCountryCode 用已有公网 IP 异步查询归属地并缓存。
func (s *Server) lookupAndCacheCountryCode(deviceID string) {
	if s.pool == nil {
		return
	}
	worker := s.pool.GetWorker(deviceID)
	if worker == nil {
		return
	}
	publicIP := worker.GetCachedIP()
	if publicIP == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		info, err := lookupIPInfoDirect(ctx, publicIP)
		if err != nil {
			logger.Warn("🌐 IP 归属地查询失败", "device", deviceID, "ip", publicIP, "err", err)
			return
		}
		setCachedCountryCode(deviceID, info.CountryCode, publicIP)
		logger.Info("🌐 IP 归属地已缓存", "device", deviceID, "ip", publicIP, "iso", info.CountryCode, "country", info.Country)
	}()
}

// EnsureOutboundProxyForDevice 为设备创建出站代理实例（基座）。
// 如果同设备+同 ICCID 的实例已存在则直接返回。
func (s *Server) EnsureOutboundProxyForDevice(deviceID, iccid string) error {
	deviceID = strings.TrimSpace(deviceID)
	iccid = strings.TrimSpace(iccid)
	if deviceID == "" {
		return fmt.Errorf("device_id 不能为空")
	}

	// 检查是否已存在同设备+同 ICCID 的实例
	existing, err := db.ListProxyInstances()
	if err != nil {
		return fmt.Errorf("查询代理实例失败: %w", err)
	}
	for _, inst := range existing {
		if inst.DeviceID == deviceID && inst.ICCID == iccid {
			logger.Info("出站代理实例已存在，跳过创建",
				"device", deviceID, "iccid", iccid, "instance_id", inst.ID)
			return nil
		}
	}

	// 探测可用端口
	port, err := server.FindAvailablePort(outboundProxyStartPort)
	if err != nil {
		return fmt.Errorf("端口探测失败: %w", err)
	}

	// 生成实例 ID：直接使用 deviceID
	instanceID := deviceID

	// 创建代理实例配置
	inst := config.ProxyInstance{
		ID:          instanceID,
		Name:        deviceID,
		DeviceID:    deviceID,
		Enabled:     true,
		Mode:        "socks5",
		ListenAddr:  outboundProxyListenAddr,
		ListenPort:  port,
		AuthEnabled: true,
		Username:    outboundProxyUsername,
		Password:    outboundProxyPassword,
		ICCID:       iccid,
	}

	// 落库
	dbInst, err := db.ProxyInstanceFromConfig(inst)
	if err != nil {
		return fmt.Errorf("代理实例格式化失败: %w", err)
	}
	if err := db.DB.Create(&dbInst).Error; err != nil {
		return fmt.Errorf("代理实例写入数据库失败: %w", err)
	}

	logger.Info("出站代理实例已创建",
		"device", deviceID, "iccid", iccid,
		"instance_id", instanceID, "port", port)

	// 同步到代理管理器
	if err := s.SyncProxyConfigs(); err != nil {
		logger.Warn("出站代理配置同步失败（实例已落库）",
			"device", deviceID, "err", err)
		return err
	}

	// 异步查询 IP 归属地并缓存（用已有公网 IP，不等代理节点创建）
	s.lookupAndCacheCountryCode(deviceID)

	return nil
}

// DestroyOutboundProxyForDevice 销毁设备的出站代理实例。
// 如果 iccid 不为空，仅销毁匹配该 ICCID 的实例；如果 iccid 为空，销毁该设备的所有出站代理实例。
func (s *Server) DestroyOutboundProxyForDevice(deviceID, iccid string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("device_id 不能为空")
	}
	iccid = strings.TrimSpace(iccid)

	instances, err := db.ListProxyInstances()
	if err != nil {
		return fmt.Errorf("查询代理实例失败: %w", err)
	}

	var toDelete []string
	for _, inst := range instances {
		if inst.DeviceID != deviceID {
			continue
		}
		if iccid != "" && inst.ICCID != iccid {
			continue
		}
		toDelete = append(toDelete, inst.ID)
	}

	if len(toDelete) == 0 {
		return nil
	}

	// 级联清理：删除对应的 Auto 来源前置代理
	for _, inst := range instances {
		if inst.DeviceID != deviceID {
			continue
		}
		if iccid != "" && inst.ICCID != iccid {
			continue
		}
		if inst.ICCID != "" {
			if err := db.DeleteAutoUpstreamProxyByIdentity(inst.ICCID); err != nil {
				logger.Warn("级联删除前置代理失败",
					"device", deviceID, "iccid", inst.ICCID, "err", err)
			}
		}
	}

	// 从 DB 删除
	if err := db.DB.Where("id IN ?", toDelete).Delete(&db.ProxyInstance{}).Error; err != nil {
		return fmt.Errorf("删除代理实例失败: %w", err)
	}

	logger.Info("出站代理实例已销毁",
		"device", deviceID, "iccid", iccid, "instances", toDelete)

	// 同步到代理管理器
	if err := s.SyncProxyConfigs(); err != nil {
		logger.Warn("出站代理配置同步失败（实例已删除）",
			"device", deviceID, "err", err)
	}
	return nil
}

// DisableOutboundProxyForDevice 禁用设备的出站代理实例（不删除）。
// 用于数据断开场景：仅设 Enabled=false，保留 DB 记录。
func (s *Server) DisableOutboundProxyForDevice(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("device_id 不能为空")
	}

	instances, err := db.ListProxyInstances()
	if err != nil {
		return fmt.Errorf("查询代理实例失败: %w", err)
	}

	var toDisable []string
	for _, inst := range instances {
		if inst.DeviceID == deviceID && inst.Enabled {
			toDisable = append(toDisable, inst.ID)
		}
	}

	if len(toDisable) == 0 {
		return nil
	}

	// 更新 DB
	if err := db.DB.Model(&db.ProxyInstance{}).
		Where("id IN ?", toDisable).
		Update("enabled", false).Error; err != nil {
		return fmt.Errorf("禁用代理实例失败: %w", err)
	}

	logger.Info("出站代理实例已禁用（数据断开）",
		"device", deviceID, "instances", toDisable)

	// 同步到代理管理器
	if err := s.SyncProxyConfigs(); err != nil {
		logger.Warn("出站代理配置同步失败（实例已禁用）",
			"device", deviceID, "err", err)
	}
	return nil
}

// EnableOutboundProxyForDevice 启用设备的出站代理实例。
// 用于数据连接恢复场景：设 Enabled=true。
func (s *Server) EnableOutboundProxyForDevice(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("device_id 不能为空")
	}

	instances, err := db.ListProxyInstances()
	if err != nil {
		return fmt.Errorf("查询代理实例失败: %w", err)
	}

	var toEnable []string
	for _, inst := range instances {
		if inst.DeviceID == deviceID && !inst.Enabled {
			toEnable = append(toEnable, inst.ID)
		}
	}

	if len(toEnable) == 0 {
		return nil
	}

	if err := db.DB.Model(&db.ProxyInstance{}).
		Where("id IN ?", toEnable).
		Update("enabled", true).Error; err != nil {
		return fmt.Errorf("启用代理实例失败: %w", err)
	}

	logger.Info("出站代理实例已启用（数据连接恢复）",
		"device", deviceID, "instances", toEnable)

	if err := s.SyncProxyConfigs(); err != nil {
		logger.Warn("出站代理配置同步失败（实例已启用）",
			"device", deviceID, "err", err)
	}
	return nil
}

// ExposeOutboundProxyAsUpstream 将出站代理信息写入 upstream_proxies 表（派生入口）。
// 自动从 IP 归属地缓存读取 ISO 绑定国家规则，不需要用户手动操作。
func (s *Server) ExposeOutboundProxyAsUpstream(deviceID, iccid string) error {
	deviceID = strings.TrimSpace(deviceID)
	iccid = strings.TrimSpace(iccid)
	if deviceID == "" {
		return fmt.Errorf("device_id 不能为空")
	}

	// 查找出站代理实例
	instances, err := db.ListProxyInstances()
	if err != nil {
		return fmt.Errorf("查询代理实例失败: %w", err)
	}

	var outboundInst *db.ProxyInstance
	for i := range instances {
		if instances[i].DeviceID == deviceID && instances[i].ICCID == iccid {
			outboundInst = &instances[i]
			break
		}
	}
	if outboundInst == nil {
		return fmt.Errorf("未找到设备 %s 的出站代理实例", deviceID)
	}

	// 构建 upstream proxy ID：直接使用实例 ID（即 deviceID）
	upstreamID := outboundInst.ID
	addr := fmt.Sprintf("%s:%d", outboundInst.ListenAddr, outboundInst.ListenPort)
	if outboundInst.ListenAddr == "0.0.0.0" {
		addr = fmt.Sprintf("127.0.0.1:%d", outboundInst.ListenPort)
	}

	// 写入 upstream_proxies 表
	upstream := db.UpstreamProxy{
		ID:         upstreamID,
		Name:       outboundInst.Name,
		Addr:       addr,
		Username:   outboundInst.Username,
		Password:   outboundInst.Password,
		Enabled:    true,
		Source:     "Auto",
		IdentityID: iccid,
	}
	if err := db.UpsertUpstreamProxy(upstream); err != nil {
		return fmt.Errorf("写入前置代理失败: %w", err)
	}

	logger.Info("出站代理已暴露为前置代理",
		"device", deviceID, "upstream_id", upstreamID, "addr", addr)

	// 从 IP 归属地缓存读取 ISO，自动绑定国家规则
	countryCode := strings.TrimSpace(getCachedCountryCode(deviceID))
	if countryCode != "" {
		rule := db.UpstreamProxyCountryRule{
			CountryCode:     strings.ToUpper(countryCode),
			UpstreamProxyID: upstreamID,
			Enabled:         true,
		}
		if err := db.UpsertUpstreamProxyCountryRule(rule); err != nil {
			logger.Warn("自动绑定国家规则失败",
				"device", deviceID, "country_code", countryCode, "err", err)
		} else {
			logger.Info("已自动绑定国家规则",
				"device", deviceID, "country_code", countryCode, "upstream_id", upstreamID)
		}
	} else {
		logger.Warn("IP 归属地缓存为空，跳过自动绑定国家规则", "device", deviceID)
	}

	// 激活代理节点后立即执行一次延迟测试 + IP 查询
	go s.monitorOneProxy(context.Background(), upstream)

	return nil
}

// UnexposeOutboundProxyAsUpstream 从 upstream_proxies 表删除出站代理的派生入口。
func (s *Server) UnexposeOutboundProxyAsUpstream(deviceID, iccid string) error {
	deviceID = strings.TrimSpace(deviceID)
	iccid = strings.TrimSpace(iccid)
	if deviceID == "" {
		return fmt.Errorf("device_id 不能为空")
	}

	// 按 identity_id 精准删除 Auto 来源的前置代理
	if iccid == "" {
		return nil
	}

	if err := db.DeleteAutoUpstreamProxyByIdentity(iccid); err != nil {
		return fmt.Errorf("删除前置代理失败: %w", err)
	}

	logger.Info("已取消暴露前置代理",
		"device", deviceID, "identity_id", iccid)
	return nil
}

// RegisterOutboundProxyHandlers 注册数据连接/断开回调，自动管理出站代理实例。
// 应在 Server 初始化时调用。
func (s *Server) RegisterOutboundProxyHandlers() {
	if s.pool == nil {
		return
	}

	// 数据连接成功 → 启用出站代理实例
	s.pool.OnDataConnected(func(deviceID string) {
		iccid := s.pool.CurrentICCIDForDevice(deviceID)
		if iccid == "" {
			return
		}

		// 检查 cardpolicy 是否开启了出站代理
		policy, err := db.GetCardPolicy(iccid)
		if err != nil {
			return
		}
		if !policy.OutboundProxyEnabled {
			return
		}

		// 确保实例存在并启用
		if err := s.EnsureOutboundProxyForDevice(deviceID, iccid); err != nil {
			logger.Warn("数据连接后创建出站代理失败",
				"device", deviceID, "err", err)
			return
		}
		if err := s.EnableOutboundProxyForDevice(deviceID); err != nil {
			logger.Warn("数据连接后启用出站代理失败",
				"device", deviceID, "err", err)
		}
	})

	// 数据断开 → 禁用出站代理实例（不销毁）
	s.pool.OnDataDisconnected(func(deviceID string) {
		if err := s.DisableOutboundProxyForDevice(deviceID); err != nil {
			logger.Warn("数据断开后禁用出站代理失败",
				"device", deviceID, "err", err)
		}
	})

	// 切卡后代理清理 → 由 SIM 身份变化触发，异步扫描代理池清理旧身份实例
	s.pool.OnProxyClearFromModem(func(deviceID, oldICCID, newICCID string) {
		if err := s.ClearProxyFromModem(deviceID, oldICCID, newICCID); err != nil {
			logger.Warn("切卡后清理旧代理失败",
				"device", deviceID, "old_iccid", oldICCID, "new_iccid", newICCID, "err", err)
		}
	})

	// 启动定时监控 goroutine（延迟测试 + IP 查询，每 3 分钟一轮）
	s.StartOutboundProxyMonitor()
}

// ClearProxyFromModem 切卡后由 SIM 身份变化触发的代理清理流程。
// 扫描代理池中匹配旧身份的出站代理和前置代理实例，全部清理。
// 三要素匹配：模组（deviceID）+ ICCID（旧） + 创建标识（source=Auto）。
func (s *Server) ClearProxyFromModem(deviceID, oldICCID, newICCID string) error {
	deviceID = strings.TrimSpace(deviceID)
	oldICCID = strings.TrimSpace(oldICCID)
	newICCID = strings.TrimSpace(newICCID)
	if deviceID == "" || oldICCID == "" {
		return nil
	}

	// 1. 清理出站代理实例：device_id 匹配 + iccid == oldICCID
	instances, err := db.ListProxyInstances()
	if err != nil {
		return fmt.Errorf("查询代理实例失败: %w", err)
	}
	var deletedInstances []string
	for _, inst := range instances {
		if inst.DeviceID == deviceID && inst.ICCID == oldICCID && inst.ICCID != newICCID {
			deletedInstances = append(deletedInstances, inst.ID)
		}
	}
	if len(deletedInstances) > 0 {
		if err := db.DB.Where("id IN ?", deletedInstances).Delete(&db.ProxyInstance{}).Error; err != nil {
			logger.Warn("清理旧出站代理实例失败",
				"device", deviceID, "old_iccid", oldICCID, "instances", deletedInstances, "err", err)
		} else {
			logger.Info("已清理旧 ICCID 的出站代理实例",
				"device", deviceID, "old_iccid", oldICCID, "new_iccid", newICCID, "instances", deletedInstances)
		}
	}

	// 2. 清理前置代理实例：source=Auto + identity_id == oldICCID
	if err := db.DeleteAutoUpstreamProxyByIdentity(oldICCID); err != nil {
		logger.Warn("清理旧 ICCID 的前置代理失败",
			"device", deviceID, "old_iccid", oldICCID, "err", err)
	} else {
		logger.Info("已清理旧 ICCID 的前置代理",
			"device", deviceID, "old_iccid", oldICCID)
	}

	// 3. 同步代理管理器
	if err := s.SyncProxyConfigs(); err != nil {
		logger.Warn("清理后同步代理配置失败",
			"device", deviceID, "err", err)
	}
	return nil
}

// GetOutboundProxyStatus 查询设备的出站代理状态。
// 返回实例信息、运行状态、连入数量等。
func (s *Server) GetOutboundProxyStatus(deviceID string) (map[string]any, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, fmt.Errorf("device_id 不能为空")
	}

	instances, err := db.ListProxyInstances()
	if err != nil {
		return nil, fmt.Errorf("查询代理实例失败: %w", err)
	}

	var inst *db.ProxyInstance
	for i := range instances {
		if instances[i].DeviceID == deviceID {
			inst = &instances[i]
			break
		}
	}
	if inst == nil {
		return map[string]any{
			"enabled":   false,
			"op_ready":  false,
			"running":   false,
			"instances": []any{},
		}, nil
	}

	// 获取运行状态
	statuses := s.proxyMgr.ListStatus()

	type instanceStatus struct {
		ID          string `json:"id"`
		Running     bool   `json:"running"`
		ListenPort  int    `json:"listen_port"`
		ActiveConns int64  `json:"active_conns"`
	}

	var statusList []instanceStatus
	opReady := false
	for _, st := range statuses {
		if st.ID == inst.ID {
			statusList = append(statusList, instanceStatus{
				ID:          st.ID,
				Running:     st.Running,
				ListenPort:  st.ListenPort,
				ActiveConns: s.proxyMgr.GetActiveConns(inst.ID),
			})
			if st.Running {
				opReady = true
			}
		}
	}

	// 检查是否已暴露为前置代理（按 source=Auto AND identity_id=ICCID 精准匹配）
	upstreamProxy, _ := db.GetAutoUpstreamProxyByIdentity(inst.ICCID)
	exposedAsUpstream := upstreamProxy != nil

	// 设备公网 IP：从 worker 缓存获取（出站代理启动时即有）
	devicePublicIP := ""
	if s.pool != nil {
		if worker := s.pool.GetWorker(deviceID); worker != nil {
			devicePublicIP = worker.GetCachedIP()
		}
	}

	// 如果已暴露为前置代理，从 upstream_proxies 表的 lookup 字段读取延迟
	// IP 归属地优先从缓存读取（使用流量创建出站代理时已查询），回退到 upstream_proxies 表
	result := map[string]any{
		"enabled":             inst.Enabled,
		"op_ready":            opReady,
		"iccid":               inst.ICCID,
		"exposed_as_upstream": exposedAsUpstream,
		"instances":           statusList,
		"public_ip":           devicePublicIP,
		"ip_country_code":     getCachedCountryCode(deviceID),
	}
	if exposedAsUpstream && upstreamProxy != nil {
		// 如果缓存为空，回退到 upstream_proxies 表的 lookup 结果
		if result["ip_country_code"] == "" && upstreamProxy.LookupCountryCode != "" {
			result["ip_country_code"] = upstreamProxy.LookupCountryCode
		}
		result["latency_ms"] = upstreamProxy.LookupLatencyMs
	}
	return result, nil
}
