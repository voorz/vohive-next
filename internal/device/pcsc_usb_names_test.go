package device

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListUSBIdentitiesFromDebugfs(t *testing.T) {
	// 创建临时 debugfs 文件
	tmpDir := t.TempDir()
	debugFile := filepath.Join(tmpDir, "devices")
	debugContent := `T:  Bus=01 Lev=00 Prnt=00 Port=00 Cnt=00 Dev#=  1 Spd=480  MxCh= 8
D:  Ver= 2.00 Cls=09(hub  ) Sub=00 Prot=01 MxPS=64 #Cfgs=  1
P:  Vendor=1d6b ProdID=0002 Rev= 6.18
S:  Manufacturer=Linux 6.18.33.2-microsoft-standard-WSL2 vhci_hcd
S:  Product=USB/IP Virtual Host Controller
S:  SerialNumber=vhci_hcd.0
C:* #Ifs= 1 Cfg#= 1 Atr=e0 MxPwr=  0mA
I:* If#= 0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=00 Prot=00 Driver=hub
E:  Ad=81(I) Atr=03(Int.) MxPS=   4 Ivl=256ms

T:  Bus=01 Lev=01 Prnt=01 Port=00 Cnt=01 Dev#=  7 Spd=12   MxCh= 0
D:  Ver= 2.01 Cls=00(>ifc ) Sub=00 Prot=00 MxPS=64 #Cfgs=  1
P:  Vendor=0bda ProdID=0165 Rev= 1.00
S:  Manufacturer=ESTKme Technology Limited
S:  Product=ESTKme-RED
S:  SerialNumber=2051315E5056
C:* #Ifs= 1 Cfg#= 1 Atr=80 MxPwr=100mA
I:* If#= 0 Alt= 0 #EPs= 3 Cls=0b(scard) Sub=00 Prot=00 Driver=(none)
E:  Ad=81(I) Atr=02(Bulk) MxPS=  64 Ivl=0ms
E:  Ad=01(O) Atr=02(Bulk) MxPS=  64 Ivl=0ms
E:  Ad=82(I) Atr=03(Int.) MxPS=   8 Ivl=16ms

T:  Bus=01 Lev=01 Prnt=01 Port=01 Cnt=02 Dev#=  6 Spd=480  MxCh= 0
D:  Ver= 2.00 Cls=ef(misc ) Sub=02 Prot=01 MxPS=64 #Cfgs=  1
P:  Vendor=2c7c ProdID=0125 Rev= 3.18
S:  Manufacturer=BAIWANG
S:  Product=Baiwang
C:* #Ifs= 8 Cfg#= 1 Atr=80 MxPwr=500mA
A:  FirstIf#= 5 IfCount= 3 Cls=01(audio) Sub=00 Prot=00
I:* If#= 0 Alt= 0 #EPs= 2 Cls=ff(vend.) Sub=ff Prot=ff Driver=option
E:  Ad=81(I) Atr=02(Bulk) MxPS= 512 Ivl=0ms
E:  Ad=01(O) Atr=02(Bulk) MxPS= 512 Ivl=0ms
I:* If#= 1 Alt= 0 #EPs= 3 Cls=ff(vend.) Sub=00 Prot=00 Driver=option
E:  Ad=83(I) Atr=03(Int.) MxPS=  10 Ivl=32ms
E:  Ad=82(I) Atr=02(Bulk) MxPS= 512 Ivl=0ms
E:  Ad=02(O) Atr=02(Bulk) MxPS= 512 Ivl=0ms
I:* If#= 4 Alt= 0 #EPs= 3 Cls=ff(vend.) Sub=ff Prot=ff Driver=qmi_wwan
E:  Ad=89(I) Atr=03(Int.) MxPS=   8 Ivl=32ms
E:  Ad=88(I) Atr=02(Bulk) MxPS= 512 Ivl=0ms
E:  Ad=05(O) Atr=02(Bulk) MxPS= 512 Ivl=0ms
I:* If#= 5 Alt= 0 #EPs= 0 Cls=01(audio) Sub=01 Prot=00 Driver=snd-usb-audio
I:* If#= 6 Alt= 0 #EPs= 0 Cls=01(audio) Sub=02 Prot=00 Driver=snd-usb-audio
I:  If#= 6 Alt= 1 #EPs= 1 Cls=01(audio) Sub=02 Prot=00 Driver=snd-usb-audio
E:  Ad=8a(I) Atr=05(Isoc) MxPS=  16 Ivl=1ms
I:* If#= 7 Alt= 0 #EPs= 0 Cls=01(audio) Sub=02 Prot=00 Driver=snd-usb-audio
I:  If#= 7 Alt= 1 #EPs= 1 Cls=01(audio) Sub=02 Prot=00 Driver=snd-usb-audio
E:  Ad=06(O) Atr=09(Isoc) MxPS=  16 Ivl=1ms
`
	if err := os.WriteFile(debugFile, []byte(debugContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 替换全局路径
	orig := usbDebugDevicesFile
	usbDebugDevicesFile = debugFile
	defer func() { usbDebugDevicesFile = orig }()

	ids := listUSBIdentitiesFromDebugfs()
	if len(ids) != 3 {
		t.Fatalf("expected 3 devices, got %d", len(ids))
	}

	// 读卡器应该是第二个设备
	reader := ids[1]
	if reader.Serial != "2051315E5056" {
		t.Errorf("reader serial = %q, want 2051315E5056", reader.Serial)
	}
	if reader.USBVersion != "2.01" {
		t.Errorf("reader USBVersion = %q, want 2.01", reader.USBVersion)
	}
	if len(reader.Interfaces) != 1 {
		t.Fatalf("reader interfaces = %d, want 1", len(reader.Interfaces))
	}
	iface := reader.Interfaces[0]
	if iface.Class != "0b(scard)" {
		t.Errorf("iface class = %q, want 0b(scard)", iface.Class)
	}
	if iface.Driver != "(none)" {
		t.Errorf("iface driver = %q, want (none)", iface.Driver)
	}
	if iface.Endpoint != "Bulk In/Out + Interrupt" {
		t.Errorf("iface endpoint = %q, want 'Bulk In/Out + Interrupt'", iface.Endpoint)
	}

	// 测试 FormatUSBInfoLine
	info := FormatUSBInfoLine(&reader, true)
	wantInfo := "SN:2051315E5056;USB-2.01;Cls=0b(scard);（Bulk In/Out + Interrupt）"
	if info != wantInfo {
		t.Errorf("reader info = %q, want %q", info, wantInfo)
	}

	// 模组应该是第三个设备
	modem := ids[2]
	modemInfo := FormatUSBInfoLine(&modem, false)
	wantModemInfo := "option*4;qmi_wwan*1;snd-usb-audio*3"
	if modemInfo != wantModemInfo {
		t.Errorf("modem info = %q, want %q", modemInfo, wantModemInfo)
	}
}
