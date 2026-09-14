package esim

import (
	"errors"
	"fmt"
	"strings"

	sgp22 "github.com/damonto/euicc-go/v2"
)

const (
	DownloadErrorGeneric                   = "download_failed"
	DownloadErrorEUICCInsufficientMemory   = "euicc_insufficient_memory"
	DownloadErrorEUICCIccidAlreadyExists   = "euicc_iccid_already_exists"
	DownloadErrorEUICCPPRNotAllowed        = "euicc_ppr_not_allowed"
	DownloadErrorEUICCProfileDataMismatch  = "euicc_profile_data_mismatch"
	DownloadErrorEUICCProfileInterrupted   = "euicc_profile_install_interrupted"
	DownloadErrorEUICCProfileInstallFailed = "euicc_profile_install_failed"
)

type DownloadErrorInfo struct {
	Code            string
	Message         string
	Details         string
	BPPCommandID    byte
	BPPErrorReason  byte
	OriginalMessage string
	// SM-DP+ 结构化错误字段（参考 NekokoLPA SmdpException）
	SmdpSubjectCode       string
	SmdpReasonCode        string
	SmdpSubjectIdentifier string
}

type DownloadProfileError struct {
	DownloadErrorInfo
	Err error
}

func NewDownloadProfileError(err error) *DownloadProfileError {
	info := ClassifyDownloadError(err)
	return &DownloadProfileError{
		DownloadErrorInfo: info,
		Err:               err,
	}
}

func (e *DownloadProfileError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "下载 profile 失败"
}

func (e *DownloadProfileError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ClassifyDownloadError(err error) DownloadErrorInfo {
	if err == nil {
		return DownloadErrorInfo{
			Code:    DownloadErrorGeneric,
			Message: "下载 profile 失败",
		}
	}

	info := DownloadErrorInfo{
		Code:            DownloadErrorGeneric,
		Message:         fmt.Sprintf("下载 profile 失败: %v", err),
		OriginalMessage: err.Error(),
	}

	// 1. BPP 安装错误（eUICC 侧）
	var bppPtr *sgp22.LoadBoundProfilePackageError
	var bppValue sgp22.LoadBoundProfilePackageError
	if errors.As(err, &bppPtr) && bppPtr != nil {
		info = applyBPPDownloadErrorInfo(info, *bppPtr)
		return info
	}
	if errors.As(err, &bppValue) {
		info = applyBPPDownloadErrorInfo(info, bppValue)
		return info
	}

	// 2. eUICC 认证错误（AuthenticateServerResponse 错误分支）
	var authErr *sgp22.AuthenticateResponseError
	if errors.As(err, &authErr) && authErr != nil {
		info.Code = "eUICC 认证失败"
		info.Message = fmt.Sprintf("eUICC 认证失败: %s", authErr.ErrorCode)
		info.Details = fmt.Sprintf("TransactionID: %s, ErrorCode: %s (%d)", authErr.TransactionID, authErr.ErrorCode, authErr.ErrorCode)
		return info
	}
	var authErrValue sgp22.AuthenticateResponseError
	if errors.As(err, &authErrValue) {
		info.Code = "eUICC 认证失败"
		info.Message = fmt.Sprintf("eUICC 认证失败: %s", authErrValue.ErrorCode)
		info.Details = fmt.Sprintf("TransactionID: %s, ErrorCode: %s (%d)", authErrValue.TransactionID, authErrValue.ErrorCode, authErrValue.ErrorCode)
		return info
	}

	// 3. SM-DP+ 服务器业务错误
	// InvokeHTTP 用 errors.New(StatusCodeData.Error()) 包装错误，导致 StatusCodeData 类型信息丢失。
	// 通过字符串匹配 SM-DP+ 标准错误表来恢复 SubjectCode/ReasonCode。
	errMsg := err.Error()
	for _, smdpErr := range smdpErrorTable {
		if smdpErr.Message != "" && strings.Contains(errMsg, smdpErr.Message) {
			info.Code = "smdp_server_error"
			info.Message = fmt.Sprintf("SM-DP+ 服务器返回错误: %s", smdpErr.Message)
			info.Details = fmt.Sprintf("SubjectCode: %s, ReasonCode: %s", smdpErr.SubjectCode, smdpErr.ReasonCode)
			return info
		}
	}

	// 4. 其他错误（网络、HTTP、序列化等）— 保留原始错误信息作为 Details
	info.Details = err.Error()
	return info
}

func applyBPPDownloadErrorInfo(info DownloadErrorInfo, err sgp22.LoadBoundProfilePackageError) DownloadErrorInfo {
	info.BPPCommandID = byte(err.BPPCommandID)
	info.BPPErrorReason = byte(err.ErrorReason)
	info.Details = err.Error()
	info.Code = classifyBPPErrorCode(err)
	info.Message = downloadBPPErrorMessage(err)
	return info
}

func classifyBPPErrorCode(err sgp22.LoadBoundProfilePackageError) string {
	if err.BPPCommandID != sgp22.BPPCommandIDLoadProfileElements {
		return DownloadErrorEUICCProfileInstallFailed
	}
	switch err.ErrorReason {
	case sgp22.BPPErrorReasonInstallFailedDueToICCIDAlreadyExistsOnEUICC:
		return DownloadErrorEUICCIccidAlreadyExists
	case sgp22.BPPErrorReasonInstallFailedDueToInsufficientMemoryForProfile:
		return DownloadErrorEUICCInsufficientMemory
	case sgp22.BPPErrorReasonInstallFailedDueToInterruption:
		return DownloadErrorEUICCProfileInterrupted
	case sgp22.BPPErrorReasonInstallFailedDueToDataMismatch:
		return DownloadErrorEUICCProfileDataMismatch
	case sgp22.BPPErrorReasonPPRNotAllowed:
		return DownloadErrorEUICCPPRNotAllowed
	default:
		return DownloadErrorEUICCProfileInstallFailed
	}
}

func downloadBPPErrorMessage(err sgp22.LoadBoundProfilePackageError) string {
	if err.BPPCommandID != sgp22.BPPCommandIDLoadProfileElements {
		return fmt.Sprintf("eUICC 安装 profile 失败（%s）", err.Error())
	}
	switch err.ErrorReason {
	case sgp22.BPPErrorReasonInstallFailedDueToICCIDAlreadyExistsOnEUICC:
		return "eUICC 已存在相同 ICCID 的 profile"
	case sgp22.BPPErrorReasonInstallFailedDueToInsufficientMemoryForProfile:
		return "eUICC 安装 profile 时空间不足，请删除未使用的 profile 后重试"
	case sgp22.BPPErrorReasonInstallFailedDueToInterruption:
		return "eUICC 安装 profile 时被中断，请稍后重试"
	case sgp22.BPPErrorReasonInstallFailedDueToDataMismatch:
		return "eUICC 安装 profile 时数据校验不匹配"
	case sgp22.BPPErrorReasonPPRNotAllowed:
		return "eUICC 策略规则不允许安装该 profile"
	default:
		return fmt.Sprintf("eUICC 安装 profile 失败（%s）", err.Error())
	}
}

// smdpErrorEntry 表示一条 SM-DP+ 标准错误条目
type smdpErrorEntry struct {
	SubjectCode string
	ReasonCode  string
	Message     string
}

// smdpErrorTable 是 SGP22 规范中定义的 SM-DP+/SM-DS 标准错误码表。
// 当 InvokeHTTP 用 errors.New(StatusCodeData.Error()) 丢失类型信息后，
// 通过字符串匹配此表来恢复 SubjectCode/ReasonCode。
var smdpErrorTable = []smdpErrorEntry{
	{"8.1", "4.8", "eUICC does not have sufficient space for this Profile"},
	{"8.1", "6.1", "eUICC signature is invalid or serverChallenge is invalid"},
	{"8.1.1", "2.2", "Indicates that the EID is missing in the context of this order"},
	{"8.1.1", "3.1", "Indicates that a different EID is already associated with this ICCID"},
	{"8.1.1", "3.8", "EID doesn't match the expected value"},
	{"8.1.1", "3.10", "Indicates that a different EID is already associated with this ICCID"},
	{"8.1.2", "6.1", "EUM Certificate is invalid"},
	{"8.1.2", "6.3", "EUM Certificate has expired"},
	{"8.1.3", "6.1", "eUICC Certificate is invalid"},
	{"8.1.3", "6.3", "eUICC Certificate has expired"},
	{"8.2", "1.2", "Profile has not yet been released"},
	{"8.2", "3.7", "BPP is not available for a new binding"},
	{"8.2.1", "1.2", "Indicates that the function caller is not allowed to perform this function on the target Profile"},
	{"8.2.1", "3.3", "Indicates that the Profile identified by the provided ICCID is not available"},
	{"8.2.1", "3.5", "Indicates that the target Profile cannot be released"},
	{"8.2.1", "3.9", "Indicates that the Profile, identified by this ICCID is unknown to the SM-DP+"},
	{"8.2.1", "3.10", "Indicates that a different EID is associated with this ICCID"},
	{"8.2.5", "1.2", "Indicates that the function caller is not allowed to perform this function on the Profile Type"},
	{"8.2.5", "3.7", "No more Profile available for the requested Profile Type"},
	{"8.2.5", "3.8", "Indicates that the Profile Type identified by this Profile Type is not aligned with the Profile Type of Profile identified by the ICCID"},
	{"8.2.5", "3.9", "Indicates that the Profile Type identified by this Profile Type is unknown to the SM-DP+"},
	{"8.2.5", "4.3", "No eligible Profile for this eUICC/Device"},
	{"8.2.6", "3.3", "Conflicting MatchingID value"},
	{"8.2.6", "3.8", "MatchingID (AC_Token or EventID) is refused"},
	{"8.2.6", "3.10", "Indicates that a different MatchingID is associated with this ICCID"},
	{"8.2.7", "2.2", "Confirmation Code is missing"},
	{"8.2.7", "3.8", "Confirmation Code is refused"},
	{"8.2.7", "6.4", "The maximum number of retries for the Confirmation Code has been exceeded"},
	{"8.8", "3.10", "The provided SM-DP+ OID is invalid"},
	{"8.8.1", "3.8", "Invalid SM-DP+ Address"},
	{"8.8.2", "3.1", "None of the proposed Public Key Identifiers is supported by the SM-DP+"},
	{"8.8.3", "3.1", "The Specification Version Number indicated by the eUICC is not supported by the SM-DP+"},
	{"8.8.4", "3.7", "The SM-DP+ has no CERT.DPauth.ECDSA signed by one of the CI Public Key supported by the eUICC"},
	{"8.8.5", "4.10", "The Download order has expired"},
	{"8.8.5", "6.4", "The maximum number of retries for the Profile download order has been exceeded"},
	{"8.9", "4.2", "The cascade SM-DS registration has failed. SMDS has raised an error"},
	{"8.9", "5.1", "Indicates that the smdsAddress is invalid or not reachable."},
	{"8.9.1", "3.8", "Invalid SM-DS Address"},
	{"8.9.2", "3.1", "None of the proposed Public Key Identifiers is supported by the SM-DS"},
	{"8.9.3", "3.1", "The Specification Version Number indicated by the eUICC is not supported by the SM-DS"},
	{"8.9.4", "3.7", "The SM-DS has no CERT.DS.ECDSA signed by one of the GSMA CI Public Key supported by the eUICC"},
	{"8.9.5", "3.3", "The Event Record already exist in the SM-DS (EventID duplicated)"},
	{"8.9.5", "3.9", "No Event identified by the Event ID for the EID exists"},
	{"8.10.1", "3.9", "The RSP session identified by the TransactionID is unknown"},
	{"8.11.1", "3.9", "Unknown CI Public Key. The CI used by the EUM Certificate is not a trusted root."},
}
