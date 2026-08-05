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
	"sort"
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
		userMap[userConfigs[i].ProfileKey] = &userConfigs[i]
	}

	// 3. 合并：系统默认 + 用户独有
	seen := make(map[string]bool)
	out := make([]ListItem, 0, len(sysAll)+len(userConfigs))

	for key, p := range sysAll {
		seen[key] = true
		item := ListItem{
			Key:              key,
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
			Key:             key,
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

	// Sort by carrier name asc for stable ordering across refreshes.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out, nil
}

// Get 返回指定运营商的详情（系统默认 + 用户配置）。
// brand 为可选参数，用于指定变体（如 "cmlink"），空字符串返回 base profile。
func Get(mcc, mnc, brand string) (*Detail, error) {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	brand = strings.TrimSpace(brand)
	if mcc == "" || mnc == "" {
		return nil, fmt.Errorf("mcc and mnc are required")
	}

	key := plmnKey(mcc, mnc)
	if brand != "" {
		key = key + "__" + brand
	}

	// 系统默认
	var sysDefault *profiles.CarrierProfile
	if p, err := profiles.LookupWithSPN(mcc, mnc, brand); err == nil && p != nil {
		sysDefault = p
	}

	// 用户配置
	uc, err := db.GetCarrierConfig(key)
	if err != nil {
		return nil, fmt.Errorf("get carrier config: %w", err)
	}

	d := &Detail{
		Key:           key,
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
	key := plmnKey(mcc, mnc)
	c := &db.CarrierConfig{
		ProfileKey:      key,
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
func Save(key string, payload SavePayload) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("profile key is required")
	}
	if payload.Config == nil {
		return fmt.Errorf("config is required")
	}
	// 从 key 解析 MCC/MNC
	mcc, mnc := parseKey(key)
	payload.Config.MCC = mcc
	payload.Config.MNC = mnc
	payload.Config.Name = strings.TrimSpace(payload.Name)
	if payload.Config.IKE.Addr == "" {
		payload.Config.IKE.Addr = strings.TrimSpace(payload.IKEAddr)
	}
	if payload.Config.Device.IMSTAC == 0 {
		payload.Config.Device.IMSTAC = payload.DeviceIMSTAC
	}
	if payload.Config.Device.IMSCellID == 0 {
		payload.Config.Device.IMSCellID = payload.DeviceIMSCellID
	}

	jsonBytes, err := json.Marshal(payload.Config)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}

	c := &db.CarrierConfig{
		ProfileKey:      key,
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

	// 热更新
	if payload.Active {
		profiles.SetUserOverrideByKey(key, payload.Config)
		logger.Info("运营商配置已热更新", "key", key, "event", "CARRIER_CONFIG_HOT_RELOAD")
	} else {
		profiles.SetUserOverrideByKey(key, nil)
	}

	return nil
}

// Remove 删除运营商用户配置（仅允许删除用户添加的运营商，系统运营商不可删除）。
func Remove(key string) error {
	key = strings.TrimSpace(key)
	mcc, mnc := parseKey(key)
	// 系统运营商不可删除
	if p, err := profiles.Lookup(mcc, mnc); err == nil && p != nil {
		return fmt.Errorf("系统运营商 %s 不可删除", key)
	}
	if err := db.DeleteCarrierConfig(key); err != nil {
		return err
	}
	profiles.SetUserOverrideByKey(key, nil)
	return nil
}

// DeleteConfig 仅删除用户配置的 ProfileJSON，保留运营商条目。
func DeleteConfig(key string) error {
	key = strings.TrimSpace(key)
	uc, err := db.GetCarrierConfig(key)
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
	profiles.SetUserOverrideByKey(key, nil)
	return nil
}

// Activate 启用指定运营商的用户配置。
func Activate(key string) error {
	key = strings.TrimSpace(key)
	uc, err := db.GetCarrierConfig(key)
	if err != nil {
		return err
	}
	if uc == nil {
		return fmt.Errorf("carrier config not found for key %s", key)
	}
	if uc.ProfileJSON == "" {
		return fmt.Errorf("no profile json to activate for key %s", key)
	}
	if err := db.SetCarrierConfigActive(key, true); err != nil {
		return err
	}
	var p profiles.CarrierProfile
	if err := json.Unmarshal([]byte(uc.ProfileJSON), &p); err != nil {
		return fmt.Errorf("unmarshal profile json: %w", err)
	}
	profiles.SetUserOverrideByKey(key, &p)
	logger.Info("运营商配置已激活", "key", key, "event", "CARRIER_CONFIG_ACTIVATED")
	return nil
}

// Deactivate 禁用指定运营商的用户配置。
func Deactivate(key string) error {
	key = strings.TrimSpace(key)
	if err := db.SetCarrierConfigActive(key, false); err != nil {
		return err
	}
	profiles.SetUserOverrideByKey(key, nil)
	logger.Info("运营商配置已禁用", "key", key, "event", "CARRIER_CONFIG_DEACTIVATED")
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
				"key", c.ProfileKey, "err", err)
			continue
		}
		profiles.SetUserOverrideByKey(c.ProfileKey, &p)
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
	for key, p := range all {
		out = append(out, ListItem{
			Key:              key,
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
		existing, err := db.GetCarrierConfig(key)
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

// plmnKey 生成 MCC-MNC 键（与 profiles 包一致）。
func plmnKey(mcc, mnc string) string {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	trimmed := strings.TrimLeft(mnc, "0")
	if trimmed == "" && mnc != "" {
		trimmed = "0"
	}
	return mcc + "-" + trimmed
}

// parseKey 从 profile key 解析 MCC 和 MNC。
// "234-33" → ("234", "33"), "234-33__cmlink" → ("234", "33")
func parseKey(key string) (mcc, mnc string) {
	base := key
	if idx := strings.Index(base, "__"); idx >= 0 {
		base = base[:idx]
	}
	parts := strings.SplitN(base, "-", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
