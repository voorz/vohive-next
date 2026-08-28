package notify

import (
	"testing"
	"time"
)

func TestBuildTelegramTextMessageKeepsRawSMSContent(t *testing.T) {
	t.Parallel()

	text := "📩 收到新短信\n内容: <#> 验证码 #123456 <b>TAG</b>"
	msg := buildTelegramTextMessage(12345, text)

	if msg.ParseMode != "MarkdownV2" {
		t.Fatalf("ParseMode = %q, want MarkdownV2", msg.ParseMode)
	}
	if msg.ChatID != 12345 {
		t.Fatalf("ChatID = %d, want 12345", msg.ChatID)
	}
}

func TestRenderTelegramMarkdownV2WithContext(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 8, 28, 21, 20, 0, 0, time.UTC)

	cases := []struct {
		name string
		ctx  NotificationContext
		want string
	}{
		{
			name: "sms_with_code",
			ctx: NotificationContext{
				Event:      "sms_received",
				Text:       "您的验证码为 542654，5分钟内有效，请勿泄露。",
				DeviceID:   "wwan0",
				DeviceName: "测试设备",
				Timestamp:  ts,
				Sender:     "10086",
				Source:     "蜂窝",
			},
			// 元信息正常 + 内容代码块 + 验证码高亮
			want: "收到新短信 / 蜂窝\n设备  测试设备 \\(wwan0\\)\n号码  10086\n时间  2026\\-08\\-28 21:20:00\n\n```\n您的验证码为 542654，5分钟内有效，请勿泄露。\n```\n验证码：```\n542654\n```",
		},
		{
			name: "sms_no_code",
			ctx: NotificationContext{
				Event:      "sms_received",
				Text:       "余额查询：当前余额100元",
				DeviceID:   "wwan0",
				DeviceName: "",
				Timestamp:  ts,
				Sender:     "10086",
				Source:     "VoWiFi",
			},
			want: "收到新短信 / VoWiFi\n设备  wwan0\n号码  10086\n时间  2026\\-08\\-28 21:20:00\n\n```\n余额查询：当前余额100元\n```",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderTelegramMarkdownV2(tc.ctx)
			if got != tc.want {
				t.Fatalf("renderTelegramMarkdownV2() =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

func TestExtractVerificationCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"您的验证码为 542654", "542654"},
		{"Your verification code: 483729", "483729"},
		{"code 1234 valid", "1234"},
		{"Please use 654321 to login", "654321"},
		{"No code here", ""},
	}

	for _, tc := range cases {
		got := extractVerificationCode(tc.input)
		if got != tc.want {
			t.Fatalf("extractVerificationCode(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNotificationContextFormatText(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	ctx := NotificationContext{
		Event:      "sms_received",
		Text:       "hello",
		DeviceID:   "wwan0",
		DeviceName: "客厅主卡",
		Timestamp:  ts,
		Sender:     "+8613800000000",
		Source:     "VoWiFi",
	}

	got := ctx.FormatText()
	want := "收到新短信 / VoWiFi\n设备  wwan0\n号码  +8613800000000\n时间  2026-04-13 12:00:00\n内容  hello"
	if got != want {
		t.Fatalf("FormatText() = %q, want %q", got, want)
	}
}

func TestUnknownCommandReplyUsesPlainTemplate(t *testing.T) {
	t.Parallel()

	got := unknownCommandReply("badcmd")
	want := "未知命令 / badcmd\n提示    请检查命令名或使用 /list、/status、/send 等已注册命令"
	if got != want {
		t.Fatalf("unknownCommandReply() = %q, want %q", got, want)
	}
}
