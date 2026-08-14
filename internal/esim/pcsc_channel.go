package esim

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
)

// PCSCExclusiveChannel 是以独占模式访问读卡器的 driver.SmartCardChannel 实现。
// 底层使用 wwan-go/ccid 包，Linux 下走 USBFS 内置驱动，非 Linux 走 PC/SC (goscard)。
type PCSCExclusiveChannel struct {
	mu     sync.Mutex
	reader string

	ccidReader  *ccid.Reader
	channel     byte
	connected   bool
	closed      bool

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
// 多个通道共享同一 mutex 时，同一时刻只有一个通道能连接读卡器。
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

// IsClosed 返回通道是否已关闭（不可重用）。
func (c *PCSCExclusiveChannel) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// CurrentChannel 返回当前打开的逻辑通道号（0 表示无），供调用方做清理。
func (c *PCSCExclusiveChannel) CurrentChannel() byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.channel
}

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

	ctx := context.Background()
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
	c.connected = true
	return nil
}

func (c *PCSCExclusiveChannel) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	c.connected = false
	c.channel = 0
	err := c.releaseLocked()
	return err
}

// releaseAccessMuLocked 释放跨通道共享的读卡器访问锁（调用方需持有 c.mu）。
func (c *PCSCExclusiveChannel) releaseAccessMuLocked() {
	if c.muHeld && c.accessMu != nil {
		c.accessMu.Unlock()
		c.muHeld = false
	}
}

// ReleaseAccessMu 释放跨通道共享的读卡器访问锁，但不关闭通道。
// 用于 VoWiFi AKA 认证完成后提前释放锁，让 eSIM 操作可以并行访问读卡器。
// 通道仍保持 connected 状态，后续 Transmit 可正常使用（但需重新获取锁）。
func (c *PCSCExclusiveChannel) ReleaseAccessMu() {
	c.mu.Lock()
	c.releaseAccessMuLocked()
	c.mu.Unlock()
}

// releaseLocked 释放卡片与上下文（调用方需持有 c.mu）。
func (c *PCSCExclusiveChannel) releaseLocked() error {
	var errs []error
	if c.ccidReader != nil {
		if err := c.ccidReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("断开卡片失败: %w", err))
		}
		c.ccidReader = nil
	}
	c.releaseAccessMuLocked()
	return errors.Join(errs...)
}

func (c *PCSCExclusiveChannel) Transmit(command []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, errors.New("PC/SC 通道已关闭")
	}
	if c.ccidReader == nil {
		return nil, errors.New("PC/SC 通道未连接")
	}
	recv, err := c.ccidReader.Transmit(context.Background(), command)
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
	resp, err := c.ccidReader.Transmit(context.Background(), []byte{0x00, 0x70, 0x00, 0x00, 0x01})
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

func (c *PCSCExclusiveChannel) selectAIDLocked(channel byte, AID []byte) error {
	cla, err := pcscClassByteForChannel(0x00, channel)
	if err != nil {
		return err
	}
	command := make([]byte, 0, 5+len(AID))
	command = append(command, cla, 0xA4, 0x04, 0x00, byte(len(AID)))
	command = append(command, AID...)

	resp, err := c.ccidReader.Transmit(context.Background(), command)
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
	resp, err := c.ccidReader.Transmit(context.Background(), []byte{0x00, 0x70, 0x80, channel, 0x00})
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
// 底层使用 wwan-go/ccid 包，Linux 下通过 USBFS 枚举，非 Linux 通过 PC/SC 枚举。
func ListPCSCReaders() ([]string, error) {
	return ccid.ListReaders(context.Background())
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
