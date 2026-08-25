package esim

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ElMostafaIdrassi/goscard"
	"github.com/voorz/wwan-go/ccid"
)

// PC/SC 独占通道参数对齐 lpac（estkme-group/lpac driver/apdu/pcsc.c）：
// SCARD_SHARE_EXCLUSIVE + SCARD_PROTOCOL_T0 + 断开时 SCARD_UNPOWER_CARD。
// 原 uicc-go/ccid 使用 Shared + 任意协议 + LeaveCard，在部分 eUICC
// （如 eSTK.me 内置 G+D 卡）上 EnableProfile 恒定返回 SW=910B（eUICC busy），
// 而独占模式下同一卡片切换正常。

const (
	pcscMaxLogicalChannel      = 19
	pcscMaxShortAPDUDataLength = 255
	pcscAPDUTimeout            = 30 * time.Second // APDU 传输超时，防止卡片不响应时永久阻塞
)

// 终端能力 APDU（与 lpac APDU_TERMINAL_CAPABILITIES 一致），失败忽略。
var pcscTerminalCapabilitiesAPDU = []byte{
	0x80, 0xAA, 0x00, 0x00, 0x0A,
	0xA9, 0x08, 0x81, 0x00, 0x82, 0x01, 0x01, 0x83, 0x01, 0x07,
}

// --- 全局传输模式切换 ---

// PCSC 驱动模式常量。
const (
	PCSCTransportUSBFS = "usbfs" // 内置 USBFS 直连（不依赖 pcscd）
	PCSCTransportPCSC  = "pcsc"  // 原生 PC/SC 驱动（通过 pcscd + libccid）
)

// pcscTransportMode 控制全局使用哪条链路。
// 0 = PCSCTransportUSBFS（默认，内置驱动）
// 1 = PCSCTransportPCSC（原生驱动）
var pcscTransportMode atomic.Int32

// SetPcscTransport 设置全局 PC/SC 传输模式。
// mode: "usbfs"（默认）或 "pcsc"。
func SetPcscTransport(mode string) {
	switch mode {
	case PCSCTransportPCSC, "pcscd":
		pcscTransportMode.Store(1)
	default:
		pcscTransportMode.Store(0)
	}
}

// PcscTransport 返回当前传输模式字符串。
func PcscTransport() string {
	if pcscTransportMode.Load() == 1 {
		return PCSCTransportPCSC
	}
	return PCSCTransportUSBFS
}

func isPcscNativeMode() bool {
	return pcscTransportMode.Load() == 1
}

// --- goscard 引用计数 ---

var goscardInit struct {
	mu   sync.Mutex
	refs int
}

func acquireGoscard() error {
	goscardInit.mu.Lock()
	defer goscardInit.mu.Unlock()
	if goscardInit.refs == 0 {
		if err := goscard.Initialize(goscard.NewDefaultLogger(goscard.LogLevelNone)); err != nil {
			return fmt.Errorf("初始化 PC/SC 库失败: %w", err)
		}
	}
	goscardInit.refs++
	return nil
}

func releaseGoscard() {
	goscardInit.mu.Lock()
	defer goscardInit.mu.Unlock()
	if goscardInit.refs == 0 {
		return
	}
	goscardInit.refs--
	if goscardInit.refs == 0 {
		goscard.Finalize()
	}
}

// --- PCSCExclusiveChannel ---

// PCSCExclusiveChannel 是以独占模式访问读卡器的 driver.SmartCardChannel 实现。
// 底层支持两条链路：
//   - USBFS：通过 wwan-go/ccid 包内置驱动（Linux 专用）
//   - PC/SC：通过 goscard 库调用系统 pcscd 服务
//
// 运行时由全局 SetPcscTransport() 切换。
type PCSCExclusiveChannel struct {
	mu     sync.Mutex
	reader string

	// USBFS 链路
	ccidReader *ccid.Reader

	// PC/SC (goscard) 链路
	pcscAcquired bool
	gctx         *goscard.Context
	gcard        *goscard.Card
	gioSend      *goscard.SCardIORequest

	channel   byte
	connected bool
	closed    bool

	accessMu *sync.Mutex // 可选：跨通道共享的读卡器访问互斥锁
	muHeld   bool        // 当前是否持有 accessMu

	// 连接参数
	shareMode   ccid.ShareMode
	protocol    ccid.Protocol
	sendTermCap bool // 是否发送终端能力 APDU
}

// NewPCSCExclusiveChannel 创建指定读卡器的独占通道（此时尚未连接）。
func NewPCSCExclusiveChannel(reader string) (*PCSCExclusiveChannel, error) {
	if reader == "" {
		return nil, errors.New("PC/SC 独占通道需要指定读卡器名称")
	}
	return &PCSCExclusiveChannel{
		reader:      reader,
		shareMode:   ccid.ShareExclusive,
		protocol:    ccid.ProtocolT0,
		sendTermCap: true,
	}, nil
}

// NewPCSCSharedChannel 创建共享模式通道（用于 USIM 访问）。
// 使用 ShareShared + ProtocolAny，不发送终端能力 APDU。
func NewPCSCSharedChannel(reader string) (*PCSCExclusiveChannel, error) {
	if reader == "" {
		return nil, errors.New("PC/SC 通道需要指定读卡器名称")
	}
	return &PCSCExclusiveChannel{
		reader:      reader,
		shareMode:   ccid.ShareShared,
		protocol:    ccid.ProtocolAny,
		sendTermCap: false,
	}, nil
}

// NewPCSCExclusiveChannelWithMutex 创建带共享互斥锁的独占通道。
func NewPCSCExclusiveChannelWithMutex(reader string, mu *sync.Mutex) (*PCSCExclusiveChannel, error) {
	ch, err := NewPCSCExclusiveChannel(reader)
	if err != nil {
		return nil, err
	}
	ch.accessMu = mu
	return ch, nil
}

// NewPCSCSharedChannelWithMutex 创建带共享互斥锁的共享模式通道。
func NewPCSCSharedChannelWithMutex(reader string, mu *sync.Mutex) (*PCSCExclusiveChannel, error) {
	ch, err := NewPCSCSharedChannel(reader)
	if err != nil {
		return nil, err
	}
	ch.accessMu = mu
	return ch, nil
}

// IsClosed 返回通道是否已关闭。
func (c *PCSCExclusiveChannel) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// CurrentChannel 返回当前打开的逻辑通道号（0 表示无）。
func (c *PCSCExclusiveChannel) CurrentChannel() byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.channel
}

// Connect 连接到读卡器。根据全局传输模式选择 USBFS 或 PC/SC 链路。
func (c *PCSCExclusiveChannel) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("PC/SC 通道已关闭")
	}
	if c.connected {
		return nil
	}

	// 获取跨通道共享的读卡器访问锁
	if c.accessMu != nil {
		c.accessMu.Lock()
		c.muHeld = true
	}

	if isPcscNativeMode() {
		return c.connectPCSC()
	}
	return c.connectUSBFS()
}

// connectUSBFS 通过 wwan-go/ccid 包连接（内置 USBFS 驱动）。
func (c *PCSCExclusiveChannel) connectUSBFS() error {
	ctx, cancel := context.WithTimeout(context.Background(), pcscAPDUTimeout)
	defer cancel()
	r, err := ccid.OpenWithOptions(ctx, c.reader, ccid.OpenOptions{
		ShareMode:                c.shareMode,
		Protocol:                 c.protocol,
		SendTerminalCapabilities: c.sendTermCap,
	})
	if err != nil {
		c.releaseAccessMuLocked()
		return fmt.Errorf("连接读卡器 %q 失败: %w", c.reader, err)
	}
	c.ccidReader = r
	// ⚠️ 警告：此终端能力 APDU 补发不可省略！
	// ccid.OpenWithOptions 在 Linux 上忽略 OpenOptions（包括 SendTerminalCapabilities），
	// 因为 USBFS 连接本质上是独占的。但部分 eSIM 卡在不收到终端能力 APDU 时
	// 会对 STORE DATA (EnableProfile) 返回 6A81 (Function not supported)。
	// 此修复曾被后续改动意外覆盖导致 6A81 复现，请勿移除！
	if c.sendTermCap {
		_, _ = r.Transmit(ctx, pcscTerminalCapabilitiesAPDU)
	}
	c.connected = true
	return nil
}

// connectPCSC 通过 goscard 库连接（原生 PC/SC 驱动）。
func (c *PCSCExclusiveChannel) connectPCSC() error {
	if err := acquireGoscard(); err != nil {
		c.releaseAccessMuLocked()
		return err
	}
	ctx, _, err := goscard.NewContext(goscard.SCardScopeSystem, nil, nil)
	if err != nil {
		releaseGoscard()
		c.releaseAccessMuLocked()
		return fmt.Errorf("创建 PC/SC 上下文失败: %w", err)
	}
	c.gctx = &ctx
	c.pcscAcquired = true

	shareMode := goscard.SCardShareShared
	if c.shareMode == ccid.ShareExclusive {
		shareMode = goscard.SCardShareExclusive
	}
	var protocol goscard.SCardProtocol
	switch c.protocol {
	case ccid.ProtocolT0:
		protocol = goscard.SCardProtocolT0
	case ccid.ProtocolT1:
		protocol = goscard.SCardProtocolT1
	default:
		protocol = goscard.SCardProtocolAny
	}

	card, _, err := ctx.Connect(c.reader, shareMode, protocol)
	if err != nil {
		c.releaseLocked()
		return fmt.Errorf("连接读卡器 %q 失败: %w", c.reader, err)
	}
	ioSend, err := pcscIORequestForProtocol(card.ActiveProtocol())
	if err != nil {
		_, _ = card.Disconnect(goscard.SCardUnpowerCard)
		c.releaseLocked()
		return err
	}
	c.gcard = &card
	c.gioSend = ioSend

	// 终端能力握手：与 lpac 行为一致，忽略响应与错误。
	if c.sendTermCap {
		_, _, _ = card.Transmit(ioSend, pcscTerminalCapabilitiesAPDU, nil)
	}

	c.connected = true
	return nil
}

// Disconnect 断开与读卡器的连接。
func (c *PCSCExclusiveChannel) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	c.connected = false
	c.channel = 0
	return c.releaseLocked()
}

// releaseAccessMuLocked 释放跨通道共享的读卡器访问锁（调用方需持有 c.mu）。
func (c *PCSCExclusiveChannel) releaseAccessMuLocked() {
	if c.muHeld && c.accessMu != nil {
		c.accessMu.Unlock()
		c.muHeld = false
	}
}

// ReleaseAccessMu 释放跨通道共享的读卡器访问锁，但不关闭通道。
func (c *PCSCExclusiveChannel) ReleaseAccessMu() {
	c.mu.Lock()
	c.releaseAccessMuLocked()
	c.mu.Unlock()
}

// releaseLocked 释放卡片与上下文（调用方需持有 c.mu）。
func (c *PCSCExclusiveChannel) releaseLocked() error {
	var errs []error

	// 释放 PC/SC (goscard) 链路
	if c.gcard != nil {
		if _, err := c.gcard.Disconnect(goscard.SCardUnpowerCard); err != nil {
			errs = append(errs, fmt.Errorf("断开卡片失败: %w", err))
		}
		c.gcard = nil
		c.gioSend = nil
	}
	if c.gctx != nil {
		if _, err := c.gctx.Release(); err != nil {
			errs = append(errs, fmt.Errorf("释放 PC/SC 上下文失败: %w", err))
		}
		c.gctx = nil
	}
	if c.pcscAcquired {
		c.pcscAcquired = false
		releaseGoscard()
	}

	// 释放 USBFS (ccid) 链路
	if c.ccidReader != nil {
		if err := c.ccidReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("断开卡片失败: %w", err))
		}
		c.ccidReader = nil
	}

	c.releaseAccessMuLocked()
	return errors.Join(errs...)
}

// Transmit 发送 APDU 命令。
func (c *PCSCExclusiveChannel) Transmit(command []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, errors.New("PC/SC 通道已关闭")
	}

	var recv []byte
	var err error

	if c.ccidReader != nil {
		ctx, cancel := context.WithTimeout(context.Background(), pcscAPDUTimeout)
		defer cancel()
		recv, err = c.ccidReader.Transmit(ctx, command)
	} else if c.gcard != nil && c.gioSend != nil {
		recv, _, err = c.gcard.Transmit(c.gioSend, command, nil)
	} else {
		return nil, errors.New("PC/SC 通道未连接")
	}
	if err != nil {
		return nil, fmt.Errorf("发送 APDU %X 失败: %w", command, err)
	}
	// 对齐 lpac (euicc.c es10x_transmit_iter) 对 SW1=0x9X 的处理：
	// lpac 用 (sw1 & 0xF0) == 0x90 判定成功，因此 SW=91xx 被视为正常结束。
	// 但 euicc-go 的 Response.OK() 严格要求 SW==0x9000，会把 91xx 当错误抛出。
	// 部分 eUICC（如 eSTK.me G+D 卡）对 STORE DATA 返回 SW=910B，
	// 响应数据在 SW 之前，重写 SW 为 9000 可让 euicc-go 正常解析响应数据。
	if len(recv) >= 2 && recv[len(recv)-2] == 0x91 {
		recv[len(recv)-2] = 0x90
		recv[len(recv)-1] = 0x00
	}
	return recv, nil
}

// OpenLogicalChannel 打开逻辑通道并选择 AID。
func (c *PCSCExclusiveChannel) OpenLogicalChannel(AID []byte) (byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, errors.New("PC/SC 通道已关闭")
	}
	if c.ccidReader == nil && (c.gcard == nil || c.gioSend == nil) {
		return 0, errors.New("PC/SC 通道未连接")
	}
	if len(AID) > pcscMaxShortAPDUDataLength {
		return 0, fmt.Errorf("AID 长度 %d 超出短 APDU 限制", len(AID))
	}

	// MANAGE CHANNEL open
	resp, err := c.transmitLocked([]byte{0x00, 0x70, 0x00, 0x00, 0x01})
	if err != nil {
		return 0, fmt.Errorf("打开逻辑通道失败: %w", err)
	}
	if len(resp) < 3 || !pcscStatusOK(resp) {
		return 0, fmt.Errorf("打开逻辑通道返回异常: %X", resp)
	}
	channel := resp[0]
	if channel == 0 || channel > pcscMaxLogicalChannel {
		return 0, fmt.Errorf("打开逻辑通道返回无效通道号 %d", channel)
	}

	// SELECT AID（通道号编码进 CLA）
	if err := c.selectAIDLocked(channel, AID); err != nil {
		_ = c.closeLogicalChannelLocked(channel)
		return 0, err
	}
	c.channel = channel
	return channel, nil
}

// transmitLocked 在已持有 c.mu 的前提下发送 APDU。
func (c *PCSCExclusiveChannel) transmitLocked(command []byte) ([]byte, error) {
	if c.ccidReader != nil {
		ctx, cancel := context.WithTimeout(context.Background(), pcscAPDUTimeout)
		defer cancel()
		return c.ccidReader.Transmit(ctx, command)
	}
	if c.gcard != nil && c.gioSend != nil {
		recv, _, err := c.gcard.Transmit(c.gioSend, command, nil)
		return recv, err
	}
	return nil, errors.New("PC/SC 通道未连接")
}

func (c *PCSCExclusiveChannel) selectAIDLocked(channel byte, AID []byte) error {
	cla, err := pcscClassByteForChannel(0x00, channel)
	if err != nil {
		return err
	}
	command := make([]byte, 0, 5+len(AID))
	command = append(command, cla, 0xA4, 0x04, 0x00, byte(len(AID)))
	command = append(command, AID...)

	resp, err := c.transmitLocked(command)
	if err != nil {
		return fmt.Errorf("SELECT AID 失败: %w", err)
	}
	if len(resp) < 2 {
		return fmt.Errorf("SELECT AID 响应过短: %X", resp)
	}
	if !pcscStatusOK(resp) && !pcscStatusHasMore(resp) {
		return fmt.Errorf("SELECT AID 返回异常: %X", resp)
	}
	return nil
}

// CloseLogicalChannel 关闭指定的逻辑通道。
func (c *PCSCExclusiveChannel) CloseLogicalChannel(channel byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	if c.ccidReader == nil && (c.gcard == nil || c.gioSend == nil) {
		return errors.New("PC/SC 通道未连接")
	}
	return c.closeLogicalChannelLocked(channel)
}

func (c *PCSCExclusiveChannel) closeLogicalChannelLocked(channel byte) error {
	if channel == 0 || channel > pcscMaxLogicalChannel {
		return fmt.Errorf("无效逻辑通道号 %d", channel)
	}
	resp, err := c.transmitLocked([]byte{0x00, 0x70, 0x80, channel, 0x00})
	if err != nil {
		return fmt.Errorf("关闭逻辑通道失败: %w", err)
	}
	if len(resp) < 2 || !pcscStatusOK(resp) {
		return fmt.Errorf("关闭逻辑通道返回异常: %X", resp)
	}
	if c.channel == channel {
		c.channel = 0
	}
	return nil
}

// ListPCSCReaders 列出系统可用的 PC/SC 读卡器名称。
// USBFS 模式下通过 wwan-go/ccid 枚举，PC/SC 模式下通过 goscard 枚举。
func ListPCSCReaders() ([]string, error) {
	if isPcscNativeMode() {
		return listReadersPCSC()
	}
	return ccid.ListReaders(context.Background())
}

// ListPCSCReaderInfo 列出系统可用的 PC/SC 读卡器完整信息（含 USBPath）。
// USBFS 模式下通过 wwan-go/ccid 枚举（含 USBPath）。
// pcscd 模式下通过 goscard 枚举读卡器名称，同时从 sysfs 读取 USBPath 补充。
// sysfs 枚举只读 /sys/bus/usb/devices/ 下的属性文件，不连接 USB 设备，不与 pcscd 冲突。
func ListPCSCReaderInfo() ([]ccid.ReaderInfo, error) {
	if isPcscNativeMode() {
		names, err := listReadersPCSC()
		if err != nil {
			return nil, err
		}
		// 从 sysfs 枚举 CCID 设备获取 USBPath（只读 sysfs，不连接设备）。
		// goscard 返回的读卡器名称中包含 SN，通过 SN 关联 sysfs 的 USBPath。
		sysfsInfos, sysfsErr := ccid.ListReaderInfo(context.Background())
		infos := make([]ccid.ReaderInfo, 0, len(names))
		for _, name := range names {
			info := ccid.ReaderInfo{Name: name, ChannelAvailable: true, Transport: "pcsc"}
			if sysfsErr == nil {
				sn := extractSerialFromReaderName(name)
				if sn != "" {
					for _, si := range sysfsInfos {
						if si.USBSerial == sn {
							info.USBPath = si.USBPath
							info.USBSerial = si.USBSerial
							info.VendorID = si.VendorID
							info.ProductID = si.ProductID
							break
						}
					}
				}
			}
			infos = append(infos, info)
		}
		return infos, nil
	}
	return ccid.ListReaderInfo(context.Background())
}

// extractSerialFromReaderName 从 PC/SC 读卡器名称中提取序列号（SN）。
// goscard/pcscd 返回的名称格式通常为 "Product Name (SN) Slot Index"，
// 例如 "Generic Smart Card Reader Interface (2051315E5056) 00 00"。
func extractSerialFromReaderName(name string) string {
	start := strings.LastIndex(name, "(")
	if start < 0 {
		return ""
	}
	end := strings.Index(name[start:], ")")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(name[start+1 : start+end])
}

// ResolveReaderByUSBPath 用 USB 路径匹配当前模式下可用的读卡器名称。
// USB 路径是稳定标识（如 /sys/bus/usb/devices/1-1），不受驱动模式影响。
// 返回空串表示未匹配到可用读卡器。
func ResolveReaderByUSBPath(usbPath string) string {
	usbPath = strings.TrimSpace(usbPath)
	if usbPath == "" {
		return ""
	}
	infos, err := ListPCSCReaderInfo()
	if err != nil || len(infos) == 0 {
		return ""
	}
	for _, info := range infos {
		if info.USBPath == usbPath {
			return info.Name
		}
	}
	return ""
}

// listReadersPCSC 通过 goscard 枚举读卡器。
func listReadersPCSC() ([]string, error) {
	if err := acquireGoscard(); err != nil {
		return nil, err
	}
	defer releaseGoscard()
	ctx, _, err := goscard.NewContext(goscard.SCardScopeSystem, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 PC/SC 上下文失败: %w", err)
	}
	defer ctx.Release()
	readers, _, err := ctx.ListReaders(nil)
	return readers, err
}

// --- 辅助函数 ---

func pcscIORequestForProtocol(protocol goscard.SCardProtocol) (*goscard.SCardIORequest, error) {
	switch protocol {
	case goscard.SCardProtocolT0:
		return &goscard.SCardIoRequestT0, nil
	case goscard.SCardProtocolT1:
		return &goscard.SCardIoRequestT1, nil
	default:
		return nil, fmt.Errorf("不支持的 PC/SC 协议: %s", protocol.String())
	}
}

func pcscClassByteForChannel(cla, channel byte) (byte, error) {
	if channel < 4 {
		return (cla & 0x9C) | channel, nil
	}
	if channel <= pcscMaxLogicalChannel {
		return (cla & 0xB0) | 0x40 | (channel - 4), nil
	}
	return 0, fmt.Errorf("逻辑通道号 %d 超出上限 %d", channel, pcscMaxLogicalChannel)
}

func pcscStatusOK(response []byte) bool {
	return len(response) >= 2 && response[len(response)-2] == 0x90 && response[len(response)-1] == 0x00
}

func pcscStatusHasMore(response []byte) bool {
	return len(response) >= 2 && response[len(response)-2] == 0x61
}
