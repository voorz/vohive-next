package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/internal/notify"
)

// buildTestSMSContext 构造一条模拟短信通知上下文
// Text 为纯短信内容（渠道自行组装格式），Sender/Source 为结构化字段
func buildTestSMSContext() notify.NotificationContext {
	now := time.Now()
	return notify.NotificationContext{
		Event:      "sms_received",
		Text:       "您的验证码为 542654，5分钟内有效，请勿泄露。",
		DeviceID:   "wwan0",
		DeviceName: "测试设备",
		Timestamp:  now,
		Sender:     "10086",
		Source:     "测试",
	}
}

// --- Telegram ---

type testTelegramRequest struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   int64  `json:"chat_id"`
	AdminID  int64  `json:"admin_id"`
	BaseURL  string `json:"base_url"`
	Proxy    string `json:"proxy"`
}

type testChannelResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// handleTestTelegramNotification
//
// @Summary      TestTelegramNotification
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/notifications/telegram/test [post]
// @Security     BearerAuth
func (s *Server) handleTestTelegramNotification(c *gin.Context) {
	var req testTelegramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
		return
	}

	if !req.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请先启用 Telegram 后再测试"})
		return
	}

	botToken := strings.TrimSpace(req.BotToken)
	if botToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Bot Token 不能为空"})
		return
	}
	if req.ChatID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Chat ID 不能为空"})
		return
	}

	ch, err := notify.NewTelegramChannel(config.TelegramConfig{
		Enabled:  true,
		BotToken: botToken,
		ChatID:   req.ChatID,
		AdminID:  req.AdminID,
		BaseURL:  strings.TrimSpace(req.BaseURL),
		Proxy:    strings.TrimSpace(req.Proxy),
	})
	if err != nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "创建 Telegram Bot 失败: " + err.Error(),
		})
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "Telegram 渠道未初始化",
		})
		return
	}
	defer ch.Close()

	// Telegram 现已实现 SendWithContext，使用结构化字段自行组装格式
	ctx := buildTestSMSContext()
	if err := ch.SendWithContext(ctx); err != nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "测试通知发送失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, testChannelResponse{
		OK:      true,
		Message: "测试通知已发送",
	})
}

// --- Pushplus ---

type testPushplusRequest struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
	Topic   string `json:"topic"`
	Channel string `json:"channel"`
}

// handleTestPushplusNotification
//
// @Summary      TestPushplusNotification
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/notifications/pushplus/test [post]
// @Security     BearerAuth
func (s *Server) handleTestPushplusNotification(c *gin.Context) {
	var req testPushplusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
		return
	}

	if !req.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请先启用 Pushplus 后再测试"})
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Token 不能为空"})
		return
	}

	ch, err := notify.NewPushplusChannel(config.PushplusConfig{
		Enabled: true,
		Token:   token,
		Topic:   strings.TrimSpace(req.Topic),
		Channel: strings.TrimSpace(req.Channel),
	})
	if err != nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "创建 Pushplus 渠道失败: " + err.Error(),
		})
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "Pushplus 渠道未初始化",
		})
		return
	}
	defer ch.Close()

	ctx := buildTestSMSContext()

	if err := ch.SendWithContext(ctx); err != nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "测试通知发送失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, testChannelResponse{
		OK:      true,
		Message: "测试通知已发送",
	})
}

// --- Feishu ---

type testFeishuRequest struct {
	Enabled   bool     `json:"enabled"`
	AppID     string   `json:"app_id"`
	AppSecret string   `json:"app_secret"`
	ChatIDs   []string `json:"chat_ids"`
}

// handleTestFeishuNotification
//
// @Summary      TestFeishuNotification
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/notifications/feishu/test [post]
// @Security     BearerAuth
func (s *Server) handleTestFeishuNotification(c *gin.Context) {
	var req testFeishuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
		return
	}

	if !req.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请先启用飞书后再测试"})
		return
	}

	appID := strings.TrimSpace(req.AppID)
	appSecret := strings.TrimSpace(req.AppSecret)
	if appID == "" || appSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "App ID 和 App Secret 不能为空"})
		return
	}
	if len(req.ChatIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "至少需要一个 Chat ID"})
		return
	}

	ch, err := notify.NewFeishuChannel(config.FeishuConfig{
		Enabled:   true,
		AppID:     appID,
		AppSecret: appSecret,
		ChatIDs:   req.ChatIDs,
	})
	if err != nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "创建飞书渠道失败: " + err.Error(),
		})
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "飞书渠道未初始化",
		})
		return
	}
	defer ch.Close()

	// 飞书未实现 SendWithContext，使用 FormatText 拼装全文
	ctx := buildTestSMSContext()
	if err := ch.Send(ctx.FormatText()); err != nil {
		c.JSON(http.StatusOK, testChannelResponse{
			OK:      false,
			Message: "测试通知发送失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, testChannelResponse{
		OK:      true,
		Message: "测试通知已发送",
	})
}

// --- QQ ---
// QQ 渠道需要 WebSocket 长连接才能发送消息（qqbot.App.Send 依赖活跃连接），
// 无法在测试 API 中即时创建+发送。返回提示信息。

// handleTestQQNotification
//
// @Summary      TestQQNotification
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/notifications/qq/test [post]
// @Security     BearerAuth
func (s *Server) handleTestQQNotification(c *gin.Context) {
	c.JSON(http.StatusOK, testChannelResponse{
		OK:      false,
		Message: "QQ 渠道需要 WebSocket 长连接，暂不支持即时测试",
	})
}
