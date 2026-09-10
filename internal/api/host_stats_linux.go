//go:build linux

package api

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// probeHostStatic gathers the hardware identities shown on the dashboard host
// card. Every probe is best-effort: empty fields render as "—" in the SPA.
func probeHostStatic() hostStaticInfo {
	return hostStaticInfo{
		CPUModel:    readHostCPUModel(),
		BoardModel:  readHostBoardModel(),
		MemoryModel: readHostMemoryModel(),
		DiskModel:   readHostDiskModel(),
	}
}

// readHostCPUModel prefers the x86-style "model name"; on ARM hosts it composes
// the device-tree SoC with the core count and Cortex part name.
func readHostCPUModel() string {
	cpuinfo, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		if model := parseCPUInfoModel(string(cpuinfo)); model != "" {
			return model
		}
		part, processors := parseCPUInfoPart(string(cpuinfo))
		if processors == 0 {
			processors = runtime.NumCPU()
		}
		soc := ""
		if compatible, err := os.ReadFile("/proc/device-tree/compatible"); err == nil {
			soc = parseCompatibleSoC(string(compatible))
		}
		if model := composeARMCPUModel(soc, part, processors); model != "" {
			return model
		}
	}
	return runtime.GOARCH
}

// readHostBoardModel reads the device-tree model on ARM boards and the DMI
// board name on x86 machines.
func readHostBoardModel() string {
	if model, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		if text := strings.TrimSpace(strings.TrimRight(string(model), "\x00")); text != "" {
			return text
		}
	}
	dmiDir := "/sys/devices/virtual/dmi/id"
	board := readSysfsTrimmed(filepath.Join(dmiDir, "board_name"))
	vendor := readSysfsTrimmed(filepath.Join(dmiDir, "board_vendor"))
	if board != "" && !isPlaceholderDMI(board) {
		if vendor != "" && !isPlaceholderDMI(vendor) && !strings.Contains(strings.ToLower(board), strings.ToLower(vendor)) {
			return vendor + " " + board
		}
		return board
	}
	if product := readSysfsTrimmed(filepath.Join(dmiDir, "product_name")); product != "" && !isPlaceholderDMI(product) {
		return product
	}
	return ""
}

// isPlaceholderDMI filters the well-known "we never filled this in" DMI
// strings so they do not surface as board models.
func isPlaceholderDMI(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default string", "to be filled by o.e.m.", "to be filled by o.e.m", "none", "unknown", "n/a", "not specified", "system manufacturer":
		return true
	}
	return false
}

// readHostMemoryModel reports the installed DIMM description when dmidecode is
// available (typical on x86 NAS/PC hosts) and falls back to total capacity,
// which is all an ARM board exposes.
func readHostMemoryModel() string {
	if path, err := exec.LookPath("dmidecode"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, path, "-t", "17").Output(); err == nil {
			if model := parseDmidecodeMemory(string(output)); model != "" {
				return model
			}
		}
	}
	if total, _, ok := readHostMemoryBytes(); ok {
		return formatHostBytes(float64(total))
	}
	return ""
}

// readHostDiskModel describes physical block devices, skipping virtual ones
// (loop, zram, device-mapper, mtd, optical). Multiple disks join with "; ".
func readHostDiskModel() string {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return ""
	}
	var disks []string
	for _, entry := range entries {
		name := entry.Name()
		if skipHostDisk(name) {
			continue
		}
		base := filepath.Join("/sys/block", name)
		sizeText := readSysfsTrimmed(filepath.Join(base, "size"))
		sectors, ok := parseUint64String(sizeText)
		if !ok || sectors == 0 {
			// An empty card reader reports size 0 and tells us nothing.
			continue
		}
		model := readSysfsTrimmed(filepath.Join(base, "device", "model"))
		if model == "" {
			// MMC/SD cards carry the product name instead of a model string.
			model = readSysfsTrimmed(filepath.Join(base, "device", "name"))
		}
		if model == "" {
			model = name
		}
		capacity := formatHostBytes(float64(sectors) * 512)
		disks = append(disks, model+" · "+capacity)
	}
	return strings.Join(disks, "; ")
}

func readSysfsTrimmed(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(string(raw), "\x00"))
}

// readHostCPUTimes reads the aggregate counters from /proc/stat.
func readHostCPUTimes() (hostCPUTimes, bool) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return hostCPUTimes{}, false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "cpu ") {
			return parseCPUTimes(line)
		}
	}
	return hostCPUTimes{}, false
}

// readHostMemoryBytes returns MemTotal and MemAvailable in bytes.
func readHostMemoryBytes() (total, available uint64, ok bool) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	return parseMeminfo(string(raw))
}

// readHostMemory reports used/total bytes and the used percentage.
func readHostMemory() (percent float64, used, total uint64) {
	total, available, ok := readHostMemoryBytes()
	if !ok || total == 0 {
		return 0, 0, 0
	}
	used = total - available
	return clampPercent(float64(used) * 100 / float64(total)), used, total
}

// readHostDisk reports root filesystem usage the way df does: usable space is
// total minus reserved blocks, and the percentage is used/(used+available).
func readHostDisk() (percent float64, used, total uint64) {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err != nil || stat.Blocks == 0 {
		return 0, 0, 0
	}
	blockSize := uint64(stat.Bsize)
	total = stat.Blocks * blockSize
	free := stat.Bfree * blockSize
	available := stat.Bavail * blockSize
	used = total - free
	if denominator := used + available; denominator > 0 {
		percent = clampPercent(float64(used) * 100 / float64(denominator))
	}
	return percent, used, total
}

// readHostNetTotals sums rx/tx counters across physical host interfaces.
func readHostNetTotals() (rx, tx uint64, ok bool) {
	raw, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0, false
	}
	rx, tx = parseNetDevCounters(string(raw))
	return rx, tx, true
}
