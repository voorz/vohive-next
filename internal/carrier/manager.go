// Package carrier provides type definitions and utility functions
// for carrier configuration management.
//
// The package defines request/response types used by the API layer
// and provides Resolve* helpers that convert profiles.CarrierProfile
// to runtime types (voiceclient.RegisterProfile, etc.).
package carrier

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vowifi-core/profiles"
	"github.com/voorz/vowifi-core/runtimehost/voiceclient"

	"github.com/voorz/vohive/pkg/logger"
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
	SystemDefault   *profiles.CarrierProfile `json:"system_default"`
	UserConfig      *profiles.CarrierProfile `json:"user_config"`
	Active          bool                   `json:"active"`
}

// SavePayload 对应前端 CarrierSavePayload 类型。
type SavePayload struct {
	Name            string                 `json:"name"`
	IKEAddr         string                 `json:"ike_addr"`
	DeviceIMSTAC    int                    `json:"device_ims_tac"`
	DeviceIMSCellID int                    `json:"device_ims_cell_id"`
	Config          *profiles.CarrierProfile `json:"config"`
	Active          bool                   `json:"active"`
}

// LoadActiveOverrides 在启动时从 carrier_templates + carrier_activation 加载 active 的用户配置到 profiles 内存。
func LoadActiveOverrides() error {
	visible, err := db.ListCarrierVisible()
	if err != nil {
		return fmt.Errorf("list carrier visible: %w", err)
	}
	loaded := 0
	for _, v := range visible {
		act, _ := db.GetCarrierActivation(v.PLMN)
		if act == nil || act.TemplateID == nil {
			continue
		}
		tpl, err := db.GetCarrierTemplate(*act.TemplateID)
		if err != nil || tpl == nil || tpl.ProfileJSON == "" {
			continue
		}
		var p profiles.CarrierProfile
		if err := json.Unmarshal([]byte(tpl.ProfileJSON), &p); err != nil {
			logger.Warn("解析运营商配置 JSON 失败，跳过",
				"key", v.PLMN, "err", err)
			continue
		}
		profiles.SetUserOverrideByKey(v.PLMN, &p)
		loaded++
	}
	if loaded > 0 {
		logger.Info("已从数据库加载运营商用户配置", "count", loaded, "event", "CARRIER_CONFIG_DB_LOADED")
	}
	return nil
}

// ResolveRegisterProfile 从 CarrierProfile 解析出 voiceclient.RegisterProfile。
func ResolveRegisterProfile(p *profiles.CarrierProfile) voiceclient.RegisterProfile {
	if p == nil {
		return voiceclient.RegisterProfile{}
	}
	return voiceclient.CarrierProfileToRegisterProfile(p)
}

// ResolveSIPInstanceURN 从 CarrierProfile 解析出 SIP Instance URN。
func ResolveSIPInstanceURN(p *profiles.CarrierProfile) string {
	if p == nil || p.Device.IMEI == "" {
		return ""
	}
	imei := strings.TrimSpace(p.Device.IMEI)
	if imei == "" {
		return ""
	}
	return "urn:gsma:imei:" + imei
}

// ResolveRegisterExpiry 从 CarrierProfile 解析出 REGISTER Expires。
func ResolveRegisterExpiry(p *profiles.CarrierProfile) time.Duration {
	if p == nil || p.IMS.Expires <= 0 {
		return 0
	}
	return time.Duration(p.IMS.Expires) * time.Second
}

// ResolvePCSCFAddr 从 CarrierProfile 解析出 P-CSCF 地址。
func ResolvePCSCFAddr(p *profiles.CarrierProfile) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.IMS.PCSCFAddr)
}
