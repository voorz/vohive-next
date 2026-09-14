package esim

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/voorz/vohive/pkg/logger"
)

// SmdpError 表示 SM-DP+ 服务器返回的结构化错误。
// 参考 NekokoLPA SmdpException，完整保留 SubjectCode/ReasonCode/SubjectIdentifier/Message。
type SmdpError struct {
	SubjectCode        string
	ReasonCode         string
	SubjectIdentifier  string
	Message            string
	HTTPStatusCode     int
	RawResponseBody    string
	Endpoint           string
}

func (e *SmdpError) Error() string {
	detail := fmt.Sprintf("SM-DP+ Error (%s, %s)", e.SubjectCode, e.ReasonCode)
	if e.HTTPStatusCode > 0 {
		detail += fmt.Sprintf(" [HTTP %d]", e.HTTPStatusCode)
	}
	return fmt.Sprintf("%s\nIdentifier: %s\nMessage: %s", detail, e.SubjectIdentifier, e.Message)
}

// smdpResponseHeader 对应 SGP.22 ES9+ 响应 JSON 中的 header 字段。
type smdpResponseHeader struct {
	FunctionExecutionStatus *smdpExecutionStatus `json:"functionExecutionStatus,omitempty"`
}

type smdpExecutionStatus struct {
	Status         string              `json:"status,omitempty"`
	StatusCodeData *smdpStatusCodeData `json:"statusCodeData,omitempty"`
}

type smdpStatusCodeData struct {
	SubjectCode       string `json:"subjectCode,omitempty"`
	ReasonCode        string `json:"reasonCode,omitempty"`
	SubjectIdentifier string `json:"subjectIdentifier,omitempty"`
	Message           string `json:"message,omitempty"`
}

// SmdpClient 是自定义的 SM-DP+ HTTP 客户端。
// 参考 NekokoLPA 的 SmdpClient，直接发起 HTTP POST 请求，
// 在响应中检查 functionExecutionStatus，保留完整的 StatusCodeData 结构信息。
//
// 与 euicc-go 的 InvokeHTTP 不同，本客户端不会用 errors.New(StatusCodeData.Error())
// 丢弃结构信息，而是返回 *SmdpError，完整保留 SubjectCode/ReasonCode。
type SmdpClient struct {
	httpClient     *http.Client
	adminProtocol  string
	timeout        time.Duration
}

// NewSmdpClient 创建一个新的 SM-DP+ HTTP 客户端。
func NewSmdpClient(timeout time.Duration) *SmdpClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &SmdpClient{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // SM-DP+ 服务器可能使用自签名证书
				},
			},
		},
		adminProtocol: "gsma/rsp/v2.5.0",
		timeout:       timeout,
	}
}

// Post 向 SM-DP+ 服务器发起 POST 请求，返回解析后的 JSON 响应。
// 如果响应中 functionExecutionStatus.status 为 "Failed"，返回 *SmdpError。
func (c *SmdpClient) Post(ctx context.Context, smdpAddress, endpoint string, bodyData map[string]any) (map[string]any, error) {
	smdpAddr := strings.TrimSpace(smdpAddress)
	if !strings.Contains(smdpAddr, "://") {
		smdpAddr = "https://" + smdpAddr
	}
	// 移除可能的尾部斜杠
	smdpAddr = strings.TrimSuffix(smdpAddr, "/")

	url := fmt.Sprintf("%s/gsma/rsp2/es9plus/%s", smdpAddr, endpoint)

	bodyBytes, err := json.Marshal(bodyData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "gsma-rsp-lpad")
	req.Header.Set("X-Admin-Protocol", c.adminProtocol)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Warn("SM-DP+ HTTP 请求失败",
			"endpoint", endpoint,
			"url", url,
			"elapsed_ms", time.Since(start).Milliseconds(),
			"err", err)
		return nil, fmt.Errorf("无法连接 SM-DP+ 服务器 %s: %w", smdpAddress, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 SM-DP+ 响应失败: %w", err)
	}

	logger.Info("SM-DP+ HTTP 响应",
		"endpoint", endpoint,
		"status_code", resp.StatusCode,
		"elapsed_ms", time.Since(start).Milliseconds(),
		"body_bytes", len(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("SM-DP+ 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if len(respBody) == 0 {
		return map[string]any{}, nil
	}

	var data map[string]any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("解析 SM-DP+ 响应 JSON 失败: %w (body: %s)", err, string(respBody))
	}

	// 检查 functionExecutionStatus
	if checkErr := checkSmdpResponse(data, resp.StatusCode, endpoint, string(respBody)); checkErr != nil {
		return data, checkErr
	}

	return data, nil
}

// checkSmdpResponse 检查 SM-DP+ 响应中的 functionExecutionStatus，
// 如果状态为 "Failed"，返回包含完整 StatusCodeData 的 *SmdpError。
// 参考 NekokoLPA SmdpClient._checkResponse。
func checkSmdpResponse(data map[string]any, httpStatus int, endpoint, rawBody string) error {
	headerRaw, ok := data["header"]
	if !ok {
		return nil
	}

	header, ok := headerRaw.(map[string]any)
	if !ok {
		return nil
	}

	statusRaw, ok := header["functionExecutionStatus"]
	if !ok {
		return nil
	}

	status, ok := statusRaw.(map[string]any)
	if !ok {
		return nil
	}

	statusStr, _ := status["status"].(string)
	if statusStr != "Failed" {
		return nil
	}

	codeDataRaw, _ := status["statusCodeData"].(map[string]any)
	if codeDataRaw == nil {
		codeDataRaw = map[string]any{}
	}

	err := &SmdpError{
		SubjectCode:       getStr(codeDataRaw, "subjectCode"),
		ReasonCode:        getStr(codeDataRaw, "reasonCode"),
		SubjectIdentifier: getStr(codeDataRaw, "subjectIdentifier"),
		Message:           getStr(codeDataRaw, "message"),
		HTTPStatusCode:    httpStatus,
		RawResponseBody:   rawBody,
		Endpoint:          endpoint,
	}

	if err.Message == "" {
		err.Message = "Unknown SM-DP+ Error"
	}

	return err
}

func getStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	if v == "" {
		return "N/A"
	}
	return v
}
