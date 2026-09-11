package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// ── 多源 IP 归属查询（通过 SOCKS5 代理出口请求免费 IP 信息 API） ──
//
// 3 个免费源轮换：
//  1. ipwho.is       — 返回 country_code (ISO2)，10000 次/月
//  2. ip-api.com     — 返回 countryCode (ISO2)，45 次/分（HTTP only for free tier）
//  3. ipapi.co       — 返回 country_code (ISO2)，1000 次/天
//
// 失败自动切换到下一个源。

// ipLookupProvider 定义一个 IP 查询源的接口。
type ipLookupProvider interface {
	Name() string
	URL() string
	Decode(body []byte) (*ipInfoResult, error)
}

// lookupIPInfoViaProxyMulti 通过 SOCKS5 代理依次尝试多个源，返回第一个成功的结果。
func lookupIPInfoViaProxyMulti(ctx context.Context, proxyAddr, username, password string) (*ipInfoResult, error) {
	providers := []ipLookupProvider{
		ipwhoisProvider{},
		ipapiProvider{},
		ipapicoProvider{},
	}

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
	client := &http.Client{Timeout: 12 * time.Second, Transport: transport}

	var lastErr error
	for _, p := range providers {
		result, err := lookupFromProvider(ctx, client, p)
		if err == nil && result != nil && result.IP != "" {
			return result, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", p.Name(), err)
		} else {
			lastErr = fmt.Errorf("%s: empty result", p.Name())
		}
	}
	if lastErr == nil {
		return nil, fmt.Errorf("所有 IP 查询源均失败")
	}
	return nil, fmt.Errorf("所有 IP 查询源均失败: %w", lastErr)
}

func lookupFromProvider(ctx context.Context, client *http.Client, p ipLookupProvider) (*ipInfoResult, error) {
	url := p.URL()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "vohive/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("JSON 解码失败: %w", err)
	}

	// 检查 API 级别的错误标志（ipwho.is 和 ipapi.co 都用 "success": false）
	if successRaw, ok := raw["success"]; ok {
		var success bool
		if err := json.Unmarshal(successRaw, &success); err == nil && !success {
			return nil, fmt.Errorf("API 返回 success=false")
		}
	}

	// 重新读取 body 不现实，所以直接用 raw map 构建结果
	return decodeFromRawMap(raw, p)
}

func decodeFromRawMap(raw map[string]json.RawMessage, p ipLookupProvider) (*ipInfoResult, error) {
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("重新序列化失败: %w", err)
	}
	return p.Decode(body)
}

// ── Provider 1: ipwho.is ──

type ipwhoisProvider struct{}

func (ipwhoisProvider) Name() string { return "ipwho.is" }
func (ipwhoisProvider) URL() string {
	return "https://ipwho.is/?t=" + fmt.Sprintf("%d", time.Now().UnixMilli())
}
func (ipwhoisProvider) Decode(body []byte) (*ipInfoResult, error) {
	var data struct {
		IP          string `json:"ip"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		City        string `json:"city"`
		Connection  struct {
			ASN int    `json:"asn"`
			Org string `json:"org"`
		} `json:"connection"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("ipwho.is 解码失败: %w", err)
	}
	asnStr := ""
	if data.Connection.ASN > 0 {
		asnStr = fmt.Sprintf("AS%d", data.Connection.ASN)
	}
	return &ipInfoResult{
		IP:           data.IP,
		Country:      data.Country,
		CountryCode:  data.CountryCode,
		Region:       data.Region,
		City:         data.City,
		ASN:          asnStr,
		Organization: data.Connection.Org,
	}, nil
}

// ── Provider 2: ip-api.com ──

type ipapiProvider struct{}

func (ipapiProvider) Name() string { return "ip-api.com" }
func (ipapiProvider) URL() string {
	return "http://ip-api.com/json/?fields=status,message,query,country,countryCode,regionName,city,as,org&ts=" + fmt.Sprintf("%d", time.Now().Unix())
}
func (ipapiProvider) Decode(body []byte) (*ipInfoResult, error) {
	var data struct {
		Status      bool   `json:"status"`
		Message     string `json:"message"`
		Query       string `json:"query"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		RegionName  string `json:"regionName"`
		City        string `json:"city"`
		AS          string `json:"as"`
		Org         string `json:"org"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("ip-api.com 解码失败: %w", err)
	}
	if !data.Status {
		return nil, fmt.Errorf("ip-api.com 错误: %s", data.Message)
	}
	return &ipInfoResult{
		IP:           data.Query,
		Country:      data.Country,
		CountryCode:  data.CountryCode,
		Region:       data.RegionName,
		City:         data.City,
		ASN:          data.AS,
		Organization: data.Org,
	}, nil
}

// ── Provider 3: ipapi.co ──

type ipapicoProvider struct{}

func (ipapicoProvider) Name() string { return "ipapi.co" }
func (ipapicoProvider) URL() string {
	return "https://ipapi.co/json/?ts=" + fmt.Sprintf("%d", time.Now().Unix())
}
func (ipapicoProvider) Decode(body []byte) (*ipInfoResult, error) {
	var data struct {
		IP          string `json:"ip"`
		CountryName string `json:"country_name"`
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		City        string `json:"city"`
		ASN         string `json:"asn"`
		Org         string `json:"org"`
		Error       bool   `json:"error"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("ipapi.co 解码失败: %w", err)
	}
	if data.Error {
		return nil, fmt.Errorf("ipapi.co 错误: %s", data.Reason)
	}
	return &ipInfoResult{
		IP:           data.IP,
		Country:      data.CountryName,
		CountryCode:  data.CountryCode,
		Region:       data.Region,
		City:         data.City,
		ASN:          data.ASN,
		Organization: data.Org,
	}, nil
}

// ── 直接用指定 IP 查询归属地（不走代理，多源轮换） ──

// ipDirectProvider 定义一个携带 IP 查询的源接口。
type ipDirectProvider interface {
	Name() string
	URL(ip string) string
	Decode(body []byte) (*ipInfoResult, error)
}

// lookupIPInfoDirect 用指定 IP 地址直接查询归属地信息（不走代理）。
// 依次尝试多个源，第一个成功即返回。
func lookupIPInfoDirect(ctx context.Context, ip string) (*ipInfoResult, error) {
	providers := []ipDirectProvider{
		ipapiDirectProvider{},
		ipwhoisDirectProvider{},
		ipapicoDirectProvider{},
	}

	client := &http.Client{Timeout: 12 * time.Second}

	var lastErr error
	for _, p := range providers {
		result, err := lookupFromDirectProvider(ctx, client, p, ip)
		if err == nil && result != nil && result.IP != "" {
			return result, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", p.Name(), err)
		} else {
			lastErr = fmt.Errorf("%s: empty result", p.Name())
		}
	}
	if lastErr == nil {
		return nil, fmt.Errorf("所有 IP 查询源均失败")
	}
	return nil, fmt.Errorf("所有 IP 查询源均失败: %w", lastErr)
}

func lookupFromDirectProvider(ctx context.Context, client *http.Client, p ipDirectProvider, ip string) (*ipInfoResult, error) {
	url := p.URL(ip)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "vohive/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("JSON 解码失败: %w", err)
	}

	// 检查 API 级别的错误标志
	if successRaw, ok := raw["success"]; ok {
		var success bool
		if err := json.Unmarshal(successRaw, &success); err == nil && !success {
			return nil, fmt.Errorf("API 返回 success=false")
		}
	}

	body, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("重新序列化失败: %w", err)
	}
	return p.Decode(body)
}

// ── Direct Provider 1: ip-api.com/json/{ip} ──

type ipapiDirectProvider struct{}

func (ipapiDirectProvider) Name() string { return "ip-api.com" }
func (ipapiDirectProvider) URL(ip string) string {
	return fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,query,country,countryCode,regionName,city,as,org&ts=%d", ip, time.Now().Unix())
}
func (ipapiDirectProvider) Decode(body []byte) (*ipInfoResult, error) {
	var data struct {
		Status      bool   `json:"status"`
		Message     string `json:"message"`
		Query       string `json:"query"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		RegionName  string `json:"regionName"`
		City        string `json:"city"`
		AS          string `json:"as"`
		Org         string `json:"org"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("ip-api.com 解码失败: %w", err)
	}
	if !data.Status {
		return nil, fmt.Errorf("ip-api.com 错误: %s", data.Message)
	}
	return &ipInfoResult{
		IP:           data.Query,
		Country:      data.Country,
		CountryCode:  data.CountryCode,
		Region:       data.RegionName,
		City:         data.City,
		ASN:          data.AS,
		Organization: data.Org,
	}, nil
}

// ── Direct Provider 2: ipwho.is/{ip} ──

type ipwhoisDirectProvider struct{}

func (ipwhoisDirectProvider) Name() string { return "ipwho.is" }
func (ipwhoisDirectProvider) URL(ip string) string {
	return fmt.Sprintf("https://ipwho.is/%s?t=%d", ip, time.Now().UnixMilli())
}
func (ipwhoisDirectProvider) Decode(body []byte) (*ipInfoResult, error) {
	var data struct {
		IP         string `json:"ip"`
		Country    string `json:"country"`
		CountryCode string `json:"country_code"`
		Region     string `json:"region"`
		City       string `json:"city"`
		Connection struct {
			ASN int    `json:"asn"`
			Org string `json:"org"`
		} `json:"connection"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("ipwho.is 解码失败: %w", err)
	}
	asnStr := ""
	if data.Connection.ASN > 0 {
		asnStr = fmt.Sprintf("AS%d", data.Connection.ASN)
	}
	return &ipInfoResult{
		IP:           data.IP,
		Country:      data.Country,
		CountryCode:  data.CountryCode,
		Region:       data.Region,
		City:         data.City,
		ASN:          asnStr,
		Organization: data.Connection.Org,
	}, nil
}

// ── Direct Provider 3: ipapi.co/{ip}/json/ ──

type ipapicoDirectProvider struct{}

func (ipapicoDirectProvider) Name() string { return "ipapi.co" }
func (ipapicoDirectProvider) URL(ip string) string {
	return fmt.Sprintf("https://ipapi.co/%s/json/?ts=%d", ip, time.Now().Unix())
}
func (ipapicoDirectProvider) Decode(body []byte) (*ipInfoResult, error) {
	var data struct {
		IP          string `json:"ip"`
		CountryName string `json:"country_name"`
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		City        string `json:"city"`
		ASN         string `json:"asn"`
		Org         string `json:"org"`
		Error       bool   `json:"error"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("ipapi.co 解码失败: %w", err)
	}
	if data.Error {
		return nil, fmt.Errorf("ipapi.co 错误: %s", data.Reason)
	}
	return &ipInfoResult{
		IP:           data.IP,
		Country:      data.CountryName,
		CountryCode:  data.CountryCode,
		Region:       data.Region,
		City:         data.City,
		ASN:          data.ASN,
		Organization: data.Org,
	}, nil
}
