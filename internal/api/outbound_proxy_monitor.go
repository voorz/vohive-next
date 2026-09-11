package api

import (
	"context"
	"strings"
	"time"

	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/pkg/logger"
)

// ── 出站代理定时监控（延迟测试 + IP 查询） ──
//
// 每 3 分钟扫描所有 Auto 来源的 upstream proxy，执行：
//  1. SOCKS5 延迟测试（measureProxyLatency）
//  2. 出口 IP 归属查询（lookupIPInfoViaProxy，多源轮换）
// 结果写入 upstream_proxies 表的 lookup 字段。

const (
	outboundProxyMonitorInterval = 3 * time.Minute
	outboundProxyMonitorTimeout  = 30 * time.Second
)

// StartOutboundProxyMonitor 启动出站代理定时监控 goroutine。
// 应在 Server 初始化时调用（由 RegisterOutboundProxyHandlers 触发）。
func (s *Server) StartOutboundProxyMonitor() {
	go s.outboundProxyMonitorLoop()
	logger.Info("🌐 出站代理定时监控已启动", "interval", outboundProxyMonitorInterval)
}

func (s *Server) outboundProxyMonitorLoop() {
	ticker := time.NewTicker(outboundProxyMonitorInterval)
	defer ticker.Stop()

	// 首次延迟 10 秒后执行一次，避免启动时并发压力
	time.Sleep(10 * time.Second)
	s.runOutboundProxyMonitorOnce()

	for range ticker.C {
		s.runOutboundProxyMonitorOnce()
	}
}

// runOutboundProxyMonitorOnce 执行一轮监控：扫描所有 Auto 来源的 upstream proxy，
// 依次执行延迟测试和 IP 查询，结果写入数据库。
func (s *Server) runOutboundProxyMonitorOnce() {
	proxies, err := db.ListUpstreamProxies()
	if err != nil {
		logger.Warn("🌐 监控扫描前置代理列表失败", "err", err)
		return
	}

	var autoProxies []db.UpstreamProxy
	for _, p := range proxies {
		if strings.EqualFold(p.Source, "Auto") && p.Enabled {
			autoProxies = append(autoProxies, p)
		}
	}

	if len(autoProxies) == 0 {
		return
	}

	logger.Info("🌐 开始出站代理监控轮次", "auto_proxies", len(autoProxies))

	ctx, cancel := context.WithTimeout(context.Background(), outboundProxyMonitorTimeout)
	defer cancel()

	for _, proxy := range autoProxies {
		s.monitorOneProxy(ctx, proxy)
	}

	logger.Info("🌐 出站代理监控轮次完成", "auto_proxies", len(autoProxies))
}

// monitorOneProxy 对单个 Auto 来源的前置代理执行延迟测试 + IP 查询，结果写入数据库。
func (s *Server) monitorOneProxy(ctx context.Context, proxy db.UpstreamProxy) {
	// 1. 延迟测试
	latencyMs, latencyErr := measureProxyLatency(proxy.Addr, proxy.Username, proxy.Password)
	if latencyErr != nil {
		logger.Warn("🌐 监控延迟测试失败",
			"id", proxy.ID, "addr", proxy.Addr, "err", latencyErr)
		if saveErr := db.SaveUpstreamProxyLookup(proxy.ID, "", "", "", "", "", "", "", -1, "监控延迟测试失败: "+latencyErr.Error()); saveErr != nil {
			logger.Error("🌐 监控保存 lookup 结果失败", "id", proxy.ID, "err", saveErr)
		}
		return
	}

	// 2. IP 归属查询
	ipInfo, ipErr := lookupIPInfoViaProxy(ctx, proxy.Addr, proxy.Username, proxy.Password)
	if ipErr != nil {
		logger.Warn("🌐 监控 IP 查询失败",
			"id", proxy.ID, "addr", proxy.Addr, "err", ipErr)
		if saveErr := db.SaveUpstreamProxyLookup(proxy.ID, "", "", "", "", "", "", "", latencyMs, "监控 IP 查询失败: "+ipErr.Error()); saveErr != nil {
			logger.Error("🌐 监控保存 lookup 结果失败", "id", proxy.ID, "err", saveErr)
		}
		return
	}

	// 3. 保存结果
	logger.Info("🌐 监控 lookup 完成",
		"id", proxy.ID, "addr", proxy.Addr,
		"exit_ip", ipInfo.IP, "latency_ms", latencyMs,
		"country", ipInfo.Country, "country_code", ipInfo.CountryCode)
	if saveErr := db.SaveUpstreamProxyLookup(
		proxy.ID, ipInfo.IP, ipInfo.Country, ipInfo.CountryCode,
		ipInfo.Region, ipInfo.City, ipInfo.ASN, ipInfo.Organization,
		latencyMs, "",
	); saveErr != nil {
		logger.Error("🌐 监控保存 lookup 结果失败", "id", proxy.ID, "err", saveErr)
	}
}

// monitorOutboundProxyOnceForTesting 仅用于测试：同步执行一次监控轮次。
func (s *Server) monitorOutboundProxyOnceForTesting() {
	s.runOutboundProxyMonitorOnce()
}
