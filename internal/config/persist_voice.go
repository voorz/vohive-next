package config

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "go.yaml.in/yaml/v3"
)

// UpdateVoiceGatewayInFile 更新配置文件中的 voice_gateway 节点
func UpdateVoiceGatewayInFile(path string, cfg VoWiFiVoiceGatewayConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 构建 voice_gateway 节点
	vg := map[string]any{}

	// SIP
	sip := map[string]any{
		"listen":       cfg.SIP.Listen,
		"transport":    cfg.SIP.Transport,
		"realm":        cfg.SIP.Realm,
		"external_ip":  cfg.SIP.ExternalIP,
		"ws_listen":    cfg.SIP.WSListen,
		"wss_listen":   cfg.SIP.WSSListen,
		"wss_cert_file": cfg.SIP.WSSCertFile,
		"wss_key_file":  cfg.SIP.WSSKeyFile,
	}
	vg["sip"] = sip

	// Users
	users := make([]any, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		users = append(users, map[string]any{
			"username":     u.Username,
			"password":     u.Password,
			"display_name": u.DisplayName,
			"device_id":    u.DeviceID,
		})
	}
	vg["users"] = users

	// Media
	vg["media"] = map[string]any{
		"rtp_port_min": cfg.Media.RTPPortMin,
		"rtp_port_max": cfg.Media.RTPPortMax,
		"codecs":       cfg.Media.Codecs,
	}

	// LinphonePush
	vg["linphone_push"] = map[string]any{
		"linphone_user":     cfg.LinphonePush.LinphoneUser,
		"linphone_password": cfg.LinphonePush.LinphonePassword,
	}

	root["voice_gateway"] = vg

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("写入临时配置文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}
