package api

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"

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

	c.JSON(http.StatusOK, gin.H{"status": "ok", "applied": true})
}

// handleRegenerateVoicePassword POST /api/settings/voice-gateway/regenerate-password
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
	vg.Password = generateVoicePassword(4)
	vg.Username = s.auth.Username

	if err := db.SaveVoiceGateway(vg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "保存失败: " + err.Error()})
		return
	}

	logger.Info("语音网关授权码已重新生成", "username", vg.Username, "ip", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"password": vg.Password,
	})
}

// generateVoicePassword 生成 n 位随机码（大写字母 + 数字，排除 I/O/0/1）
func generateVoicePassword(n int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}
