package db

import (
	"path/filepath"
	"testing"
)

func initEsimNotificationTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "esim_notif.db")
	if err := Init(dbPath); err != nil {
		t.Fatalf("Init() error=%v", err)
	}
	t.Cleanup(func() { DB = nil })
}

func TestSaveAndGetEsimNotification(t *testing.T) {
	initEsimNotificationTestDB(t)
	record := EsimNotificationRecord{
		EID:      "EID001",
		SeqNumber: 11,
		ICCID:    "8986001234567890123",
		Content:  "dGVzdA==",
		Status:   0,
		NotificationServer: "install.example.com",
		NotificationType:  "install",
	}
	if err := SaveEsimNotification(record); err != nil {
		t.Fatalf("SaveEsimNotification() error=%v", err)
	}
	got, err := GetEsimNotification("EID001", 11, "8986001234567890123")
	if err != nil {
		t.Fatalf("GetEsimNotification() error=%v", err)
	}
	if got.Status != 0 || got.NotificationType != "install" {
		t.Fatalf("got=%+v want status=0 type=install", got)
	}
}

func TestUpdateEsimNotificationStatus(t *testing.T) {
	initEsimNotificationTestDB(t)
	record := EsimNotificationRecord{
		EID: "EID002", SeqNumber: 12, ICCID: "8986001234567890124",
		Status: 0, NotificationType: "enable",
	}
	if err := SaveEsimNotification(record); err != nil {
		t.Fatalf("SaveEsimNotification() error=%v", err)
	}
	if err := UpdateEsimNotificationStatus("EID002", 12, "8986001234567890124", 1, nil, ""); err != nil {
		t.Fatalf("UpdateEsimNotificationStatus() error=%v", err)
	}
	sent, err := IsEsimNotificationSent("EID002", 12, "8986001234567890124")
	if err != nil {
		t.Fatalf("IsEsimNotificationSent() error=%v", err)
	}
	if !sent {
		t.Fatal("sent=false want=true after status=1")
	}
}

func TestDeleteEsimUnsentNotOnCard(t *testing.T) {
	initEsimNotificationTestDB(t)
	// seq 11: pending(0), on card → keep
	// seq 12: sent(1), on card → keep (status==1 not affected)
	// seq 13: failed(2), NOT on card → delete
	SaveEsimNotification(EsimNotificationRecord{EID: "EID003", SeqNumber: 11, ICCID: "iccid1", Status: 0})
	SaveEsimNotification(EsimNotificationRecord{EID: "EID003", SeqNumber: 12, ICCID: "iccid2", Status: 1})
	SaveEsimNotification(EsimNotificationRecord{EID: "EID003", SeqNumber: 13, ICCID: "iccid3", Status: 2})

	if err := DeleteEsimUnsentNotOnCard("EID003", []int64{11, 12}); err != nil {
		t.Fatalf("DeleteEsimUnsentNotOnCard() error=%v", err)
	}

	records, err := GetEsimNotificationsByEID("EID003")
	if err != nil {
		t.Fatalf("GetEsimNotificationsByEID() error=%v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records)=%d want 2 (seq 11 pending + seq 12 sent)", len(records))
	}
}

func TestCountSentNotificationsByEID(t *testing.T) {
	initEsimNotificationTestDB(t)
	SaveEsimNotification(EsimNotificationRecord{EID: "EID004", SeqNumber: 1, ICCID: "icc1", Status: 1})
	SaveEsimNotification(EsimNotificationRecord{EID: "EID004", SeqNumber: 2, ICCID: "icc2", Status: 1})
	SaveEsimNotification(EsimNotificationRecord{EID: "EID004", SeqNumber: 3, ICCID: "icc3", Status: 0})

	count, err := CountSentNotificationsByEID("EID004")
	if err != nil {
		t.Fatalf("CountSentNotificationsByEID() error=%v", err)
	}
	if count != 2 {
		t.Fatalf("count=%d want 2", count)
	}

	seqs, err := GetSentNotificationSeqsByEID("EID004")
	if err != nil {
		t.Fatalf("GetSentNotificationSeqsByEID() error=%v", err)
	}
	if len(seqs) != 2 {
		t.Fatalf("len(seqs)=%d want 2", len(seqs))
	}
}

func TestGetEsimNotificationSettingsDefault(t *testing.T) {
	initEsimNotificationTestDB(t)
	settings, err := GetEsimNotificationSettings("dev-unknown")
	if err != nil {
		t.Fatalf("GetEsimNotificationSettings() error=%v", err)
	}
	if !settings.AutoSendEnable || !settings.AutoRemoveEnable {
		t.Fatalf("default settings: AutoSendEnable=%v AutoRemoveEnable=%v, want true/true", settings.AutoSendEnable, settings.AutoRemoveEnable)
	}
	if settings.AutoRemoveInstall {
		t.Fatal("default AutoRemoveInstall=true, want false")
	}
}

func TestUpsertEsimNotificationSettings(t *testing.T) {
	initEsimNotificationTestDB(t)
	settings := EsimNotificationSettings{
		DeviceID: "dev-test",
		AutoSendInstall: false,
		AutoRemoveEnable: false,
	}
	if err := UpsertEsimNotificationSettings(settings); err != nil {
		t.Fatalf("UpsertEsimNotificationSettings() error=%v", err)
	}
	got, err := GetEsimNotificationSettings("dev-test")
	if err != nil {
		t.Fatalf("GetEsimNotificationSettings() error=%v", err)
	}
	if got.AutoSendInstall {
		t.Fatal("AutoSendInstall=true, want false")
	}
	if got.AutoRemoveEnable {
		t.Fatal("AutoRemoveEnable=true, want false")
	}
}
