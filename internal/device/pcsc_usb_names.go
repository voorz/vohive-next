package device

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// USBIdentity 描述一个 USB 设备的描述符身份信息
type USBIdentity struct {
	Manufacturer string
	Product      string
	Serial       string
	VendorID     string
	ProductID    string
}

var sysfsUSBDevicesDir = "/sys/bus/usb/devices"
var usbDebugDevicesFile = "/sys/kernel/debug/usb/devices"

// ListUSBIdentities 枚举系统中 USB 设备的描述符信息。
// 优先读取 sysfs（常规 Linux），字符串描述符缺失时（如 WSL usbip 透传场景）
// 回退解析内核 debugfs 的 USB 设备清单。
func ListUSBIdentities() []USBIdentity {
	if ids := listUSBIdentitiesFromSysfs(); len(ids) > 0 {
		return ids
	}
	return listUSBIdentitiesFromDebugfs()
}

func readSysfsAttr(dir, name string) string {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func listUSBIdentitiesFromSysfs() []USBIdentity {
	entries, err := os.ReadDir(sysfsUSBDevicesDir)
	if err != nil {
		return nil
	}
	var out []USBIdentity
	for _, e := range entries {
		dir := filepath.Join(sysfsUSBDevicesDir, e.Name())
		vid := readSysfsAttr(dir, "idVendor")
		pid := readSysfsAttr(dir, "idProduct")
		if vid == "" || pid == "" {
			continue
		}
		product := readSysfsAttr(dir, "product")
		manufacturer := readSysfsAttr(dir, "manufacturer")
		serial := readSysfsAttr(dir, "serial")
		// sysfs 字符串描述符全部缺失（WSL usbip 场景）→ 视为不可用，交给 debugfs 回退
		if product == "" && manufacturer == "" && serial == "" {
			return nil
		}
		out = append(out, USBIdentity{
			Manufacturer: manufacturer,
			Product:      product,
			Serial:       serial,
			VendorID:     strings.ToLower(vid),
			ProductID:    strings.ToLower(pid),
		})
	}
	return out
}

// debugfs 清单格式（每个设备由 T: 行起始）：
//
//	T:  Bus=01 Lev=01 Prnt=01 Port=01 Cnt=02 Dev#=  3 Spd=480  MxCh= 0
//	P:  Vendor=0bda ProdID=0165 Rev= 1.00
//	S:  Manufacturer=ESTKme Technology Limited
//	S:  Product=ESTKme-RED
//	S:  SerialNumber=2051315E5056
func listUSBIdentitiesFromDebugfs() []USBIdentity {
	f, err := os.Open(usbDebugDevicesFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []USBIdentity
	var cur *USBIdentity
	flush := func() {
		if cur != nil && cur.VendorID != "" && cur.ProductID != "" {
			out = append(out, *cur)
		}
		cur = nil
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "T:"):
			flush()
			cur = &USBIdentity{}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "P:"):
			for _, field := range strings.Fields(line) {
				if v, ok := strings.CutPrefix(field, "Vendor="); ok {
					cur.VendorID = strings.ToLower(v)
				}
				if v, ok := strings.CutPrefix(field, "ProdID="); ok {
					cur.ProductID = strings.ToLower(v)
				}
			}
		case strings.HasPrefix(line, "S:"):
			if v, ok := strings.CutPrefix(line, "S:  Manufacturer="); ok {
				cur.Manufacturer = strings.TrimSpace(v)
			} else if v, ok := strings.CutPrefix(line, "S:  Product="); ok {
				cur.Product = strings.TrimSpace(v)
			} else if v, ok := strings.CutPrefix(line, "S:  SerialNumber="); ok {
				cur.Serial = strings.TrimSpace(v)
			}
		}
	}
	flush()
	return out
}

var pcscReaderSerialRe = regexp.MustCompile(`\(([^)]+)\)`)

// pcscReaderSerial 从 pcscd 读卡器名称中提取序列号，
// 例如 "Generic Smart Card Reader Interface (2051315E5056) 00 00" → "2051315E5056"
func pcscReaderSerial(reader string) string {
	m := pcscReaderSerialRe.FindStringSubmatch(reader)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// ResolvePCSCReaderDisplayName 通过序列号把 pcscd 读卡器匹配到 USB 设备，
// 返回 "Product · Manufacturer · vid:pid" 格式的可读名称；匹配失败返回空串。
func ResolvePCSCReaderDisplayName(reader string, identities []USBIdentity) string {
	product, manufacturer, vid, pid := ResolvePCSCReaderUSBInfo(reader, identities)
	if product == "" && manufacturer == "" && vid == "" {
		return ""
	}
	parts := make([]string, 0, 3)
	if product != "" {
		parts = append(parts, product)
	}
	if manufacturer != "" {
		parts = append(parts, manufacturer)
	}
	if vid != "" && pid != "" {
		parts = append(parts, vid+":"+pid)
	}
	return strings.Join(parts, " · ")
}

// ResolvePCSCReaderUSBInfo 通过序列号把 pcscd 读卡器匹配到 USB 设备，
// 返回结构化的 product, manufacturer, vid, pid（全为字符串小写形式）；
// 匹配失败返回空值。
func ResolvePCSCReaderUSBInfo(reader string, identities []USBIdentity) (product, manufacturer, vid, pid string) {
	serial := pcscReaderSerial(reader)
	if serial == "" {
		return
	}
	for _, id := range identities {
		if id.Serial != "" && strings.EqualFold(id.Serial, serial) {
			product = id.Product
			manufacturer = id.Manufacturer
			vid = id.VendorID
			pid = id.ProductID
			return
		}
	}
	return
}
