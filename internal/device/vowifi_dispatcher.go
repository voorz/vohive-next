package device

import (
	"context"
	"strings"

	"github.com/voorz/vohive/internal/smsnotify"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vowifi-core/runtimehost/eventhost"
)

type poolVoWiFiRuntimeDispatcher struct {
	pool *Pool
}

func isCompatVoWiFiIncomingSMSLog(msg string) bool {
	return strings.HasPrefix(strings.TrimSpace(msg), "收到新短信 / VoWiFi\n")
}

func (d poolVoWiFiRuntimeDispatcher) Dispatch(ctx context.Context, e eventhost.Event) {
	if d.pool == nil || e == nil {
		return
	}
	recorder := vowifiSMSHistoryRecorder{pool: d.pool}
	switch v := e.(type) {
	case eventhost.SMSReceived:
		_, err := recorder.RecordReceived(v)
		if err != nil {
			logger.Warn("VoWiFi 上层入库入站短信失败", "device", v.DevID, "sender", v.Sender, "err", err)
		}
	case eventhost.SMSSent:
		if err := recorder.RecordSent(v); err != nil {
			logger.Warn("VoWiFi 上层入库出站短信失败", "device", v.DevID, "to", v.TargetURI, "err", err)
		}
	case eventhost.LocalNumberLearned:
		if err := recorder.RecordLocalNumberLearned(v); err != nil {
			logger.Warn("VoWiFi 上层持久化本机号码失败", "device", v.DevID, "imsi", v.IMSI, "phone", v.Number, "err", err)
		}
	}

	notifier := d.pool.getNotifier()
	if notifier == nil {
		return
	}

	if sms, ok := e.(eventhost.SMSReceived); ok {
		if smsnotify.ShouldSuppressReceivedSMS(sms.Content) {
			logger.Info("VoWiFi 短信已过滤（运营商 OTA/无法解码二进制包），不进行通知推送", "device", sms.DevID, "sender", sms.Sender)
			return
		}
		if withSource, ok := notifier.(SMSSourceNotifier); ok {
			withSource.NotifySMSWithSource(sms.DevID, sms.Sender, sms.Content, "VoWiFi", sms.Time)
		} else {
			notifier.NotifySMS(sms.DevID, sms.Sender, sms.Content, sms.Time)
		}
	}

	logNotify, ok := e.(eventhost.LogNotify)
	if !ok || strings.TrimSpace(logNotify.Message) == "" || isCompatVoWiFiIncomingSMSLog(logNotify.Message) {
		return
	}
	notifier.NotifyRaw(logNotify.Message)
}
