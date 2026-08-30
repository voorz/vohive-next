package device

import (
	"github.com/voorz/vohive/internal/vowifihost"
)

func (p *Pool) voWiFiHost() *vowifihost.Manager {
	if p == nil {
		return vowifihost.NewManager()
	}
	if p.vowifiHost == nil {
		p.vowifiHost = vowifihost.NewManager()
	}
	return p.vowifiHost
}

// GetDesiredRecoverSnapshot 返回指定设备的 VoWiFi 目标态恢复快照（含下次重试时间）。
// 供 API 层序列化倒计时秒数使用。
func (p *Pool) GetDesiredRecoverSnapshot(deviceID string) (vowifihost.DesiredRecoverSnapshot, bool) {
	return p.voWiFiHost().DesiredRecoverState(deviceID)
}

// SetVoWiFiCallEventPublisher 注入通话事件发布器给 VoWiFi 管理器
// 同时保存到 Pool，供 SIP 回调中追踪通话事件
func (p *Pool) SetVoWiFiCallEventPublisher(pub vowifihost.CallEventPublisher) {
	if p == nil {
		return
	}
	p.callEventPub = pub
	if p.vowifiHost != nil {
		p.vowifiHost.SetCallEventPublisher(pub)
	}
}
