package db

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CarrierConfig 存储用户自定义的运营商配置（覆盖系统内置 JSON 模板）。
// 主键为 MCC+MNC 组合，ProfileJSON 存储完整的 CarrierProfile JSON。
type CarrierConfig struct {
	MCC             string    `gorm:"column:mcc;primaryKey" json:"mcc"`
	MNC             string    `gorm:"column:mnc;primaryKey" json:"mnc"`
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
	if c.MCC == "" || c.MNC == "" {
		return errors.New("mcc and mnc are required")
	}
	now := time.Now()
	c.UpdatedAt = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "mcc"}, {Name: "mnc"}},
		DoUpdates: clause.Assignments(map[string]any{
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
	err := DB.Order("mcc asc, mnc asc").Find(&out).Error
	return out, err
}

// GetCarrierConfig 按 MCC+MNC 查询单条用户配置。
func GetCarrierConfig(mcc, mnc string) (*CarrierConfig, error) {
	if DB == nil {
		return nil, nil
	}
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	if mcc == "" || mnc == "" {
		return nil, errors.New("mcc and mnc are required")
	}
	var out CarrierConfig
	err := DB.Where("mcc = ? AND mnc = ?", mcc, mnc).First(&out).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// DeleteCarrierConfig 删除指定运营商的用户配置。
func DeleteCarrierConfig(mcc, mnc string) error {
	if DB == nil {
		return nil
	}
	return DB.Where("mcc = ? AND mnc = ?", mcc, mnc).Delete(&CarrierConfig{}).Error
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
func SetCarrierConfigActive(mcc, mnc string, active bool) error {
	if DB == nil {
		return nil
	}
	return DB.Model(&CarrierConfig{}).
		Where("mcc = ? AND mnc = ?", mcc, mnc).
		Update("active", active).Error
}
