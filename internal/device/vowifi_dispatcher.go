package device

import (
	"strings"

	"github.com/voorz/ims-go/ims"

	"github.com/voorz/vohive/internal/smsnotify"
	"github.com/voorz/vohive/pkg/logger"
)

type poolVoWiFiRuntimeDispatcher struct {
	pool *Pool
}

func isCompatVoWiFiIncomingSMSLog(msg string) bool {
	return strings.HasPrefix(strings.TrimSpace(msg), "收到新短信 / VoWiFi\n")
}

// ForDevice 返回指定设备的事件处理器（捕获 deviceID）。
func (d poolVoWiFiRuntimeDispatcher) ForDevice(deviceID string) ims.EventHandler {
	return func(e ims.Event) {
		d.Dispatch(deviceID, e)
	}
}

// Dispatch 处理 ims-go 事件（A1：Event.Data 业务负载）。
func (d poolVoWiFiRuntimeDispatcher) Dispatch(deviceID string, e ims.Event) {
	if d.pool == nil {
		return
	}
	recorder := vowifiSMSHistoryRecorder{pool: d.pool}
	switch e.Type {
	case ims.EventSMSReceived:
		if data, ok := e.Data.(ims.SMSReceivedData); ok {
			if _, err := recorder.RecordReceived(deviceID, data); err != nil {
				logger.Warn("VoWiFi 上层入库入站短信失败", "device", deviceID, "sender", data.From, "err", err)
			}
		}
	case ims.EventSMSSent:
		if data, ok := e.Data.(ims.SMSSentData); ok {
			if err := recorder.RecordSent(deviceID, data); err != nil {
				logger.Warn("VoWiFi 上层入库出站短信失败", "device", deviceID, "to", data.To, "err", err)
			}
		}
	case ims.EventLocalNumberLearned:
		if data, ok := e.Data.(ims.LocalNumberLearnedData); ok {
			if err := recorder.RecordLocalNumber(deviceID, data); err != nil {
				logger.Warn("VoWiFi 上层持久化本机号码失败", "device", deviceID, "phone", data.Number, "err", err)
			}
		}
	}

	notifier := d.pool.getNotifier()
	if notifier == nil {
		return
	}

	if e.Type == ims.EventSMSReceived {
		if data, ok := e.Data.(ims.SMSReceivedData); ok {
			if smsnotify.ShouldSuppressReceivedSMS(data.Content) {
				logger.Info("VoWiFi 短信已过滤（运营商 OTA/无法解码二进制包），不进行通知推送", "device", deviceID, "sender", data.From)
				return
			}
			if withSource, ok := notifier.(SMSSourceNotifier); ok {
				withSource.NotifySMSWithSource(deviceID, data.From, data.Content, "VoWiFi", data.At)
			} else {
				notifier.NotifySMS(deviceID, data.From, data.Content, data.At)
			}
		}
	}

}
