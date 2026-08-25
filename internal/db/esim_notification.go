package db

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// EsimNotificationRecord 对标 NekoKo NotificationRecord + notifications 表。
// 记录每条 eSIM 通知的发送状态、备份内容和 RSP 响应。
//
// status 值: 0=未发送(pending), 1=已发送(sent), 2=发送失败(failed), 3=未发送直接删除(deleted)
type EsimNotificationRecord struct {
	EID                string `gorm:"column:eid;primaryKey" json:"eid"`
	SeqNumber          int64  `gorm:"column:seq_number;primaryKey" json:"seq_number"`
	ICCID              string `gorm:"column:iccid;primaryKey" json:"iccid"`
	ProfileName        string `gorm:"column:profile_name" json:"profile_name"`             // 关联 profile 的名称（手机号/卡名）
	MCC                string `gorm:"column:mcc" json:"mcc"`                               // 关联 profile 的 MCC（用于国旗显示）
	Content            string `gorm:"column:content" json:"content"`                           // base64 编码的 PendingNotification
	Timestamp          int64  `gorm:"column:timestamp" json:"timestamp"`                     // 毫秒时间戳
	Status             int    `gorm:"column:status" json:"status"`                           // 0/1/2/3
	ResponseCode       *int   `gorm:"column:response_code" json:"response_code"`             // HTTP 响应码
	ResponseContent    string `gorm:"column:response_content" json:"response_content"`       // HTTP 响应内容
	NotificationServer string `gorm:"column:notification_server" json:"notification_server"`   // RSP 服务器地址
	NotificationType   string `gorm:"column:notification_type" json:"notification_type"`       // install/enable/disable/delete
	DeletePending      bool   `gorm:"column:delete_pending;default:false" json:"delete_pending"` // 卡上移除失败，待重试
	CreatedAt          time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (EsimNotificationRecord) TableName() string {
	return "esim_notification_records"
}

// EsimNotificationSettings 对标 NekoKo AppSettings 的 notif* 开关。
// 每设备一行，控制 autoClean 行为。
type EsimNotificationSettings struct {
	DeviceID                   string `gorm:"column:device_id;primaryKey" json:"device_id"`
	AutoSendInstall            bool   `gorm:"column:auto_send_install;default:true" json:"auto_send_install"`
	AutoRemoveInstall          bool   `gorm:"column:auto_remove_install;default:false" json:"auto_remove_install"`
	AutoSendEnable             bool   `gorm:"column:auto_send_enable;default:true" json:"auto_send_enable"`
	AutoRemoveEnable           bool   `gorm:"column:auto_remove_enable;default:true" json:"auto_remove_enable"`
	DeleteWithoutSendingEnable bool   `gorm:"column:delete_without_sending_enable;default:false" json:"delete_without_sending_enable"`
	AutoSendDisable            bool   `gorm:"column:auto_send_disable;default:true" json:"auto_send_disable"`
	AutoRemoveDisable          bool   `gorm:"column:auto_remove_disable;default:true" json:"auto_remove_disable"`
	DeleteWithoutSendingDisable bool  `gorm:"column:delete_without_sending_disable;default:false" json:"delete_without_sending_disable"`
	AutoSendDelete             bool   `gorm:"column:auto_send_delete;default:true" json:"auto_send_delete"`
	AutoRemoveDelete           bool   `gorm:"column:auto_remove_delete;default:false" json:"auto_remove_delete"`
	ProcessInitialLoad         bool   `gorm:"column:process_initial_load;default:true" json:"process_initial_load"`
	ProcessAfterSwitch         bool   `gorm:"column:process_after_switch;default:true" json:"process_after_switch"`
	ProcessAfterDelete         bool   `gorm:"column:process_after_delete;default:true" json:"process_after_delete"`
	ProcessBeforeDownload      bool   `gorm:"column:process_before_download;default:true" json:"process_before_download"`
	ProcessAfterInstall        bool   `gorm:"column:process_after_install;default:true" json:"process_after_install"`
	CreatedAt                  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt                  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (EsimNotificationSettings) TableName() string {
	return "esim_notification_settings"
}

// --- EsimNotificationRecord CRUD ---

// SaveEsimNotification 保存或更新通知记录（upsert）
func SaveEsimNotification(record EsimNotificationRecord) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	record.UpdatedAt = time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	return DB.Save(&record).Error
}

// GetEsimNotification 获取单条通知记录
func GetEsimNotification(eid string, seqNumber int64, iccid string) (*EsimNotificationRecord, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}
	var record EsimNotificationRecord
	err := DB.Where("eid = ? AND seq_number = ? AND iccid = ?", eid, seqNumber, iccid).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetEsimNotificationsByEID 获取指定 EID 的所有通知记录
func GetEsimNotificationsByEID(eid string) ([]EsimNotificationRecord, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}
	var records []EsimNotificationRecord
	err := DB.Where("eid = ?", eid).Find(&records).Error
	return records, err
}

// UpdateEsimNotificationStatus 更新通知状态（对标 NekoKo updateNotificationStatus）
func UpdateEsimNotificationStatus(eid string, seqNumber int64, iccid string, status int, responseCode *int, responseContent string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	updates := map[string]interface{}{
		"status":            status,
		"response_code":     responseCode,
		"response_content":  responseContent,
		"updated_at":        time.Now(),
	}
	result := DB.Model(&EsimNotificationRecord{}).
		Where("eid = ? AND seq_number = ? AND iccid = ?", eid, seqNumber, iccid).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("notification record not found: eid=%s seq=%d iccid=%s", eid, seqNumber, iccid)
	}
	return nil
}

// IsEsimNotificationSent 检查通知是否已发送（status==1）
func IsEsimNotificationSent(eid string, seqNumber int64, iccid string) (bool, error) {
	if DB == nil {
		return false, errors.New("database not initialized")
	}
	var count int64
	err := DB.Model(&EsimNotificationRecord{}).
		Where("eid = ? AND seq_number = ? AND iccid = ? AND status = ?", eid, seqNumber, iccid, 1).
		Count(&count).Error
	return count > 0, err
}

// SetEsimNotificationDeletePending 设置/清除 deletePending 标记（对标 NekoKo setDeletePending）
func SetEsimNotificationDeletePending(eid string, seqNumber int64, iccid string, pending bool) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	return DB.Model(&EsimNotificationRecord{}).
		Where("eid = ? AND seq_number = ? AND iccid = ?", eid, seqNumber, iccid).
		Update("delete_pending", pending).Error
}

// GetEsimDeletePendingNotifications 获取所有 deletePending=true 的通知（对标 NekoKo getDeletePendingNotifications）
func GetEsimDeletePendingNotifications(eid string) ([]EsimNotificationRecord, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}
	var records []EsimNotificationRecord
	err := DB.Where("eid = ? AND delete_pending = ?", eid, true).Find(&records).Error
	return records, err
}

// DeleteEsimUnsentNotOnCard 删除不在卡上的未发送/失败通知（对标 NekoKo syncAndGetCount 的清理逻辑）
// seqNumbersOnCard 为当前卡上通知的 seq 列表。若为空，删除该 EID 下所有 status IN (0,2) 的记录。
func DeleteEsimUnsentNotOnCard(eid string, seqNumbersOnCard []int64) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	query := DB.Model(&EsimNotificationRecord{}).
		Where("eid = ? AND status IN ?", eid, []int{0, 2})
	if len(seqNumbersOnCard) > 0 {
		query = query.Where("seq_number NOT IN ?", seqNumbersOnCard)
	}
	return query.Delete(&EsimNotificationRecord{}).Error
}

// CountSentNotificationsByEID 统计指定 EID 下已发送(status==1) 的通知数量（G4: 用于 overview 角标排除已发送）
func CountSentNotificationsByEID(eid string) (int64, error) {
	if DB == nil {
		return 0, errors.New("database not initialized")
	}
	var count int64
	err := DB.Model(&EsimNotificationRecord{}).
		Where("eid = ? AND status = ?", eid, 1).
		Count(&count).Error
	return count, err
}

// GetSentNotificationSeqsByEID 获取指定 EID 下所有已发送(status==1) 通知的 seq 列表（G4: overview 排除已发送通知）
func GetSentNotificationSeqsByEID(eid string) ([]int64, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}
	var seqs []int64
	err := DB.Model(&EsimNotificationRecord{}).
		Where("eid = ? AND status = ?", eid, 1).
		Pluck("seq_number", &seqs).Error
	return seqs, err
}

// --- EsimNotificationSettings CRUD ---

// GetEsimNotificationSettings 获取设备的通知设置，不存在则返回默认值
func GetEsimNotificationSettings(deviceID string) (*EsimNotificationSettings, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}
	var settings EsimNotificationSettings
	err := DB.Where("device_id = ?", deviceID).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 返回默认设置
			return defaultEsimNotificationSettings(deviceID), nil
		}
		return nil, err
	}
	return &settings, nil
}

// UpsertEsimNotificationSettings 创建或更新设备通知设置
func UpsertEsimNotificationSettings(settings EsimNotificationSettings) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	settings.UpdatedAt = time.Now()
	if settings.CreatedAt.IsZero() {
		settings.CreatedAt = time.Now()
	}
	return DB.Save(&settings).Error
}

func defaultEsimNotificationSettings(deviceID string) *EsimNotificationSettings {
	return &EsimNotificationSettings{
		DeviceID:                    deviceID,
		AutoSendInstall:             true,
		AutoRemoveInstall:           true,
		AutoSendEnable:              true,
		AutoRemoveEnable:            true,
		DeleteWithoutSendingEnable:  false,
		AutoSendDisable:             true,
		AutoRemoveDisable:           true,
		DeleteWithoutSendingDisable: false,
		AutoSendDelete:              true,
		AutoRemoveDelete:            false,
		ProcessInitialLoad:          true,
		ProcessAfterSwitch:          true,
		ProcessAfterDelete:          true,
		ProcessBeforeDownload:       true,
		ProcessAfterInstall:         true,
		CreatedAt:                   time.Now(),
		UpdatedAt:                   time.Now(),
	}
}

// DeleteEsimNotificationsByEID 删除指定 EID 下的所有通知历史记录
func DeleteEsimNotificationsByEID(eid string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	return DB.Where("eid = ?", eid).Delete(&EsimNotificationRecord{}).Error
}
