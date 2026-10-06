// Package carrier provides type definitions and utility functions
// for carrier configuration management.
//
// The package defines request/response types used by the API layer
// and provides Resolve* helpers that convert carrier profiles
// to runtime types.
//
// CarrierProfile 是运营商配置的 JSON 表示（map 别名，保持与 DB/前端的 JSON 兼容）。
// 迁移后不再依赖 vowifi-core 的 CarrierProfile 结构体。
package carrier

import (
	"fmt"
	"strings"
	"time"
)

// CarrierProfile 是运营商配置（JSON map 别名）。
type CarrierProfile = map[string]interface{}

// ListItem 对应前端 CarrierListItem 类型。
type ListItem struct {
	Key              string `json:"key"`
	MCC              string `json:"mcc"`
	MNC              string `json:"mnc"`
	Name             string `json:"name"`
	IKEAddr          string `json:"ike_addr"`
	DeviceIMSTAC     int    `json:"device_ims_tac"`
	DeviceIMSCellID  int    `json:"device_ims_cell_id"`
	HasUserConfig    bool   `json:"has_user_config"`
	Active           bool   `json:"active"`
	HasSystemDefault bool   `json:"has_system_default"`
}

// Detail 对应前端 CarrierDetail 类型。
type Detail struct {
	Key             string          `json:"key"`
	MCC             string          `json:"mcc"`
	MNC             string          `json:"mnc"`
	Name            string          `json:"name"`
	IKEAddr         string          `json:"ike_addr"`
	DeviceIMSTAC    int             `json:"device_ims_tac"`
	DeviceIMSCellID int             `json:"device_ims_cell_id"`
	SystemDefault   *CarrierProfile `json:"system_default"`
	UserConfig      *CarrierProfile `json:"user_config"`
	Active          bool            `json:"active"`
}

// SavePayload 对应前端 CarrierSavePayload 类型。
type SavePayload struct {
	Name            string          `json:"name"`
	IKEAddr         string          `json:"ike_addr"`
	DeviceIMSTAC    int             `json:"device_ims_tac"`
	DeviceIMSCellID int             `json:"device_ims_cell_id"`
	Config          *CarrierProfile `json:"config"`
	Active          bool            `json:"active"`
}

// LoadActiveOverrides 已移除：运营商配置现在纯 DB 查询，不需要启动时加载到内存。

// getNested 从 map 中取嵌套字段（path 如 "device.imei"）。
func getNested(p CarrierProfile, path string) interface{} {
	if p == nil {
		return nil
	}
	var cur interface{} = map[string]interface{}(p)
	for _, k := range strings.Split(path, ".") {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur, ok = m[k]
		if !ok {
			return nil
		}
	}
	return cur
}

func getString(p CarrierProfile, path string) string {
	if v, ok := getNested(p, path).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func getInt(p CarrierProfile, path string) int {
	switch v := getNested(p, path).(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// ResolveRegisterProfile 已移除：RegisterProfile 概念合并到 ims-go 的 SIPConfig + 变体矩阵。
// 调用方改用 ims.SIPConfig（见 vowifihost/runtime_start.go 迁移）。

// ResolveSIPInstanceURN 从 CarrierProfile 解析出 SIP Instance URN。
// GSMA 标准格式化：urn:gsma:imei:<tac>-<snr>-<svn>
func ResolveSIPInstanceURN(p *CarrierProfile) string {
	if p == nil {
		return ""
	}
	imei := getString(*p, "device.imei")
	if imei == "" {
		return ""
	}
	imei = strings.TrimSpace(imei)
	// 简单格式化：取前 15 位数字
	digits := ""
	for _, c := range imei {
		if c >= '0' && c <= '9' {
			digits += string(c)
		}
	}
	if len(digits) < 14 {
		return ""
	}
	return "urn:gsma:imei:" + digits[:8] + "-" + digits[8:14] + "-" + digits[14:]
}

// ResolveRegisterExpiry 从 CarrierProfile 解析出 REGISTER Expires。
func ResolveRegisterExpiry(p *CarrierProfile) time.Duration {
	if p == nil {
		return 0
	}
	exp := getInt(*p, "ims.expires")
	if exp <= 0 {
		return 0
	}
	return time.Duration(exp) * time.Second
}

// ResolvePCSCFAddr 从 CarrierProfile 解析出 P-CSCF 地址。
func ResolvePCSCFAddr(p *CarrierProfile) string {
	if p == nil {
		return ""
	}
	return getString(*p, "ims.pcscf_addr")
}

// PlmnKey 规范化 PLMN key（MCC+MNC，MNC 补零到 3 位）。
func PlmnKey(mcc, mnc string) string {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	if len(mnc) < 3 {
		mnc = strings.Repeat("0", 3-len(mnc)) + mnc
	}
	return mcc + mnc
}

// LookupWithIdentity 按身份查找运营商配置（简化：只用 PLMN key 查 DB）。
func LookupWithIdentity(mcc, mnc, gid1, gid2, spn string) (*CarrierProfile, error) {
	r := &DBProfileResolver{}
	return r.LookupActiveProfile(PlmnKey(mcc, mnc))
}

// Generic 返回通用模板（新设计中无嵌入通用模板，返回 nil）。
func Generic() (*CarrierProfile, error) {
	return nil, nil
}

// IsVoWiFiBlockedMCC 检查 MCC 是否被运营商策略阻止 VoWiFi。
// 简化实现：目前无阻止列表，返回 false。
func IsVoWiFiBlockedMCC(mcc string) bool {
	return false
}

// IsVoWiFiPolicyBlockedError 检查错误是否为运营商策略阻止。
func IsVoWiFiPolicyBlockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "VoWiFiBlocked")
}

// NewVoWiFiBlockedMCCError 创建 MCC 阻止错误。
func NewVoWiFiBlockedMCCError(mcc string) error {
	return fmt.Errorf("VoWiFiBlocked: MCC %s 被运营商策略阻止", mcc)
}

// ResolveEPDGAddr 解析 ePDG 地址（5 参数覆盖之一）。
func ResolveEPDGAddr(p *CarrierProfile) string {
	if p == nil {
		return ""
	}
	return getString(*p, "epdg")
}

// ResolveIPsecEnabled 解析 IPsec 开关（5 参数覆盖之一）。
func ResolveIPsecEnabled(p *CarrierProfile) bool {
	if p == nil {
		return true // 默认启用
	}
	if v, ok := (*p)["ipsec"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return true
}

// ResolveAKAPreference 解析 AKA 偏好（5 参数覆盖之一）。
func ResolveAKAPreference(p *CarrierProfile) string {
	if p == nil {
		return ""
	}
	return getString(*p, "aka_preference")
}
