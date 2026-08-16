package db

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"gorm.io/gorm"
)

// VoiceGateway 语音网关配置（单行表，ID 固定为 1）
type VoiceGateway struct {
	ID int `gorm:"primaryKey;autoIncrement:false" json:"id"`
	// SIP 服务
	SIPListen     string `gorm:"column:sip_listen" json:"sip_listen"`
	SIPTransport  string `gorm:"column:sip_transport" json:"sip_transport"`
	SIPRealm      string `gorm:"column:sip_realm" json:"sip_realm"`
	SIPExternalIP string `gorm:"column:sip_external_ip" json:"sip_external_ip"`
	// WebSocket
	WSListen    string `gorm:"column:ws_listen" json:"ws_listen"`
	WSSListen   string `gorm:"column:wss_listen" json:"wss_listen"`
	WSSCertFile string `gorm:"column:wss_cert_file" json:"wss_cert_file"`
	WSSKeyFile  string `gorm:"column:wss_key_file" json:"wss_key_file"`
	// SIP 用户（单用户，username = Web 管理员用户名）
	Username string `gorm:"column:username" json:"username"`
	Password string `gorm:"column:password" json:"password"`
	DeviceID string `gorm:"column:device_id" json:"device_id"`
	// 媒体
	RTPPortMin int    `gorm:"column:rtp_port_min" json:"rtp_port_min"`
	RTPPortMax int    `gorm:"column:rtp_port_max" json:"rtp_port_max"`
	Codecs     string `gorm:"column:codecs" json:"codecs"` // JSON 数组字符串
	// Linphone 官方推送账户（用于 sip.linphone.org 推送唤醒）
	LinphoneUser     string `gorm:"column:linphone_user" json:"linphone_user"`
	LinphonePassword string `gorm:"column:linphone_password" json:"linphone_password"`

	UpdatedAt time.Time `json:"updated_at"`
}

func (VoiceGateway) TableName() string { return "voice_gateway" }

// GetVoiceGateway 获取语音网关配置（单行，ID=1）
func GetVoiceGateway() (*VoiceGateway, error) {
	if DB == nil {
		return nil, nil
	}
	var vg VoiceGateway
	err := DB.First(&vg, 1).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &vg, nil
}

// SaveVoiceGateway 保存语音网关配置（upsert，固定 ID=1）
func SaveVoiceGateway(vg *VoiceGateway) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	vg.ID = 1
	vg.UpdatedAt = time.Now()
	return DB.Save(vg).Error
}

// GetOrCreateVoiceGateway 获取语音网关配置，首次启动自动创建并生成授权码
func GetOrCreateVoiceGateway(webUsername string) *VoiceGateway {
	vg, _ := GetVoiceGateway()
	if vg == nil {
		vg = &VoiceGateway{
			SIPListen:    "0.0.0.0:5060",
			SIPTransport: "udp",
			SIPRealm:     "vohive.local",
			WSListen:     "0.0.0.0:5061",
			RTPPortMin:   10000,
			RTPPortMax:   20000,
			Codecs:       `["PCMU/8000","PCMA/8000"]`,
			Username:     webUsername,
			Password:     GenerateVoicePassword(4),
		}
		_ = SaveVoiceGateway(vg)
	} else if vg.Password == "" {
		vg.Password = GenerateVoicePassword(4)
		_ = SaveVoiceGateway(vg)
	}
	return vg
}

// GenerateVoicePassword 生成 n 位随机码（大写字母 + 数字，排除 I/O/0/1）
func GenerateVoicePassword(n int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}
