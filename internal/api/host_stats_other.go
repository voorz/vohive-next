//go:build !linux

package api

// Host statistics are only meaningful on the Linux deployment target; on other
// platforms every probe reports empty/zero and the dashboard renders "—".

func probeHostStatic() hostStaticInfo { return hostStaticInfo{} }

func readHostCPUTimes() (hostCPUTimes, bool) { return hostCPUTimes{}, false }

func readHostNetTotals() (uint64, uint64, bool) { return 0, 0, false }

func readHostMemory() (float64, uint64, uint64) { return 0, 0, 0 }

func readHostDisk() (float64, uint64, uint64) { return 0, 0, 0 }
