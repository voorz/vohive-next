package device

import "github.com/voorz/vohive/internal/vowifihost"

func (p *Pool) voWiFiHost() *vowifihost.Manager {
	if p == nil {
		return vowifihost.NewManager()
	}
	if p.vowifiHost == nil {
		p.vowifiHost = vowifihost.NewManager()
	}
	return p.vowifiHost
}

// SetVoWiFiCallEventPublisher 注入通话事件发布器给 VoWiFi 管理器
func (p *Pool) SetVoWiFiCallEventPublisher(pub vowifihost.CallEventPublisher) {
	if p == nil || p.vowifiHost == nil {
		return
	}
	p.vowifiHost.SetCallEventPublisher(pub)
}
