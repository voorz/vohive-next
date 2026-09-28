package esim

import (
	"errors"
	"strings"
)

// EUICCScanErrorCode 标识 eUICC 扫描失败的具体原因。
// 用于 EUICCAvailable() 三态判定和 UIM gate 决策。
type EUICCScanErrorCode string

const (
	// ScanErrCardNotPresent: 卡不在位（QMI/AT 返回 NO_CARD / card removed / Secure Element is not present）
	ScanErrCardNotPresent EUICCScanErrorCode = "CARD_NOT_PRESENT"

	// ScanErrCardUnsupported: 卡在位但不是 eUICC（所有候选 AID 均返回 6A82/6A88，
	// 表示 SELECT AID 失败——文件未找到或引用数据未找到）
	ScanErrCardUnsupported EUICCScanErrorCode = "CARD_UNSUPPORTED"

	// ScanErrEUICCFailure: eUICC 硬件故障（APDU 超时/通道拒绝/6985/设备不就绪/
	// MANAGE CHANNEL 失败但非 6A82 等），代表 eUICC 存在但无法正常通信
	ScanErrEUICCFailure EUICCScanErrorCode = "EUICC_FAILURE"

	// ScanErrBusy: 操作进行中或 APDU 仲裁冲突（ErrOperationInProgress）
	ScanErrBusy EUICCScanErrorCode = "BUSY"

	// ScanErrTransportUnavailable: 传输层不可用（QMI/MBIM/PCSC 未就绪或控制设备缺失）
	ScanErrTransportUnavailable EUICCScanErrorCode = "TRANSPORT_UNAVAILABLE"
)

// EUICCScanError 是 forEachEUICC / doForEachEUICC 在所有候选 AID 均失败时返回的结构化错误。
// 包含错误类别码、最后尝试的 AID（hex 字符串）和原始错误。
type EUICCScanError struct {
	Code EUICCScanErrorCode
	AID  string // 最后尝试的 AID（大写 hex），用于日志诊断
	Err  error  // 原始错误（来自 createLPAWithAID / client.EID / 回调）
}

func (e *EUICCScanError) Error() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("eUICC 扫描失败: ")
	b.WriteString(string(e.Code))
	if e.AID != "" {
		b.WriteString(" (AID=")
		b.WriteString(e.AID)
		b.WriteString(")")
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

func (e *EUICCScanError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// newScanError 根据原始错误构造 EUICCScanError。
// aidHex 是最后尝试的 AID 的 hex 字符串（用于日志）。
func newScanError(aidHex string, err error) *EUICCScanError {
	return &EUICCScanError{
		Code: classifyAIDError(err),
		AID:  aidHex,
		Err:  err,
	}
}

// classifyAIDError 根据单个 AID 的失败错误判定错误类别。
//
// 判定依据：
//   - ErrOperationInProgress → BUSY
//   - ErrQMITransportNotAvailable / ErrQMIControlDeviceMissing / ErrQMIUIMNotSupported
//     / ErrQMIUIMNotAvailable → TRANSPORT_UNAVAILABLE
//   - 错误消息包含 "select AID: 6A82" 或 "select AID: 6A88" → CARD_UNSUPPORTED
//   - 错误消息包含 "no card" / "card removed" / "Secure Element is not present"
//     / "NO_CARD" → CARD_NOT_PRESENT
//   - 超时错误 → EUICC_FAILURE
//   - 其他 → EUICC_FAILURE
func classifyAIDError(err error) EUICCScanErrorCode {
	if err == nil {
		return ScanErrEUICCFailure
	}

	// 1. 操作进行中
	if errors.Is(err, ErrOperationInProgress) {
		return ScanErrBusy
	}

	// 2. 传输层不可用
	if errors.Is(err, ErrQMITransportNotAvailable) ||
		errors.Is(err, ErrQMIControlDeviceMissing) ||
		errors.Is(err, ErrQMIUIMNotSupported) ||
		errors.Is(err, ErrQMIUIMNotAvailable) {
		return ScanErrTransportUnavailable
	}

	// 3. 卡片复位（eUICC 内部 RESET，属于预期行为，不归类为故障）
	if errors.Is(err, ErrQMIUIMCardReset) || errors.Is(err, ErrMBIMUICCInvalidChannel) {
		return ScanErrBusy
	}

	lower := strings.ToLower(err.Error())

	// 4. 卡不在位
	if strings.Contains(lower, "no card") ||
		strings.Contains(lower, "card removed") ||
		strings.Contains(lower, "secure element is not present") ||
		strings.Contains(lower, "no_card") ||
		strings.Contains(lower, "card_not_present") {
		return ScanErrCardNotPresent
	}

	// 5. SELECT AID 返回 6A82 (File not found) 或 6A88 (Referenced data not found)
	// euicc-go 的 selectAID 失败格式: "select AID: 6A82"
	// 也匹配 QMI/AT 层透传的原始 SW
	if strings.Contains(lower, "6a82") || strings.Contains(lower, "6a88") {
		return ScanErrCardUnsupported
	}

	// 5b. QMI UIM OpenLogicalChannel 返回 error=0x0050
	// 在 Quectel 模组上表示"指定的 AID 在当前卡上找不到对应的应用"，
	// 等同于 APDU 层面的 6A82，常见于物理 SIM 卡（无 ISD-R 应用）。
	// 错误格式: "QMI error: service=0x0b msg=0x0042 result=0x0001 error=0x0050"
	if strings.Contains(lower, "error=0x0050") {
		return ScanErrCardUnsupported
	}

	// 6. 超时
	if strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "deadline exceeded") ||
		strings.Contains(lower, "context deadline exceeded") {
		return ScanErrEUICCFailure
	}

	// 7. 条件不满足 (6985) — 卡片拒绝操作
	if strings.Contains(lower, "6985") {
		return ScanErrEUICCFailure
	}

	// 8. 默认归类为 eUICC 故障
	return ScanErrEUICCFailure
}

// IsScanTemporary 判断扫描错误是否为临时性（可通过重试解决）。
// 用于 EUICCAvailable() 决定是否阻止 VoWiFi 启动。
func IsScanTemporary(err error) bool {
	var scanErr *EUICCScanError
	if !errors.As(err, &scanErr) {
		return false
	}
	return scanErr.Code == ScanErrBusy || scanErr.Code == ScanErrTransportUnavailable
}

// IsScanCardAbsent 判断扫描错误是否因为卡不在位或不是 eUICC。
// 此类情况不应阻止 VoWiFi 启动（物理 SIM 卡也需要 VoWiFi）。
func IsScanCardAbsent(err error) bool {
	var scanErr *EUICCScanError
	if !errors.As(err, &scanErr) {
		return false
	}
	return scanErr.Code == ScanErrCardNotPresent || scanErr.Code == ScanErrCardUnsupported
}

// IsScanEUICCFailure 判断扫描错误是否为 eUICC 硬件故障。
// 此类情况应阻止 VoWiFi 启动（UIM gate 生效）。
func IsScanEUICCFailure(err error) bool {
	var scanErr *EUICCScanError
	if !errors.As(err, &scanErr) {
		return false
	}
	return scanErr.Code == ScanErrEUICCFailure
}
