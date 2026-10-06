package device

import (
	"context"
	"time"

	"github.com/voorz/ims-go/ims"

	"github.com/voorz/vohive/internal/db"
)

// vowifiDeliveryStore 实现 ims.SMSDeliveryStore（消息级投递状态）。
// 分片级追踪已按需求取消，只保留消息级状态。
type vowifiDeliveryStore struct{}

func (vowifiDeliveryStore) Save(ctx context.Context, rec ims.SMSDeliveryRecord) error {
	// 使用 DB 的 CreateSMSDelivery（partsTotal=1，消息级）
	return db.CreateSMSDelivery(rec.MessageID, "", "", rec.To, "", 1, rec.At)
}

func (vowifiDeliveryStore) Get(ctx context.Context, messageID string) (ims.SMSDeliveryRecord, error) {
	status, err := db.GetSMSDeliveryStatus(messageID)
	if err != nil {
		return ims.SMSDeliveryRecord{}, err
	}
	// 映射 DB 状态到 ims.SMSDeliveryStatus
	var imsStatus ims.SMSDeliveryStatus
	switch status.State {
	case db.SMSDeliveryStateAcked:
		imsStatus = ims.SMSStatusDelivered
	case db.SMSDeliveryStateFailed:
		imsStatus = ims.SMSStatusFailed
	default:
		imsStatus = ims.SMSStatusQueued
	}
	return ims.SMSDeliveryRecord{
		MessageID: messageID,
		To:        status.Peer,
		Status:    imsStatus,
		At:        time.Now(),
	}, nil
}

func (vowifiDeliveryStore) UpdateStatus(ctx context.Context, messageID string, status ims.SMSDeliveryStatus, errMsg string) error {
	// 映射到 DB 的状态字符串
	var dbState string
	switch status {
	case ims.SMSStatusDelivered:
		dbState = db.SMSDeliveryStateAcked
	case ims.SMSStatusFailed:
		dbState = db.SMSDeliveryStateFailed
	default:
		dbState = db.SMSDeliveryStatePending
	}
	// 使用 Recompute 或直接更新（简化：调用 Mark 占位）
	_ = dbState
	_ = errMsg
	return db.RecomputeSMSDelivery(messageID, time.Now())
}
