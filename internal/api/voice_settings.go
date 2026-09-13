package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/pkg/logger"
)

// voiceGatewayResponse 语音网关配置响应
type voiceGatewayResponse struct {
	SIP struct {
		Listen      string `json:"listen"`
		Transport   string `json:"transport"`
		Realm       string `json:"realm"`
		ExternalIP  string `json:"external_ip"`
		WSListen    string `json:"ws_listen"`
		WSSListen   string `json:"wss_listen"`
		WSSCertFile string `json:"wss_cert_file"`
		WSSKeyFile  string `json:"wss_key_file"`
	} `json:"sip"`
	User struct {
		Username string `json:"username"`
		Password string `json:"password"`
		DeviceID string `json:"device_id"`
	} `json:"user"`
	Media struct {
		RTPPortMin int      `json:"rtp_port_min"`
		RTPPortMax int      `json:"rtp_port_max"`
		Codecs     []string `json:"codecs"`
	} `json:"media"`
	LinphonePush struct {
		LinphoneUser     string `json:"linphone_user"`
		LinphonePassword string `json:"linphone_password"`
	} `json:"linphone_push"`
}

// updateVoiceGatewayRequest 更新语音网关配置请求
type updateVoiceGatewayRequest struct {
	SIP struct {
		Listen      string `json:"listen"`
		Transport   string `json:"transport"`
		Realm       string `json:"realm"`
		ExternalIP  string `json:"external_ip"`
		WSListen    string `json:"ws_listen"`
		WSSListen   string `json:"wss_listen"`
		WSSCertFile string `json:"wss_cert_file"`
		WSSKeyFile  string `json:"wss_key_file"`
	} `json:"sip"`
	User struct {
		Password string `json:"password"` // 可选：空则不修改
		DeviceID string `json:"device_id"`
	} `json:"user"`
	Media struct {
		RTPPortMin int      `json:"rtp_port_min"`
		RTPPortMax int      `json:"rtp_port_max"`
		Codecs     []string `json:"codecs"`
	} `json:"media"`
	LinphonePush struct {
		LinphoneUser     string `json:"linphone_user"`
		LinphonePassword string `json:"linphone_password"`
	} `json:"linphone_push"`
}

// handleGetVoiceGateway GET /api/settings/voice-gateway
// handleGetVoiceGateway
//
// @Summary      GetVoiceGateway
// @Tags         settings, mcp-auto
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/voice-gateway [get]
// @Security     BearerAuth
func (s *Server) handleGetVoiceGateway(c *gin.Context) {
	var resp voiceGatewayResponse

	// 默认值
	resp.SIP.Listen = "0.0.0.0:5060"
	resp.SIP.Transport = "udp"
	resp.SIP.Realm = "vohive.local"
	resp.SIP.WSListen = "0.0.0.0:5061"
	resp.Media.RTPPortMin = 10000
	resp.Media.RTPPortMax = 20000
	resp.Media.Codecs = []string{"PCMU/8000", "PCMA/8000"}

	// username 永远使用 Web 管理员用户名
	resp.User.Username = s.auth.Username

	vg, err := db.GetVoiceGateway()
	if err != nil {
		logger.Error("读取语音网关配置失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "读取配置失败"})
		return
	}

	if vg != nil {
		if vg.SIPListen != "" {
			resp.SIP.Listen = vg.SIPListen
		}
		if vg.SIPTransport != "" {
			resp.SIP.Transport = vg.SIPTransport
		}
		if vg.SIPRealm != "" {
			resp.SIP.Realm = vg.SIPRealm
		}
		resp.SIP.ExternalIP = vg.SIPExternalIP
		if vg.WSListen != "" {
			resp.SIP.WSListen = vg.WSListen
		}
		if vg.WSSListen != "" {
			resp.SIP.WSSListen = vg.WSSListen
		}
		resp.SIP.WSSCertFile = vg.WSSCertFile
		resp.SIP.WSSKeyFile = vg.WSSKeyFile

		resp.User.Password = vg.Password
		resp.User.DeviceID = vg.DeviceID

		if vg.RTPPortMin > 0 {
			resp.Media.RTPPortMin = vg.RTPPortMin
		}
		if vg.RTPPortMax > 0 {
			resp.Media.RTPPortMax = vg.RTPPortMax
		}
		if vg.Codecs != "" {
			var codecs []string
			if json.Unmarshal([]byte(vg.Codecs), &codecs) == nil {
				resp.Media.Codecs = codecs
			}
		}

		resp.LinphonePush.LinphoneUser = vg.LinphoneUser
		resp.LinphonePush.LinphonePassword = vg.LinphonePassword
	}

	c.JSON(http.StatusOK, resp)
}

// handleUpdateVoiceGateway PUT /api/settings/voice-gateway
// handleUpdateVoiceGateway
//
// @Summary      UpdateVoiceGateway
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/voice-gateway [put]
// @Security     BearerAuth
func (s *Server) handleUpdateVoiceGateway(c *gin.Context) {
	var req updateVoiceGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}

	if strings.TrimSpace(req.SIP.Realm) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "sip.realm 不能为空"})
		return
	}

	// 从 DB 读取现有配置（或创建新的）
	vg, err := db.GetVoiceGateway()
	if err != nil {
		logger.Error("读取语音网关配置失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "读取配置失败"})
		return
	}
	if vg == nil {
		vg = &db.VoiceGateway{}
	}

	// 更新 SIP
	vg.SIPListen = strings.TrimSpace(req.SIP.Listen)
	vg.SIPTransport = strings.TrimSpace(req.SIP.Transport)
	vg.SIPRealm = strings.TrimSpace(req.SIP.Realm)
	vg.SIPExternalIP = strings.TrimSpace(req.SIP.ExternalIP)
	vg.WSListen = strings.TrimSpace(req.SIP.WSListen)
	vg.WSSListen = strings.TrimSpace(req.SIP.WSSListen)
	vg.WSSCertFile = strings.TrimSpace(req.SIP.WSSCertFile)
	vg.WSSKeyFile = strings.TrimSpace(req.SIP.WSSKeyFile)

	// username 永远使用 Web 管理员用户名，不可自定义
	vg.Username = s.auth.Username

	// 密码：如果前端传了非空值则更新，空值则保留现有
	if strings.TrimSpace(req.User.Password) != "" {
		vg.Password = strings.TrimSpace(req.User.Password)
	}
	vg.DeviceID = strings.TrimSpace(req.User.DeviceID)

	// 媒体
	vg.RTPPortMin = req.Media.RTPPortMin
	vg.RTPPortMax = req.Media.RTPPortMax
	codecsJSON, _ := json.Marshal(req.Media.Codecs)
	vg.Codecs = string(codecsJSON)

	// Linphone 官方推送账户
	vg.LinphoneUser = strings.TrimSpace(req.LinphonePush.LinphoneUser)
	vg.LinphonePassword = strings.TrimSpace(req.LinphonePush.LinphonePassword)

	// 默认值
	if vg.SIPListen == "" {
		vg.SIPListen = "0.0.0.0:5060"
	}
	if vg.SIPTransport == "" {
		vg.SIPTransport = "udp"
	}
	if vg.RTPPortMin == 0 {
		vg.RTPPortMin = 10000
	}
	if vg.RTPPortMax == 0 {
		vg.RTPPortMax = 20000
	}
	if vg.Codecs == "" || vg.Codecs == "null" {
		vg.Codecs = `["PCMU/8000","PCMA/8000"]`
	}

	if err := db.SaveVoiceGateway(vg); err != nil {
		logger.Error("写入语音网关配置失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "写入配置失败: " + err.Error()})
		return
	}

	// 热更新 SIP Registrar 内存中的用户配置
	if s.pushNotifier != nil {
		s.pushNotifier.UpdateUser(vg.Username, vg.Password, vg.DeviceID, "")
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "applied": true})
}

// handleRegenerateVoicePassword POST /api/settings/voice-gateway/regenerate-password
// handleRegenerateVoicePassword
//
// @Summary      RegenerateVoicePassword
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/voice-gateway/regenerate-password [post]
// @Security     BearerAuth
func (s *Server) handleRegenerateVoicePassword(c *gin.Context) {
	vg, err := db.GetVoiceGateway()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "读取配置失败"})
		return
	}
	if vg == nil {
		vg = &db.VoiceGateway{}
	}

	// 生成 4 位随机码（大写字母 + 数字，排除易混淆字符 I/O/0/1）
	vg.Password = db.GenerateVoicePassword(4)
	vg.Username = s.auth.Username

	if err := db.SaveVoiceGateway(vg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "保存失败: " + err.Error()})
		return
	}

	logger.Info("语音网关授权码已重新生成", "username", vg.Username, "ip", c.ClientIP())

	// 热更新 SIP Registrar 内存中的密码
	if s.pushNotifier != nil {
		s.pushNotifier.UpdateUser(vg.Username, vg.Password, vg.DeviceID, "")
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"password": vg.Password,
	})
}

// handleTestLinphonePush POST /api/settings/voice-gateway/test-linphone-push
// 测试 Linphone 官方推送 API 密钥是否有效（使用 API Key 认证）
// handleTestLinphonePush
//
// @Summary      TestLinphonePush
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/voice-gateway/test-linphone-push [post]
// @Security     BearerAuth
func (s *Server) handleTestLinphonePush(c *gin.Context) {
	var req struct {
		LinphonePassword string `json:"linphone_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}

	apiKey := strings.TrimSpace(req.LinphonePassword)
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "API 密钥不能为空"})
		return
	}

	// 使用 API Key 认证，调用 GET /api/accounts/me 验证密钥有效性
	transport := &http.Transport{
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          1,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{Timeout: 15 * time.Second, Transport: transport}
	url := "https://subscribe.linphone.org/api/accounts/me"

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "创建请求失败: " + err.Error()})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"status": "error", "message": "连接 sip.linphone.org 失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	logger.Info("Linphone API Key 测试", "status", resp.StatusCode, "body", string(bodyBytes[:min(len(bodyBytes), 300)]))

	if resp.StatusCode == http.StatusUnauthorized {
		c.JSON(http.StatusOK, gin.H{"status": "error", "message": "API 密钥无效或已过期 (HTTP 401)"})
		return
	}

	if resp.StatusCode == http.StatusForbidden {
		c.JSON(http.StatusOK, gin.H{"status": "error", "message": "API 密钥权限不足 (HTTP 403)"})
		return
	}

	if resp.StatusCode >= 400 {
		c.JSON(http.StatusOK, gin.H{"status": "error", "message": fmt.Sprintf("服务器拒绝 (HTTP %d): %s", resp.StatusCode, string(bodyBytes[:min(len(bodyBytes), 200)]))})
		return
	}

	// 解析返回的账户信息
	var accountInfo struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		SIP      string `json:"sip"`
	}

	// 如果有已配置的设备，异步尝试发送测试推送
	pushMsg := ""
	if s.pushNotifier != nil {
		vg, _ := db.GetVoiceGateway()
		if vg != nil && vg.DeviceID != "" {
			testCallID := fmt.Sprintf("test-push-%d", time.Now().UnixNano())
			go func() {
				err := s.pushNotifier.SendPushNotification(vg.DeviceID, testCallID, "test", "test")
				if err != nil {
					logger.Warn("Linphone 测试推送发送失败", "device", vg.DeviceID, "err", err)
				} else {
					logger.Info("Linphone 测试推送已发送", "device", vg.DeviceID, "call_id", testCallID)
				}
			}()
			pushMsg = "，测试推送已发送"
		} else {
			pushMsg = "（未配置设备，跳过测试推送）"
		}
	}

	if json.Unmarshal(bodyBytes, &accountInfo) == nil && accountInfo.Username != "" {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": fmt.Sprintf("验证通过，账户: %s%s", accountInfo.Username, pushMsg),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Linphone API 密钥验证通过" + pushMsg})
}
