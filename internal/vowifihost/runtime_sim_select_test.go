package vowifihost

import (
	"testing"

	"github.com/voorz/ims-go/ims"
)

var _ ims.AKAProvider = missingSIMProvider{}

func TestBuildVoWiFiSIMAdapterPrefersOverride(t *testing.T) {
	override := &testSIMAdapter{imsi: "222"}
	got, err := buildVoWiFiSIMAdapter(override, nil, "222")
	if err != nil {
		t.Fatalf("override 应被返回: %v", err)
	}
	if got == nil {
		t.Fatal("override 应被返回")
	}
	_, err = buildVoWiFiSIMAdapter(nil, nil, "333")
	if err == nil {
		t.Fatal("无 override 时应返回错误")
	}
}

type testSIMAdapter struct {
	imsi string
}

func (t *testSIMAdapter) GetIMSI() (string, error) { return t.imsi, nil }
func (t *testSIMAdapter) CalculateAKA(rand16, autn16 []byte) (ims.AKAResult, error) {
	return ims.AKAResult{}, nil
}
func (t *testSIMAdapter) Close() error { return nil }
