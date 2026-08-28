package esim

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
// 注意：原生 PC/SC (goscard) 驱动已移除（引入 purego/fakecgo 导致 ARM64 动态链接，
// 在无 ld-linux 的嵌入式设备上无法执行）。仅保留 USBFS 内置驱动。
const (
	PCSCTransportUSBFS = "usbfs" // 内置 USBFS 直连（不依赖 pcscd）
	PCSCTransportPCSC  = "pcsc"  // 原生 PC/SC 驱动（已移除，仅保留常量兼容配置）
)

// pcscTransportMode 控制全局使用哪条链路。
// 始终为 USBFS（0），因为原生 PC/SC 驱动已移除。
var pcscTransportMode atomic.Int32

// SetPcscTransport 设置全局 PC/SC 传输模式。
// 原生 PC/SC 驱动已移除，此函数仅保留兼容性，始终设置为 USBFS。
func SetPcscTransport(mode string) {
	// 忽略任何 pcscd/pcsc 模式，强制 USBFS
	pcscTransportMode.Store(0)
}

// PcscTransport 返回当前传输模式字符串。
func PcscTransport() string {
	return PCSCTransportUSBFS
}

func isPcscNativeMode() bool {
	return false
}

// --- PCSCExclusiveChannel ---

// PCSCExclusiveChannel 是以独占模式访问读卡器的 driver.SmartCardChannel 实现。
// 底层仅支持 USBFS 链路（通过 wwan-go/ccid 包内置驱动，Linux 专用）。
type PCSCExclusiveChannel struct {
	mu      sync.Mutex
	reader  string
	usbPath string // sysfs USB 路径，用于区分相同 name 的山寨读卡器

	// USBFS 链路
	ccidReader *ccid.Reader

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
// usbPath 用于区分相同 reader name 的山寨读卡器，可为空（正规设备）。
func NewPCSCExclusiveChannel(reader, usbPath string) (*PCSCExclusiveChannel, error) {
	if reader == "" {
		return nil, errors.New("PC/SC 独占通道需要指定读卡器名称")
	}
	return &PCSCExclusiveChannel{
		reader:      reader,
		usbPath:      usbPath,
		shareMode:   ccid.ShareExclusive,
		protocol:    ccid.ProtocolT0,
		sendTermCap: true,
	}, nil
}

// NewPCSCSharedChannel 创建共享模式通道（用于 USIM 访问）。
// 使用 ShareShared + ProtocolAny，不发送终端能力 APDU。
// usbPath 用于区分相同 reader name 的山寨读卡器，可为空（正规设备）。
func NewPCSCSharedChannel(reader, usbPath string) (*PCSCExclusiveChannel, error) {
	if reader == "" {
		return nil, errors.New("PC/SC 通道需要指定读卡器名称")
	}
	return &PCSCExclusiveChannel{
		reader:      reader,
		usbPath:      usbPath,
		shareMode:   ccid.ShareShared,
		protocol:    ccid.ProtocolAny,
		sendTermCap: false,
	}, nil
}

// NewPCSCExclusiveChannelWithMutex 创建带共享互斥锁的独占通道。
func NewPCSCExclusiveChannelWithMutex(reader, usbPath string, mu *sync.Mutex) (*PCSCExclusiveChannel, error) {
	ch, err := NewPCSCExclusiveChannel(reader, usbPath)
	if err != nil {
		return nil, err
	}
	ch.accessMu = mu
	return ch, nil
}

// NewPCSCSharedChannelWithMutex 创建带共享互斥锁的共享模式通道。
func NewPCSCSharedChannelWithMutex(reader, usbPath string, mu *sync.Mutex) (*PCSCExclusiveChannel, error) {
	ch, err := NewPCSCSharedChannel(reader, usbPath)
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

// Connect 连接到读卡器。仅使用 USBFS 链路。
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
		// ⚠️ 警告：USBPath 切勿删除！山寨读卡器 serial 相同（000000000001），
		// reader name 完全一样，不传 USBPath 会导致所有同名读卡器打开同一个 USB 设备，
		// APDU 通信串设备，读到相同的 EID 和 profiles。此修复切勿覆盖！
		USBPath: c.usbPath,
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
	if c.ccidReader == nil {
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
	if c.ccidReader == nil {
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
// 通过 wwan-go/ccid 枚举（USBFS 内置驱动）。
func ListPCSCReaders() ([]string, error) {
	return ccid.ListReaders(context.Background())
}

// ListPCSCReaderInfo 列出系统可用的 PC/SC 读卡器完整信息（含 USBPath）。
// 通过 wwan-go/ccid 枚举（含 USBPath）。
func ListPCSCReaderInfo() ([]ccid.ReaderInfo, error) {
	return ccid.ListReaderInfo(context.Background())
}

// extractSerialFromReaderName 从 PC/SC 读卡器名称中提取序列号（SN）。
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

// ResolveReaderByUSBPath 用 USB 路径和可选 SN 匹配当前模式下可用的读卡器名称。
// 匹配优先级：USB 路径 > SN（仅正规设备）。
// USB 路径是稳定标识（如 1-2），不受驱动模式影响，适用于所有读卡器。
// 当 USB 路径匹配失败时，若 SN 非空且非固定值（山寨读卡器常见 "000000000001"），
// 则回退到 SN 匹配——正规设备插拔换 USB 接口后 SN 不变，仍可正确解析 reader。
// 返回空串表示未匹配到可用读卡器。
func ResolveReaderByUSBPath(usbPath, sn string) string {
	usbPath = strings.TrimSpace(usbPath)
	sn = strings.TrimSpace(sn)
	infos, err := ListPCSCReaderInfo()
	if err != nil || len(infos) == 0 {
		return ""
	}
	// 优先按 USB 路径匹配（容错：支持完整路径和简短路径两种格式）
	if usbPath != "" {
		for _, info := range infos {
			if matchUSBPath(info.USBPath, usbPath) {
				return info.Name
			}
		}
	}
	// USB 路径匹配失败时，按 SN 回退匹配（排除山寨读卡器固定 SN）
	if sn != "" && !strings.Contains(sn, "000000000001") {
		for _, info := range infos {
			if info.USBSerial == sn {
				return info.Name
			}
		}
	}
	return ""
}

// matchUSBPath 容错匹配两个 USB 路径。
// 支持完整路径（/sys/bus/usb/devices/1-2）和简短路径（1-2）之间的互相匹配。
func matchUSBPath(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	// 一方是完整路径时，检查另一方是否是其后缀
	if strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a) {
		return true
	}
	return false
}

// --- 辅助函数 ---

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
