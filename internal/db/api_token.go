package db

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// APIToken API 访问令牌（多 Token 管理）
type APIToken struct {
	ID         uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string     `gorm:"not null" json:"name"`
	TokenHash  string     `gorm:"column:token_hash;not null" json:"-"`       // SHA-256 hash，不返回前端
	Prefix     string     `gorm:"not null" json:"prefix"`                     // 前 8 位用于展示辨识
	Expiry     int64      `gorm:"default:0" json:"expiry"`                    // Unix 时间戳，0=永不过期
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func (APIToken) TableName() string { return "api_tokens" }

// CreateAPIToken 生成新 Token，存储 hash，返回明文（仅一次）
func CreateAPIToken(name string, expiryDays int) (token string, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("名称不能为空")
	}
	if DB == nil {
		return "", errors.New("数据库未初始化")
	}

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", errors.New("生成令牌失败")
	}
	token = hex.EncodeToString(rawBytes)

	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var expiry int64 = 0
	if expiryDays > 0 {
		expiry = time.Now().Add(time.Duration(expiryDays) * 24 * time.Hour).Unix()
	}

	row := APIToken{
		Name:      name,
		TokenHash: tokenHash,
		Prefix:    token[:8],
		Expiry:    expiry,
	}
	if err := DB.Create(&row).Error; err != nil {
		return "", err
	}
	return token, nil
}

// ListAPITokens 列出全部 Token（不含 hash）
func ListAPITokens() ([]APIToken, error) {
	if DB == nil {
		return nil, nil
	}
	var out []APIToken
	err := DB.Order("created_at desc").Find(&out).Error
	return out, err
}

// DeleteAPIToken 按 ID 删除
func DeleteAPIToken(id uint) error {
	if DB == nil {
		return nil
	}
	return DB.Delete(&APIToken{}, id).Error
}

// ValidateAPIToken 校验 token 是否有效，有效则更新 last_used_at
func ValidateAPIToken(token string) bool {
	if DB == nil || strings.TrimSpace(token) == "" {
		return false
	}
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var row APIToken
	err := DB.Select("id", "expiry").Where("token_hash = ?", tokenHash).First(&row).Error
	if err != nil {
		return false
	}
	if row.Expiry > 0 && time.Now().Unix() > row.Expiry {
		return false
	}
	now := time.Now()
	DB.Model(&APIToken{}).Where("id = ?", row.ID).Update("last_used_at", &now)
	return true
}
