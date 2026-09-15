package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/esim"
)

// handleEsimPreviewProfile 预览 eSIM Profile 元数据（SSE 流式进度）
//
// 请求方式：GET，通过 Query 参数传递预览参数：
//
//	smdp=<SM-DP+ 地址>（必填）
//	matching_id=<Matching ID>（可选）
//	confirmation_code=<确认码>（可选）
//	aid_hex=<目标 AID hex>（可选）
//	imei=<下载使用的 IMEI>（可选）
//
// 响应为 text/event-stream，每条事件 data 为 JSON：
//
//	{"step":"preflight","msg":"正在读取 eUICC 信息...","pct":15}
//	{"step":"initiate_auth","msg":"...","pct":30}
//	{"step":"auth_server","msg":"...","pct":50}
//	{"step":"auth_client","msg":"...","pct":70}
//	{"step":"preview","msg":"查询完成","pct":100,"metadata":{...},"euicc_info2":{...},"cc_required":false}
//	{"step":"error","msg":"<错误信息>","pct":0}
//
// handleEsimPreviewProfile
//
// @Summary      EsimPreviewProfile
// @Tags         devices
// @Produce      json
// @Param        device_id  path      string  true  "device_id"
// @Param        smdp       query     string  true  "SM-DP+ 地址"
// @Param        matching_id query    string  false "Matching ID"
// @Param        confirmation_code query string false "确认码"
// @Param        aid_hex    query     string  false "目标 AID hex"
// @Param        imei       query     string  false "IMEI"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/{device_id}/esim/actions/preview [get]
// @Security     BearerAuth
func (s *Server) handleEsimPreviewProfile(c *gin.Context) {
	id := deviceIDParam(c)
	worker := s.pool.GetWorker(id)
	if worker == nil || worker.EsimMgr == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备或esim管理器未找到"})
		return
	}

	smdp := strings.TrimSpace(c.Query("smdp"))
	if smdp == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "smdp 为必填项"})
		return
	}
	matchingID := c.Query("matching_id")
	confirmationCode := c.Query("confirmation_code")
	aidHex := c.Query("aid_hex")
	imei := strings.TrimSpace(c.Query("imei"))

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "流式输出不支持"})
		return
	}

	// sseWrite 向客户端推送一条 SSE 事件
	sseWrite := func(step, msg string, pct int) {
		fmt.Fprintf(c.Writer, "data: {\"step\":%q,\"msg\":%q,\"pct\":%d}\n\n", step, msg, pct)
		flusher.Flush()
	}

	// sseWriteRaw 推送原始 JSON 字符串
	sseWriteRaw := func(payload string) {
		fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
		flusher.Flush()
	}

	progressFn := func(event esim.DownloadProgressEvent) {
		sseWrite(event.Step, event.Msg, event.Pct)
	}

	result, err := worker.EsimMgr.PreviewProfile(c.Request.Context(), aidHex, smdp, matchingID, confirmationCode, imei, progressFn)
	if err != nil {
		writeEsimPreviewErrorEvent(c, err)
		return
	}

	// 构建 preview 事件 payload
	writeEsimPreviewDoneEvent(c, result)
	_ = sseWriteRaw
}

// writeEsimPreviewDoneEvent 推送 preview 完成事件
func writeEsimPreviewDoneEvent(c *gin.Context, result *esim.PreviewResult) {
	payload := map[string]any{
		"step":           "preview",
		"msg":            "查询完成",
		"pct":            100,
		"metadata":       result.ProfileMetadata,
		"euicc_info2":    result.EuiccInfo2,
		"cc_required":    result.CCRequired,
		"euicc_cert_der": result.EuiccCertDER,
		"eum_cert_der":   result.EumCertDER,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		// 降级：手动构建
		fmt.Fprintf(c.Writer, "data: {\"step\":\"preview\",\"msg\":\"查询完成\",\"pct\":100}\n\n")
	} else {
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonBytes))
	}

	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// writeEsimPreviewErrorEvent 推送 preview 错误事件
func writeEsimPreviewErrorEvent(c *gin.Context, err error) {
	// 复用下载错误格式化逻辑
	evt := formatEsimDownloadErrorEvent(err)
	fmt.Fprintf(c.Writer, "data: %s\n\n", evt)
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}
