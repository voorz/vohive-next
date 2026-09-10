package api

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// hostStaticInfo describes hardware identities that do not change while the
// process runs, so they are probed once and cached.
type hostStaticInfo struct {
	CPUModel    string `json:"cpu_model"`
	BoardModel  string `json:"board_model"`
	MemoryModel string `json:"memory_model"`
	DiskModel   string `json:"disk_model"`
}

// hostPerfSnapshot is one rendered read of host utilization for the dashboard.
type hostPerfSnapshot struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryUsed    uint64  `json:"memory_used_bytes"`
	MemoryTotal   uint64  `json:"memory_total_bytes"`
	DiskPercent   float64 `json:"disk_percent"`
	DiskUsed      uint64  `json:"disk_used_bytes"`
	DiskTotal     uint64  `json:"disk_total_bytes"`
	NetRxBps      float64 `json:"net_rx_bps"`
	NetTxBps      float64 `json:"net_tx_bps"`
}

// hostCPUTimes is one cumulative /proc/stat reading: idle already includes
// iowait, total sums every other column (guest time is already folded into
// user/nice and therefore excluded).
type hostCPUTimes struct {
	idle  uint64
	total uint64
}

const (
	// hostStatsMinGap keeps back-to-back polls from dividing a handful of
	// jiffies by a few milliseconds; the previous rate is reused instead.
	hostStatsMinGap = 300 * time.Millisecond
	// hostStatsMaxGap mirrors liveNetMaxGap: a gap past this means the tab was
	// closed or the browser was hidden; re-baseline instead of averaging a
	// long dead interval.
	hostStatsMaxGap = 15 * time.Second
	// hostStatsFirstSample is how long the very first request blocks so CPU
	// and network readings have a real interval to average over. It must
	// exceed hostStatsMinGap so the re-read survives the min-gap guard below.
	hostStatsFirstSample = 400 * time.Millisecond
)

// hostStatsSampler derives live host utilization from cumulative kernel
// counters. Like liveNetTracker it is driven on demand by dashboard polling,
// so no background goroutine is required.
type hostStatsSampler struct {
	mu        sync.Mutex
	static    *hostStaticInfo
	sampledAt time.Time
	prevCPU   hostCPUTimes
	prevNetRx uint64
	prevNetTx uint64
	lastCPU   float64
	lastRxBps float64
	lastTxBps float64
}

func newHostStatsSampler() *hostStatsSampler {
	return &hostStatsSampler{}
}

// info returns the cached static hardware description, probing it on first use.
func (s *hostStatsSampler) info() hostStaticInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.static == nil {
		info := probeHostStatic()
		s.static = &info
	}
	return *s.static
}

// perf renders one utilization snapshot. CPU and network rates need a baseline,
// so the first-ever call takes a short inline second reading; later calls
// average against the previous request.
func (s *hostStatsSampler) perf() hostPerfSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cpu, cpuOK := readHostCPUTimes()
	rx, tx, netOK := readHostNetTotals()

	// No usable baseline yet (first request, or the tab was hidden past the
	// max gap): establish one, then re-read after a short interval so the
	// first dashboard paint already reports real numbers.
	needBaseline := s.sampledAt.IsZero() || now.Sub(s.sampledAt) > hostStatsMaxGap
	if needBaseline && (cpuOK || netOK) {
		s.sampledAt = now
		if cpuOK {
			s.prevCPU = cpu
		}
		if netOK {
			s.prevNetRx, s.prevNetTx = rx, tx
		}
		time.Sleep(hostStatsFirstSample)
		now = time.Now()
		if next, ok := readHostCPUTimes(); ok {
			cpu, cpuOK = next, true
		}
		if nextRx, nextTx, ok := readHostNetTotals(); ok {
			rx, tx, netOK = nextRx, nextTx, true
		}
	}

	memPercent, memUsed, memTotal := readHostMemory()
	diskPercent, diskUsed, diskTotal := readHostDisk()

	gap := now.Sub(s.sampledAt)
	if gap >= hostStatsMinGap && (cpuOK || netOK) {
		if cpuOK {
			if busyDelta, totalDelta := cpuDelta(s.prevCPU, cpu); totalDelta > 0 {
				s.lastCPU = clampPercent(float64(busyDelta) * 100 / float64(totalDelta))
			}
			s.prevCPU = cpu
		}
		if netOK {
			// Counter resets (interface flap) must not produce a giant spike.
			if rx >= s.prevNetRx {
				s.lastRxBps = float64(rx-s.prevNetRx) / gap.Seconds()
			} else {
				s.lastRxBps = 0
			}
			if tx >= s.prevNetTx {
				s.lastTxBps = float64(tx-s.prevNetTx) / gap.Seconds()
			} else {
				s.lastTxBps = 0
			}
			s.prevNetRx, s.prevNetTx = rx, tx
		}
		s.sampledAt = now
	}

	return hostPerfSnapshot{
		CPUPercent:    s.lastCPU,
		MemoryPercent: memPercent,
		MemoryUsed:    memUsed,
		MemoryTotal:   memTotal,
		DiskPercent:   diskPercent,
		DiskUsed:      diskUsed,
		DiskTotal:     diskTotal,
		NetRxBps:      s.lastRxBps,
		NetTxBps:      s.lastTxBps,
	}
}

// cpuDelta returns the busy and total jiffies elapsed between two cumulative
// readings. A backwards counter (theoretically impossible for /proc/stat)
// reports zero rather than wrapping.
func cpuDelta(prev, next hostCPUTimes) (busy, total uint64) {
	if next.total <= prev.total || next.idle < prev.idle {
		return 0, 0
	}
	totalDelta := next.total - prev.total
	idleDelta := next.idle - prev.idle
	if idleDelta >= totalDelta {
		return 0, totalDelta
	}
	return totalDelta - idleDelta, totalDelta
}

func clampPercent(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

// hostNetIgnoredPrefixes are virtual interface name prefixes whose counters
// would double-count physical traffic (bridges, tunnels, vocat's own links) or
// carry no real host traffic at all.
var hostNetIgnoredPrefixes = []string{
	"lo", "br-", "docker", "veth", "virbr", "vmnet", "vboxnet",
	"ip6tnl", "ip6gre", "sit", "gre", "gretap", "erspan",
	"tun", "tap", "utun", "vocat", "wg", "zt", "tailscale",
	"ifb", "bond", "vlan", "macvlan", "dummy", "lxc", "cali", "flannel", "cni",
}

// hostNetInterfaceCounted reports whether an interface's byte counters feed the
// host-level upload/download rates.
func hostNetInterfaceCounted(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, prefix := range hostNetIgnoredPrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

// parseNetDevCounters sums rx/tx bytes across counted interfaces in
// /proc/net/dev content.
func parseNetDevCounters(content string) (rx, tx uint64) {
	for _, line := range strings.Split(content, "\n") {
		name, rest, found := strings.Cut(line, ":")
		if !found || !hostNetInterfaceCounted(name) {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		rxBytes, okRx := parseUint64String(fields[0])
		txBytes, okTx := parseUint64String(fields[8])
		if !okRx || !okTx {
			continue
		}
		rx += rxBytes
		tx += txBytes
	}
	return rx, tx
}

func parseUint64String(text string) (uint64, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(text), 10, 64)
	return value, err == nil
}

// parseCPUTimes parses the aggregate "cpu" line of /proc/stat.
func parseCPUTimes(line string) (hostCPUTimes, bool) {
	fields := strings.Fields(line)
	// cpu user nice system idle iowait irq softirq steal [guest guest_nice]
	if len(fields) < 9 || fields[0] != "cpu" {
		return hostCPUTimes{}, false
	}
	var times hostCPUTimes
	for index, field := range fields[1:9] {
		value, ok := parseUint64String(field)
		if !ok {
			return hostCPUTimes{}, false
		}
		times.total += value
		if index == 3 || index == 4 { // idle + iowait
			times.idle += value
		}
	}
	return times, true
}

// parseMeminfo extracts MemTotal and MemAvailable (bytes) from /proc/meminfo.
func parseMeminfo(content string) (total, available uint64, ok bool) {
	for _, line := range strings.Split(content, "\n") {
		key, rest, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		var value uint64
		switch strings.TrimSpace(key) {
		case "MemTotal":
			value, ok = parseUint64String(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "kB")))
			if ok {
				total = value * 1024
			}
		case "MemAvailable":
			if value, parsed := parseUint64String(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "kB"))); parsed {
				available = value * 1024
			}
		}
	}
	return total, available, total > 0
}

// parseCPUInfoModel returns the x86-style "model name" from /proc/cpuinfo, or
// an empty string on ARM hosts that only carry CPU part numbers.
func parseCPUInfoModel(content string) string {
	for _, line := range strings.Split(content, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "model name", "Model", "Hardware":
			if model := strings.TrimSpace(value); model != "" {
				return model
			}
		}
	}
	return ""
}

// parseCPUInfoPart returns the first ARM "CPU part" hex identifier (e.g.
// 0xd03) and the number of processors listed.
func parseCPUInfoPart(content string) (part string, processors int) {
	for _, line := range strings.Split(content, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "processor":
			processors++
		case "CPU part":
			if part == "" {
				part = strings.ToLower(strings.TrimSpace(value))
			}
		}
	}
	return part, processors
}

// armCPUPartNames maps ARM CPU part identifiers to marketing core names.
var armCPUPartNames = map[string]string{
	"0xd03": "Cortex-A53",
	"0xd04": "Cortex-A35",
	"0xd05": "Cortex-A55",
	"0xd06": "Cortex-A65",
	"0xd07": "Cortex-A57",
	"0xd08": "Cortex-A72",
	"0xd09": "Cortex-A73",
	"0xd0a": "Cortex-A75",
	"0xd0b": "Cortex-A76",
	"0xd0c": "Neoverse-N1",
	"0xd0d": "Cortex-A77",
	"0xd0e": "Cortex-A76AE",
	"0xd40": "Neoverse-V1",
	"0xd41": "Cortex-A78",
	"0xd42": "Cortex-A78AE",
	"0xd44": "Cortex-X1",
	"0xd46": "Cortex-A510",
	"0xd47": "Cortex-A710",
	"0xd48": "Cortex-X2",
	"0xd4b": "Cortex-A715",
	"0xd4d": "Cortex-A520",
	"0xd4e": "Cortex-X3",
}

// socVendorNames prettifies the vendor half of a device-tree compatible entry.
var socVendorNames = map[string]string{
	"allwinner":   "Allwinner",
	"amlogic":     "Amlogic",
	"broadcom":    "Broadcom",
	"mediatek":    "MediaTek",
	"nvidia":      "NVIDIA",
	"qualcomm":    "Qualcomm",
	"raspberrypi": "Raspberry Pi",
	"rockchip":    "Rockchip",
	"samsung":     "Samsung",
	"ti":          "TI",
	"xunlong":     "Xunlong",
}

// parseCompatibleSoC extracts the SoC half of a device-tree compatible list
// (NUL-separated, most specific first): "xunlong,orangepi-zero3\0allwinner,
// sun50i-h618\0" yields "Allwinner sun50i-h618".
func parseCompatibleSoC(raw string) string {
	entries := strings.FieldsFunc(raw, func(r rune) bool { return r == 0 || r == '\n' })
	// The last entry is the least specific compatible, which on ARM boards is
	// the SoC rather than the board.
	for index := len(entries) - 1; index >= 0; index-- {
		entry := strings.TrimSpace(entries[index])
		vendor, soc, found := strings.Cut(entry, ",")
		if !found || soc == "" {
			continue
		}
		if pretty, ok := socVendorNames[strings.ToLower(vendor)]; ok {
			vendor = pretty
		} else {
			vendor = strings.ToUpper(vendor[:1]) + vendor[1:]
		}
		return vendor + " " + soc
	}
	return ""
}

// composeARMCPUModel renders e.g. "Allwinner sun50i-h618 · 4× Cortex-A53".
func composeARMCPUModel(soc, part string, processors int) string {
	core := armCPUPartNames[part]
	var result string
	switch {
	case soc != "" && core != "" && processors > 0:
		result = soc + " · " + strconv.Itoa(processors) + "× " + core
	case soc != "" && processors > 0:
		result = soc + " · " + strconv.Itoa(processors) + "× CPU"
	case soc != "" && core != "":
		result = soc + " · " + core
	default:
		result = soc
	}
	return result
}

// skipHostDisk reports whether a /sys/block entry is a virtual device whose
// "model" would only clutter the host card.
func skipHostDisk(name string) bool {
	for _, prefix := range []string{"loop", "ram", "zram", "sr", "nbd", "dm-", "md", "mtdblock", "ubi", "ubiblock"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// parseDmidecodeMemory extracts a compact "8 GB DDR4 M471A1K43CB1-CRC" style
// description from `dmidecode -t 17` output, preferring the first populated
// slot. Empty when no installed module can be described.
func parseDmidecodeMemory(output string) string {
	var size, memType, partNumber string
	flush := func() string {
		if size != "" && partNumber != "" {
			return strings.TrimSpace(size + " " + memType + " " + partNumber)
		}
		if size != "" && memType != "" {
			return strings.TrimSpace(size + " " + memType)
		}
		return ""
	}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "Memory Device") {
			if composed := flush(); composed != "" {
				return composed
			}
			size, memType, partNumber = "", "", ""
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "Size":
			if !strings.Contains(value, "No Module") && value != "" && value != "Unknown" {
				size = value
			}
		case "Type":
			if value != "Unknown" && value != "Other" && !strings.HasPrefix(value, "<OUT OF SPEC") {
				memType = value
			}
		case "Part Number":
			if value != "Unknown" && value != "None" && value != "" {
				partNumber = value
			}
		}
	}
	return flush()
}

// formatHostBytes mirrors the SPA's formatBytes so the live strings match the
// chart's formatting: 1024-based units, rounded once the value reaches 100.
func formatHostBytes(value float64) string {
	if value < 0 {
		value = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := value
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	if size >= 100 {
		return strconv.FormatFloat(size, 'f', 0, 64) + " " + units[unit]
	}
	return strconv.FormatFloat(size, 'f', 1, 64) + " " + units[unit]
}
