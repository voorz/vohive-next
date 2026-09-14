package db

import (
	"testing"
)

func TestCardPolicyTableMigrated(t *testing.T) {
	openTestDB(t)
	if !DB.Migrator().HasTable(&CardPolicy{}) {
		t.Fatal("card_policies 表未建")
	}
	for _, col := range []string{"iccid", "network_enabled", "vowifi_enabled", "airplane_enabled", "ip_version", "apn", "source"} {
		if !DB.Migrator().HasColumn(&CardPolicy{}, col) {
			t.Fatalf("card_policies 缺列 %s", col)
		}
	}
}

func TestDefaultCardPolicy(t *testing.T) {
	p := DefaultCardPolicy("8986001234567890123")
	if p.ICCID != "8986001234567890123" {
		t.Fatalf("ICCID=%q", p.ICCID)
	}
	if p.NetworkEnabled || p.VoWiFiEnabled || p.AirplaneEnabled {
		t.Fatal("默认应全关")
	}
	if p.IPVersion != "v4" {
		t.Fatalf("默认 ip=%q，应 v4", p.IPVersion)
	}
	if p.Source != "auto" {
		t.Fatalf("source=%q，应 auto", p.Source)
	}
}

// vowifi=on 时强制 airplane=on：VoWiFi 接管射频等效飞行模式；
// 关 vowifi 时不动 airplane（保持飞行意图，避免关后卡回基站风控）。
func TestNormalizeCardPolicyVowifiForcesAirplane(t *testing.T) {
	// vowifi=on 但用户未开飞行：airplane 应被强制为 true
	p := CardPolicy{ICCID: "x", VoWiFiEnabled: true, AirplaneEnabled: false, IPVersion: ""}
	NormalizeCardPolicy(&p)
	if !p.AirplaneEnabled {
		t.Fatal("vowifi=on 应强制 airplane=on")
	}
	if p.IPVersion != "v4" {
		t.Fatalf("空 ip 应归一为 v4，得 %q", p.IPVersion)
	}

	// vowifi=on 且用户已开飞行：airplane 保持 true
	q := CardPolicy{ICCID: "x", VoWiFiEnabled: true, AirplaneEnabled: true}
	NormalizeCardPolicy(&q)
	if !q.AirplaneEnabled {
		t.Fatal("vowifi=on 且 airplane=on 应保持 true")
	}

	// vowifi=off 且 airplane=off：airplane 保持 false
	r := CardPolicy{ICCID: "x", VoWiFiEnabled: false, AirplaneEnabled: false}
	NormalizeCardPolicy(&r)
	if r.AirplaneEnabled {
		t.Fatal("vowifi=off 且 airplane=off 应保持 false")
	}
}

func TestResolveCardPolicyAutoCreates(t *testing.T) {
	openTestDB(t)
	iccid := "8986009999999999999"

	p, err := ResolveCardPolicy(iccid)
	if err != nil {
		t.Fatalf("ResolveCardPolicy error=%v", err)
	}
	if p.Source != "auto" || p.IPVersion != "v4" {
		t.Fatalf("自动建档默认不符: %+v", p)
	}

	var count int64
	DB.Model(&CardPolicy{}).Where("iccid = ?", iccid).Count(&count)
	if count != 1 {
		t.Fatalf("应已落库一行，得 %d", count)
	}
}

func TestUpsertCardPolicyUserOverride(t *testing.T) {
	openTestDB(t)
	iccid := "8986001111111111111"
	if _, err := ResolveCardPolicy(iccid); err != nil { // 先自动建档
		t.Fatal(err)
	}

	in := CardPolicy{ICCID: iccid, NetworkEnabled: true, VoWiFiEnabled: true, AirplaneEnabled: true, IPVersion: "v4v6", Source: "user"}
	if err := UpsertCardPolicy(in); err != nil {
		t.Fatalf("Upsert error=%v", err)
	}

	got, err := GetCardPolicy(iccid)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NetworkEnabled || !got.VoWiFiEnabled || !got.AirplaneEnabled {
		t.Fatalf("写入的字段应原样落库（airplane 独立存储）: %+v", got)
	}
	if got.IPVersion != "v4v6" || got.Source != "user" {
		t.Fatalf("覆盖未生效: %+v", got)
	}
}

func TestGetCardPolicyMissing(t *testing.T) {
	openTestDB(t)
	if _, err := GetCardPolicy("nope"); err == nil {
		t.Fatal("缺失应报错")
	}
}
