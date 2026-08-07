package db

import (
	"strings"
	"time"
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

// CarrierTemplate 存储用户创建/导入的 VoWiFi 配置模板。
// 一个运营商可以有多个模板，通过 carrier_activation 选择激活哪个。
type CarrierTemplate struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Key         string    `gorm:"column:key;index" json:"key"` // PLMN 或 PLMN__brand
	Name        string    `gorm:"column:name" json:"name"`
	ProfileJSON string    `gorm:"column:profile_json" json:"profile_json"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (CarrierTemplate) TableName() string { return "carrier_templates" }

// CarrierActivation 记录每个运营商当前激活的模板。
// template_id 为 NULL 表示使用内置模板（profiles/*.json）或 3GPP 默认。
type CarrierActivation struct {
	Key        string  `gorm:"column:key;primaryKey" json:"key"`
	TemplateID *int64  `gorm:"column:template_id" json:"template_id"`
}

func (CarrierActivation) TableName() string { return "carrier_activation" }

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
func GetCarrierIndex(plmn string) (*CarrierIndex, error) {
	if DB == nil {
		return nil, nil
	}
	plmn = strings.TrimSpace(plmn)
	if plmn == "" {
		return nil, nil
	}
	var out CarrierIndex
	err := DB.Where("plmn = ?", plmn).First(&out).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
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
// 如果该模板已激活，同时清除激活记录。
func DeleteCarrierTemplate(id int64) error {
	if DB == nil {
		return nil
	}
	// 先检查是否被激活
	var act CarrierActivation
	if err := DB.Where("template_id = ?", id).First(&act).Error; err == nil {
		// 清除激活
		DB.Model(&CarrierActivation{}).Where("template_id = ?", id).Update("template_id", nil)
	}
	return DB.Where("id = ?", id).Delete(&CarrierTemplate{}).Error
}

// --- CarrierActivation CRUD ---

// GetCarrierActivation 查询指定运营商的激活模板。
func GetCarrierActivation(key string) (*CarrierActivation, error) {
	if DB == nil {
		return nil, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	var out CarrierActivation
	err := DB.Where("key = ?", key).First(&out).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// SetCarrierActivation 设置指定运营商的激活模板。
// templateID 为 nil 表示取消激活（回退到内置模板）。
func SetCarrierActivation(key string, templateID *int64) error {
	if DB == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	return DB.Save(&CarrierActivation{
		Key:        key,
		TemplateID: templateID,
	}).Error
}

// ClearCarrierActivation 清除指定运营商的激活记录。
func ClearCarrierActivation(key string) error {
	if DB == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	return DB.Where("key = ?", key).Delete(&CarrierActivation{}).Error
}

// --- Helpers ---

func isRecordNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "record not found")
}
