package device

import (
	"github.com/voorz/ims-go/ims"
	"testing"
	"time"

	"github.com/voorz/vohive/internal/db"
	)

func TestVoWiFiSMSHistoryRecorderPersistsSentSMS(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	if err := db.UpdateSIMCardVoWiFiPhoneNumberByIMSI("imsi-vowifi-1", "+8613800000000"); err != nil {
		t.Fatalf("UpdateSIMCardVoWiFiPhoneNumberByIMSI() error=%v", err)
	}
	p := NewPool(nil)
	p.workers["dev-1"] = &Worker{ID: "dev-1", Backend: &workerPhoneBackendStub{imsi: "imsi-vowifi-1"}}

	err := vowifiSMSHistoryRecorder{pool: p}.RecordSent("dev-1", ims.SMSSentData{To: "+10010", MsgID: "test-msg", Success: true})
	if err != nil {
		t.Fatalf("RecordSent() error=%v", err)
	}

	var sms db.SMS
	if err := db.DB.Where("imsi = ? AND type = ?", "imsi-vowifi-1", 2).First(&sms).Error; err != nil {
		t.Fatalf("First(sent sms) error=%v", err)
	}
	if sms.Sender != "+8613800000000" || sms.Recipient != "+10010" || sms.Content != "hello" || sms.Status != 2 {
		t.Fatalf("sent sms=%+v", sms)
	}
}

func TestVoWiFiSMSHistoryRecorderPersistsReceivedSMS(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	if err := db.UpdateSIMCardVoWiFiPhoneNumberByIMSI("imsi-vowifi-2", "+8613900000000"); err != nil {
		t.Fatalf("UpdateSIMCardVoWiFiPhoneNumberByIMSI() error=%v", err)
	}
	p := NewPool(nil)
	p.workers["dev-2"] = &Worker{ID: "dev-2", Backend: &workerPhoneBackendStub{imsi: "imsi-vowifi-2"}}

	_, err := vowifiSMSHistoryRecorder{pool: p}.RecordReceived("dev-2", ims.SMSReceivedData{From: "+10086", Content: "inbound", At: time.Now()})
	if err != nil {
		t.Fatalf("RecordReceived() error=%v", err)
	}

	var sms db.SMS
	if err := db.DB.Where("imsi = ? AND type = ?", "imsi-vowifi-2", 1).First(&sms).Error; err != nil {
		t.Fatalf("First(received sms) error=%v", err)
	}
	if sms.Sender != "+10086" || sms.Recipient != "+8613900000000" || sms.Content != "inbound" || sms.Status != 0 {
		t.Fatalf("received sms=%+v", sms)
	}
}

func TestVoWiFiSMSHistoryRecorderSkipsSuppressedReceivedSMS(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	p := NewPool(nil)
	p.workers["dev-ota"] = &Worker{ID: "dev-ota", Backend: &workerPhoneBackendStub{imsi: "imsi-ota"}}

	_, err := vowifiSMSHistoryRecorder{pool: p}.RecordReceived("dev-ota", ims.SMSReceivedData{From: "+10086", Content: "[SIM OTA 23.048]\ndecrypt=not_attempted\nsecurity=可能加密\nraw=0011", At: time.Now()})
	if err != nil {
		t.Fatalf("RecordReceived() error=%v", err)
	}

	var count int64
	if err := db.DB.Model(&db.SMS{}).Where("imsi = ? AND type = ?", "imsi-ota", 1).Count(&count).Error; err != nil {
		t.Fatalf("Count(received sms) error=%v", err)
	}
	if count != 0 {
		t.Fatalf("suppressed received sms count=%d want 0", count)
	}
}

func TestWorkerProcessSMSSkipsSuppressedReceivedSMS(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	p := NewPool(nil)
	w := &Worker{
		ID:      "dev-worker-ota",
		Pool:    p,
		Backend: &workerPhoneBackendStub{imsi: "imsi-worker-ota"},
	}

	w.processSMS("+10086", "[SIM OTA 23.048]\ndecrypt=not_attempted\nsecurity=可能加密\nraw=0011", time.Now())

	var count int64
	if err := db.DB.Model(&db.SMS{}).Where("imsi = ? AND type = ?", "imsi-worker-ota", 1).Count(&count).Error; err != nil {
		t.Fatalf("Count(received sms) error=%v", err)
	}
	if count != 0 {
		t.Fatalf("suppressed worker sms count=%d want 0", count)
	}
}

func TestVoWiFiSMSHistoryRecorderPersistsSendFailure(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	p := NewPool(nil)
	p.workers["dev-fail"] = &Worker{ID: "dev-fail", Backend: &workerPhoneBackendStub{imsi: "imsi-fail"}}

	err := vowifiSMSHistoryRecorder{pool: p}.RecordSendFailure("dev-fail", "+10010", "failed sms", time.Now())
	if err != nil {
		t.Fatalf("RecordSendFailure() error=%v", err)
	}

	var sms db.SMS
	if err := db.DB.Where("imsi = ? AND type = ? AND status = ?", "imsi-fail", 2, 3).First(&sms).Error; err != nil {
		t.Fatalf("First(failed sms) error=%v", err)
	}
	if sms.Recipient != "+10010" || sms.Content != "failed sms" {
		t.Fatalf("failed sms=%+v", sms)
	}
}

func TestVoWiFiSMSHistoryRecorderPersistsLocalNumberLearned(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	p := NewPool(nil)
	p.workers["dev-3"] = &Worker{ID: "dev-3", Backend: &workerPhoneBackendStub{imsi: "imsi-vowifi-3"}}

	err := vowifiSMSHistoryRecorder{pool: p}.RecordLocalNumber("dev-1", ims.LocalNumberLearnedData{Number: "+8613700000000"})
	if err != nil {
		t.Fatalf("RecordLocalNumberLearned() error=%v", err)
	}

	sub := loadDeviceTestSIMSubscriptionByIMSI(t, "imsi-vowifi-3")
	if sub.VowifiPhoneNumber != "+8613700000000" || sub.PhoneNumber != "+8613700000000" {
		t.Fatalf("subscription=%+v", sub)
	}
}

func TestRecordLocalNumberLearnedStagesByICCIDWhenIMSIEmpty(t *testing.T) {
	initDevicePhoneNumberTestDB(t)

	p := NewPool(nil)
	defer p.cancel()
	w := &Worker{ID: "dev-1"}
	w.state.Identity.Ready = true
	w.state.Identity.ICCID = "8944000000000000111"
	p.workers["dev-1"] = w

	rec := vowifiSMSHistoryRecorder{pool: p}
	err := rec.RecordLocalNumber("dev-1", ims.LocalNumberLearnedData{Number: "+447700900200"})
	if err != nil {
		t.Fatalf("RecordLocalNumberLearned error=%v", err)
	}
	got, err := db.GetPhoneNumberByIMSIOrICCID("", "8944000000000000111")
	if err != nil {
		t.Fatalf("GetPhoneNumberByIMSIOrICCID error=%v", err)
	}
	if got != "+447700900200" {
		t.Fatalf("phone=%q, want +447700900200 staged by ICCID", got)
	}
}

func TestVoWiFiRuntimeDispatcherPersistsSMSSentWithoutNotifier(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	p := NewPool(nil)
	p.workers["dev-dispatch"] = &Worker{ID: "dev-dispatch", Backend: &workerPhoneBackendStub{imsi: "imsi-dispatch"}}

	poolVoWiFiRuntimeDispatcher{pool: p}.Dispatch("dev-1", ims.Event{Type: ims.EventSMSSent, Data: ims.SMSSentData{To: "+10010", MsgID: "test"}})

	var count int64
	if err := db.DB.Model(&db.SMS{}).Where("imsi = ? AND type = ? AND status = ?", "imsi-dispatch", 2, 2).Count(&count).Error; err != nil {
		t.Fatalf("Count(sent sms) error=%v", err)
	}
	if count != 1 {
		t.Fatalf("sent sms count=%d want 1", count)
	}
}

func TestVoWiFiSMSHistoryRecorderSkipsDuplicateReceivedSMS(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	if err := db.UpdateSIMCardVoWiFiPhoneNumberByIMSI("imsi-vowifi-dup", "+8613900000000"); err != nil {
		t.Fatalf("UpdateSIMCardVoWiFiPhoneNumberByIMSI() error=%v", err)
	}
	p := NewPool(nil)
	p.workers["dev-dup"] = &Worker{ID: "dev-dup", Backend: &workerPhoneBackendStub{imsi: "imsi-vowifi-dup"}}

	rec := vowifiSMSHistoryRecorder{pool: p}

	first, err := rec.RecordReceived("dev-dup", ims.SMSReceivedData{From: "+447751284582", Content: "ij991818短信登录验证码，5分钟内有效，请勿泄露。", At: time.Now()})
	if err != nil {
		t.Fatalf("first RecordReceived() error=%v", err)
	}
	if !first.Stored || first.Duplicate || first.Suppressed {
		t.Fatalf("first result=%+v, want stored only", first)
	}

	second, err := rec.RecordReceived("dev-dup", ims.SMSReceivedData{From: "+447751284582", Content: "ij991818短信登录验证码，5分钟内有效，请勿泄露。", At: time.Now()})
	if err != nil {
		t.Fatalf("second RecordReceived() error=%v", err)
	}
	if second.Stored || !second.Duplicate || second.Suppressed {
		t.Fatalf("second result=%+v, want duplicate only", second)
	}

	var count int64
	if err := db.DB.Model(&db.SMS{}).Where("imsi = ? AND type = ?", "imsi-vowifi-dup", 1).Count(&count).Error; err != nil {
		t.Fatalf("Count(received sms) error=%v", err)
	}
	if count != 1 {
		t.Fatalf("received sms count=%d want 1", count)
	}
}

type countingVoWiFiNotifier struct {
	smsCount int
	rawCount int
}

func (n *countingVoWiFiNotifier) NotifySMS(deviceID, sender, content string, timestamp time.Time) {
	n.smsCount++
}

func (n *countingVoWiFiNotifier) NotifySMSWithSource(deviceID, sender, content, source string, timestamp time.Time) {
	n.smsCount++
}

func (n *countingVoWiFiNotifier) NotifyRaw(msg string) {
	n.rawCount++
}

func (n *countingVoWiFiNotifier) NotifyIPRotated(deviceID, oldIP, newIP string, duration time.Duration) {
}

func TestVoWiFiRuntimeDispatcherSkipsDuplicateReceivedNotification(t *testing.T) {
	initDevicePhoneNumberTestDB(t)
	if err := db.UpdateSIMCardVoWiFiPhoneNumberByIMSI("imsi-vowifi-dispatch-dup", "+8613900000000"); err != nil {
		t.Fatalf("UpdateSIMCardVoWiFiPhoneNumberByIMSI() error=%v", err)
	}
	p := NewPool(nil)
	p.workers["dev-dispatch-dup"] = &Worker{ID: "dev-dispatch-dup", Backend: &workerPhoneBackendStub{imsi: "imsi-vowifi-dispatch-dup"}}
	notifier := &countingVoWiFiNotifier{}
	p.SetNotifier(notifier)

	ev := ims.Event{
		Type: ims.EventSMSReceived,
		Data: ims.SMSReceivedData{
			From:    "+447751284582",
			Content: "ij991818短信登录验证码，5分钟内有效，请勿泄露。",
			At:      time.Date(2026, 6, 29, 23, 58, 55, 0, time.UTC),
		},
	}
	poolVoWiFiRuntimeDispatcher{pool: p}.Dispatch("dev-dispatch-dup", ev)
	ev.Data = ims.SMSReceivedData{
		From:    "+447751284582",
		Content: "ij991818短信登录验证码，5分钟内有效，请勿泄露。",
		At:      time.Date(2026, 6, 29, 23, 58, 55, 0, time.UTC).Add(73 * time.Second),
	}
	poolVoWiFiRuntimeDispatcher{pool: p}.Dispatch("dev-dispatch-dup", ev)

	if notifier.smsCount != 1 {
		t.Fatalf("sms notification count=%d want 1", notifier.smsCount)
	}
	var count int64
	if err := db.DB.Model(&db.SMS{}).Where("imsi = ? AND type = ?", "imsi-vowifi-dispatch-dup", 1).Count(&count).Error; err != nil {
		t.Fatalf("Count(received sms) error=%v", err)
	}
	if count != 1 {
		t.Fatalf("received sms count=%d want 1", count)
	}
}
