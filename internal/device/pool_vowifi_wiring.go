package device

import (
	"fmt"

	"github.com/voorz/sipgo/sip"
	"github.com/voorz/vohive/internal/sipgw"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vowifi-core/runtimehost/voicehost"
)

// SetVoiceGateway 注入 VoWiFi 语音网关，用于优先走 IMS 外呼/挂断路径。
func (p *Pool) SetVoiceGateway(g *voicehost.Gateway) {
	p.mu.Lock()
	p.voiceGateway = g
	p.mu.Unlock()
	p.voWiFiHost().ConfigureRuntimeDependencies(g, vowifiDeliveryStore{}, poolVoWiFiRuntimeDispatcher{pool: p})
}

// UpdateVoWiFiBehavior 热更新 VoWiFi 行为参数到运行时。
func (p *Pool) UpdateVoWiFiBehavior(ikeRetryCount int) {
	if p == nil {
		return
	}
	p.mu.RLock()
	host := p.vowifiHost
	p.mu.RUnlock()
	if host != nil {
		host.SetIKERetryCount(ikeRetryCount)
	}
}

// SetVoWiFiSIPRegistrar 注入 sipgw.Registrar，用于 VoWiFi 来电转发到 Linphone。
func (p *Pool) SetVoWiFiSIPRegistrar(r *sipgw.Registrar) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.sipRegistrar = r
	p.mu.Unlock()
	p.voWiFiHost().SetSIPRegistrar(r)
}

// GetVoiceGateway 返回绑定的 VoiceGateway 实例
func (p *Pool) GetVoiceGateway() *voicehost.Gateway {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.voiceGateway
}

// SetSIPRegistrar 注入 SIP 注册器，统一注册通话路由回调。
// 此方法是 SIP 回调的唯一注册点——main.go 不应再单独注册 onInvite/onBye/onCancel。
// 回调按设备状态路由：VoWiFi 在线→VoWiFi VoiceGateway，4G/LTE→CS 域语音桥接。
func (p *Pool) SetSIPRegistrar(r *sipgw.Registrar) {
	p.mu.Lock()
	p.sipRegistrar = r
	for _, w := range p.workers {
		logger.Debug(fmt.Sprintf("[%s] SetSIPRegistrar 回扫: AudioDevice=%q, CSCallMgr=%v, Modem=%v", w.ID, w.Config.AudioDevice, w.CSCallMgr != nil, w.Modem != nil))
		if w.CSCallMgr == nil {
			w.CSCallMgr = newCSCallManagerForWorker(w, r)
			if w.CSCallMgr != nil {
				logger.Info(fmt.Sprintf("[%s] 已启用 CS 域语音桥接 (AudioDev: %s)", w.ID, w.Config.AudioDevice))
			}
		}
	}
	p.mu.Unlock()

	// sipCalleeFromReq 从 SIP Request 中提取被叫号码
	sipCalleeFromReq := func(req *sip.Request) string {
		if req == nil {
			return ""
		}
		if to := req.To(); to != nil {
			if user := to.Address.User; user != "" {
				return user
			}
		}
		return ""
	}

	r.SetOnInvite(func(deviceID string, req *sip.Request, tx sip.ServerTransaction) {
		callID := req.CallID().Value()
		callee := sipCalleeFromReq(req)

		// 通话事件追踪
		p.mu.RLock()
		pub := p.callEventPub
		p.mu.RUnlock()
		if pub != nil {
			pub.OnOutboundInvite(deviceID, callID, callee)
		}

		p.mu.RLock()
		w, ok := p.workers[deviceID]
		voiceGW := p.voiceGateway
		p.mu.RUnlock()

		// 路由判断：按设备状态选择通话通道
		// 1. VoWiFi 在线 → 走 VoWiFi IMS VoiceGateway
		// 2. 有 CSCallMgr → 走 CS 域语音桥接
		// 3. 都没有 → 503
		if voiceGW != nil && voiceGW.GetAgent(deviceID) != nil {
			logger.Info(fmt.Sprintf("[%s] 外呼 INVITE: 走 VoWiFi IMS VoiceGateway", deviceID))
			voiceGW.HandleClientInvite(deviceID, req, tx)
			return
		}

		if ok && w.CSCallMgr != nil {
			logger.Info(fmt.Sprintf("[%s] 外呼 INVITE: 走 CS 域语音桥接", deviceID))
			w.CSCallMgr.HandleOutboundInvite(deviceID, req, tx)
			return
		}

		logger.Warn(fmt.Sprintf("[%s] 外呼 INVITE: 无可用语音通道 (VoWiFi=%v, CSCall=%v)",
			deviceID, voiceGW != nil && voiceGW.GetAgent(deviceID) != nil, ok && w.CSCallMgr != nil))
		tx.Respond(sip.NewResponseFromRequest(req, 503, "No voice channel available", nil))
	})

	r.SetOnBye(func(deviceID string, req *sip.Request, tx sip.ServerTransaction) {
		callID := req.CallID().Value()

		// 通话事件追踪
		p.mu.RLock()
		pub := p.callEventPub
		p.mu.RUnlock()
		if pub != nil {
			pub.OnCallEnded(deviceID, callID)
		}

		p.mu.RLock()
		w, ok := p.workers[deviceID]
		voiceGW := p.voiceGateway
		p.mu.RUnlock()

		// 按通话类型路由 BYE
		if ok && w.CSCallMgr != nil && w.CSCallMgr.HasCall(callID) {
			w.CSCallMgr.HandleClientBye(callID)
			return
		}
		if voiceGW != nil && voiceGW.GetAgent(deviceID) != nil {
			voiceGW.HandleClientBye(deviceID, req, tx)
			return
		}
	})

	r.SetOnCancel(func(deviceID string, req *sip.Request, tx sip.ServerTransaction) {
		callID := req.CallID().Value()

		// 通话事件追踪
		p.mu.RLock()
		pub := p.callEventPub
		p.mu.RUnlock()
		if pub != nil {
			pub.OnCallEnded(deviceID, callID)
		}

		p.mu.RLock()
		w, ok := p.workers[deviceID]
		voiceGW := p.voiceGateway
		p.mu.RUnlock()

		// 按通话类型路由 CANCEL
		if ok && w.CSCallMgr != nil && w.CSCallMgr.HasCall(callID) {
			w.CSCallMgr.HandleClientCancel(callID)
			tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
			return
		}
		if voiceGW != nil && voiceGW.GetAgent(deviceID) != nil {
			voiceGW.HandleClientCancel(deviceID, req, tx)
			return
		}
	})
}
