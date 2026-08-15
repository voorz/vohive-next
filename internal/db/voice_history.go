package db

import "time"

// VoiceHistory 通话记录
type VoiceHistory struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	IMSI       string    `gorm:"column:imsi;index:idx_voice_imsi_ts,priority:1,sort:desc" json:"imsi"`
	ICCID      string    `gorm:"column:iccid;index" json:"iccid"`
	DeviceID   string    `gorm:"column:device_id;index" json:"device_id"`
	Peer       string    `gorm:"column:peer;index:idx_voice_peer_ts" json:"peer"`
	Number     string    `gorm:"column:number" json:"number"`
	Type       string    `gorm:"column:type" json:"type"`           // incoming, outgoing, missed
	Direction  string    `gorm:"column:direction" json:"direction"` // incoming, outgoing
	Duration   int       `gorm:"column:duration" json:"duration"`   // 秒
	CallID     string    `gorm:"column:call_id;index" json:"call_id"`
	Timestamp  time.Time `gorm:"index:idx_voice_imsi_ts,priority:2,sort:desc;index:idx_voice_peer_ts,sort:desc;index:idx_voice_ts,sort:desc" json:"timestamp"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateVoiceHistory 创建通话记录
func CreateVoiceHistory(record *VoiceHistory) error {
	if DB == nil {
		return nil
	}
	return DB.Create(record).Error
}

// GetVoiceHistory 获取通话记录列表
func GetVoiceHistory(imsi string, limit int, beforeTs *time.Time, beforePeer string) ([]VoiceHistory, error) {
	if DB == nil {
		return []VoiceHistory{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	q := DB.Model(&VoiceHistory{})
	if imsi != "" {
		q = q.Where("imsi = ?", imsi)
	}
	if beforeTs != nil && !beforeTs.IsZero() {
		if beforePeer != "" {
			q = q.Where("timestamp < ? OR (timestamp = ? AND peer < ?)", *beforeTs, *beforeTs, beforePeer)
		} else {
			q = q.Where("timestamp < ?", *beforeTs)
		}
	}

	var out []VoiceHistory
	err := q.Order("timestamp desc, peer desc").Limit(limit).Find(&out).Error
	return out, err
}

// DeleteVoiceHistory 删除单条通话记录
func DeleteVoiceHistory(id uint) error {
	if DB == nil {
		return nil
	}
	return DB.Delete(&VoiceHistory{}, id).Error
}

// DeleteVoiceHistoryByIMSI 删除某 IMSI 的所有通话记录
func DeleteVoiceHistoryByIMSI(imsi string) error {
	if DB == nil {
		return nil
	}
	return DB.Where("imsi = ?", imsi).Delete(&VoiceHistory{}).Error
}
