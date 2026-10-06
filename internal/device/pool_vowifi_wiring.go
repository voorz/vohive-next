package device

import (
	"fmt"

	"github.com/voorz/sipgo/sip"
	"github.com/voorz/vohive/internal/sipgw"
	"github.com/voorz/vohive/pkg/logger"
)

// SetVoiceGateway 注入 VoWiFi 语音网关（语音暂不处理，保留接口兼容）。
func (p *Pool) SetVoiceGateway(g interface{}) {
	// 语音部分暂不处理，此为空实现
}

// UpdateVoWiFiBehavior 热更新 VoWiFi 行为参数到运行时。
func (p *Pool) UpdateVoWiFiBehavior(ikeRetryCount int, recoverIntervalSeconds int) {
	if p == nil {
		return
	}
	p.mu.RLock()
	host := p.vowifiHost
	p.mu.RUnlock()
	if host != nil {
		host.SetIKERetryCount(ikeRetryCount)
		host.SetRecoverInterval(recoverIntervalSeconds)
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

// GetVoiceGateway 返回绑定的 VoiceGateway 实例（语音暂不处理，返回 nil）
func (p *Pool) GetVoiceGateway() interface{} {
	return nil
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
			p.mu.RUnlock()

		// 路由判断：语音暂缓，只走 CS 域语音桥接
		// 1. 有 CSCallMgr → 走 CS 域语音桥接
		// 2. 没有 → 503
		if ok && w.CSCallMgr != nil {
			logger.Info(fmt.Sprintf("[%s] 外呼 INVITE: 走 CS 域语音桥接", deviceID))
			w.CSCallMgr.HandleOutboundInvite(deviceID, req, tx)
			return
		}

		logger.Warn(fmt.Sprintf("[%s] 外呼 INVITE: 无可用语音通道", deviceID))
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
			p.mu.RUnlock()

		// 按通话类型路由 BYE
		if ok && w.CSCallMgr != nil && w.CSCallMgr.HasCall(callID) {
			w.CSCallMgr.HandleClientBye(callID)
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
			p.mu.RUnlock()

		// 按通话类型路由 CANCEL
		if ok && w.CSCallMgr != nil && w.CSCallMgr.HasCall(callID) {
			w.CSCallMgr.HandleClientCancel(callID)
			tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
			return
		}
	})
}
