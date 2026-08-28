package notify

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/pkg/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramChannel 实现 Channel 接口的 Telegram 通知渠道
type TelegramChannel struct {
	api      *tgbotapi.BotAPI
	chatID   int64
	handlers map[string]CommandHandler
}

// NewTelegramChannel 根据配置创建 Telegram 渠道
func NewTelegramChannel(cfg config.TelegramConfig) (*TelegramChannel, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	endpoint := tgbotapi.APIEndpoint
	if cfg.BaseURL != "" {
		endpoint = cfg.BaseURL
		if !strings.Contains(endpoint, "bot%s/%s") {
			endpoint = strings.TrimSuffix(endpoint, "/") + "/bot%s/%s"
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			logger.Error("解析 Telegram 代理地址失败", "proxy", cfg.Proxy, "err", err)
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
			logger.Info("Telegram Bot 使用代理", "proxy", cfg.Proxy)
		}
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}

	bot, err := tgbotapi.NewBotAPIWithClient(cfg.BotToken, endpoint, httpClient)
	if err != nil {
		msg := err.Error()
		if cfg.BotToken != "" {
			msg = strings.ReplaceAll(msg, cfg.BotToken, "<redacted>")
		}
		return nil, fmt.Errorf("创建 telegram bot 失败: %s", msg)
	}

	logger.Info("已授权账户 (TG)", "username", bot.Self.UserName)

	return &TelegramChannel{
		api:      bot,
		chatID:   cfg.ChatID,
		handlers: make(map[string]CommandHandler),
	}, nil
}

func (t *TelegramChannel) Name() string { return "telegram" }

func buildTelegramTextMessage(chatID int64, text string) tgbotapi.MessageConfig {
	// 过滤非法的 UTF-8 字符，防止 Telegram API 报错
	cleanText := strings.Map(func(r rune) rune {
		if r == utf8.RuneError {
			return -1 // 丢弃非法字符
		}
		return r
	}, text)

	// Send 路径（命令回复等）：纯文本转义，验证码用代码块高亮
	code := extractVerificationCode(cleanText)

	var buf strings.Builder
	buf.WriteString(escapeMarkdownV2(cleanText))
	if code != "" {
	buf.WriteString("\n\n")
	buf.WriteString(escapeMarkdownV2("验证码："))
	buf.WriteString("```\n")
	buf.WriteString(code)
	buf.WriteString("\n```")
	}

	msg := tgbotapi.NewMessage(chatID, buf.String())
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	return msg
}

// verificationCodePattern 匹配短信中的验证码：
// 1. 关键词（验证码/verification code/code/OTP/动态码/校验码/验证码为）后跟 4-8 位数字
// 2. 独立的 4-8 位纯数字（前后有空白或行首行尾）
var verificationCodePattern = regexp.MustCompile(
	`(?i)(?:验证码|verification\s*code|code|OTP|动态码|校验码|验证码为)\s*[:：]?\s*(\d{4,8})` +
		`|(?:(?:^|\s)(\d{4,8})(?:\s|$|\n))`,
)

// markdownV2EscapeChars 需要转义的字符（MarkdownV2 规范）
const markdownV2EscapeChars = `_*[]()~` + "`" + `>#+-=|{}.!`

// escapeMarkdownV2 转义 MarkdownV2 特殊字符
func escapeMarkdownV2(s string) string {
	var buf strings.Builder
	for _, r := range s {
		if strings.ContainsRune(markdownV2EscapeChars, r) {
			buf.WriteByte('\\')
		}
		buf.WriteRune(r)
	}
	return buf.String()
}

// renderTelegramMarkdownV2 将通知上下文渲染为 Telegram MarkdownV2
// 格式：元信息正常显示 + 短信内容代码块 + 验证码高亮
func renderTelegramMarkdownV2(ctx NotificationContext) string {
	var buf strings.Builder

	// 元信息（正常文本，需转义 MarkdownV2 特殊字符）
	source := ctx.Source
	if source == "" {
		source = "通知"
	}

	buf.WriteString(escapeMarkdownV2("收到新短信 / " + source))
	buf.WriteString("\n")
	buf.WriteString(escapeMarkdownV2("设备  " + ctx.DeviceLabel()))
	buf.WriteString("\n")
	buf.WriteString(escapeMarkdownV2("号码  " + ctx.Sender))
	buf.WriteString("\n")
	buf.WriteString(escapeMarkdownV2("时间  " + ctx.Timestamp.Format("2006-01-02 15:04:05")))
	buf.WriteString("\n\n")

	// 短信内容用代码块包裹（``` 后换行，内容在新行，避免同行文字被标记为代码类型）
	buf.WriteString("```\n")
	buf.WriteString(ctx.Text)
	buf.WriteString("\n```")

	// 如果提取到验证码，在下方单独用代码块高亮
	code := extractVerificationCode(ctx.Text)
	if code != "" {
		buf.WriteString("\n")
	buf.WriteString(escapeMarkdownV2("验证码："))
	buf.WriteString("```\n")
	buf.WriteString(code)
	buf.WriteString("\n```")
	}

	return buf.String()
}

// extractVerificationCode 从文本中提取验证码数字（4-8位）
func extractVerificationCode(text string) string {
	matches := verificationCodePattern.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		// m[1] = 关键词后的数字组, m[2] = 独立数字组
		if m[1] != "" {
			return m[1]
		}
		if m[2] != "" {
			return m[2]
		}
	}
	return ""
}

// SendWithContext 实现 contextualChannel 接口
// Telegram 自行组装格式：元信息正常显示 + 短信内容代码块 + 验证码高亮
func (t *TelegramChannel) SendWithContext(ctx NotificationContext) error {
	if t == nil || t.api == nil {
		return nil
	}

	rendered := renderTelegramMarkdownV2(ctx)
	msg := tgbotapi.NewMessage(t.chatID, rendered)
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	_, err := t.api.Send(msg)
	if err != nil {
		logger.Error("发送 telegram 消息失败", "err", err)
		return err
	}
	return nil
}

func (t *TelegramChannel) Send(text string) error {
	if t == nil || t.api == nil {
		return nil
	}

	msg := buildTelegramTextMessage(t.chatID, text)
	_, err := t.api.Send(msg)
	if err != nil {
		logger.Error("发送 telegram 消息失败", "err", err)
		return err
	}
	return nil
}

func (t *TelegramChannel) RegisterCommand(cmd string, handler CommandHandler) {
	if t == nil {
		return
	}
	t.handlers[cmd] = handler
	logger.Info("注册 Telegram 命令", "command", "/"+cmd)
}

// tgCommandContext 实现了 CommandContext 接口
type tgCommandContext struct {
	channel *TelegramChannel
}

func (c *tgCommandContext) Reply(text string) {
	if c == nil || c.channel == nil {
		return
	}
	// 避免 Telegram API 短时阻塞拖住命令处理主循环
	go func() {
		_ = c.channel.Send(text)
	}()
}

// Start 启动 Telegram long-polling 命令监听（阻塞式）
func (t *TelegramChannel) Start() error {
	if t == nil || t.api == nil {
		return nil
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := t.api.GetUpdatesChan(u)
	logger.Info("Telegram Bot 命令监听已启动")

	for update := range updates {
		if update.Message == nil || !update.Message.IsCommand() {
			continue
		}

		// 仅处理来自授权用户的命令
		if update.Message.Chat.ID != t.chatID {
			continue
		}

		command := update.Message.Command()
		args := strings.Fields(update.Message.CommandArguments())

		logger.Info("收到 Telegram 命令", "command", command, "args", args)

		handler, ok := t.handlers[command]
		if !ok {
			ctx := &tgCommandContext{channel: t}
			ctx.Reply(unknownCommandReply(command))
			continue
		}

		ctx := &tgCommandContext{channel: t}
		response := t.invokeHandler(handler, ctx, command, args)
		if response != "" {
			ctx.Reply(response)
		}
	}
	return nil
}

// invokeHandler 执行命令处理器，捕获 panic 避免单个命令异常拖垮整个长轮询循环
func (t *TelegramChannel) invokeHandler(handler CommandHandler, ctx CommandContext, command string, args []string) (response string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Telegram 命令处理器 panic", "command", command, "recover", r)
			response = ""
		}
	}()
	return handler(ctx, args)
}

func (t *TelegramChannel) Close() error {
	if t == nil || t.api == nil {
		return nil
	}
	t.api.StopReceivingUpdates()
	return nil
}
