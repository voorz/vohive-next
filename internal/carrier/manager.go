// Package carrier provides type definitions and utility functions
// for carrier configuration management.
//
// The package defines request/response types used by the API layer
// and provides Resolve* helpers that convert carrier.CarrierProfile
// to runtime types (voiceclient.RegisterProfile, etc.).
package carrier

import (
	"strings"
	"time"

	corevcarrier "github.com/voorz/vowifi-core/runtimehost/carrier"
	"github.com/voorz/vowifi-core/runtimehost/voiceclient"
)

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
	Key             string                 `json:"key"`
	MCC             string                 `json:"mcc"`
	MNC             string                 `json:"mnc"`
	Name            string                 `json:"name"`
	IKEAddr         string                 `json:"ike_addr"`
	DeviceIMSTAC    int                    `json:"device_ims_tac"`
	DeviceIMSCellID int                    `json:"device_ims_cell_id"`
	SystemDefault   *corevcarrier.CarrierProfile `json:"system_default"`
	UserConfig      *corevcarrier.CarrierProfile `json:"user_config"`
	Active          bool                   `json:"active"`
}

// SavePayload 对应前端 CarrierSavePayload 类型。
type SavePayload struct {
	Name            string                 `json:"name"`
	IKEAddr         string                 `json:"ike_addr"`
	DeviceIMSTAC    int                    `json:"device_ims_tac"`
	DeviceIMSCellID int                    `json:"device_ims_cell_id"`
	Config          *corevcarrier.CarrierProfile `json:"config"`
	Active          bool                   `json:"active"`
}

// LoadActiveOverrides 已移除：运营商配置现在纯 DB 查询，不需要启动时加载到内存。

// ResolveRegisterProfile 从 CarrierProfile 解析出 voiceclient.RegisterProfile。
func ResolveRegisterProfile(p *corevcarrier.CarrierProfile) voiceclient.RegisterProfile {
	if p == nil {
		return voiceclient.RegisterProfile{}
	}
	return voiceclient.CarrierProfileToRegisterProfile(corevcarrier.ProfileToIMSFields(p))
}

// ResolveSIPInstanceURN 从 CarrierProfile 解析出 SIP Instance URN。
// 使用 FormatGSMAIMEIURN 进行 GSMA 标准格式化 (TAC-SNR-SVN)。
func ResolveSIPInstanceURN(p *corevcarrier.CarrierProfile) string {
	if p == nil || p.Device.IMEI == "" {
		return ""
	}
	return voiceclient.FormatGSMAIMEIURN(strings.TrimSpace(p.Device.IMEI))
}

// ResolveRegisterExpiry 从 CarrierProfile 解析出 REGISTER Expires。
func ResolveRegisterExpiry(p *corevcarrier.CarrierProfile) time.Duration {
	if p == nil || p.IMS.Expires <= 0 {
		return 0
	}
	return time.Duration(p.IMS.Expires) * time.Second
}

// ResolvePCSCFAddr 从 CarrierProfile 解析出 P-CSCF 地址。
func ResolvePCSCFAddr(p *corevcarrier.CarrierProfile) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.IMS.PCSCFAddr)
}
