package db

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// CarrierIndex 存储从 plmn-index 仓库同步的运营商索引数据。
// 主键为 PLMN key（如 "234-33"），raw_json 保存完整的 plmn-index 条目。
type CarrierIndex struct {
	PLMN        string    `gorm:"column:plmn;primaryKey" json:"plmn"`
	MCC         string    `gorm:"column:mcc" json:"mcc"`
	MNC         string    `gorm:"column:mnc" json:"mnc"`
	CountryName string    `gorm:"column:country_name" json:"country_name"`
	CountryISO  string    `gorm:"column:country_iso" json:"country_iso"`
	CountryCode string    `gorm:"column:country_code" json:"country_code"`
	Region      string    `gorm:"column:region" json:"region"`
	RawJSON     string    `gorm:"column:raw_json" json:"raw_json"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (CarrierIndex) TableName() string { return "carrier_index" }

// CarrierVisible 存储用户添加到可见列表的运营商。
// hidden=true 表示从列表隐藏（软删除），用户可重新添加。
type CarrierVisible struct {
	PLMN     string    `gorm:"column:plmn;primaryKey" json:"plmn"`
	Hidden   bool      `gorm:"column:hidden" json:"hidden"`
	AddedAt  time.Time `gorm:"column:added_at" json:"added_at"`
}

func (CarrierVisible) TableName() string { return "carrier_visible" }

// CarrierTemplate 存储运营商 VoWiFi 配置模板。
// 每个运营商+来源一行，通过 active 字段标记当前生效行。
// source: "system"（系统默认）或 "user"（用户自定义）
type CarrierTemplate struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Key         string    `gorm:"column:key;index" json:"key"` // PLMN 或 PLMN__brand
	Name        string    `gorm:"column:name" json:"name"`
	Source      string    `gorm:"column:source;default:user" json:"source"` // system | user
	Active      bool      `gorm:"column:active;default:false" json:"active"`
	ProfileJSON string    `gorm:"column:profile_json" json:"profile_json"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (CarrierTemplate) TableName() string { return "carrier_templates" }

// --- CarrierIndex CRUD ---

// UpsertCarrierIndex 创建或更新运营商索引条目。
func UpsertCarrierIndex(c *CarrierIndex) error {
	if DB == nil {
		return nil
	}
	c.PLMN = strings.TrimSpace(c.PLMN)
	if c.PLMN == "" {
		return nil
	}
	c.UpdatedAt = time.Now()
	return DB.Save(c).Error
}

// BatchUpsertCarrierIndex 批量创建或更新运营商索引条目。
func BatchUpsertCarrierIndex(items []*CarrierIndex) error {
	if DB == nil || len(items) == 0 {
		return nil
	}
	now := time.Now()
	for _, c := range items {
		c.UpdatedAt = now
	}
	return DB.Save(items).Error
}

// GetCarrierIndex 按 PLMN 查询运营商索引。
// 支持 trimmed/padded 兼容查询：如果精确匹配失败，会尝试对 MNC 部分
// 进行 trim/padding 后再查。例如传入 "234-015" 会先查 "234-015"，
// 找不到再查 "234-15"（stripped）；反之亦然。
func GetCarrierIndex(plmn string) (*CarrierIndex, error) {
	if DB == nil {
		return nil, nil
	}
	plmn = strings.TrimSpace(plmn)
	if plmn == "" {
		return nil, nil
	}
	// 1. 尝试精确匹配
	var out CarrierIndex
	err := DB.Where("plmn = ?", plmn).First(&out).Error
	if err == nil {
		return &out, nil
	}
	if !isRecordNotFound(err) {
		return nil, err
	}
	// 2. 尝试 MNC 前导零兼容
	altKey := altPlmnKey(plmn)
	if altKey != "" && altKey != plmn {
		err = DB.Where("plmn = ?", altKey).First(&out).Error
		if err == nil {
			return &out, nil
		}
		if !isRecordNotFound(err) {
			return nil, err
		}
	}
	return nil, nil
}

// altPlmnKey 生成一个 PLMN key 的替代形式（trimmed ↔ padded）。
// "234-015" → "234-15"（strip）；"234-15" → "234-015"（pad to 3 digits）。
// 如果无法生成替代形式，返回空字符串。
func altPlmnKey(plmn string) string {
	parts := strings.SplitN(plmn, "-", 2)
	if len(parts) != 2 {
		return ""
	}
	mcc, mnc := parts[0], parts[1]
	// 如果 MNC 有前导零，strip 掉
	trimmed := strings.TrimLeft(mnc, "0")
	if trimmed == "" && mnc != "" {
		trimmed = "0"
	}
	if trimmed != mnc {
		return mcc + "-" + trimmed
	}
	// 如果 MNC 是 2 位（stripped），尝试 padding 到 3 位
	if len(mnc) == 2 {
		return mcc + "-0" + mnc
	}
	return ""
}

// SearchCarrierIndex 搜索运营商索引（按 PLMN/名称/国家）。
// 返回匹配结果，limit 控制返回数量。
func SearchCarrierIndex(query string, limit int) ([]CarrierIndex, error) {
	if DB == nil {
		return nil, nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	// raw_json 中包含 operator/brand 信息，用 LIKE 搜索
	pattern := "%" + query + "%"
	var out []CarrierIndex
	err := DB.Where(
		"plmn LIKE ? OR mcc LIKE ? OR country_name LIKE ? OR country_iso LIKE ? OR raw_json LIKE ?",
		pattern, pattern, pattern, pattern, pattern,
	).Limit(limit).Find(&out).Error
	return out, err
}

// CountCarrierIndex 返回索引中的运营商总数。
func CountCarrierIndex() (int64, error) {
	if DB == nil {
		return 0, nil
	}
	var count int64
	err := DB.Model(&CarrierIndex{}).Count(&count).Error
	return count, err
}

// --- CarrierVisible CRUD ---

// AddCarrierVisible 添加运营商到可见列表。
func AddCarrierVisible(plmn string) error {
	if DB == nil {
		return nil
	}
	plmn = strings.TrimSpace(plmn)
	if plmn == "" {
		return nil
	}
	return DB.Save(&CarrierVisible{
		PLMN:    plmn,
		Hidden:  false,
		AddedAt: time.Now(),
	}).Error
}

// BatchAddCarrierVisible 批量添加运营商到可见列表。
func BatchAddCarrierVisible(plmns []string) error {
	if DB == nil || len(plmns) == 0 {
		return nil
	}
	now := time.Now()
	items := make([]*CarrierVisible, 0, len(plmns))
	for _, p := range plmns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		items = append(items, &CarrierVisible{
			PLMN:    p,
			Hidden:  false,
			AddedAt: now,
		})
	}
	return DB.Save(items).Error
}

// RemoveCarrierVisible 从可见列表移除运营商（硬删除）。
func RemoveCarrierVisible(plmn string) error {
	if DB == nil {
		return nil
	}
	plmn = strings.TrimSpace(plmn)
	if plmn == "" {
		return nil
	}
	return DB.Where("plmn = ?", plmn).Delete(&CarrierVisible{}).Error
}

// ListCarrierVisible 返回所有未隐藏的运营商 PLMN 列表。
func ListCarrierVisible() ([]CarrierVisible, error) {
	if DB == nil {
		return nil, nil
	}
	var out []CarrierVisible
	err := DB.Where("hidden = ?", false).Order("added_at desc").Find(&out).Error
	return out, err
}

// IsCarrierVisible 检查运营商是否在可见列表中。
func IsCarrierVisible(plmn string) (bool, error) {
	if DB == nil {
		return false, nil
	}
	plmn = strings.TrimSpace(plmn)
	if plmn == "" {
		return false, nil
	}
	var count int64
	err := DB.Model(&CarrierVisible{}).Where("plmn = ? AND hidden = ?", plmn, false).Count(&count).Error
	return count > 0, err
}

// --- CarrierTemplate CRUD ---

// GetCarrierTemplateByKey 按 key 查询运营商的用户模板（每个 key 最多一个模板）。
func GetCarrierTemplateByKey(key string) (*CarrierTemplate, error) {
	if DB == nil {
		return nil, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	var out CarrierTemplate
	err := DB.Where("key = ?", key).First(&out).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// ListCarrierTemplates 返回指定运营商的所有模板。
func ListCarrierTemplates(key string) ([]CarrierTemplate, error) {
	if DB == nil {
		return nil, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	var out []CarrierTemplate
	err := DB.Where("key = ?", key).Order("updated_at desc").Find(&out).Error
	return out, err
}

// GetCarrierTemplate 按 ID 查询模板。
func GetCarrierTemplate(id int64) (*CarrierTemplate, error) {
	if DB == nil {
		return nil, nil
	}
	var out CarrierTemplate
	err := DB.Where("id = ?", id).First(&out).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// CreateCarrierTemplate 创建新模板。
func CreateCarrierTemplate(t *CarrierTemplate) error {
	if DB == nil {
		return nil
	}
	t.Key = strings.TrimSpace(t.Key)
	t.Name = strings.TrimSpace(t.Name)
	if t.Key == "" {
		return nil
	}
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	return DB.Create(t).Error
}

// UpdateCarrierTemplate 更新模板。
func UpdateCarrierTemplate(t *CarrierTemplate) error {
	if DB == nil {
		return nil
	}
	t.Name = strings.TrimSpace(t.Name)
	t.UpdatedAt = time.Now()
	return DB.Save(t).Error
}

// DeleteCarrierTemplate 删除模板。
func DeleteCarrierTemplate(id int64) error {
	if DB == nil {
		return nil
	}
	return DB.Where("id = ?", id).Delete(&CarrierTemplate{}).Error
}

// --- CarrierTemplate 激活管理 ---

// GetActiveCarrierTemplate 返回指定 key 的当前激活模板。
// 先精确匹配 key，查不到则用 PLMN 前缀模糊匹配（处理无 SPN 但 DB 有 brand 后缀的情况）。
func GetActiveCarrierTemplate(key string) (*CarrierTemplate, error) {
	if DB == nil {
		return nil, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	var out CarrierTemplate
	// 1. 精确匹配
	err := DB.Where("key = ? AND active = ?", key, true).First(&out).Error
	if err == nil {
		return &out, nil
	}
	if !isRecordNotFound(err) {
		return nil, err
	}
	// 2. PLMN 前缀模糊匹配（key 或 key + "__%"）
	//    例如 key="262-002"，匹配 "262-002__Vodafone DE"
	err = DB.Where("key LIKE ? AND active = ?", key+"__%", true).First(&out).Error
	if err == nil {
		return &out, nil
	}
	if !isRecordNotFound(err) {
		return nil, err
	}
	return nil, nil
}

// ActivateCarrierTemplate 激活指定 key 的模板（source 类型），同时禁用同 key 其他行。
func ActivateCarrierTemplate(key, source string) error {
	if DB == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		// 先禁用同 key 的所有行
		if err := tx.Model(&CarrierTemplate{}).Where("key = ?", key).Update("active", false).Error; err != nil {
			return err
		}
		// 激活指定 source 行
		return tx.Model(&CarrierTemplate{}).Where("key = ? AND source = ?", key, source).Update("active", true).Error
	})
}

// --- Helpers ---

func isRecordNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "record not found")
}
