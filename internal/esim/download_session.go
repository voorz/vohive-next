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

// PreviewResult 是查询阶段（Preview）返回的结构化数据。
// 参考 NekokoLPA 的 _buildPreview 数据采集。
type PreviewResult struct {
	ProfileMetadata *PreviewMetadata  `json:"metadata"`
	EuiccInfo2      *PreviewEuiccInfo2 `json:"euicc_info2"`
	CCRequired      bool              `json:"cc_required"`
	// 证书 DER 编码（Base64），供开发者模式导出
	EuiccCertDER string `json:"euicc_cert_der,omitempty"`
	EumCertDER  string `json:"eum_cert_der,omitempty"`
}

// PreviewMetadata 是从 ProfileMetadata TLV 解析出的 Profile 元数据。
type PreviewMetadata struct {
	ICCID                string `json:"iccid"`
	ProfileName          string `json:"profile_name"`
	ServiceProviderName string `json:"service_provider_name"`
	ProfileClass         string `json:"profile_class,omitempty"`
	IconType             string `json:"icon_type,omitempty"`
	IconBase64           string `json:"icon_base64,omitempty"`
	ProfileOwnerMCC      string `json:"profile_owner_mcc,omitempty"`
	ProfileOwnerMNC      string `json:"profile_owner_mnc,omitempty"`
	EstimatedProfileSize int   `json:"estimated_profile_size,omitempty"` // 预估 Profile 大小（字节）
}

// PreviewEuiccInfo2 是从 EUICCInfo2 中提取的空间信息。
type PreviewEuiccInfo2 struct {
	FreeNonVolatileMemory int `json:"free_non_volatile_memory"`
	FreeVolatileMemory    int `json:"free_volatile_memory,omitempty"`
	InstalledApplication  int `json:"installed_application,omitempty"`
}

// PreviewSession 执行查询阶段（Step1-Step4），返回 Profile 元数据。
// 不执行 PrepareDownload / GetBPP / LoadBPP。
// 参考 NekokoLPA 的 _startPreview() 流程。
func (s *DownloadSession) PreviewSession(imei string, progressFn DownloadProgressFn) (*PreviewResult, error) {
	report := func(step, msg string, pct int) {
		if progressFn != nil {
			progressFn(DownloadProgressEvent{Step: step, Msg: msg, Pct: pct})
		}
	}

	// Step 1: 从 eUICC 获取 EUICCInfo1 和 Challenge
	report("preflight", "正在读取 eUICC 信息...", 15)
	if err := s.Step1GetEuiccInfoAndChallenge(); err != nil {
		return nil, err
	}

	// Step 2: 向 SM-DP+ 发起 InitiateAuthentication
	report("initiate_auth", "正在向 SM-DP+ 发起认证请求...", 30)
	if err := s.Step2InitiateAuthentication(); err != nil {
		return nil, err
	}

	// Step 3: eUICC 验证 SM-DP+ 服务器签名
	report("auth_server", "正在验证 SM-DP+ 服务器签名...", 50)
	if err := s.Step3AuthenticateServer(imei); err != nil {
		return nil, err
	}

	// Step 4: 向 SM-DP+ 发送 AuthenticateClient，获取 ProfileMetadata
	report("auth_client", "正在获取 Profile 元数据...", 70)
	if err := s.Step4AuthenticateClient(); err != nil {
		return nil, err
	}

	// 解析 ProfileInfo
	result := &PreviewResult{}
	if s.state.ProfileMetadata != nil {
		profileInfo := new(sgp22.ProfileInfo)
		if err := profileInfo.UnmarshalBERTLV(s.state.ProfileMetadata); err == nil {
			result.ProfileMetadata = buildPreviewMetadata(profileInfo)
		}
	}

	// 获取 EUICCInfo2 中的空间信息
	// 从 AuthenticateServerResponse 中提取，或者直接调用 EUICCInfo2()
	euiccInfo2TLV, err := s.client.EUICCInfo2()
	if err == nil && euiccInfo2TLV != nil {
		result.EuiccInfo2 = extractPreviewEuiccInfo2(euiccInfo2TLV)
	}

	// 解析 SmdpSigned2 中的 ccRequiredFlag
	if s.state.SmdpSigned2 != nil {
		result.CCRequired = parseCCRequiredFlag(s.state.SmdpSigned2)
	}

	// 从 AuthenticateServerResponse 中提取 eUICC 证书和 EUM 证书
	// AuthenticateServerResponse (tag 0xBF38, Constructed) 包含一个子节点：
	//   AuthenticateResponseOk SEQUENCE: [0]euiccSigned1 [1]euiccSignature1 [2]euiccCertificate [3]nextCertInChain
	if s.state.AuthenticateServerResponse != nil {
		outerChildren := s.state.AuthenticateServerResponse.Children
		logger.Info("PreviewSession 证书提取调试",
			"outerChildren_len", len(outerChildren),
			"resp_tag", s.state.AuthenticateServerResponse.Tag.Value())
		if len(outerChildren) > 0 && outerChildren[0] != nil {
			authRespOk := outerChildren[0]
			children := authRespOk.Children
			logger.Info("PreviewSession AuthenticateResponseOk 子节点",
				"children_len", len(children))
			if len(children) >= 3 && children[2] != nil {
				result.EuiccCertDER = base64.StdEncoding.EncodeToString(children[2].Bytes())
				logger.Info("PreviewSession 提取到 eUICC 证书", "der_len", len(children[2].Bytes()))
			}
			if len(children) >= 4 && children[3] != nil {
				result.EumCertDER = base64.StdEncoding.EncodeToString(children[3].Bytes())
				logger.Info("PreviewSession 提取到 EUM 证书", "der_len", len(children[3].Bytes()))
			}
		}
	} else {
		logger.Warn("PreviewSession AuthenticateServerResponse 为空")
	}

	report("preview", "查询完成", 100)
	return result, nil
}

// buildPreviewMetadata 从 sgp22.ProfileInfo 构建 PreviewMetadata。
func buildPreviewMetadata(info *sgp22.ProfileInfo) *PreviewMetadata {
	if info == nil {
		return nil
	}
	md := &PreviewMetadata{
		ProfileName:          info.ProfileName,
		ServiceProviderName: info.ServiceProviderName,
		ICCID:                info.ICCID.String(),
		ProfileClass:         info.ProfileClass.String(),
		ProfileOwnerMCC:     info.ProfileOwner.MCC(),
		ProfileOwnerMNC:     info.ProfileOwner.MNC(),
	}
	return md
}

// extractPreviewEuiccInfo2 从 EUICCInfo2 TLV 中提取空间信息。
// 参考 euicc_info.go applyEUICCInfoTLV 中的 extCardResource 解析逻辑。
func extractPreviewEuiccInfo2(tlv *bertlv.TLV) *PreviewEuiccInfo2 {
	if tlv == nil {
		return nil
	}
	info := &PreviewEuiccInfo2{}
	// extCardResource: tag = ContextSpecific.Constructed(4) = 0xA4
	// 但实际在 EUICCInfo2 中 tag 是 0x24 (ContextSpecific.Primitive(4))
	// 参考 euicc_info.go: tlv.First(bertlv.ContextSpecific.Primitive(4))
	if resource := tlv.First(bertlv.ContextSpecific.Primitive(4)); resource != nil {
		data, _ := resource.MarshalBinary()
		if len(data) > 0 {
			data[0] = 0x30 // 改为 Constructed
			if err := resource.UnmarshalBinary(data); err == nil {
				// freeNonVolatileMemory: tag = ContextSpecific.Primitive(2) = 0x82
				if freeNv := resource.First(bertlv.ContextSpecific.Primitive(2)); freeNv != nil {
					// 解析整数
					for _, b := range freeNv.Value {
						info.FreeNonVolatileMemory = (info.FreeNonVolatileMemory << 8) | int(b)
					}
				}
				// freeVolatileMemory: tag = ContextSpecific.Primitive(3) = 0x83
				if freeV := resource.First(bertlv.ContextSpecific.Primitive(3)); freeV != nil {
					for _, b := range freeV.Value {
						info.FreeVolatileMemory = (info.FreeVolatileMemory << 8) | int(b)
					}
				}
				// installedApplication: tag = ContextSpecific.Primitive(1) = 0x81
				if installed := resource.First(bertlv.ContextSpecific.Primitive(1)); installed != nil {
					for _, b := range installed.Value {
						info.InstalledApplication = (info.InstalledApplication << 8) | int(b)
					}
				}
			}
		}
	}
	return info
}

// parseCCRequiredFlag 从 SmdpSigned2 TLV 中解析 ccRequiredFlag。
// SmdpSigned2 结构（SGP.22）:
//   SEQUENCE {
//     transactionId [1] OCTET STRING,       -- tag 0x80
//     ccRequiredFlag [2] BOOLEAN,           -- tag 0x01 (Primitive, context 1)
//     bppEuiccOtpk [3] SubjectKeyIdentifier,-- tag 0x5F49
//     rpmPending [4] OCTET STRING            -- tag 0x04
//   }
// ccRequiredFlag 的 tag 是 0x01 (context-specific, primitive, number 1)
func parseCCRequiredFlag(smdpSigned2 *bertlv.TLV) bool {
	if smdpSigned2 == nil {
		return false
	}
	// 在 TLV children 中查找 tag value 1 (context-specific primitive 1 = ccRequiredFlag)
	for _, child := range smdpSigned2.Children {
		if child.Tag.Value() == 1 && child.Tag.ContextSpecific() && child.Tag.Primitive() {
			if len(child.Value) > 0 && child.Value[0] != 0x00 {
				return true
			}
		}
	}
	return false
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
