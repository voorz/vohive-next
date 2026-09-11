package db

import (
	"errors"
	"strings"
	"time"

	"github.com/voorz/vohive/internal/upstreamproxy"
	"gorm.io/gorm"
)

// UpstreamProxy 前置代理实例（用于代理 VoWiFi 的 ePDG 连接）
// 通过 Socks5 UDP Associate 将 IKE/ESP 流量转发到 ePDG
type UpstreamProxy struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Addr      string    `json:"addr"`               // Socks5 服务器地址 (host:port)
	Username  string    `json:"username"`           // 可选鉴权用户名
	Password  string    `json:"password,omitempty"` // 可选鉴权密码
	Enabled   bool      `json:"enabled"`            // 是否启用
	Source    string    `gorm:"default:'Manual'" json:"source"`     // 来源：Manual（用户手动添加）/ Auto（出站代理自动创建）
	IdentityID string   `gorm:"index" json:"identity_id,omitempty"` // 身份标识：Auto 时为 ICCID；Manual 时为空
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 持久化的 lookup 结果（由 /lookup 接口写入）
	LookupIP           string `gorm:"column:lookup_ip" json:"lookup_ip,omitempty"`
	LookupCountry      string `gorm:"column:lookup_country" json:"lookup_country,omitempty"`
	LookupRegion       string `gorm:"column:lookup_region" json:"lookup_region,omitempty"`
	LookupCity         string `gorm:"column:lookup_city" json:"lookup_city,omitempty"`
	LookupASN          string `gorm:"column:lookup_asn" json:"lookup_asn,omitempty"`
	LookupOrganization string `gorm:"column:lookup_organization" json:"lookup_organization,omitempty"`
	LookupLatencyMs    int64  `gorm:"column:lookup_latency_ms" json:"lookup_latency_ms,omitempty"`
	LookupError        string `gorm:"column:lookup_error" json:"lookup_error,omitempty"`
	LookupAt           time.Time `gorm:"column:lookup_at" json:"lookup_at,omitempty"`
}

// UpstreamProxyCountryRule 将 SIM home country 路由到指定前置代理。
type UpstreamProxyCountryRule struct {
	CountryCode     string    `gorm:"primaryKey" json:"country_code"`
	UpstreamProxyID string    `gorm:"index" json:"upstream_proxy_id"`
	Enabled         bool      `json:"enabled"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (UpstreamProxyCountryRule) TableName() string {
	return "upstream_proxy_country_rules"
}

// ── UpstreamProxy CRUD ──

// ListUpstreamProxies 列出所有前置代理实例
func ListUpstreamProxies() ([]UpstreamProxy, error) {
	var out []UpstreamProxy
	if err := DB.Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// GetUpstreamProxyByID 根据 ID 获取前置代理
func GetUpstreamProxyByID(id string) (*UpstreamProxy, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("empty id")
	}
	var out UpstreamProxy
	err := DB.First(&out, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// UpsertUpstreamProxy 创建或更新前置代理
func UpsertUpstreamProxy(p UpstreamProxy) error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("empty id")
	}
	if strings.TrimSpace(p.Addr) == "" {
		return errors.New("empty addr")
	}
	return DB.Save(&p).Error
}

// GetAutoUpstreamProxyByIdentity 查询指定身份（ICCID）的自动创建的前置代理
func GetAutoUpstreamProxyByIdentity(identityID string) (*UpstreamProxy, error) {
	identityID = strings.TrimSpace(identityID)
	if identityID == "" {
		return nil, nil
	}
	var out UpstreamProxy
	err := DB.First(&out, "source = ? AND identity_id = ?", "Auto", identityID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// DeleteAutoUpstreamProxyByIdentity 删除指定身份（ICCID）的自动创建的前置代理
func DeleteAutoUpstreamProxyByIdentity(identityID string) error {
	identityID = strings.TrimSpace(identityID)
	if identityID == "" {
		return nil
	}
	// 先查到 ID，再通过 DeleteUpstreamProxy 清理关联的国家规则
	var proxies []UpstreamProxy
	if err := DB.Where("source = ? AND identity_id = ?", "Auto", identityID).Find(&proxies).Error; err != nil {
		return err
	}
	for _, p := range proxies {
		if err := DeleteUpstreamProxy(p.ID); err != nil {
			return err
		}
	}
	return nil
}

// SaveUpstreamProxyLookup 保存代理 lookup 结果到数据库
func SaveUpstreamProxyLookup(id string, ip, country, region, city, asn, organization string, latencyMs int64, lookupErr string) error {
	updates := map[string]interface{}{
		"lookup_ip":           ip,
		"lookup_country":      country,
		"lookup_region":       region,
		"lookup_city":         city,
		"lookup_asn":          asn,
		"lookup_organization": organization,
		"lookup_latency_ms":   latencyMs,
		"lookup_error":        lookupErr,
		"lookup_at":           time.Now(),
	}
	return DB.Model(&UpstreamProxy{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteUpstreamProxy 删除前置代理（同时清理关联的国家规则）
func DeleteUpstreamProxy(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("empty id")
	}
	if err := DB.Delete(&UpstreamProxyCountryRule{}, "upstream_proxy_id = ?", id).Error; err != nil {
		return err
	}
	return DB.Delete(&UpstreamProxy{}, "id = ?", id).Error
}

// ── UpstreamProxyCountryRule 国家规则管理 ──

func ListUpstreamProxyCountryRules() ([]UpstreamProxyCountryRule, error) {
	var out []UpstreamProxyCountryRule
	if err := DB.Order("country_code asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func UpsertUpstreamProxyCountryRule(rule UpstreamProxyCountryRule) error {
	rule.CountryCode = upstreamproxy.NormalizeCountryCode(rule.CountryCode)
	rule.UpstreamProxyID = strings.TrimSpace(rule.UpstreamProxyID)
	if rule.CountryCode == "" {
		return errors.New("empty country_code")
	}
	if rule.UpstreamProxyID == "" {
		return errors.New("empty upstream_proxy_id")
	}
	rule.UpdatedAt = time.Now()
	return DB.Save(&rule).Error
}

func DeleteUpstreamProxyCountryRule(countryCode string) error {
	countryCode = upstreamproxy.NormalizeCountryCode(countryCode)
	if countryCode == "" {
		return errors.New("empty country_code")
	}
	return DB.Delete(&UpstreamProxyCountryRule{}, "country_code = ?", countryCode).Error
}

func GetCountryUpstreamProxy(countryCode string) (*UpstreamProxy, error) {
	countryCode = upstreamproxy.NormalizeCountryCode(countryCode)
	if countryCode == "" {
		return nil, nil
	}
	var rule UpstreamProxyCountryRule
	err := DB.First(&rule, "country_code = ?", countryCode).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if !rule.Enabled || strings.TrimSpace(rule.UpstreamProxyID) == "" {
		return nil, nil
	}
	proxy, err := GetUpstreamProxyByID(rule.UpstreamProxyID)
	if err != nil || proxy == nil || !proxy.Enabled {
		return nil, err
	}
	return proxy, nil
}

func GetHomeMCCUpstreamProxy(homeMCC string) (*UpstreamProxy, string, error) {
	countryCode, ok := upstreamproxy.CountryCodeFromHomeMCC(homeMCC)
	if !ok {
		return nil, "", nil
	}
	proxy, err := GetCountryUpstreamProxy(countryCode)
	return proxy, countryCode, err
}
