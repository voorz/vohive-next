package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/config"
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
	Users []voiceUserResponse `json:"users"`
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

type voiceUserResponse struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	DeviceID    string `json:"device_id"`
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
	Users []voiceUserResponse `json:"users"`
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
	resp.SIP.Realm = "voice.local"
	resp.SIP.WSListen = "0.0.0.0:5061"
	resp.Media.RTPPortMin = 10000
	resp.Media.RTPPortMax = 20000
	resp.Media.Codecs = []string{"PCMU/8000", "PCMA/8000"}

	if s.fullCfg != nil {
		vg := s.fullCfg.VoWiFi.VoiceGateway
		if vg.SIP.Listen != "" {
			resp.SIP.Listen = vg.SIP.Listen
		}
		if vg.SIP.Transport != "" {
			resp.SIP.Transport = vg.SIP.Transport
		}
		if vg.SIP.Realm != "" {
			resp.SIP.Realm = vg.SIP.Realm
		}
		resp.SIP.ExternalIP = vg.SIP.ExternalIP
		if vg.SIP.WSListen != "" {
			resp.SIP.WSListen = vg.SIP.WSListen
		}
		if vg.SIP.WSSListen != "" {
			resp.SIP.WSSListen = vg.SIP.WSSListen
		}
		resp.SIP.WSSCertFile = vg.SIP.WSSCertFile
		resp.SIP.WSSKeyFile = vg.SIP.WSSKeyFile

		for _, u := range vg.Users {
			resp.Users = append(resp.Users, voiceUserResponse{
				Username:    u.Username,
				Password:    u.Password,
				DisplayName: u.DisplayName,
				DeviceID:    u.DeviceID,
			})
		}

		if vg.Media.RTPPortMin > 0 {
			resp.Media.RTPPortMin = vg.Media.RTPPortMin
		}
		if vg.Media.RTPPortMax > 0 {
			resp.Media.RTPPortMax = vg.Media.RTPPortMax
		}
		if len(vg.Media.Codecs) > 0 {
			resp.Media.Codecs = vg.Media.Codecs
		}

		resp.LinphonePush.LinphoneUser = vg.LinphonePush.LinphoneUser
		resp.LinphonePush.LinphonePassword = vg.LinphonePush.LinphonePassword
	}

	if resp.Users == nil {
		resp.Users = []voiceUserResponse{}
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

	// 验证必填字段
	if strings.TrimSpace(req.SIP.Realm) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "sip.realm 不能为空"})
		return
	}

	// 构建 config 结构
	var cfg config.VoWiFiVoiceGatewayConfig
	cfg.SIP.Listen = strings.TrimSpace(req.SIP.Listen)
	cfg.SIP.Transport = strings.TrimSpace(req.SIP.Transport)
	cfg.SIP.Realm = strings.TrimSpace(req.SIP.Realm)
	cfg.SIP.ExternalIP = strings.TrimSpace(req.SIP.ExternalIP)
	cfg.SIP.WSListen = strings.TrimSpace(req.SIP.WSListen)
	cfg.SIP.WSSListen = strings.TrimSpace(req.SIP.WSSListen)
	cfg.SIP.WSSCertFile = strings.TrimSpace(req.SIP.WSSCertFile)
	cfg.SIP.WSSKeyFile = strings.TrimSpace(req.SIP.WSSKeyFile)

	for _, u := range req.Users {
		username := strings.TrimSpace(u.Username)
		if username == "" {
			continue
		}
		cfg.Users = append(cfg.Users, config.VoWiFiVoiceUserConfig{
			Username:    username,
			Password:    strings.TrimSpace(u.Password),
			DisplayName: strings.TrimSpace(u.DisplayName),
			DeviceID:    strings.TrimSpace(u.DeviceID),
		})
	}

	cfg.Media.RTPPortMin = req.Media.RTPPortMin
	cfg.Media.RTPPortMax = req.Media.RTPPortMax
	cfg.Media.Codecs = req.Media.Codecs

	cfg.LinphonePush.LinphoneUser = strings.TrimSpace(req.LinphonePush.LinphoneUser)
	cfg.LinphonePush.LinphonePassword = strings.TrimSpace(req.LinphonePush.LinphonePassword)

	// 默认值
	if cfg.SIP.Listen == "" {
		cfg.SIP.Listen = "0.0.0.0:5060"
	}
	if cfg.SIP.Transport == "" {
		cfg.SIP.Transport = "udp"
	}
	if cfg.Media.RTPPortMin == 0 {
		cfg.Media.RTPPortMin = 10000
	}
	if cfg.Media.RTPPortMax == 0 {
		cfg.Media.RTPPortMax = 20000
	}
	if len(cfg.Media.Codecs) == 0 {
		cfg.Media.Codecs = []string{"PCMU/8000", "PCMA/8000"}
	}

	if err := config.UpdateVoiceGatewayInFile(s.configPath, cfg); err != nil {
		logger.Error("写入语音网关配置失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "写入配置文件失败: " + err.Error()})
		return
	}

	// 更新内存中的配置
	if s.fullCfg != nil {
		s.fullCfg.VoWiFi.VoiceGateway = cfg
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "applied": true})
}
