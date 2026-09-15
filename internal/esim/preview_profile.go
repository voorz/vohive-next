package esim

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	sgp22 "github.com/damonto/euicc-go/v2"
	"github.com/damonto/euicc-go/lpa"
	"github.com/voorz/vohive/pkg/logger"
)

// PreviewProfile 执行查询阶段（Preview），返回 Profile 元数据。
// 不执行实际下载（PrepareDownload / GetBPP / LoadBPP）。
// 参考 NekokoLPA 的 _startPreview() 流程。
func (m *Manager) PreviewProfile(ctx context.Context, aidHex, smdp, matchingID, confirmationCode, downloadIMEI string, progressFn DownloadProgressFn) (*PreviewResult, error) {
	report := func(step, msg string, pct int) {
		if progressFn != nil {
			progressFn(DownloadProgressEvent{Step: step, Msg: msg, Pct: pct})
		}
	}

	m.opMu.Lock()
	writeStarted := time.Now()
	defer func() {
		m.logWriteOperationHold("preview_profile", writeStarted)
		m.opMu.Unlock()
		m.notifyWriteDone()
	}()

	// 创建 LPA client
	var client *lpa.Client
	var targetAID []byte
	if aidHex != "" {
		aid, err := hex.DecodeString(aidHex)
		if err != nil {
			return nil, fmt.Errorf("无效的 AID hex %q: %w", aidHex, err)
		}
		targetAID = aid
		client, err = m.createLPAWithAID(targetAID)
		if err != nil {
			return nil, err
		}
	} else {
		aids := m.getEffectiveAIDs()
		for _, aid := range aids {
			c, err := m.createLPAWithAID(aid)
			if err != nil {
				continue
			}
			client = c
			targetAID = append([]byte(nil), aid...)
			aidHex = fmt.Sprintf("%X", targetAID)
			break
		}
		if client == nil {
			return nil, fmt.Errorf("未找到可用的 eUICC AID")
		}
	}
	defer func() {
		if client != nil {
			m.closeLPAClientForOperation("preview_profile", client)
		}
	}()

	// 解析 IMEI
	imei, err := m.resolveDownloadIMEI(ctx, downloadIMEI)
	if err != nil {
		return nil, err
	}

	// 解析 SM-DP+ 地址
	smdpAddr := strings.TrimSpace(smdp)
	if smdpAddr == "" {
		return nil, fmt.Errorf("SM-DP+ 地址不能为空")
	}
	if !strings.Contains(smdpAddr, "://") {
		smdpAddr = "https://" + smdpAddr
	}
	parsedURL, err := url.Parse(smdpAddr)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的 SM-DP+ 地址 %q", smdp)
	}

	logger.Info("开始预览 eSIM profile",
		"device", m.deviceID,
		"smdp", parsedURL.Host,
		"matchingID", matchingID,
		"AID", aidHex)

	// 创建 DownloadSession 并执行 PreviewSession
	session := NewDownloadSession(ctx, client, parsedURL.Host, matchingID, strings.TrimSpace(confirmationCode))
	previewResult, err := session.PreviewSession(imei, progressFn)
	if err != nil {
		// 尝试取消会话
		_ = session.CancelSession(sgp22.CancelSessionReasonPostponed)

		// 分类错误（复用 download 的错误分类逻辑）
		downloadErr := m.classifyDownloadStepError(err)
		logger.Warn("预览 eSIM profile 失败",
			"device", m.deviceID,
			"smdp", parsedURL.Host,
			"matchingID", matchingID,
			"AID", aidHex,
			"error_code", downloadErr.Code,
			"err", err)
		return nil, downloadErr
	}

	// 查询预估 Profile 大小（复用 esimSizeStore）
	if previewResult.ProfileMetadata != nil && m.esimSizeStore != nil {
		md := previewResult.ProfileMetadata
		plmn := strings.TrimSpace(md.ProfileOwnerMCC) + strings.TrimSpace(md.ProfileOwnerMNC)
		// 从缓存获取 EID（避免在 opMu 锁定期间调用 GetEID 导致重入冲突）
		m.cacheMu.RLock()
		cached := m.chipInfoCache
		m.cacheMu.RUnlock()
		var eid string
		if cached != nil && len(cached.EIDs) > 0 {
			eid = cached.EIDs[0].EID
		}
		logger.Info("PreviewProfile 预估大小查询",
			"plmn", plmn,
			"spn", md.ServiceProviderName,
			"eid", eid)
		if plmn != "" && md.ServiceProviderName != "" && eid != "" {
			if size, found := m.esimSizeStore.LookupProfileSize(eid, plmn, md.ServiceProviderName); found && size > 0 {
				md.EstimatedProfileSize = size
				logger.Info("PreviewProfile 预估大小命中", "size", size)
			} else {
				logger.Info("PreviewProfile 预估大小未命中", "found", found)
			}
		}
	} else {
		logger.Warn("PreviewProfile 跳过预估大小",
			"has_metadata", previewResult.ProfileMetadata != nil,
			"has_store", m.esimSizeStore != nil)
	}

	report("preview", "查询完成", 100)
	return previewResult, nil
}
