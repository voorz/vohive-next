package esim

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/damonto/euicc-go/bertlv"
	sgp22 "github.com/damonto/euicc-go/v2"
	"github.com/damonto/euicc-go/lpa"
	"github.com/voorz/vohive/pkg/logger"
)

// DownloadSessionStep 表示分步下载的步骤标识。
type DownloadSessionStep string

const (
	StepInitiateAuthentication  DownloadSessionStep = "initiate_auth"
	StepAuthenticateServer      DownloadSessionStep = "auth_server"
	StepAuthenticateClient      DownloadSessionStep = "auth_client"
	StepPrepareDownload         DownloadSessionStep = "prepare_download"
	StepGetBoundProfilePackage  DownloadSessionStep = "get_bpp"
	StepLoadBoundProfilePackage DownloadSessionStep = "load_bpp"
)

// DownloadSessionState 保存分步下载过程中的中间状态。
// 参考 NekokoLPA ProfileDownloadSession，管理每一步的 response 数据。
type DownloadSessionState struct {
	// ES10b 数据（来自 eUICC）
	EuiccInfo1     *bertlv.TLV
	EuiccChallenge []byte

	// ES9+ InitiateAuthentication 响应数据（来自 SM-DP+）
	TransactionID  []byte
	ServerSigned1  *bertlv.TLV
	ServerSignature1 *bertlv.TLV
	EuiccCiPKId    *bertlv.TLV
	ServerCertificate *bertlv.TLV

	// ES10b AuthenticateServer 响应数据（来自 eUICC）
	AuthenticateServerResponse *bertlv.TLV

	// ES9+ AuthenticateClient 响应数据（来自 SM-DP+）
	ProfileMetadata  *bertlv.TLV
	SmdpSigned2      *bertlv.TLV
	SmdpSignature2   *bertlv.TLV
	SmdpCertificate  *bertlv.TLV

	// 辅助字段
	SmdpAddress      string
	MatchingID       string
	ConfirmationCode string
}

// DownloadSessionResult 是分步下载的最终结果。
type DownloadSessionResult struct {
	// 安装结果
	LoadBoundProfilePackageResponse *sgp22.LoadBoundProfilePackageResponse
	// Profile 元数据
	ProfileInfo *sgp22.ProfileInfo
	// 安装的 ICCID
	ICCID sgp22.ICCID
}

// DownloadSession 是分步下载会话管理器。
// 参考 NekokoLPA ProfileDownloadSession，将 lpa.Client.DownloadProfile 的一步封装
// 拆分为 6 个独立步骤，每一步都能保留完整的 response 数据和结构化错误。
type DownloadSession struct {
	client    *lpa.Client
	smdp      *SmdpClient
	state     *DownloadSessionState
	ctx       context.Context
}

// NewDownloadSession 创建一个新的分步下载会话。
func NewDownloadSession(ctx context.Context, client *lpa.Client, smdpAddress, matchingID, confirmationCode string) *DownloadSession {
	return &DownloadSession{
		client: client,
		smdp:   NewSmdpClient(30 * time.Second),
		state: &DownloadSessionState{
			SmdpAddress:      strings.TrimSpace(smdpAddress),
			MatchingID:       strings.TrimSpace(matchingID),
			ConfirmationCode: strings.TrimSpace(confirmationCode),
		},
		ctx: ctx,
	}
}

// Step1GetEuiccInfoAndChallenge 从 eUICC 获取 EUICCInfo1 和 Challenge。
func (s *DownloadSession) Step1GetEuiccInfoAndChallenge() error {
	info1, err := s.client.EUICCInfo1()
	if err != nil {
		return fmt.Errorf("获取 EUICCInfo1 失败: %w", err)
	}
	s.state.EuiccInfo1 = info1

	challenge, err := s.client.EUICCChallenge()
	if err != nil {
		return fmt.Errorf("获取 EUICCChallenge 失败: %w", err)
	}
	s.state.EuiccChallenge = challenge

	logger.Info("Step1: 获取 eUICC Info1 和 Challenge 完成",
		"info1_len", len(info1.Bytes()),
		"challenge_len", len(challenge))
	return nil
}

// Step2InitiateAuthentication 向 SM-DP+ 发起 InitiateAuthentication 请求。
// 参考 NekokoLPA Es9PlusService.initiateAuthentication。
func (s *DownloadSession) Step2InitiateAuthentication() error {
	// 构建 ES9+ InitiateAuthentication 请求体
	euiccChallengeB64 := base64.StdEncoding.EncodeToString(s.state.EuiccChallenge)
	euiccInfo1B64 := base64.StdEncoding.EncodeToString(s.state.EuiccInfo1.Bytes())

	body := map[string]any{
		"euiccChallenge": euiccChallengeB64,
		"euiccInfo1":     euiccInfo1B64,
		"smdpAddress":    s.state.SmdpAddress,
	}

	resp, err := s.smdp.Post(s.ctx, s.state.SmdpAddress, "initiateAuthentication", body)
	if err != nil {
		return fmt.Errorf("InitiateAuthentication 失败: %w", err)
	}

	// 解析响应
	txnID, ok := resp["transactionId"].(string)
	if !ok {
		return fmt.Errorf("InitiateAuthentication 响应缺少 transactionId")
	}
	// transactionId 是 hex 编码的
	txnIDBytes, err := hex.DecodeString(txnID)
	if err != nil {
		// 有些 SM-DP+ 返回的不是 hex，直接用
		txnIDBytes = []byte(txnID)
	}
	s.state.TransactionID = txnIDBytes

	s.state.ServerSigned1, err = decodeEs9Base64TLV(resp, "serverSigned1")
	if err != nil {
		return fmt.Errorf("解析 serverSigned1 失败: %w", err)
	}
	s.state.ServerSignature1, err = decodeEs9Base64TLV(resp, "serverSignature1")
	if err != nil {
		return fmt.Errorf("解析 serverSignature1 失败: %w", err)
	}
	s.state.EuiccCiPKId, err = decodeEs9Base64TLV(resp, "euiccCiPKIdToBeUsed")
	if err != nil {
		return fmt.Errorf("解析 euiccCiPKIdToBeUsed 失败: %w", err)
	}
	s.state.ServerCertificate, err = decodeEs9Base64TLV(resp, "serverCertificate")
	if err != nil {
		return fmt.Errorf("解析 serverCertificate 失败: %w", err)
	}

	logger.Info("Step2: InitiateAuthentication 完成",
		"transactionId", txnID,
		"has_serverSigned1", s.state.ServerSigned1 != nil,
		"has_serverCertificate", s.state.ServerCertificate != nil)
	return nil
}

// Step3AuthenticateServer 让 eUICC 验证 SM-DP+ 服务器签名。
// 参考 NekokoLPA profileManager.authenticateServer。
func (s *DownloadSession) Step3AuthenticateServer(imei string) error {
	parsedIMEI, err := sgp22.NewIMEI(imei)
	if err != nil {
		return fmt.Errorf("无效的 IMEI %q: %w", imei, err)
	}

	// 构建 AuthenticateServerRequest
	request := &sgp22.AuthenticateServerRequest{
		TransactionID: s.state.TransactionID,
		Signed1:       s.state.ServerSigned1,
		Signature1:    s.state.ServerSignature1,
		UsedIssuer:    s.state.EuiccCiPKId,
		Certificate:   s.state.ServerCertificate,
		IMEI:          parsedIMEI,
		MatchingID:    []byte(s.state.MatchingID),
	}

	// 通过 APDU 调用 eUICC 的 AuthenticateServer
	// lpa.Client.AuthenticateClient 内部会先调用 InvokeAPDU（ES10b）再调用 InvokeHTTP（ES9+）
	// 但我们不需要它调用 ES9+，只需要 ES10b 部分的 APDU 响应
	// 所以我们需要直接使用 InvokeAPDU
	authenticateClientRequest, err := sgp22.InvokeAPDU(s.client.APDU, request)
	if err != nil {
		return fmt.Errorf("eUICC AuthenticateServer 失败: %w", err)
	}

	// authenticateClientRequest.Response 就是 authenticateServerResponse
	s.state.AuthenticateServerResponse = authenticateClientRequest.Response

	logger.Info("Step3: AuthenticateServer (eUICC) 完成",
		"response_len", len(authenticateClientRequest.Response.Bytes()))
	return nil
}

// Step4AuthenticateClient 向 SM-DP+ 发送 AuthenticateClient 请求。
// 参考 NekokoLPA Es9PlusService.authenticateClient。
func (s *DownloadSession) Step4AuthenticateClient() error {
	transactionIDStr := strings.ToUpper(hex.EncodeToString(s.state.TransactionID))
	authServerRespB64 := base64.StdEncoding.EncodeToString(s.state.AuthenticateServerResponse.Bytes())

	body := map[string]any{
		"transactionId":             transactionIDStr,
		"authenticateServerResponse": authServerRespB64,
	}

	resp, err := s.smdp.Post(s.ctx, s.state.SmdpAddress, "authenticateClient", body)
	if err != nil {
		return fmt.Errorf("AuthenticateClient 失败: %w", err)
	}

	// 解析响应
	s.state.ProfileMetadata, err = decodeEs9Base64TLV(resp, "profileMetadata")
	if err != nil {
		return fmt.Errorf("解析 profileMetadata 失败: %w", err)
	}
	s.state.SmdpSigned2, err = decodeEs9Base64TLV(resp, "smdpSigned2")
	if err != nil {
		return fmt.Errorf("解析 smdpSigned2 失败: %w", err)
	}
	s.state.SmdpSignature2, err = decodeEs9Base64TLV(resp, "smdpSignature2")
	if err != nil {
		return fmt.Errorf("解析 smdpSignature2 失败: %w", err)
	}
	s.state.SmdpCertificate, err = decodeEs9Base64TLV(resp, "smdpCertificate")
	if err != nil {
		return fmt.Errorf("解析 smdpCertificate 失败: %w", err)
	}

	// 解码 ProfileInfo
	if s.state.ProfileMetadata != nil {
		profileInfo := new(sgp22.ProfileInfo)
		if err := profileInfo.UnmarshalBERTLV(s.state.ProfileMetadata); err != nil {
			logger.Warn("解析 ProfileInfo 失败", "err", err)
		}
		_ = profileInfo // 存储到结果中
	}

	logger.Info("Step4: AuthenticateClient (SM-DP+) 完成",
		"has_profileMetadata", s.state.ProfileMetadata != nil,
		"has_smdpSigned2", s.state.SmdpSigned2 != nil,
		"has_smdpCertificate", s.state.SmdpCertificate != nil)
	return nil
}

// Step5PrepareDownload 让 eUICC 准备下载。
// 参考 NekokoLPA profileManager.prepareDownload。
func (s *DownloadSession) Step5PrepareDownload() ([]byte, error) {
	request := &sgp22.PrepareDownloadRequest{
		TransactionID:    s.state.TransactionID,
		ProfileMetadata:  s.state.ProfileMetadata,
		Signed2:          s.state.SmdpSigned2,
		Signature2:       s.state.SmdpSignature2,
		Certificate:      s.state.SmdpCertificate,
		ConfirmationCode: []byte(s.state.ConfirmationCode),
	}

	// 通过 APDU 调用 eUICC 的 PrepareDownload
	bppRequest, err := sgp22.InvokeAPDU(s.client.APDU, request)
	if err != nil {
		return nil, fmt.Errorf("eUICC PrepareDownload 失败: %w", err)
	}

	// bppRequest.Response 是 prepareDownloadResponse
	prepareDownloadResponse := bppRequest.Response

	logger.Info("Step5: PrepareDownload (eUICC) 完成",
		"response_len", len(prepareDownloadResponse.Bytes()))
	return prepareDownloadResponse.Bytes(), nil
}

// Step6GetBoundProfilePackage 向 SM-DP+ 请求 BoundProfilePackage，然后安装到 eUICC。
// 参考 NekokoLPA Es9PlusService.getBoundProfilePackage + profileManager.loadBoundProfilePackage。
func (s *DownloadSession) Step6GetBoundProfilePackage(prepareDownloadResponse []byte, onProgress func(sent, total int)) (*DownloadSessionResult, error) {
	transactionIDStr := strings.ToUpper(hex.EncodeToString(s.state.TransactionID))
	prepareDownloadB64 := base64.StdEncoding.EncodeToString(prepareDownloadResponse)

	body := map[string]any{
		"transactionId":           transactionIDStr,
		"prepareDownloadResponse": prepareDownloadB64,
	}

	resp, err := s.smdp.Post(s.ctx, s.state.SmdpAddress, "getBoundProfilePackage", body)
	if err != nil {
		return nil, fmt.Errorf("GetBoundProfilePackage 失败: %w", err)
	}

	bppTLV, err := decodeEs9Base64TLV(resp, "boundProfilePackage")
	if err != nil {
		return nil, fmt.Errorf("解析 boundProfilePackage 失败: %w", err)
	}
	if bppTLV == nil {
		return nil, fmt.Errorf("SM-DP+ 未返回 boundProfilePackage")
	}

	bppBytes := bppTLV.Bytes()
	logger.Info("Step6: GetBoundProfilePackage (SM-DP+) 完成",
		"bpp_bytes", len(bppBytes))

	// 安装 BoundProfilePackage 到 eUICC
	// 使用 sgp22.SegmentedBoundProfilePackage 分段发送 APDU
	segments, err := sgp22.SegmentedBoundProfilePackage(bppTLV)
	if err != nil {
		return nil, fmt.Errorf("分段 BoundProfilePackage 失败: %w", err)
	}

	totalSegments := len(segments)
	var lastResponse []byte
	for i, segment := range segments {
		lastResponse, err = sgp22.InvokeRawAPDU(s.client.APDU, segment)
		if err != nil {
			return nil, fmt.Errorf("安装 BoundProfilePackage 段 %d/%d 失败: %w", i+1, totalSegments, err)
		}
		if onProgress != nil {
			onProgress(i+1, totalSegments)
		}
		if len(lastResponse) > 0 {
			break // eUICC 返回了最终响应
		}
	}

	// 解析安装结果
	var tlv bertlv.TLV
	if err := tlv.UnmarshalBinary(lastResponse); err != nil {
		return nil, fmt.Errorf("解析安装响应 TLV 失败: %w", err)
	}

	var lbppResponse sgp22.LoadBoundProfilePackageResponse
	if err := lbppResponse.UnmarshalBERTLV(&tlv); err != nil {
		return nil, fmt.Errorf("解析 LoadBoundProfilePackageResponse 失败: %w", err)
	}

	// 检查安装是否成功
	if err := lbppResponse.Valid(); err != nil {
		return nil, fmt.Errorf("eUICC 安装 Profile 失败: %w", err)
	}

	result := &DownloadSessionResult{
		LoadBoundProfilePackageResponse: &lbppResponse,
	}

	// 解析 ProfileInfo
	if s.state.ProfileMetadata != nil {
		profileInfo := new(sgp22.ProfileInfo)
		if err := profileInfo.UnmarshalBERTLV(s.state.ProfileMetadata); err == nil {
			result.ProfileInfo = profileInfo
			result.ICCID = profileInfo.ICCID
		}
	}

	logger.Info("Step6: LoadBoundProfilePackage (eUICC) 完成",
		"iccid", result.ICCID)
	return result, nil
}

// CancelSession 向 SM-DP+ 发送取消会话请求。
func (s *DownloadSession) CancelSession(reason sgp22.CancelSessionReason) error {
	cancelRequest, err := sgp22.InvokeAPDU(s.client.APDU, &sgp22.CancelSessionRequest{
		TransactionID: s.state.TransactionID,
		Reason:        reason,
	})
	if err != nil {
		return err
	}

	// 通过自定义 HTTP 客户端发送 CancelSession
	transactionIDStr := strings.ToUpper(hex.EncodeToString(s.state.TransactionID))
	cancelRespB64 := base64.StdEncoding.EncodeToString(cancelRequest.Response.Bytes())

	body := map[string]any{
		"transactionId":          transactionIDStr,
		"cancelSessionResponse":  cancelRespB64,
	}

	_, err = s.smdp.Post(s.ctx, s.state.SmdpAddress, "cancelSession", body)
	return err
}

// State 返回当前会话状态（只读）。
func (s *DownloadSession) State() *DownloadSessionState {
	return s.state
}

// decodeEs9Base64TLV 从 SM-DP+ JSON 响应中解析 Base64 编码的 BER-TLV 字段。
func decodeEs9Base64TLV(data map[string]any, key string) (*bertlv.TLV, error) {
	raw, ok := data[key]
	if !ok || raw == nil {
		return nil, nil
	}

	str, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("%s 不是字符串", key)
	}

	if str == "" {
		return nil, nil
	}

	binary, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("%s Base64 解码失败: %w", key, err)
	}

	var tlv bertlv.TLV
	if err := tlv.UnmarshalBinary(binary); err != nil {
		return nil, fmt.Errorf("%s TLV 解析失败: %w", key, err)
	}

	return &tlv, nil
}
