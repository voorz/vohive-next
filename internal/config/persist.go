package config

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "go.yaml.in/yaml/v3"
)

func UpdateNotificationInFile(path string, telegram TelegramConfig, feishu FeishuConfig, qq QQConfig, webhook WebhookConfig, bark BarkConfig, email EmailConfig, pushplus PushplusConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	root["telegram"] = map[string]any{
		"enabled":   telegram.Enabled,
		"bot_token": telegram.BotToken,
		"chat_id":   telegram.ChatID,
		"admin_id":  telegram.AdminID,
		"base_url":  telegram.BaseURL,
		"proxy":     telegram.Proxy,
	}

	root["feishu"] = map[string]any{
		"enabled":    feishu.Enabled,
		"app_id":     feishu.AppID,
		"app_secret": feishu.AppSecret,
		"chat_ids":   feishu.ChatIDs,
	}

	root["qq"] = map[string]any{
		"enabled":    qq.Enabled,
		"app_id":     qq.AppID,
		"app_secret": qq.AppSecret,
		"group_ids":  qq.GroupIDs,
		"direct_ids": qq.DirectIDs,
	}

	root["webhook"] = map[string]any{
		"enabled":       webhook.Enabled,
		"urls":          webhook.URLs,
		"secret":        webhook.Secret,
		"timeout_ms":    webhook.TimeoutMs,
		"retry_max":     webhook.RetryMax,
		"text_template": webhook.TextTemplate,
		"headers":       webhook.Headers,
	}

	root["bark"] = map[string]any{
		"enabled": bark.Enabled,
		"urls":    bark.URLs,
		"group":   bark.Group,
		"icon":    bark.Icon,
		"level":   bark.Level,
	}

	root["email"] = map[string]any{
		"enabled":      email.Enabled,
		"smtp_host":    email.SMTPHost,
		"smtp_port":    email.SMTPPort,
		"username":     email.Username,
		"password":     email.Password,
		"from_address": email.FromAddress,
		"to_addresses": email.ToAddresses,
	}

	root["pushplus"] = map[string]any{
		"enabled": pushplus.Enabled,
		"token":   pushplus.Token,
		"topic":   pushplus.Topic,
		"channel": pushplus.Channel,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
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

// UpdateUpdateRepoInFile 更新配置文件中的 update_repo 节点（GitHub release 源）
func UpdateUpdateRepoInFile(path string, owner, name string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	root["update_repo"] = map[string]any{
		"owner": owner,
		"name":  name,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
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

// UpdateSMSRateLimitInFile 更新配置文件中的 sms_rate_limit 节点
func UpdateSMSRateLimitInFile(path string, hourly, daily int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	root["sms_rate_limit"] = map[string]any{
		"hourly_limit": hourly,
		"daily_limit":  daily,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("写入临时配置文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}

// DeleteUpdateRepoInFile 从配置文件中删除 update_repo 节点
func DeleteUpdateRepoInFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	delete(root, "update_repo")

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
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

func UpdateWebCredentialsInFile(path string, username, password string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 更新 web 节点
	root["web"] = map[string]any{
		"username": username,
		"password": password,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
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

// UpdateSecurityInFile 更新配置文件中的 security 节点
func UpdateSecurityInFile(path string, loginWindowMinutes, loginMaxAttempts, tokenTTLHours int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	root["security"] = map[string]any{
		"login_window_minutes": loginWindowMinutes,
		"login_max_attempts":   loginMaxAttempts,
		"token_ttl_hours":      tokenTTLHours,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
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

// UpdateAPITokenInFile 更新配置文件中的 security.api_token 和 security.api_token_expiry
func UpdateAPITokenInFile(path string, token string, expiry int64) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	sec, ok := root["security"].(map[string]any)
	if !ok {
		sec = make(map[string]any)
	}
	sec["api_token"] = token
	sec["api_token_expiry"] = expiry
	root["security"] = sec

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
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

// UpdateServerConfigInFile 更新配置文件中的 server 节点（port/debug）
func UpdateServerConfigInFile(path string, port string, debug bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	root["server"] = map[string]any{
		"port":  port,
		"debug": debug,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
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

// UpdateSiteInFile 更新配置文件中的 site 节点
func UpdateSiteInFile(path string, name, subtitle, logoExt, faviconExt string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	root["site"] = map[string]any{
		"name":        name,
		"subtitle":    subtitle,
		"logo_ext":    logoExt,
		"favicon_ext": faviconExt,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
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
