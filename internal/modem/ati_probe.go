package modem

import (
	"errors"
	"strings"
	"time"

	"go.bug.st/serial"
)

// ProbeATIResult 包含 ATI 探测结果
type ProbeATIResult struct {
	Manufacturer string
	Model        string
	Revision     string
}

// ProbeATI 打开串口发送 ATI 命令，解析厂商/型号/固件版本。
// 用于扫描发现阶段在尚未创建 Manager 时获取模组身份信息。
func ProbeATI(atPort string, timeout time.Duration) (ProbeATIResult, error) {
	atPort = strings.TrimSpace(atPort)
	if atPort == "" {
		return ProbeATIResult{}, errors.New("empty at port")
	}
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	p, err := serial.Open(atPort, mode)
	if err != nil {
		return ProbeATIResult{}, err
	}
	defer p.Close()

	_ = p.SetReadTimeout(80 * time.Millisecond)

	deadline := time.Now().Add(timeout)
	buf := make([]byte, 1024)
	var acc strings.Builder

	write := func(s string) {
		_, _ = p.Write([]byte(s))
	}

	write("AT\r\n")
	time.Sleep(40 * time.Millisecond)
	write("ATI\r\n")

	for time.Now().Before(deadline) {
		n, rerr := p.Read(buf)
		if n > 0 {
			acc.Write(buf[:n])
			manu, mdl, rev, perr := parseATI(acc.String())
			if perr == nil && manu != "" {
				return ProbeATIResult{
					Manufacturer: manu,
					Model:        mdl,
					Revision:     rev,
				}, nil
			}
		}
		if rerr != nil {
			if strings.Contains(strings.ToLower(rerr.Error()), "timeout") {
				continue
			}
		}
	}

	manu, mdl, rev, perr := parseATI(acc.String())
	if perr == nil && (manu != "" || mdl != "" || rev != "") {
		return ProbeATIResult{
			Manufacturer: manu,
			Model:        mdl,
			Revision:     rev,
		}, nil
	}
	return ProbeATIResult{}, errors.New("ati probe timeout")
}
