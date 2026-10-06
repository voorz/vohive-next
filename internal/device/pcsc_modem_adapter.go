package device

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/internal/esim"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vowifi-core/runtimehost"
	"github.com/voorz/vowifi-core/runtimehost/identity"
)

// PC/SC APDU 常量 (ISO 7816 / 3GPP TS 31.102)
var (
	usimAID = []byte{0xA0, 0x00, 0x00, 0x00, 0x87, 0x10, 0x02}
	efIMSI  = []byte{0x6F, 0x07}
	efICCID = []byte{0x2F, 0xE2}
	efAD    = []byte{0x6F, 0xAD}
	efGID1  = []byte{0x6F, 0x3E} // Group Identifier 1 (3GPP TS 31.102 §4.2.6)
	efGID2  = []byte{0x6F, 0x3F} // Group Identifier 2 (3GPP TS 31.102 §4.2.7)
	efMF    = []byte{0x3F, 0x00}
)

// pcscModemAdapter 通过 PC/SC 读卡器实现 vowifihost.Modem 接口。
// 用于 PC/SC 设备的 VoWiFi 启动流程，使 AKA 认证可通过 PC/SC 通道执行。
type pcscModemAdapter struct {
	deviceID  string
	usbPath   string // USB 路径，运行时匹配 reader 字符串
	sn        string // 读卡器 SN（正规设备 USB 路径匹配失败时回退匹配）
	channel   *esim.PCSCExclusiveChannel
	connected bool
	accessMu  *sync.Mutex // 可选：与 eSIM 管理器共享的读卡器访问锁
}

var _ vowifihost.Modem = (*pcscModemAdapter)(nil)

func newPCSCModemAdapter(deviceID, usbPath, sn string, mu *sync.Mutex) (*pcscModemAdapter, error) {
	readerName := esim.ResolveReaderByUSBPath(usbPath, sn)
	if readerName == "" {
		return nil, fmt.Errorf("[%s] 未找到 USB 路径 %s 或 SN %s 的读卡器", deviceID, usbPath, sn)
	}
	var ch *esim.PCSCExclusiveChannel
	var err error
	if mu != nil {
		// VoWiFi USIM 访问用共享模式 (ShareShared + ProtocolAny)
		ch, err = esim.NewPCSCSharedChannelWithMutex(readerName, usbPath, mu)
	} else {
		ch, err = esim.NewPCSCSharedChannel(readerName, usbPath)
	}
	if err != nil {
		return nil, fmt.Errorf("创建 PC/SC 通道失败: %w", err)
	}
	return &pcscModemAdapter{
		deviceID: deviceID,
		usbPath:  usbPath,
		sn:       sn,
		channel:  ch,
		accessMu: mu,
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
	if a.connected && a.channel != nil && !a.channel.IsClosed() {
		return nil
	}
	// 通道已关闭（如上次 AKA 完成后自动断开），需要重建
	if a.channel == nil || a.channel.IsClosed() {
		// 用 USB 路径优先匹配，失败时按 SN 回退匹配（可能已切换驱动模式或换接口）
		readerName := esim.ResolveReaderByUSBPath(a.usbPath, a.sn)
		if readerName == "" {
			return fmt.Errorf("[%s] 未找到 USB 路径 %s 或 SN %s 的读卡器", a.deviceID, a.usbPath, a.sn)
		}
		var ch *esim.PCSCExclusiveChannel
		var err error
		if a.accessMu != nil {
			ch, err = esim.NewPCSCSharedChannelWithMutex(readerName, a.usbPath, a.accessMu)
		} else {
			ch, err = esim.NewPCSCSharedChannel(readerName, a.usbPath)
		}
		if err != nil {
			return fmt.Errorf("重建 PC/SC 通道失败: %w", err)
		}
		a.channel = ch
		a.connected = false
	}
	if err := a.channel.Connect(); err != nil {
		return fmt.Errorf("PC/SC 连接失败: %w", err)
	}
	a.connected = true
	logger.Info(fmt.Sprintf("[%s] PC/SC 读卡器已连接 (usb_path: %s)", a.deviceID, a.usbPath))
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

// ResolveLogicalChannelAID 返回 USIM/ISIM 的完整 AID。
// PC/SC 模式下通过 EF_DIR 发现完整 AID（eUICC 上标准 7 字节 AID 不够）。
func (a *pcscModemAdapter) ResolveLogicalChannelAID(app string, fallbackAID string) (string, string, error) {
	app = strings.TrimSpace(strings.ToLower(app))
	if err := a.ensureConnected(); err != nil {
		logger.Warn(fmt.Sprintf("[%s] ResolveLogicalChannelAID: 连接失败，使用 fallback: %v", a.deviceID, err))
		return fallbackAID, "pcsc_fallback", nil // 连接失败则退回 fallback
	}

	// 确定 AID 前缀
	var prefix []byte
	switch app {
	case "usim":
		prefix = []byte{0xA0, 0x00, 0x00, 0x00, 0x87, 0x10, 0x02}
	case "isim":
		prefix = []byte{0xA0, 0x00, 0x00, 0x00, 0x87, 0x10, 0x11}
	default:
		return fallbackAID, "pcsc_fallback", nil
	}

	// 从 EF_DIR 发现完整 AID
	aidHex, err := a.discoverAIDFromEFDIR(prefix)
	if err != nil {
		logger.Warn(fmt.Sprintf("[%s] EF_DIR 发现 %s AID 失败，使用 fallback", a.deviceID, app), "err", err)
		return fallbackAID, "pcsc_fallback", nil
	}
	logger.Info(fmt.Sprintf("[%s] EF_DIR 发现 %s 完整 AID: %s", a.deviceID, app, aidHex))
	return aidHex, "pcsc_efdir", nil
}

// discoverAIDFromEFDIR 从 EF_DIR (0x2F00) 扫描记录，返回匹配 prefix 的完整 AID (hex)。
func (a *pcscModemAdapter) discoverAIDFromEFDIR(prefix []byte) (string, error) {
	// SELECT MF by FID
	resp, err := a.transmitOnChannel(0, selectByFIDCmd(efMF))
	if err != nil || checkSW(resp) != nil {
		return "", fmt.Errorf("SELECT MF 失败: %w", err)
	}
	// SELECT EF_DIR (0x2F00) by FID, P2=04 (FCI), Le=00
	// transmitOnChannel 自动处理 61XX (GET RESPONSE)
	resp, err = a.transmitOnChannel(0, []byte{0x00, 0xA4, 0x00, 0x04, 0x02, 0x2F, 0x00, 0x00})
	if err != nil || checkSW(resp) != nil || len(resp) < 8 {
		return "", fmt.Errorf("SELECT EF_DIR 失败: err=%v resp=%X", err, resp)
	}
	recLen := resp[7]
	if recLen == 0 {
		return "", fmt.Errorf("EF_DIR record length = 0")
	}

	// 扫描 EF_DIR 记录
	for rec := 1; rec <= 10; rec++ {
		cmd := []byte{0x00, 0xB2, byte(rec), 0x04, recLen}
		resp, err = a.transmitOnChannel(0, cmd)
		if err != nil || checkSW(resp) != nil || len(resp) < 5 {
			break
		}
		if resp[0] != 0x61 || resp[2] != 0x4F {
			break
		}
		aidLen := int(resp[3])
		if aidLen > 0 && len(resp) >= 4+aidLen {
			aid := resp[4 : 4+aidLen]
			if bytes.HasPrefix(aid, prefix) {
				return strings.ToUpper(hex.EncodeToString(aid)), nil
			}
		}
	}
	return "", fmt.Errorf("EF_DIR 未发现匹配前缀 % X 的应用", prefix)
}

func (a *pcscModemAdapter) CloseLogicalChannel(channel int) error {
	err := a.channel.CloseLogicalChannel(byte(channel))
	// AKA 认证完成后（defer CloseLogicalChannel）立即断开 PC/SC 物理连接，
	// 释放 accessMu 锁，让 eSIM 操作可以访问读卡器。
	// 下次操作时 ensureConnected() 会检测通道已关闭并重建。
	if a.connected {
		_ = a.channel.Disconnect()
		a.connected = false
	}
	return err
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

func (a *pcscModemAdapter) GetISIMIdentity() (vowifihost.IdentityProfile, error) {
	return vowifihost.IdentityProfile{}, fmt.Errorf("ISIM not available, using USIM fallback")
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

// transmitOnChannel 在指定逻辑通道上发送 APDU（通道号编码进 CLA）。
func (a *pcscModemAdapter) transmitOnChannel(channel byte, apdu []byte) ([]byte, error) {
	if err := a.ensureConnected(); err != nil {
		return nil, err
	}
	if len(apdu) == 0 {
		return nil, fmt.Errorf("空 APDU")
	}
	cla := apdu[0]
	if channel > 0 {
		if channel < 4 {
			apdu[0] = (cla & 0x9C) | channel
		} else if channel < 20 {
			apdu[0] = (cla & 0xB0) | 0x40 | (channel - 4)
		}
	}
	resp, err := a.channel.Transmit(apdu)
	if err != nil {
		return nil, err
	}
	// 处理 Wrong Le (SW1=0x6C): 用正确长度重试
	if len(resp) >= 2 && resp[len(resp)-2] == 0x6C && len(apdu) > 0 {
		apdu[len(apdu)-1] = resp[len(resp)-1]
		resp, err = a.channel.Transmit(apdu)
		if err != nil {
			return nil, err
		}
	}
	// 处理 GET RESPONSE (SW1=0x61)
	if len(resp) >= 2 && resp[len(resp)-2] == 0x61 {
		le := resp[len(resp)-1]
		getResp := []byte{0x00, 0xC0, 0x00, 0x00, le}
		if channel > 0 {
			if channel < 4 {
				getResp[0] = channel
			} else if channel < 20 {
				getResp[0] = 0x40 | (channel - 4)
			}
		}
		resp2, err := a.channel.Transmit(getResp)
		if err != nil {
			return nil, err
		}
		return resp2, nil
	}
	return resp, nil
}

// selectByFIDCmd 构建 SELECT (by file ID) APDU。
// P2=0x04: 通过文件 ID 选择并返回 FCI（ISO 7816-4 标准）。
// 使用 0x04 而非 0x00 是因为部分 eUICC 读卡器/卡片组合在选中 AID 后
// 对 P2=0x00 返回 6B00（参数错误），P2=0x04 兼容性更好。
func selectByFIDCmd(fid []byte) []byte {
	cmd := make([]byte, 0, 7)
	cmd = append(cmd, 0x00, 0xA4, 0x00, 0x04, 0x02)
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

// openUSIMChannel 尝试多种方式打开 USIM 应用通道。
// 返回通道号（0 表示基本通道）和清理函数。
func (a *pcscModemAdapter) openUSIMChannel() (byte, func(), error) {
	// 方式 1: 基本通道 + SELECT AID (P1=04, P2=04) — vowifi-sms 验证可行
	cmd := append([]byte{0x00, 0xA4, 0x04, 0x04, byte(len(usimAID))}, usimAID...)
	resp, err := a.transmitOnChannel(0, cmd)
	if err == nil {
		// 处理 GET RESPONSE
		if len(resp) >= 2 && resp[len(resp)-2] == 0x61 {
			le := resp[len(resp)-1]
			resp, err = a.transmitOnChannel(0, []byte{0x00, 0xC0, 0x00, 0x00, le})
		}
		if err == nil && checkSW(resp) == nil {
			return 0, func() {}, nil
		}
	}
	logger.Warn(fmt.Sprintf("[%s] 基本通道 SELECT AID 失败 (%v, %X)，尝试 EF_DIR 发现", a.deviceID, err, resp))

	// 方式 2: EF_DIR 发现 — vowifi_gateway 验证可行
	if ch, ok := a.openUSIMViaEFDIR(); ok {
		return ch, func() {
			if ch > 0 {
				_ = a.channel.CloseLogicalChannel(ch)
			}
		}, nil
	}

	// 方式 3: 逻辑通道 + 标准 USIM AID (fallback)
	ch, err := a.channel.OpenLogicalChannel(usimAID)
	if err == nil {
		return ch, func() { _ = a.channel.CloseLogicalChannel(ch) }, nil
	}

	return 0, nil, fmt.Errorf("打开 USIM 通道失败 (基本通道 AID: %v, EF_DIR: 无匹配, 逻辑通道: %v)", err, err)
}

// openUSIMViaEFDIR 通过 EF_DIR (0x2F00) 发现 USIM AID 并选择。
// 返回通道号（0 表示基本通道）和是否成功。
func (a *pcscModemAdapter) openUSIMViaEFDIR() (byte, bool) {
	// SELECT MF by FID
	resp, err := a.transmitOnChannel(0, selectByFIDCmd(efMF))
	if err != nil || checkSW(resp) != nil {
		logger.Warn(fmt.Sprintf("[%s] EF_DIR: SELECT MF 失败 (%v)", a.deviceID, err))
		return 0, false
	}
	// SELECT EF_DIR (0x2F00) by FID, P2=04 (FCI), Le=00
	resp, err = a.transmitOnChannel(0, []byte{0x00, 0xA4, 0x00, 0x04, 0x02, 0x2F, 0x00, 0x00})
	if err != nil || checkSW(resp) != nil || len(resp) < 8 {
		return 0, false
	}
	recLen := resp[7] // FCP 第 8 字节 = record length
	if recLen == 0 {
		return 0, false
	}

	// 扫描 EF_DIR 记录查找 USIM AID
	usimAIDPrefix := []byte{0xA0, 0x00, 0x00, 0x00, 0x87, 0x10, 0x02}
	var foundAID []byte
	for rec := 1; rec <= 10; rec++ {
		cmd := []byte{0x00, 0xB2, byte(rec), 0x04, recLen}
		resp, err = a.transmitOnChannel(0, cmd)
		if err != nil || checkSW(resp) != nil || len(resp) < 5 {
			break
		}
		// 记录格式: 61 <len> 4F <aidlen> <AID...> [50 <len> label]
		if resp[0] != 0x61 || resp[2] != 0x4F {
			break
		}
		aidLen := int(resp[3])
		if aidLen > 0 && len(resp) >= 4+aidLen {
			aid := resp[4 : 4+aidLen]
			if bytes.HasPrefix(aid, usimAIDPrefix) {
				foundAID = aid
				break
			}
		}
	}

	if foundAID == nil {
		logger.Warn(fmt.Sprintf("[%s] EF_DIR: 未找到 USIM AID", a.deviceID))
		return 0, false
	}

	logger.Info(fmt.Sprintf("[%s] EF_DIR 发现 USIM AID: % X", a.deviceID, foundAID))

	// SELECT AID (P1=04, P2=04)
	cmd := append([]byte{0x00, 0xA4, 0x04, 0x04, byte(len(foundAID))}, foundAID...)
	resp, err = a.transmitOnChannel(0, cmd)
	if err != nil {
		return 0, false
	}
	if len(resp) >= 2 && resp[len(resp)-2] == 0x61 {
		le := resp[len(resp)-1]
		_, _ = a.transmitOnChannel(0, []byte{0x00, 0xC0, 0x00, 0x00, le})
	}
	// 重新检查最终 SW
	if len(resp) >= 2 && resp[len(resp)-2] == 0x90 {
		return 0, true // 基本通道
	}
	return 0, false
}

// ReadIMSI 通过 PC/SC 读取 EF_IMSI 并解析 IMSI 字符串。
// 使用逻辑通道打开 USIM 应用（eUICC 上基本通道 SELECT ADF 可能返回 6A82）。
func (a *pcscModemAdapter) ReadIMSI() (string, error) {
	if err := a.ensureConnected(); err != nil {
		return "", err
	}
	ch, cleanup, err := a.openUSIMChannel()
	if err != nil {
		return "", fmt.Errorf("打开 USIM 逻辑通道失败: %w", err)
	}
	defer cleanup()

	// SELECT EF_IMSI
	resp, err := a.transmitOnChannel(ch, selectByFIDCmd(efIMSI))
	if err != nil {
		return "", fmt.Errorf("SELECT EF_IMSI 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("SELECT EF_IMSI: %w", err)
	}

	// READ BINARY
	resp, err = a.transmitOnChannel(ch, readBinaryCmd(0x10))
	if err != nil {
		return "", fmt.Errorf("READ EF_IMSI 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("READ EF_IMSI: %w", err)
	}

	data := resp[:len(resp)-2]
	imsi, _, _, _, perr := parseIMSI(data)
	if perr != nil {
		return "", perr
	}
	return imsi, nil
}

// ReadICCID 通过 PC/SC 读取 EF_ICCID 并解析 ICCID 字符串。
func (a *pcscModemAdapter) ReadICCID() (string, error) {
	// SELECT MF (回到根目录) — 基本通道
	resp, err := a.transmitOnChannel(0, selectByFIDCmd(efMF))
	if err != nil {
		return "", fmt.Errorf("SELECT MF 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("SELECT MF: %w", err)
	}

	// SELECT EF_ICCID
	resp, err = a.transmitOnChannel(0, selectByFIDCmd(efICCID))
	if err != nil {
		return "", fmt.Errorf("SELECT EF_ICCID 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("SELECT EF_ICCID: %w", err)
	}

	// READ BINARY (10 字节)
	resp, err = a.transmitOnChannel(0, readBinaryCmd(0x0A))
	if err != nil {
		return "", fmt.Errorf("READ EF_ICCID 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("READ EF_ICCID: %w", err)
	}

	data := resp[:len(resp)-2]
	return parseICCID(data), nil
}

// ReadMNCLength 通过 PC/SC 读取 EF_AD 并解析 MNC 长度。
func (a *pcscModemAdapter) ReadMNCLength() (int, error) {
	if err := a.ensureConnected(); err != nil {
		return 0, err
	}
	ch, cleanup, err := a.openUSIMChannel()
	if err != nil {
		return 0, fmt.Errorf("打开 USIM 逻辑通道失败: %w", err)
	}
	defer cleanup()

	// SELECT EF_AD
	resp, err := a.transmitOnChannel(ch, selectByFIDCmd(efAD))
	if err != nil {
		return 0, fmt.Errorf("SELECT EF_AD 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return 0, fmt.Errorf("SELECT EF_AD: %w", err)
	}

	// READ BINARY (4 字节)
	resp, err = a.transmitOnChannel(ch, readBinaryCmd(0x04))
	if err != nil {
		return 0, fmt.Errorf("READ EF_AD 失败: %w", err)
	}
	if err := checkSW(resp); err != nil {
		return 0, fmt.Errorf("READ EF_AD: %w", err)
	}

	data := resp[:len(resp)-2]
	if len(data) < 4 {
		return 2, nil
	}
	mncLen := int(data[3] & 0x0F)
	if mncLen != 2 && mncLen != 3 {
		mncLen = 2
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

// ReadGID1 通过 PC/SC 读取 EF_GID1 并返回 hex 字符串（去除尾部 0xFF 填充）。
func (a *pcscModemAdapter) ReadGID1() (string, error) {
	return a.readEFHex(efGID1, "EF_GID1")
}

// ReadGID2 通过 PC/SC 读取 EF_GID2 并返回 hex 字符串（去除尾部 0xFF 填充）。
func (a *pcscModemAdapter) ReadGID2() (string, error) {
	return a.readEFHex(efGID2, "EF_GID2")
}

// readEFHex 读取一个透明 EF 文件并返回去除 0xFF 填充后的 hex 字符串。
func (a *pcscModemAdapter) readEFHex(fid []byte, label string) (string, error) {
	if err := a.ensureConnected(); err != nil {
		return "", err
	}
	ch, cleanup, err := a.openUSIMChannel()
	if err != nil {
		return "", fmt.Errorf("打开 USIM 逻辑通道失败: %w", err)
	}
	defer cleanup()

	resp, err := a.transmitOnChannel(ch, selectByFIDCmd(fid))
	if err != nil {
		return "", fmt.Errorf("SELECT %s 失败: %w", label, err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("SELECT %s: %w", label, err)
	}

	// READ BINARY (最多 32 字节，GID 通常 1-20 字节)
	resp, err = a.transmitOnChannel(ch, readBinaryCmd(0x20))
	if err != nil {
		return "", fmt.Errorf("READ %s 失败: %w", label, err)
	}
	if err := checkSW(resp); err != nil {
		return "", fmt.Errorf("READ %s: %w", label, err)
	}

	data := resp[:len(resp)-2] // 去掉 SW 字节
	// 去除尾部 0xFF 填充
	for len(data) > 0 && data[len(data)-1] == 0xFF {
		data = data[:len(data)-1]
	}
	if len(data) == 0 {
		return "", nil
	}
	return hex.EncodeToString(data), nil
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
