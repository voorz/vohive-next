package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/internal/upstreamproxy"
	"github.com/voorz/vohive/pkg/logger"
	"golang.org/x/net/proxy"
)

// ── 前置代理管理 API（主服务） ──

func normalizeUpstreamProxyPayload(existing *db.UpstreamProxy, req db.UpstreamProxy) db.UpstreamProxy {
	out := req
	out.ID = strings.TrimSpace(out.ID)
	out.Name = strings.TrimSpace(out.Name)
	out.Addr = strings.TrimSpace(out.Addr)
	out.Username = strings.TrimSpace(out.Username)
	out.Password = strings.TrimSpace(out.Password)

	if existing != nil {
		out.CreatedAt = existing.CreatedAt
		// 部分更新：空字段从现有记录继承（支持只传 enabled 的 toggle 操作）
		if out.Name == "" {
			out.Name = existing.Name
		}
		if out.Addr == "" {
			out.Addr = existing.Addr
		}
		if out.Username == "" {
			out.Username = existing.Username
		}
		if out.Password == "" {
			out.Password = existing.Password
		}
	}
	return out
}

func probeUpstreamProxyConfig(c *gin.Context, proxy db.UpstreamProxy) (upstreamproxy.ProbeResult, error) {
	return upstreamproxy.ProbeSOCKS5(c.Request.Context(), upstreamproxy.ProbeConfig{
		ProxyAddr: proxy.Addr,
		Username:  proxy.Username,
		Password:  proxy.Password,
		Timeout:   5 * time.Second,
	})
}

// handleListUpstreamProxies 获取所有前置代理实例
func (s *Server) handleListUpstreamProxies(c *gin.Context) {
	proxies, err := db.ListUpstreamProxies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, proxies)
}

// handleCreateUpstreamProxy 创建前置代理实例
func (s *Server) handleCreateUpstreamProxy(c *gin.Context) {
	var req db.UpstreamProxy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数解析失败: " + err.Error()})
		return
	}
	req = normalizeUpstreamProxyPayload(nil, req)
	logger.Info("🌐 创建前置代理", "id", req.ID, "name", req.Name, "addr", req.Addr, "username", req.Username, "enabled", req.Enabled)
	if req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "id 不能为空"})
		return
	}
	if req.Addr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "addr 不能为空"})
		return
	}
	if err := db.UpsertUpstreamProxy(req); err != nil {
		logger.Error("🌐 前置代理保存失败", "id", req.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	logger.Info("🌐 前置代理已保存", "id", req.ID)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "前置代理已保存",
	})
}

// handleUpdateUpstreamProxy 更新前置代理实例
func (s *Server) handleUpdateUpstreamProxy(c *gin.Context) {
	id := upstreamProxyIDParam(c)
	var req db.UpstreamProxy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数解析失败: " + err.Error()})
		return
	}
	existing, err := db.GetUpstreamProxyByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "前置代理不存在"})
		return
	}
	req.ID = id
	req = normalizeUpstreamProxyPayload(existing, req)
	logger.Info("🌐 更新前置代理", "id", req.ID, "name", req.Name, "addr", req.Addr, "username", req.Username, "enabled", req.Enabled)
	if req.Addr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "addr 不能为空"})
		return
	}
	if err := db.UpsertUpstreamProxy(req); err != nil {
		logger.Error("🌐 前置代理保存失败", "id", req.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	logger.Info("🌐 前置代理已更新", "id", req.ID)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "前置代理已更新",
	})
}

// handleDeleteUpstreamProxy 删除前置代理实例
func (s *Server) handleDeleteUpstreamProxy(c *gin.Context) {
	id := upstreamProxyIDParam(c)
	if err := db.DeleteUpstreamProxy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "前置代理已删除"})
}

// handleProbeUpstreamProxy 探测前置代理是否支持标准 Socks5 + UDP Associate。
func (s *Server) handleProbeUpstreamProxy(c *gin.Context) {
	id := upstreamProxyIDParam(c)
	proxy, err := db.GetUpstreamProxyByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if proxy == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "前置代理不存在"})
		return
	}

	result, probeErr := upstreamproxy.ProbeSOCKS5(c.Request.Context(), upstreamproxy.ProbeConfig{
		ProxyAddr: proxy.Addr,
		Username:  proxy.Username,
		Password:  proxy.Password,
		Timeout:   5 * time.Second,
	})
	if probeErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"status":  "error",
			"message": "前置代理探测失败: " + result.FailureSummary(),
			"result":  result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "前置代理探测成功",
		"result":  result,
	})
}

type upstreamProxyCountryRuleResponse struct {
	CountryCode     string    `json:"country_code"`
	CountryName     string    `json:"country_name"`
	MCCs            []string  `json:"mccs"`
	UpstreamProxyID string    `json:"upstream_proxy_id"`
	Enabled         bool      `json:"enabled"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func buildUpstreamProxyCountryRuleResponse(rule db.UpstreamProxyCountryRule) upstreamProxyCountryRuleResponse {
	display := upstreamproxy.CountryRuleDisplay(rule.CountryCode)
	return upstreamProxyCountryRuleResponse{
		CountryCode:     display.CountryCode,
		CountryName:     display.CountryName,
		MCCs:            display.MCCs,
		UpstreamProxyID: strings.TrimSpace(rule.UpstreamProxyID),
		Enabled:         rule.Enabled,
		UpdatedAt:       rule.UpdatedAt,
	}
}

func (s *Server) handleListUpstreamProxyCountries(c *gin.Context) {
	if !upstreamproxy.CountryTableReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mcc_mnc_table_unavailable"})
		return
	}
	c.JSON(http.StatusOK, upstreamproxy.ListCountryDisplays())
}

func (s *Server) handleListUpstreamProxyCountryRules(c *gin.Context) {
	rules, err := db.ListUpstreamProxyCountryRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	out := make([]upstreamProxyCountryRuleResponse, 0, len(rules))
	for _, rule := range rules {
		out = append(out, buildUpstreamProxyCountryRuleResponse(rule))
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) handleUpsertUpstreamProxyCountryRule(c *gin.Context) {
	if !upstreamproxy.CountryTableReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mcc_mnc_table_unavailable"})
		return
	}
	countryCode := upstreamproxy.NormalizeCountryCode(countryCodeParam(c))
	if _, ok := upstreamproxy.MCCsForCountryCode(countryCode); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "国家代码不在 MCC/MNC 表中"})
		return
	}
	var req struct {
		UpstreamProxyID string `json:"upstream_proxy_id"`
		Enabled         bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数解析失败: " + err.Error()})
		return
	}
	proxy, err := db.GetUpstreamProxyByID(req.UpstreamProxyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if proxy == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "前置代理不存在"})
		return
	}
	rule := db.UpstreamProxyCountryRule{
		CountryCode:     countryCode,
		UpstreamProxyID: proxy.ID,
		Enabled:         req.Enabled,
	}
	if err := db.UpsertUpstreamProxyCountryRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	rule.UpstreamProxyID = proxy.ID
	rule.CountryCode = countryCode
	c.JSON(http.StatusOK, buildUpstreamProxyCountryRuleResponse(rule))
}

func (s *Server) handleDeleteUpstreamProxyCountryRule(c *gin.Context) {
	countryCode := upstreamproxy.NormalizeCountryCode(countryCodeParam(c))
	if err := db.DeleteUpstreamProxyCountryRule(countryCode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// maskSecret 将密码脱敏为 **** 格式
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "****"
}

// handleLookupUpstreamProxy 查询前置代理 IP 归属与延迟
func (s *Server) handleLookupUpstreamProxy(c *gin.Context) {
	id := upstreamProxyIDParam(c)
	proxy, err := db.GetUpstreamProxyByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if proxy == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "前置代理不存在"})
		return
	}

	host, port, err := net.SplitHostPort(proxy.Addr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "代理地址格式无效: " + proxy.Addr})
		return
	}

	// TCP 连通性检查
	conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(host, port), 5*time.Second)
	if dialErr != nil {
		logger.Warn("🌐 前置代理连接失败", "id", id, "addr", proxy.Addr, "err", dialErr)
		if saveErr := db.SaveUpstreamProxyLookup(id, "", "", "", "", "", "", -1, "TCP 连接失败: "+dialErr.Error()); saveErr != nil {
			logger.Error("🌐 保存 lookup 结果失败", "id", id, "err", saveErr)
		}
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"ip":         host,
			"latency_ms": -1,
			"error":      "TCP 连接失败: " + dialErr.Error(),
		})
		return
	}
	conn.Close()

	// 通过 SOCKS5 代理连接 1.1.1.1:80 测量纯代理往返延迟
	latencyMs, latencyErr := measureProxyLatency(proxy.Addr, proxy.Username, proxy.Password)
	if latencyErr != nil {
		logger.Warn("🌐 前置代理延迟测试失败", "id", id, "addr", proxy.Addr, "err", latencyErr)
		if saveErr := db.SaveUpstreamProxyLookup(id, "", "", "", "", "", "", -1, "SOCKS5 延迟测试失败: "+latencyErr.Error()); saveErr != nil {
			logger.Error("🌐 保存 lookup 结果失败", "id", id, "err", saveErr)
		}
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"ip":         host,
			"latency_ms": -1,
			"error":      "SOCKS5 延迟测试失败: " + latencyErr.Error(),
		})
		return
	}

	// 通过 SOCKS5 代理获取出口 IP 与归属
	ipInfo, ipErr := lookupIPInfoViaProxy(c.Request.Context(), proxy.Addr, proxy.Username, proxy.Password)
	if ipErr != nil {
		logger.Warn("🌐 出口 IP 查询失败", "id", id, "proxy_addr", proxy.Addr, "err", ipErr)
		if saveErr := db.SaveUpstreamProxyLookup(id, "", "", "", "", "", "", latencyMs, "出口 IP 查询失败: "+ipErr.Error()); saveErr != nil {
			logger.Error("🌐 保存 lookup 结果失败", "id", id, "err", saveErr)
		}
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"ip":         host,
			"latency_ms": latencyMs,
			"error":      "出口 IP 查询失败: " + ipErr.Error(),
		})
		return
	}

	logger.Info("🌐 前置代理 lookup 完成", "id", id, "addr", proxy.Addr, "exit_ip", ipInfo.IP, "latency_ms", latencyMs, "country", ipInfo.Country)
	if saveErr := db.SaveUpstreamProxyLookup(id, ipInfo.IP, ipInfo.Country, ipInfo.Region, ipInfo.City, ipInfo.ASN, ipInfo.Organization, latencyMs, ""); saveErr != nil {
		logger.Error("🌐 保存 lookup 结果失败", "id", id, "err", saveErr)
	}
	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"ip":           ipInfo.IP,
		"country":      ipInfo.Country,
		"region":       ipInfo.Region,
		"city":         ipInfo.City,
		"asn":          ipInfo.ASN,
		"organization": ipInfo.Organization,
		"latency_ms":   latencyMs,
	})
}

type ipwhoResponse struct {
	IP         string `json:"ip"`
	Country    string `json:"country"`
	Region     string `json:"region"`
	City       string `json:"city"`
	Connection struct {
		ASN int    `json:"asn"`
		Org string `json:"org"`
	} `json:"connection"`
}

type ipInfoResult struct {
	IP           string
	Country      string
	Region       string
	City         string
	ASN          string
	Organization string
}

// measureProxyLatency 通过 SOCKS5 代理连接 1.1.1.1:80 测量纯代理往返延迟
func measureProxyLatency(proxyAddr, username, password string) (int64, error) {
	auth := (*proxy.Auth)(nil)
	if strings.TrimSpace(username) != "" {
		auth = &proxy.Auth{User: strings.TrimSpace(username), Password: strings.TrimSpace(password)}
	}
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, &net.Dialer{Timeout: 10 * time.Second})
	if err != nil {
		return -1, fmt.Errorf("SOCKS5 dialer 创建失败: %w", err)
	}
	start := time.Now()
	conn, err := dialer.Dial("tcp", "1.1.1.1:80")
	if err != nil {
		return -1, fmt.Errorf("SOCKS5 连接失败: %w", err)
	}
	latencyMs := time.Since(start).Milliseconds()
	conn.Close()
	return latencyMs, nil
}

// lookupIPInfoViaProxy 通过 SOCKS5 代理请求 ipwho.is，获取出口 IP 与归属信息
func lookupIPInfoViaProxy(ctx context.Context, proxyAddr, username, password string) (*ipInfoResult, error) {
	var auth *proxy.Auth
	if strings.TrimSpace(username) != "" {
		auth = &proxy.Auth{User: strings.TrimSpace(username), Password: strings.TrimSpace(password)}
	}
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, &net.Dialer{Timeout: 10 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 dialer 创建失败: %w", err)
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport}
	url := "https://ipwho.is/?t=" + fmt.Sprintf("%d", time.Now().UnixMilli())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data ipwhoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	asnStr := ""
	if data.Connection.ASN > 0 {
		asnStr = fmt.Sprintf("AS%d", data.Connection.ASN)
	}
	return &ipInfoResult{
		IP:           data.IP,
		Country:      data.Country,
		Region:       data.Region,
		City:         data.City,
		ASN:          asnStr,
		Organization: data.Connection.Org,
	}, nil
}
