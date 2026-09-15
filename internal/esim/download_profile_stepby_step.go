package esim

import (
	"context"
	"errors"
	"fmt"

	sgp22 "github.com/voorz/euicc-go/v2"
	"github.com/voorz/euicc-go/lpa"
	"github.com/voorz/vohive/pkg/logger"
)

// downloadProfileStepByStep 使用分步 ES9+ 调用下载 eSIM Profile。
// 参考 NekokoLPA 的 DownloadProfile 实现，将 lpa.Client.DownloadProfile 的一步封装
// 拆分为 6 个独立步骤，每一步都保留完整的 response 数据和结构化错误。
//
// 与 client.DownloadProfile 的关键区别：
// 1. SM-DP+ HTTP 调用使用自定义 SmdpClient（保留 StatusCodeData 结构信息）
// 2. 每一步的进度和错误可以精确上报
// 3. SM-DP+ 服务器错误返回 *SmdpError（包含 SubjectCode/ReasonCode），不会被丢失
func (m *Manager) downloadProfileStepByStep(
	ctx context.Context,
	client *lpa.Client,
	smdpAddress, matchingID, confirmationCode, imei string,
	progressFn DownloadProgressFn,
) (*DownloadSessionResult, error) {
	report := func(step, msg string, pct int) {
		if progressFn != nil {
			progressFn(DownloadProgressEvent{Step: step, Msg: msg, Pct: pct})
		}
	}

	session := NewDownloadSession(ctx, client, smdpAddress, matchingID, confirmationCode)

	// Step 1: 从 eUICC 获取 EUICCInfo1 和 Challenge
	report("preflight", "正在读取 eUICC 信息...", 15)
	if err := session.Step1GetEuiccInfoAndChallenge(); err != nil {
		return nil, NewDownloadProfileError(err)
	}

	// Step 2: 向 SM-DP+ 发起 InitiateAuthentication
	report("initiate_auth", "正在向 SM-DP+ 发起认证请求...", 25)
	if err := session.Step2InitiateAuthentication(); err != nil {
		return nil, m.handleStepError(session, err, sgp22.CancelSessionReasonPostponed)
	}

	// Step 3: eUICC 验证 SM-DP+ 服务器签名
	report("auth_server", "正在验证 SM-DP+ 服务器签名...", 40)
	if err := session.Step3AuthenticateServer(imei); err != nil {
		return nil, m.handleStepError(session, err, sgp22.CancelSessionReasonPostponed)
	}

	// Step 4: 向 SM-DP+ 发送 AuthenticateClient
	report("auth_client", "正在向 SM-DP+ 进行客户端认证...", 55)
	if err := session.Step4AuthenticateClient(); err != nil {
		return nil, m.handleStepError(session, err, sgp22.CancelSessionReasonMetadataMismatch)
	}

	// Step 5: eUICC 准备下载
	report("prepare_download", "正在准备 eUICC 下载环境...", 65)
	prepareDownloadResponse, err := session.Step5PrepareDownload()
	if err != nil {
		return nil, m.handleStepError(session, err, sgp22.CancelSessionReasonPostponed)
	}

	// Step 6: 获取 BPP 并安装到 eUICC
	report("get_bpp", "正在从 SM-DP+ 下载 Profile 数据包...", 75)
	result, err := session.Step6GetBoundProfilePackage(prepareDownloadResponse, func(sent, total int) {
		pct := 75 + (sent*20)/total // 75% -> 95%
		msg := fmt.Sprintf("正在安装 Profile 到 eUICC (%d/%d)...", sent, total)
		report("install", msg, pct)
	})
	if err != nil {
		// 安装失败，尝试取消会话并返回错误
		// 注意：安装 finalize 恢复逻辑在 manager.go DownloadProfile 中处理
		return nil, m.handleStepError(session, err, sgp22.CancelSessionReasonLoadBppExecutionError)
	}

	report("installed", "Profile 安装完成", 85)
	return result, nil
}

// handleStepError 处理分步下载过程中的错误：
// 1. 尝试向 SM-DP+ 发送 CancelSession（非致命错误可忽略）
// 2. 将原始错误转换为 DownloadProfileError
func (m *Manager) handleStepError(session *DownloadSession, err error, cancelReason sgp22.CancelSessionReason) error {
	// 尝试取消会话（非致命，忽略取消失败）
	if cancelErr := session.CancelSession(cancelReason); cancelErr != nil {
		logger.Debug("CancelSession 失败（可忽略）",
			"device", m.deviceID,
			"cancel_err", cancelErr,
			"original_err", err)
	}

	// 如果已经是 SmdpError，完整保留结构化字段（参考 NekokoLPA SmdpException）
	var smdpErr *SmdpError
	if errors.As(err, &smdpErr) {
		msg := smdpErr.Message
		if msg == "" || msg == "N/A" {
			msg = "Unknown SM-DP+ Error"
		}
		return &DownloadProfileError{
			DownloadErrorInfo: DownloadErrorInfo{
				Code:                  "smdp_server_error",
				Message:               msg,
				SmdpSubjectCode:       smdpErr.SubjectCode,
				SmdpReasonCode:        smdpErr.ReasonCode,
				SmdpSubjectIdentifier: smdpErr.SubjectIdentifier,
				OriginalMessage:       smdpErr.Error(),
		},
			Err: err,
		}
	}

	// 其他类型的错误，走通用分类逻辑
	return NewDownloadProfileError(err)
}

// classifyDownloadStepError 将分步下载过程中的错误转换为 *DownloadProfileError。
// 用于 manager.go DownloadProfile 中的错误处理。
func (m *Manager) classifyDownloadStepError(err error) *DownloadProfileError {
	if err == nil {
		return NewDownloadProfileError(nil)
	}

	// 如果已经是 DownloadProfileError（handleStepError 返回的），直接使用
	var downloadErr *DownloadProfileError
	if errors.As(err, &downloadErr) {
		return downloadErr
	}

	// 如果是 SmdpError，完整保留结构化字段
	var smdpErr *SmdpError
	if errors.As(err, &smdpErr) {
		msg := smdpErr.Message
		if msg == "" || msg == "N/A" {
			msg = "Unknown SM-DP+ Error"
		}
		return &DownloadProfileError{
			DownloadErrorInfo: DownloadErrorInfo{
				Code:                  "smdp_server_error",
				Message:               msg,
				SmdpSubjectCode:       smdpErr.SubjectCode,
				SmdpReasonCode:        smdpErr.ReasonCode,
				SmdpSubjectIdentifier: smdpErr.SubjectIdentifier,
				OriginalMessage:       smdpErr.Error(),
			},
			Err: err,
		}
	}

	// 其他错误走通用分类
	return NewDownloadProfileError(err)
}
