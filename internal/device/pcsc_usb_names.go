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
	SysPath      string // 完整 sysfs 路径，如 /sys/bus/usb/devices/1-1
	Manufacturer string
	Product      string
	Serial       string
	VendorID     string
	ProductID    string
	// 扩展信息（来自 debugfs D:/I:/E: 行）
	USBVersion  string           // 例如 "2.01"
	DeviceClass string           // 例如 "0b(scard)"
	Interfaces  []USBInterface  // 接口驱动列表
}

// USBInterface 描述一个 USB 接口的驱动和端点摘要
type USBInterface struct {
	Driver   string // 驱动名，如 "option"、"qmi_wwan"、"snd-usb-audio"；无驱动时为 "(none)"
	Class    string // 接口类，如 "ff(vend.)"、"0b(scard)"、"01(audio)"
	Endpoint string // 端点摘要，如 "Bulk In/Out + Interrupt"
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
		usbVer := readSysfsAttr(dir, "version")
		// sysfs 字符串描述符全部缺失（WSL usbip 场景）→ 视为不可用，交给 debugfs 回退
		if product == "" && manufacturer == "" && serial == "" {
			return nil
		}
		// 读取接口驱动信息
		interfaces := readSysfsInterfaces(dir)
		out = append(out, USBIdentity{
			SysPath:      dir,
			Manufacturer: manufacturer,
			Product:      product,
			Serial:       serial,
			VendorID:     strings.ToLower(vid),
			ProductID:    strings.ToLower(pid),
			USBVersion:   usbVer,
			Interfaces:   interfaces,
		})
	}
	return out
}

// readSysfsInterfaces 从 sysfs 读取 USB 设备的接口驱动列表。
// sysfs 中每个接口在 <dev>/<bus-port:cfg.iface>/ 目录中。
// 驱动名在 <iface>/driver 链接的目标 basename 中。
// 端点信息在 <iface>/ep_XX/type 文件中（Bulk/Interrupt/Isochronous）。
func readSysfsInterfaces(devDir string) []USBInterface {
	entries, err := os.ReadDir(devDir)
	if err != nil {
		return nil
	}
	var out []USBInterface
	for _, e := range entries {
		name := e.Name()
		// 接口目录名格式如 "1-2:1.0"（bus-port:config.interface）
		if !strings.Contains(name, ":") {
			continue
		}
		ifaceDir := filepath.Join(devDir, name)
		info, err := e.Info()
		if err != nil || !info.IsDir() {
			continue
		}
		var iface USBInterface
		iface.Class = readSysfsAttr(ifaceDir, "bInterfaceClass")
		if subClass := readSysfsAttr(ifaceDir, "bInterfaceSubClass"); subClass != "" {
			iface.Class += "(" + subClass + ")"
		}
		// driver 是一个符号链接
		driverLink := filepath.Join(ifaceDir, "driver")
		if target, err := os.Readlink(driverLink); err == nil {
			iface.Driver = filepath.Base(target)
		}
		if iface.Driver == "" {
			iface.Driver = "(none)"
		}
		// 读取端点信息
		iface.Endpoint = readSysfsEndpointSummary(ifaceDir)
		out = append(out, iface)
	}
	return out
}

// readSysfsEndpointSummary 从接口目录中读取端点类型并生成摘要。
func readSysfsEndpointSummary(ifaceDir string) string {
	entries, err := os.ReadDir(ifaceDir)
	if err != nil {
		return ""
	}
	var epTypes []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "ep_") {
			continue
		}
		epDir := filepath.Join(ifaceDir, name)
		info, err := e.Info()
		if err != nil || !info.IsDir() {
			continue
		}
		// type 文件值为 Bulk / Interrupt / Isochronous / Control
		epType := readSysfsAttr(epDir, "type")
		if epType != "" {
			epTypes = append(epTypes, epType)
		}
	}
	return summarizeEndpoints(epTypes)
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
	var curIface *USBInterface
	var epTypes []string // 收集当前接口的端点类型
	flush := func() {
		if cur != nil && cur.VendorID != "" && cur.ProductID != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	flushIface := func() {
		if curIface != nil && cur != nil {
			curIface.Endpoint = summarizeEndpoints(epTypes)
			cur.Interfaces = append(cur.Interfaces, *curIface)
		}
		curIface = nil
		epTypes = nil
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "T:"):
			flushIface()
			flush()
			cur = &USBIdentity{}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "D:"):
			// D:  Ver= 2.01 Cls=00(>ifc ) Sub=00 Prot=00 MxPS=64 #Cfgs=  1
			// 注意 Cls= 字段含括号，括号内可能有空格（如 "00(>ifc )"），
			// strings.Fields 会把它拆开，所以用正则提取
			if m := clsRe.FindStringSubmatch(line); m != nil {
				cur.DeviceClass = m[1]
			}
			if m := verRe.FindStringSubmatch(line); m != nil {
				cur.USBVersion = m[1]
			}
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
		case strings.HasPrefix(line, "I:"):
			// 新接口开始 → 先 flush 上一个
			flushIface()
			curIface = &USBInterface{}
			// I:* If#= 0 Alt= 0 #EPs= 3 Cls=0b(scard) Sub=00 Prot=00 Driver=(none)
			// 注意 Cls= 字段含括号，括号内可能有空格
			if m := clsRe.FindStringSubmatch(line); m != nil {
				curIface.Class = m[1]
			}
			if m := driverRe.FindStringSubmatch(line); m != nil {
				curIface.Driver = m[1]
			}
		case strings.HasPrefix(line, "E:"):
			// E:  Ad=81(I) Atr=02(Bulk) MxPS=  64 Ivl=0ms
			if i := strings.Index(line, "Atr="); i >= 0 {
				rest := line[i+4:]
				// 提取括号内类型，如 "Bulk"、"Int."、"Isoc"
				if j := strings.Index(rest, "("); j >= 0 {
					end := strings.Index(rest[j:], ")")
					if end > 0 {
						epTypes = append(epTypes, rest[j+1:j+end])
					}
				}
			}
		}
	}
	flushIface()
	flush()
	return out
}

// summarizeEndpoints 将端点类型列表摘要为可读字符串，
// 例如 ["Bulk", "Bulk", "Int."] → "Bulk In/Out + Interrupt"
func summarizeEndpoints(epTypes []string) string {
	if len(epTypes) == 0 {
		return ""
	}
	// 统计端点类型
	bulkCount := 0
	intCount := 0
	isocCount := 0
	for _, t := range epTypes {
		switch strings.ToLower(t) {
		case "bulk":
			bulkCount++
		case "int.", "int", "interrupt":
			intCount++
		case "isoc", "isochronous":
			isocCount++
		}
	}
	var parts []string
	if bulkCount > 0 {
		parts = append(parts, "Bulk In/Out")
	}
	if intCount > 0 {
		parts = append(parts, "Interrupt")
	}
	if isocCount > 0 {
		parts = append(parts, "Isoc")
	}
	return strings.Join(parts, " + ")
}

var pcscReaderSerialRe = regexp.MustCompile(`\(([^)]+)\)`)

// PcscReaderSerial 从 pcscd/USBFS 读卡器名称中提取序列号。
// 例如 "ESTKme-RED (2051315E5056) 00 00" → "2051315E5056"
// 仅用于设备发现页展示 SN，不用于匹配（匹配用 USBPath）。
func PcscReaderSerial(reader string) string {
	m := pcscReaderSerialRe.FindStringSubmatch(reader)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// ResolvePCSCReaderDisplayName 通过 USB 路径匹配 USB 设备，
// 返回 "Product · Manufacturer · vid:pid" 格式的可读名称；匹配失败返回空串。
func ResolvePCSCReaderDisplayName(usbPath string, identities []USBIdentity) string {
	product, manufacturer, vid, pid := ResolvePCSCReaderUSBInfo(usbPath, identities)
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

// debugfs 正则：匹配 Cls=xxx(yyy) 格式（括号内可能含空格）
var clsRe = regexp.MustCompile(`Cls=(\S+\([^)]*\))`)
var verRe = regexp.MustCompile(`Ver=(\S+)`)
var driverRe = regexp.MustCompile(`Driver=(\S+)`)

// ResolvePCSCReaderUSBInfo 通过 USB 路径匹配 USB 设备，
// 返回结构化的 product, manufacturer, vid, pid；匹配失败返回空值。
// usbPath 可以是完整 sysfs 路径（/sys/bus/usb/devices/1-1）或简短路径（1-1）。
func ResolvePCSCReaderUSBInfo(usbPath string, identities []USBIdentity) (product, manufacturer, vid, pid string) {
	usbPath = strings.TrimSpace(usbPath)
	if usbPath == "" {
		return
	}
	for _, id := range identities {
		if id.SysPath == usbPath || strings.HasSuffix(id.SysPath, "/"+usbPath) {
			product = id.Product
			manufacturer = id.Manufacturer
			vid = id.VendorID
			pid = id.ProductID
			return
		}
	}
	return
}

// ResolvePCSCReaderUSBDetail 通过 USB 路径匹配 USB 设备，
// 返回完整的 USBIdentity（含 USBVersion/DeviceClass/Interfaces 等扩展信息）。
func ResolvePCSCReaderUSBDetail(usbPath string, identities []USBIdentity) *USBIdentity {
	usbPath = strings.TrimSpace(usbPath)
	if usbPath == "" {
		return nil
	}
	for i := range identities {
		if identities[i].SysPath == usbPath || strings.HasSuffix(identities[i].SysPath, "/"+usbPath) {
			return &identities[i]
		}
	}
	return nil
}

// ResolveUSBIdentityByVIDPID 通过 VID:PID 匹配 USB 设备，返回完整 USBIdentity。
func ResolveUSBIdentityByVIDPID(vid, pid string, identities []USBIdentity) *USBIdentity {
	vid = strings.ToLower(strings.TrimSpace(vid))
	pid = strings.ToLower(strings.TrimSpace(pid))
	if vid == "" || pid == "" {
		return nil
	}
	for i := range identities {
		if strings.EqualFold(identities[i].VendorID, vid) && strings.EqualFold(identities[i].ProductID, pid) {
			return &identities[i]
		}
	}
	return nil
}

// FormatUSBInfoLine 格式化 USB 设备的 info 行，用于前端展示。
// 读卡器格式：SSN:2051315E5056;USB-2.01;Cls=0b(scard);（Bulk In/Out + Interrupt）
// 模组格式：option（AT/PPP）*4;qmi_wwan*1;snd-usb-audio*3
func FormatUSBInfoLine(id *USBIdentity, isReader bool) string {
	if id == nil {
		return ""
	}
	if isReader {
		var parts []string
		if id.USBVersion != "" {
			parts = append(parts, "USB-"+id.USBVersion)
		}
		// 读卡器设备类通常在接口层，不在 D: 行
		if len(id.Interfaces) > 0 && id.Interfaces[0].Class != "" {
			parts = append(parts, "Cls="+id.Interfaces[0].Class)
		} else if id.DeviceClass != "" {
			parts = append(parts, "Cls="+id.DeviceClass)
		}
		// 取第一个接口的端点摘要（读卡器通常只有一个接口）
		if len(id.Interfaces) > 0 && id.Interfaces[0].Endpoint != "" {
			parts = append(parts, "("+id.Interfaces[0].Endpoint+")")
		}
		return strings.Join(parts, ";")
	}
	// 模组：按驱动名分组计数
	if len(id.Interfaces) == 0 {
		return ""
	}
	driverCounts := map[string]int{}
	var order []string
	for _, iface := range id.Interfaces {
		d := iface.Driver
		if d == "" || d == "(none)" {
			d = "unknown"
		}
		if driverCounts[d] == 0 {
			order = append(order, d)
		}
		driverCounts[d]++
	}
	var parts []string
	for _, d := range order {
		cnt := driverCounts[d]
		parts = append(parts, d+"*"+itoa(cnt))
	}
	return strings.Join(parts, ";")
}

// itoa 简单整数转字符串（避免引入 strconv）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// 反转
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
