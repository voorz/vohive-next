package db

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CarrierConfig 存储用户自定义的运营商配置（覆盖系统内置 JSON 模板）。
// 主键为 ProfileKey（如 "234-33" 或 "234-33__cmlink"），
// 允许同 PLMN 多变体各自独立存储用户配置。
type CarrierConfig struct {
	ProfileKey      string    `gorm:"column:profile_key;primaryKey" json:"profile_key"`
	MCC             string    `gorm:"column:mcc" json:"mcc"`
	MNC             string    `gorm:"column:mnc" json:"mnc"`
	Name            string    `gorm:"column:name" json:"name"`
	IKEAddr         string    `gorm:"column:ike_addr" json:"ike_addr"`
	DeviceIMSTAC    int       `gorm:"column:device_ims_tac" json:"device_ims_tac"`
	DeviceIMSCellID int       `gorm:"column:device_ims_cell_id" json:"device_ims_cell_id"`
	ProfileJSON     string    `gorm:"column:profile_json" json:"profile_json"`
	Active          bool      `gorm:"column:active" json:"active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (CarrierConfig) TableName() string { return "carrier_configs" }

// UpsertCarrierConfig 创建或更新运营商配置。
func UpsertCarrierConfig(c *CarrierConfig) error {
	if DB == nil {
		return nil
	}
	c.MCC = strings.TrimSpace(c.MCC)
	c.MNC = strings.TrimSpace(c.MNC)
	c.ProfileKey = strings.TrimSpace(c.ProfileKey)
	if c.ProfileKey == "" && c.MCC != "" && c.MNC != "" {
		c.ProfileKey = c.MCC + "-" + c.MNC
	}
	if c.ProfileKey == "" {
		return errors.New("profile_key is required")
	}
	now := time.Now()
	c.UpdatedAt = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "profile_key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"mcc":               c.MCC,
			"mnc":               c.MNC,
			"name":              c.Name,
			"ike_addr":          c.IKEAddr,
			"device_ims_tac":    c.DeviceIMSTAC,
			"device_ims_cell_id": c.DeviceIMSCellID,
			"profile_json":      c.ProfileJSON,
			"active":            c.Active,
			"updated_at":        now,
		}),
	}).Create(c).Error
}

// ListCarrierConfigs 返回所有用户自定义运营商配置。
func ListCarrierConfigs() ([]CarrierConfig, error) {
	if DB == nil {
		return nil, nil
	}
	var out []CarrierConfig
	err := DB.Order("profile_key asc").Find(&out).Error
	return out, err
}

// GetCarrierConfig 按 ProfileKey 查询单条用户配置。
func GetCarrierConfig(profileKey string) (*CarrierConfig, error) {
	if DB == nil {
		return nil, nil
	}
	profileKey = strings.TrimSpace(profileKey)
	if profileKey == "" {
		return nil, errors.New("profile_key is required")
	}
	var out CarrierConfig
	err := DB.Where("profile_key = ?", profileKey).First(&out).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// GetCarrierConfigByPLMN 按 MCC+MNC 查询单条用户配置（向后兼容，返回 base profile）。
func GetCarrierConfigByPLMN(mcc, mnc string) (*CarrierConfig, error) {
	return GetCarrierConfig(mcc + "-" + mnc)
}

// DeleteCarrierConfig 删除指定运营商的用户配置。
func DeleteCarrierConfig(profileKey string) error {
	if DB == nil {
		return nil
	}
	return DB.Where("profile_key = ?", profileKey).Delete(&CarrierConfig{}).Error
}

// ListActiveCarrierConfigs 返回所有 active=true 的用户配置。
func ListActiveCarrierConfigs() ([]CarrierConfig, error) {
	if DB == nil {
		return nil, nil
	}
	var out []CarrierConfig
	err := DB.Where("active = ?", true).Find(&out).Error
	return out, err
}

// SetCarrierConfigActive 设置指定运营商配置的启用状态。
func SetCarrierConfigActive(profileKey string, active bool) error {
	if DB == nil {
		return nil
	}
	return DB.Model(&CarrierConfig{}).
		Where("profile_key = ?", profileKey).
		Update("active", active).Error
}

// MigrateCarrierConfigPK 迁移 carrier_configs 表 PK 到 (profile_key)。
// 处理两种旧 PK：2列 (mcc, mnc) 或 3列 (mcc, mnc, gid1)（TASK 6 GID1 残留）。
// SQLite 不支持直接修改 PK，需要重建表：建新表 → 复制数据 → 删旧表 → 重命名。
func MigrateCarrierConfigPK() error {
	if DB == nil {
		return nil
	}
	var pkColumns []struct {
		Name string
	}
	DB.Raw("SELECT name FROM pragma_table_info('carrier_configs') WHERE pk > 0 ORDER BY pk").Scan(&pkColumns)

	// 如果已经是单列 PK (profile_key)，无需迁移
	if len(pkColumns) == 1 && pkColumns[0].Name == "profile_key" {
		return nil
	}

	// 旧表有复合 PK (2列或3列)，需要迁移
	if len(pkColumns) >= 2 {
		DB.Exec(`CREATE TABLE carrier_configs_new (
			profile_key TEXT PRIMARY KEY,
			mcc TEXT NOT NULL DEFAULT '',
			mnc TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			ike_addr TEXT DEFAULT '',
			device_ims_tac INTEGER DEFAULT 0,
			device_ims_cell_id INTEGER DEFAULT 0,
			profile_json TEXT DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`)
		DB.Exec(`INSERT INTO carrier_configs_new (profile_key, mcc, mnc, name, ike_addr, device_ims_tac, device_ims_cell_id, profile_json, active, created_at, updated_at)
			SELECT COALESCE(profile_key, mcc || '-' || mnc), mcc, mnc, name, ike_addr, device_ims_tac, device_ims_cell_id, profile_json, active, created_at, updated_at
			FROM carrier_configs`)
		DB.Exec("DROP TABLE carrier_configs")
		DB.Exec("ALTER TABLE carrier_configs_new RENAME TO carrier_configs")
	}
	return nil
}
