package device

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/damonto/euicc-go/driver/ccid"
	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vowifi-core/runtimehost"
	"github.com/voorz/vowifi-core/runtimehost/identity"
)

// PC/SC APDU 常量 (ISO 7816 / 3GPP TS 31.102)
var (
	usimAID        = []byte{0xA0, 0x00, 0x00, 0x00, 0x87, 0x10, 0x02}
	efIMSI         = []byte{0x6F, 0x07}
	efICCID        = []byte{0x2F, 0xE2}
	efAD           = []byte{0x6F, 0xAD}
	efMF           = []byte{0x3F, 0x00}
)

// pcscModemAdapter 通过 PC/SC 读卡器实现 runtimehost.Modem 接口。
// 用于 PC/SC 设备的 VoWiFi 启动流程，使 AKA 认证可通过 PC/SC 通道执行。
type pcscModemAdapter struct {
	deviceID   string
	readerName string
	channel    *ccid.CCIDReader
	connected  bool
}

var _ runtimehost.Modem = (*pcscModemAdapter)(nil)

func newPCSCModemAdapter(deviceID, readerName string) (*pcscModemAdapter, error) {
	ch, err := ccid.NewWithReader(readerName)
	if err != nil {
		return nil, fmt.Errorf("创建 PC/SC 通道失败: %w", err)
	}
	return &pcscModemAdapter{
		deviceID:   deviceID,
		readerName: readerName,
		channel:    ch,
	}, nil
}

func (a *pcscModemAdapter) DeviceID() string { return a.deviceID }

func (a *pcscModemAdapter) IsHealthy() bool {
	// PC/SC 读卡器只要连接成功即视为健康
	return a.connected
}

func (a *pcscModemAdapter) IsSimInserted() bool {
	// PC/SC 读卡器有卡才能 Connect，连接成功即视为卡已插入
	return a.connected
}

func (a *pcscModemAdapter) QuerySIMInserted() (bool, error) {
	return a.connected, nil
}

func (a *pcscModemAdapter) GetRegStatus() (int, string) {
	// PC/SC 设备无 modem 网络注册
	return 0, ""
}

func (a *pcscModemAdapter) ExecuteATSilent(cmd string, timeout time.Duration) (string, error) {
	return "", fmt.Errorf("PC/SC 设备不支持 AT 命令")
}

func (a *pcscModemAdapter) ensureConnected() error {
	if a.connected {
		return nil
	}
	if err := a.channel.Connect(); err != nil {
		return fmt.Errorf("PC/SC 连接失败: %w", err)
	}
	a.connected = true
	logger.Info(fmt.Sprintf("[%s] PC/SC 读卡器已连接 (reader: %s)", a.deviceID, a.readerName))
	return nil
}

func (a *pcscModemAdapter) OpenLogicalChannel(aid string) (int, error) {
	if err := a.ensureConnected(); err != nil {
		return 0, err
	}
	aidBytes, err := hex.DecodeString(aid)
	if err != nil {
		return 0, fmt.Errorf("无效的 AID hex: %w", err)
	}
	ch, err := a.channel.OpenLogicalChannel(aidBytes)
	if err != nil {
		return 0, fmt.Errorf("PC/SC 打开逻辑通道失败: %w", err)
	}
	return int(ch), nil
}

// ResolveLogicalChannelAID 返回 USIM/ISIM 的 AID。
// PC/SC 模式下直接使用 fallback AID，不做运行时探测。
func (a *pcscModemAdapter) ResolveLogicalChannelAID(app string, fallbackAID string) (string, string, error) {
	app = strings.TrimSpace(strings.ToLower(app))
	switch app {
	case "usim":
		return fallbackAID, "pcsc_fallback", nil
	case "isim":
		return fallbackAID, "pcsc_fallback", nil
	default:
		return fallbackAID, "pcsc_fallback", nil
	}
}

func (a *pcscModemAdapter) CloseLogicalChannel(channel int) error {
	return a.channel.CloseLogicalChannel(byte(channel))
}

func (a *pcscModemAdapter) TransmitAPDU(channel int, hexAPDU string) (string, error) {
	apdu, err := hex.DecodeString(hexAPDU)
	if err != nil {
		return "", fmt.Errorf("无效的 APDU hex: %w", err)
	}
	// 将逻辑通道号设置到 CLA 字节（ISO 7816 / euicc-go setChannelToCLA 逻辑）
	if len(apdu) > 0 {
		cla := apdu[0]
		ch := byte(channel)
		if ch > 0 {
			if ch < 4 {
				apdu[0] = (cla & 0x9C) | ch
			} else if ch < 20 {
				apdu[0] = (cla & 0xB0) | 0x40 | (ch - 4)
			}
		}
	}
	resp, err := a.channel.Transmit(apdu)
	if err != nil {
		return "", fmt.Errorf("PC/SC Transmit 失败: %w", err)
	}
	return hex.EncodeToString(resp), nil
}

func (a *pcscModemAdapter) GetISIMIdentity() (identity.Identity, error) {
	return identity.Identity{}, fmt.Errorf("ISIM not available, using USIM fallback")
}

func (a *pcscModemAdapter) GetNetworkMode() string {
	// PC/SC 设备无 modem 网络模式
	return ""
}

func (a *pcscModemAdapter) Stop() {
	if a.channel != nil && a.connected {
		_ = a.channel.Disconnect()
		a.connected = false
	}
}

// isPCSCDevice 判断 worker 是否为 PC/SC 读卡器设备。
func isPCSCDevice(w *Worker) bool {
	if w == nil {
		return false
	}
	return config.NormalizeESIMTransport(w.Config.ESIMTransport) == config.ESIMTransportPCSC
}

// --- PC/SC SIM 身份读取 (参照 3GPP TS 31.102 / vowifi-sms imsi.go) ---

// transmitRaw 通过基本通道直接发送 APDU 字节。
func (a *pcscModemAdapter) transmitRaw(apdu []byte) ([]byte, error) {
	if err := a.ensureConnected(); err != nil {
		return nil, err
	}
	resp, err := a.channel.Transmit(apdu)
	if err != nil {
		return nil, err
	}
	// 处理 GET RESPONSE (SW1=0x61)
	if len(resp) >= 2 && resp[len(resp)-2] == 0x61 {
		le := resp[len(resp)-1]
		getResp := []byte{0x00, 0xC0, 0x00, 0x00, le}
		resp2, err := a.channel.Transmit(getResp)
		if err != nil {
			return nil, err
		}
		return resp2, nil
	}
	return resp, nil
}

func selectByAIDCmd(aid []byte) []byte {
	cmd := make([]byte, 0, 5+len(aid))
	cmd = append(cmd, 0x00, 0xA4, 0x04, 0x00, byte(len(aid)))
	cmd = append(cmd, aid...)
	return cmd
}

func selectByFIDCmd(fid []byte) []byte {
	cmd := make([]byte, 0, 7)
	cmd = append(cmd, 0x00, 0xA4, 0x00, 0x00, 0x02)
	cmd = append(cmd, fid...)
	return cmd
}

func readBinaryCmd(le byte) []byte {
	return []byte{0x00, 0xB0, 0x00, 0x00, le}
}

func checkSW(resp []byte) error {
	if len(resp) < 2 {
		return fmt.Errorf("APDU 响应过短: %d 字节", len(resp))
	}
	sw1, sw2 := resp[len(resp)-2], resp[len(resp)-1]
	if sw1 != 0x90 || sw2 != 0x00 {
		return fmt.Errorf("APDU 错误 SW: %02X%02X", sw1, sw2)
	}
	return nil
}

// ReadIMSI 通过 PC/SC 读取 EF_IMSI 并解析 IMSI 字符串。
func (a *pcscModemAdapter) ReadIMSI() (string, error) {
	// SELECT ADF USIM
	resp, err := a.transmitRaw(selectByAIDCmd(usimAID))
	if err != nil {
		return "", fmt.Errorf("SELECT ADF USIM 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("SELECT ADF USIM: %w", err)
	}

	// SELECT EF_IMSI
	resp, err = a.transmitRaw(selectByFIDCmd(efIMSI))
	if err != nil {
		return "", fmt.Errorf("SELECT EF_IMSI 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("SELECT EF_IMSI: %w", err)
	}

	// READ BINARY
	resp, err = a.transmitRaw(readBinaryCmd(0x10))
	if err != nil {
		return "", fmt.Errorf("READ EF_IMSI 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("READ EF_IMSI: %w", err)
	}

	// resp 包含数据 + SW(9000)，裁剪最后 2 字节
	data := resp[:len(resp)-2]
	imsi, _, _, _, perr := parseIMSI(data)
	if perr != nil {
		return "", perr
	}
	return imsi, nil
}

// ReadICCID 通过 PC/SC 读取 EF_ICCID 并解析 ICCID 字符串。
func (a *pcscModemAdapter) ReadICCID() (string, error) {
	// SELECT MF (回到根目录)
	resp, err := a.transmitRaw(selectByFIDCmd(efMF))
	if err != nil {
		return "", fmt.Errorf("SELECT MF 失败: %w", err)
	}
	// MF 可能返回 61xx，transmitRaw 已处理 GET RESPONSE
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("SELECT MF: %w", err)
	}

	// SELECT EF_ICCID
	resp, err = a.transmitRaw(selectByFIDCmd(efICCID))
	if err != nil {
		return "", fmt.Errorf("SELECT EF_ICCID 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("SELECT EF_ICCID: %w", err)
	}

	// READ BINARY (10 字节)
	resp, err = a.transmitRaw(readBinaryCmd(0x0A))
	if err != nil {
		return "", fmt.Errorf("READ EF_ICCID 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("READ EF_ICCID: %w", err)
	}

	// resp 包含数据 + SW(9000)，裁剪最后 2 字节
	data := resp[:len(resp)-2]
	return parseICCID(data), nil
}

// ReadMNCLength 通过 PC/SC 读取 EF_AD 并解析 MNC 长度。
func (a *pcscModemAdapter) ReadMNCLength() (int, error) {
	// SELECT ADF USIM
	resp, err := a.transmitRaw(selectByAIDCmd(usimAID))
	if err != nil {
		return 0, fmt.Errorf("SELECT ADF USIM 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return 0, fmt.Errorf("SELECT ADF USIM: %w", err)
	}

	// SELECT EF_AD
	resp, err = a.transmitRaw(selectByFIDCmd(efAD))
	if err != nil {
		return 0, fmt.Errorf("SELECT EF_AD 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return 0, fmt.Errorf("SELECT EF_AD: %w", err)
	}

	// READ BINARY (4 字节)
	resp, err = a.transmitRaw(readBinaryCmd(0x04))
	if err != nil {
		return 0, fmt.Errorf("READ EF_AD 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return 0, fmt.Errorf("READ EF_AD: %w", err)
	}

	// resp 包含数据 + SW(9000)，裁剪最后 2 字节
	data := resp[:len(resp)-2]
	if len(data) < 4 {
		return 2, nil // 默认 2 位
	}
	// EF_AD 第 4 字节 (index 3) 低 4 位 = MNC 长度
	mncLen := int(data[3] & 0x0F)
	if mncLen != 2 && mncLen != 3 {
		mncLen = 2 // 默认
	}
	return mncLen, nil
}

// ReadSIMIdentity 一次性读取 IMSI、ICCID 并解析 MCC/MNC。
func (a *pcscModemAdapter) ReadSIMIdentity() (imsi, iccid, mcc, mnc string, err error) {
	imsi, err = a.ReadIMSI()
	if err != nil {
		return
	}
	iccid, err = a.ReadICCID()
	if err != nil {
		// ICCID 读取失败不阻断流程
		logger.Warn(fmt.Sprintf("[%s] PC/SC 读取 ICCID 失败，继续: %v", a.deviceID, err))
		iccid = ""
		err = nil
	}
	mncLen, mncErr := a.ReadMNCLength()
	if mncErr != nil {
		mncLen = 2 // 默认
	}
	_, mcc, mnc, _, _ = parseIMSIMCCMNC(imsi, mncLen)
	return
}

// parseIMSI 解析 EF_IMSI 原始字节为 IMSI/MCC/MNC/MSIN。
func parseIMSI(raw []byte) (imsi, mcc, mnc, msin string, err error) {
	if len(raw) < 2 {
		return "", "", "", "", fmt.Errorf("EF_IMSI 数据过短: %d", len(raw))
	}
	// 第 1 字节是长度（不包含自身），但我们直接解析所有 nibble
	nibs := make([]byte, 0, 15)
	for i := 1; i < len(raw); i++ {
		nibs = append(nibs, raw[i]&0x0F, (raw[i]>>4)&0x0F)
	}
	// 跳过第 1 个 nibble（奇偶位）
	if len(nibs) == 0 {
		return "", "", "", "", fmt.Errorf("IMSI nibbles 为空")
	}
	nibs = nibs[1:]
	digits := make([]byte, 0, len(nibs))
	for _, n := range nibs {
		if n == 0xF {
			break
		}
		digits = append(digits, '0'+n)
	}
	imsi = string(digits)
	if len(imsi) < 3 {
		return "", "", "", "", fmt.Errorf("IMSI 过短: %s", imsi)
	}
	return parseIMSIMCCMNC(imsi, 2)
}

// parseIMSIMCCMNC 从 IMSI 字符串解析 MCC/MNC/MSIN。
func parseIMSIMCCMNC(imsi string, mncLen int) (fullIMSI, mcc, mnc, msin string, err error) {
	if len(imsi) < 5 {
		return imsi, "", "", "", fmt.Errorf("IMSI 过短: %s", imsi)
	}
	if mncLen != 2 && mncLen != 3 {
		mncLen = 2
	}
	mcc = imsi[:3]
	mnc = imsi[3 : 3+mncLen]
	msin = imsi[3+mncLen:]
	return imsi, mcc, mnc, msin, nil
}

// parseICCID 解析 EF_ICCID 原始字节为 ICCID 字符串（BCD 交换 nibble）。
func parseICCID(raw []byte) string {
	var sb strings.Builder
	for i := 0; i < len(raw); i++ {
		lo := raw[i] & 0x0F
		hi := (raw[i] >> 4) & 0x0F
		if lo == 0xF {
			break
		}
		sb.WriteByte('0' + lo)
		if hi == 0xF {
			break
		}
		sb.WriteByte('0' + hi)
	}
	return sb.String()
}
