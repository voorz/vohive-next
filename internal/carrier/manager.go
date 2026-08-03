// Package carrier provides CRUD operations and DB ↔ CarrierProfile conversion
// for user-defined carrier configurations stored in the vohive-next database.
//
// The package bridges the vohive-next DB layer (db.CarrierConfig) with the
// vowifi-core profiles layer (profiles.CarrierProfile), enabling JSON-first
// carrier configuration with user overrides that take priority over embedded
// system defaults.
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
	MCC             string `json:"mcc"`
	MNC             string `json:"mnc"`
	Name            string `json:"name"`
	IKEAddr         string `json:"ike_addr"`
	DeviceIMSTAC    int    `json:"device_ims_tac"`
	DeviceIMSCellID int    `json:"device_ims_cell_id"`
	HasUserConfig   bool   `json:"has_user_config"`
	Active          bool   `json:"active"`
	HasSystemDefault bool  `json:"has_system_default"`
}

// Detail 对应前端 CarrierDetail 类型。
type Detail struct {
	MCC             string                `json:"mcc"`
	MNC             string                `json:"mnc"`
	Name            string                `json:"name"`
	IKEAddr         string                `json:"ike_addr"`
	DeviceIMSTAC    int                   `json:"device_ims_tac"`
	DeviceIMSCellID int                   `json:"device_ims_cell_id"`
	SystemDefault   *profiles.CarrierProfile `json:"system_default"`
	UserConfig      *profiles.CarrierProfile `json:"user_config"`
	Active          bool                  `json:"active"`
}

// SavePayload 对应前端 CarrierSavePayload 类型。
type SavePayload struct {
	Name            string                `json:"name"`
	IKEAddr         string                `json:"ike_addr"`
	DeviceIMSTAC    int                   `json:"device_ims_tac"`
	DeviceIMSCellID int                   `json:"device_ims_cell_id"`
	Config          *profiles.CarrierProfile `json:"config"`
	Active          bool                  `json:"active"`
}

// AddPayload 对应前端 CarrierAddPayload 类型。
type AddPayload struct {
	Name            string `json:"name"`
	MCC             string `json:"mcc"`
	MNC             string `json:"mnc"`
	IKEAddr         string `json:"ike_addr,omitempty"`
	DeviceIMSTAC    int    `json:"device_ims_tac,omitempty"`
	DeviceIMSCellID int    `json:"device_ims_cell_id,omitempty"`
}

// BatchImportResult 对应前端 CarrierBatchImportResult 类型。
type BatchImportResult struct {
	Total       int      `json:"total"`
	Imported    int      `json:"imported"`
	Skipped     int      `json:"skipped"`
	SkippedList []string `json:"skipped_list"`
}

// List 返回所有运营商列表项（合并系统默认 + 用户配置）。
func List() ([]ListItem, error) {
	// 1. 获取系统默认
	sysAll, err := profiles.All()
	if err != nil {
		return nil, fmt.Errorf("load system profiles: %w", err)
	}

	// 2. 获取用户配置
	userConfigs, err := db.ListCarrierConfigs()
	if err != nil {
		return nil, fmt.Errorf("list carrier configs: %w", err)
	}
	userMap := make(map[string]*db.CarrierConfig, len(userConfigs))
	for i := range userConfigs {
		key := plmnKey(userConfigs[i].MCC, userConfigs[i].MNC)
		userMap[key] = &userConfigs[i]
	}

	// 3. 合并：系统默认 + 用户独有
	seen := make(map[string]bool)
	out := make([]ListItem, 0, len(sysAll)+len(userConfigs))

	for key, p := range sysAll {
		seen[key] = true
		item := ListItem{
			MCC:              p.MCC,
			MNC:              p.MNC,
			Name:             p.Name,
			IKEAddr:          p.IKE.Addr,
			DeviceIMSTAC:     p.Device.IMSTAC,
			DeviceIMSCellID:  p.Device.IMSCellID,
			HasSystemDefault: true,
		}
		if uc, ok := userMap[key]; ok {
			item.HasUserConfig = true
			item.Active = uc.Active
			// 用户配置的展示字段优先
			if uc.Name != "" {
				item.Name = uc.Name
			}
			if uc.IKEAddr != "" {
				item.IKEAddr = uc.IKEAddr
			}
			if uc.DeviceIMSTAC != 0 {
				item.DeviceIMSTAC = uc.DeviceIMSTAC
			}
			if uc.DeviceIMSCellID != 0 {
				item.DeviceIMSCellID = uc.DeviceIMSCellID
			}
		}
		out = append(out, item)
	}

	// 用户独有（无系统默认）
	for key, uc := range userMap {
		if seen[key] {
			continue
		}
		out = append(out, ListItem{
			MCC:             uc.MCC,
			MNC:             uc.MNC,
			Name:            uc.Name,
			IKEAddr:         uc.IKEAddr,
			DeviceIMSTAC:    uc.DeviceIMSTAC,
			DeviceIMSCellID: uc.DeviceIMSCellID,
			HasUserConfig:   true,
			Active:          uc.Active,
		})
	}

	return out, nil
}

// Get 返回指定运营商的详情（系统默认 + 用户配置）。
func Get(mcc, mnc string) (*Detail, error) {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	if mcc == "" || mnc == "" {
		return nil, fmt.Errorf("mcc and mnc are required")
	}

	// 系统默认
	var sysDefault *profiles.CarrierProfile
	if p, err := profiles.Lookup(mcc, mnc); err == nil && p != nil {
		sysDefault = p
	}

	// 用户配置
	uc, err := db.GetCarrierConfig(mcc, mnc)
	if err != nil {
		return nil, fmt.Errorf("get carrier config: %w", err)
	}

	d := &Detail{
		MCC:           mcc,
		MNC:           mnc,
		SystemDefault: sysDefault,
	}
	if uc != nil {
		d.Name = uc.Name
		d.IKEAddr = uc.IKEAddr
		d.DeviceIMSTAC = uc.DeviceIMSTAC
		d.DeviceIMSCellID = uc.DeviceIMSCellID
		d.Active = uc.Active
		if uc.ProfileJSON != "" {
			var p profiles.CarrierProfile
			if err := json.Unmarshal([]byte(uc.ProfileJSON), &p); err != nil {
				return nil, fmt.Errorf("unmarshal profile json: %w", err)
			}
			d.UserConfig = &p
		}
	} else if sysDefault != nil {
		d.Name = sysDefault.Name
		d.IKEAddr = sysDefault.IKE.Addr
		d.DeviceIMSTAC = sysDefault.Device.IMSTAC
		d.DeviceIMSCellID = sysDefault.Device.IMSCellID
	}

	return d, nil
}

// Add 添加新运营商（仅创建空壳，不写入 ProfileJSON）。
func Add(payload AddPayload) error {
	mcc := strings.TrimSpace(payload.MCC)
	mnc := strings.TrimSpace(payload.MNC)
	if mcc == "" || mnc == "" {
		return fmt.Errorf("mcc and mnc are required")
	}
	if strings.TrimSpace(payload.Name) == "" {
		return fmt.Errorf("name is required")
	}
	c := &db.CarrierConfig{
		MCC:             mcc,
		MNC:             mnc,
		Name:            strings.TrimSpace(payload.Name),
		IKEAddr:         strings.TrimSpace(payload.IKEAddr),
		DeviceIMSTAC:    payload.DeviceIMSTAC,
		DeviceIMSCellID: payload.DeviceIMSCellID,
		CreatedAt:       time.Now(),
	}
	return db.UpsertCarrierConfig(c)
}

// Save 保存用户配置（包含完整 CarrierProfile JSON）。
func Save(mcc, mnc string, payload SavePayload) error {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	if mcc == "" || mnc == "" {
		return fmt.Errorf("mcc and mnc are required")
	}
	if payload.Config == nil {
		return fmt.Errorf("config is required")
	}
	// 确保 MCC/MNC 一致
	payload.Config.MCC = mcc
	payload.Config.MNC = mnc

	jsonBytes, err := json.Marshal(payload.Config)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}

	c := &db.CarrierConfig{
		MCC:             mcc,
		MNC:             mnc,
		Name:            strings.TrimSpace(payload.Name),
		IKEAddr:         strings.TrimSpace(payload.IKEAddr),
		DeviceIMSTAC:    payload.DeviceIMSTAC,
		DeviceIMSCellID: payload.DeviceIMSCellID,
		ProfileJSON:     string(jsonBytes),
		Active:          payload.Active,
	}
	if err := db.UpsertCarrierConfig(c); err != nil {
		return err
	}

	// 热更新：如果 active=true，立即注入到 profiles 内存
	if payload.Active {
		profiles.SetUserOverride(mcc, mnc, payload.Config)
		logger.Info("运营商配置已热更新", "mcc", mcc, "mnc", mnc, "event", "CARRIER_CONFIG_HOT_RELOAD")
	} else {
		// 如果 deactive 了，清除 override
		profiles.SetUserOverride(mcc, mnc, nil)
	}

	return nil
}

// Remove 删除运营商用户配置。
func Remove(mcc, mnc string) error {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	if err := db.DeleteCarrierConfig(mcc, mnc); err != nil {
		return err
	}
	// 清除内存中的 override
	profiles.SetUserOverride(mcc, mnc, nil)
	return nil
}

// DeleteConfig 仅删除用户配置的 ProfileJSON，保留运营商条目。
func DeleteConfig(mcc, mnc string) error {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	uc, err := db.GetCarrierConfig(mcc, mnc)
	if err != nil {
		return err
	}
	if uc == nil {
		return nil
	}
	uc.ProfileJSON = ""
	uc.Active = false
	if err := db.UpsertCarrierConfig(uc); err != nil {
		return err
	}
	profiles.SetUserOverride(mcc, mnc, nil)
	return nil
}

// Activate 启用指定运营商的用户配置。
func Activate(mcc, mnc string) error {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	uc, err := db.GetCarrierConfig(mcc, mnc)
	if err != nil {
		return err
	}
	if uc == nil {
		return fmt.Errorf("carrier config not found for %s-%s", mcc, mnc)
	}
	if uc.ProfileJSON == "" {
		return fmt.Errorf("no profile json to activate for %s-%s", mcc, mnc)
	}
	if err := db.SetCarrierConfigActive(mcc, mnc, true); err != nil {
		return err
	}
	// 热更新
	var p profiles.CarrierProfile
	if err := json.Unmarshal([]byte(uc.ProfileJSON), &p); err != nil {
		return fmt.Errorf("unmarshal profile json: %w", err)
	}
	profiles.SetUserOverride(mcc, mnc, &p)
	logger.Info("运营商配置已激活", "mcc", mcc, "mnc", mnc, "event", "CARRIER_CONFIG_ACTIVATED")
	return nil
}

// Deactivate 禁用指定运营商的用户配置。
func Deactivate(mcc, mnc string) error {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	if err := db.SetCarrierConfigActive(mcc, mnc, false); err != nil {
		return err
	}
	profiles.SetUserOverride(mcc, mnc, nil)
	logger.Info("运营商配置已禁用", "mcc", mcc, "mnc", mnc, "event", "CARRIER_CONFIG_DEACTIVATED")
	return nil
}

// LoadActiveOverrides 在启动时从 DB 加载所有 active 的用户配置到 profiles 内存。
func LoadActiveOverrides() error {
	configs, err := db.ListActiveCarrierConfigs()
	if err != nil {
		return fmt.Errorf("list active carrier configs: %w", err)
	}
	for _, c := range configs {
		if c.ProfileJSON == "" {
			continue
		}
		var p profiles.CarrierProfile
		if err := json.Unmarshal([]byte(c.ProfileJSON), &p); err != nil {
			logger.Warn("解析运营商配置 JSON 失败，跳过",
				"mcc", c.MCC, "mnc", c.MNC, "err", err)
			continue
		}
		profiles.SetUserOverride(c.MCC, c.MNC, &p)
	}
	if len(configs) > 0 {
		logger.Info("已从数据库加载运营商用户配置", "count", len(configs), "event", "CARRIER_CONFIG_DB_LOADED")
	}
	return nil
}

// ListSystemDefaults 返回所有系统内置运营商配置列表。
func ListSystemDefaults() ([]ListItem, error) {
	all, err := profiles.All()
	if err != nil {
		return nil, err
	}
	out := make([]ListItem, 0, len(all))
	for _, p := range all {
		out = append(out, ListItem{
			MCC:              p.MCC,
			MNC:              p.MNC,
			Name:             p.Name,
			IKEAddr:          p.IKE.Addr,
			DeviceIMSTAC:     p.Device.IMSTAC,
			DeviceIMSCellID:  p.Device.IMSCellID,
			HasSystemDefault: true,
		})
	}
	return out, nil
}

// GetSystemDefault 返回指定运营商的系统内置配置。
func GetSystemDefault(mcc, mnc string) (*profiles.CarrierProfile, error) {
	return profiles.Lookup(mcc, mnc)
}

// BatchImport 批量导入运营商。
func BatchImport(payloads []AddPayload, mode string) (*BatchImportResult, error) {
	result := &BatchImportResult{
		Total:       len(payloads),
		SkippedList: []string{},
	}
	for _, p := range payloads {
		key := plmnKey(p.MCC, p.MNC)
		// 检查是否已存在
		existing, err := db.GetCarrierConfig(p.MCC, p.MNC)
		if err != nil {
			return nil, fmt.Errorf("check existing %s: %w", key, err)
		}
		if existing != nil && mode == "skip_existing" {
			result.Skipped++
			result.SkippedList = append(result.SkippedList, key)
			continue
		}
		if err := Add(p); err != nil {
			result.Skipped++
			result.SkippedList = append(result.SkippedList, key)
			continue
		}
		result.Imported++
	}
	return result, nil
}

// ResolveRegisterProfile 从 CarrierProfile 解析出 voiceclient.RegisterProfile。
// 直接复用 vowifi-core 的导出转换函数，确保字段映射一致。
func ResolveRegisterProfile(p *profiles.CarrierProfile) voiceclient.RegisterProfile {
	if p == nil {
		return voiceclient.RegisterProfile{}
	}
	return voiceclient.CarrierProfileToRegisterProfile(p)
}

// ResolveSIPInstanceURN 从 CarrierProfile 解析出 SIP Instance URN。
// 目前从 Device.IMEI 派生（urn:gsma:imei:xxx），未来可扩展为独立字段。
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

// plmnKey 生成 MCC-MNC 键（与 profiles 包一致）。
func plmnKey(mcc, mnc string) string {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	// strip leading zeros from MNC
	trimmed := strings.TrimLeft(mnc, "0")
	if trimmed == "" && mnc != "" {
		trimmed = "0"
	}
	return mcc + "-" + trimmed
}
