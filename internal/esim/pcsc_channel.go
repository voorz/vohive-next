package esim

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ElMostafaIdrassi/goscard"
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

// 连接后发送的终端能力 APDU（与 lpac APDU_TERMINAL_CAPABILITIES 一致），
// 失败忽略（lpac 亦不检查其响应）。
var pcscTerminalCapabilitiesAPDU = []byte{
	0x80, 0xAA, 0x00, 0x00, 0x0A,
	0xA9, 0x08, 0x81, 0x00, 0x82, 0x01, 0x01, 0x83, 0x01, 0x07,
}

// goscardInitialize 封装 goscard 库级初始化/释放的引用计数，
// 与 uicc-go/ccid 的做法保持一致（全局只能 Initialize/Finalize 一次配对）。
var goscardInitialize struct {
	mu   sync.Mutex
	refs int
}

func acquireGoscard() error {
	goscardInitialize.mu.Lock()
	defer goscardInitialize.mu.Unlock()
	if goscardInitialize.refs == 0 {
		if err := goscard.Initialize(goscard.NewDefaultLogger(goscard.LogLevelNone)); err != nil {
			return fmt.Errorf("初始化 PC/SC 库失败: %w", err)
		}
	}
	goscardInitialize.refs++
	return nil
}

func releaseGoscard() {
	goscardInitialize.mu.Lock()
	defer goscardInitialize.mu.Unlock()
	if goscardInitialize.refs == 0 {
		return
	}
	goscardInitialize.refs--
	if goscardInitialize.refs == 0 {
		goscard.Finalize()
	}
}

// ListPCSCReaders 列出系统可用的 PC/SC 读卡器名称。
// 使用 goscard 库（与 PCSCExclusiveChannel 共享初始化），避免与 ccid/scard 库冲突。
func ListPCSCReaders() ([]string, error) {
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

// PCSCExclusiveChannel 是以独占模式访问读卡器的 driver.SmartCardChannel 实现。
type PCSCExclusiveChannel struct {
	mu     sync.Mutex
	reader string

	pcscAcquired bool
	ctx          *goscard.Context
	card         *goscard.Card
	ioSend       *goscard.SCardIORequest
	channel      byte
	connected    bool
	closed       bool

	accessMu *sync.Mutex // 可选：跨通道共享的读卡器访问互斥锁
	muHeld   bool        // 当前是否持有 accessMu

	// 连接参数
	shareMode   goscard.SCardShareMode
	protocol    goscard.SCardProtocol
	sendTermCap bool // 是否发送终端能力 APDU
}

// NewPCSCExclusiveChannel 创建指定读卡器的独占通道（此时尚未连接）。
func NewPCSCExclusiveChannel(reader string) (*PCSCExclusiveChannel, error) {
	if reader == "" {
		return nil, errors.New("PC/SC 独占通道需要指定读卡器名称")
	}
	return &PCSCExclusiveChannel{
		reader:      reader,
		shareMode:   goscard.SCardShareExclusive,
		protocol:    goscard.SCardProtocolT0,
		sendTermCap: true,
	}, nil
}

// NewPCSCSharedChannel 创建共享模式通道（用于 USIM 访问）。
// 使用 SCardShareShared + SCardProtocolAny，不发送终端能力 APDU。
func NewPCSCSharedChannel(reader string) (*PCSCExclusiveChannel, error) {
	if reader == "" {
		return nil, errors.New("PC/SC 通道需要指定读卡器名称")
	}
	return &PCSCExclusiveChannel{
		reader:      reader,
		shareMode:   goscard.SCardShareShared,
		protocol:    goscard.SCardProtocolAny,
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

	if err := acquireGoscard(); err != nil {
		c.releaseAccessMuLocked()
		return err
	}
	ctx, _, err := goscard.NewContext(goscard.SCardScopeSystem, nil, nil)
	if err != nil {
		releaseGoscard()
		return fmt.Errorf("创建 PC/SC 上下文失败: %w", err)
	}
	c.ctx = &ctx
	c.pcscAcquired = true

	card, _, err := ctx.Connect(c.reader, c.shareMode, c.protocol)
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
	c.card = &card
	c.ioSend = ioSend

	// 终端能力握手：与 lpac 行为一致，忽略响应与错误。
	if c.sendTermCap {
		_, _, _ = card.Transmit(ioSend, pcscTerminalCapabilitiesAPDU, nil)
	}

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

// releaseLocked 释放卡片与上下文（调用方需持有 c.mu）。
func (c *PCSCExclusiveChannel) releaseLocked() error {
	var errs []error
	if c.card != nil {
		if _, err := c.card.Disconnect(goscard.SCardUnpowerCard); err != nil {
			errs = append(errs, fmt.Errorf("断开卡片失败: %w", err))
		}
		c.card = nil
		c.ioSend = nil
	}
	if c.ctx != nil {
		if _, err := c.ctx.Release(); err != nil {
			errs = append(errs, fmt.Errorf("释放 PC/SC 上下文失败: %w", err))
		}
		c.ctx = nil
	}
	if c.pcscAcquired {
		c.pcscAcquired = false
		releaseGoscard()
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
	if c.card == nil || c.ioSend == nil {
		return nil, errors.New("PC/SC 通道未连接")
	}
	recv, _, err := c.card.Transmit(c.ioSend, command, nil)
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
	if c.card == nil || c.ioSend == nil {
		return 0, errors.New("PC/SC 通道未连接")
	}
	if len(AID) > pcscMaxShortAPDUDataLength {
		return 0, fmt.Errorf("AID 长度 %d 超出短 APDU 限制", len(AID))
	}

	// MANAGE CHANNEL open
	resp, _, err := c.card.Transmit(c.ioSend, []byte{0x00, 0x70, 0x00, 0x00, 0x01}, nil)
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

	resp, _, err := c.card.Transmit(c.ioSend, command, nil)
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
	if c.card == nil || c.ioSend == nil {
		return errors.New("PC/SC 通道未连接")
	}
	return c.closeLogicalChannelLocked(channel)
}

func (c *PCSCExclusiveChannel) closeLogicalChannelLocked(channel byte) error {
	if channel == 0 || channel > pcscMaxLogicalChannel {
		return fmt.Errorf("无效逻辑通道号 %d", channel)
	}
	resp, _, err := c.card.Transmit(c.ioSend, []byte{0x00, 0x70, 0x80, channel, 0x00}, nil)
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
