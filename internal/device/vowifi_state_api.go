package device

import (
	"context"
	"fmt"
	"time"

	"github.com/voorz/vohive/pkg/smscodec"
	"github.com/voorz/vowifi-core/runtimehost"
	"github.com/voorz/vowifi-core/runtimehost/messaging"
)

func (p *Pool) GetVoWiFiApp() *runtimehost.Instance {
	return p.GetVoWiFiAppForDevice()
}

func (p *Pool) GetVoWiFiAppForDevice(deviceID ...string) *runtimehost.Instance {
	if len(deviceID) > 0 && deviceID[0] != "" {
		return p.voWiFiHost().Instance(deviceID[0])
	} else {
		for _, app := range p.voWiFiHost().Instances() {
			return app
		}
	}

	return nil
}

func (p *Pool) GetAllVoWiFiApps() map[string]*runtimehost.Instance {
	return p.voWiFiHost().Instances()
}

func (p *Pool) SendVoWiFiSMS(ctx context.Context, deviceID, to, text string) error {
	_, err := p.SendVoWiFiSMSWithResult(ctx, deviceID, to, text)
	return err
}

func (p *Pool) SendVoWiFiSMSWithResult(ctx context.Context, deviceID, to, text string) (messaging.SendOutcome, error) {
	return p.SendVoWiFiSMSWithOptions(ctx, deviceID, to, text, smscodec.SubmitOptions{})
}

func (p *Pool) SendVoWiFiSMSWithOptions(ctx context.Context, deviceID, to, text string, opts smscodec.SubmitOptions) (messaging.SendOutcome, error) {
	inst := p.voWiFiHost().Instance(deviceID)
	if inst == nil {
		return messaging.SendOutcome{}, fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
	}
	svc := inst.Service()
	if svc == nil {
		return messaging.SendOutcome{}, fmt.Errorf("设备 %s 的 VoWiFi IMS 服务未就绪", deviceID)
	}

	// 编码 TPDU（支持长短信自动分段）
	tpdus, _, err := smscodec.BuildSubmitTPDUsWithOptions(to, text, opts)
	if err != nil {
		return messaging.SendOutcome{}, fmt.Errorf("VoWiFi SMS TPDU 编码失败: %w", err)
	}
	if len(tpdus) == 0 {
		return messaging.SendOutcome{}, fmt.Errorf("VoWiFi SMS TPDU 编码结果为空")
	}

	// 获取 SMSC（从 worker 侧读取，与 VoWiFi 启动时一致）
	smsc := ""
	if w := p.GetWorker(deviceID); w != nil {
		smscCtx, smscCancel := context.WithTimeout(ctx, 5*time.Second)
		smsc, _ = w.getSMSCWithContext(smscCtx)
		smscCancel()
	}

	// 每个 TPDU 包装为 RP-DATA(SUBMIT)，构造 SMSPart
	parts := make([]messaging.SMSPart, 0, len(tpdus))
	for i, tpdu := range tpdus {
		rpMr := byte(i + 1) // RP-Message-Reference 从 1 开始递增
		rpData := smscodec.BuildRPData(rpMr, tpdu, smsc)
		parts = append(parts, messaging.SMSPart{
			RPMR: rpMr,
			Body: rpData,
		})
	}

	return svc.SendSMS(ctx, to, text, parts)
}

func (p *Pool) IsVoWiFiActive(deviceID string) bool {
	return p.voWiFiHost().Active(deviceID)
}

// SendVoWiFiUSSD 通过 VoWiFi 发送 USSD 请求（首轮）。
func (p *Pool) SendVoWiFiUSSD(ctx context.Context, deviceID, command string) (*messaging.USSDResult, error) {
	if inst := p.voWiFiHost().Instance(deviceID); inst != nil {
		svc := inst.Service()
		if svc == nil {
			return nil, fmt.Errorf("设备 %s 的 VoWiFi IMS 服务未就绪", deviceID)
		}
		return svc.SendUSSD(ctx, command)
	}
	return nil, fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
}

// ContinueVoWiFiUSSD 在已有 VoWiFi USSD 会话中发送后续输入。
func (p *Pool) ContinueVoWiFiUSSD(ctx context.Context, deviceID, sessionID, input string) (*messaging.USSDResult, error) {
	if inst := p.voWiFiHost().Instance(deviceID); inst != nil {
		svc := inst.Service()
		if svc == nil {
			return nil, fmt.Errorf("设备 %s 的 VoWiFi IMS 服务未就绪", deviceID)
		}
		return svc.ContinueUSSD(ctx, sessionID, input)
	}
	return nil, fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
}

// CancelVoWiFiUSSD 取消 VoWiFi USSD 会话。
func (p *Pool) CancelVoWiFiUSSD(ctx context.Context, deviceID, sessionID string) error {
	if inst := p.voWiFiHost().Instance(deviceID); inst != nil {
		svc := inst.Service()
		if svc == nil {
			return fmt.Errorf("设备 %s 的 VoWiFi IMS 服务未就绪", deviceID)
		}
		return svc.CancelUSSD(ctx, sessionID)
	}
	return fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
}

func (p *Pool) GetVoWiFiStatus() (enabled bool, deviceID string, status string) {
	for devID, inst := range p.voWiFiHost().Instances() {
		if inst == nil {
			return true, devID, "VoWiFi: STOPPED"
		}
		return true, devID, inst.Status()
	}
	return false, "", "未初始化"
}

func (p *Pool) GetVoWiFiStatusAll() map[string]string {
	result := make(map[string]string)
	for devID, inst := range p.voWiFiHost().Instances() {
		result[devID] = inst.Status()
	}
	return result
}

func (p *Pool) GetVoWiFiObs(deviceID string) map[string]interface{} {
	if inst := p.voWiFiHost().Instance(deviceID); inst != nil {
		return inst.Obs()
	}
	return nil
}

func (p *Pool) GetVoWiFiRuntimeState(deviceID string) (runtimehost.State, bool) {
	return p.voWiFiHost().State(deviceID)
}

func (p *Pool) SubscribeVoWiFiState(deviceID string) (<-chan struct{}, func()) {
	return p.voWiFiHost().SubscribeState(deviceID)
}

func (p *Pool) recordVoWiFiStartupState(deviceID string, state runtimehost.State) {
	p.voWiFiHost().RecordStartupState(deviceID, state)
}

func (p *Pool) clearVoWiFiStartupState(deviceID string) {
	p.voWiFiHost().ClearStartupState(deviceID)
}

func (p *Pool) clearVoWiFiStartupStateAndBroadcast(deviceID string) {
	p.voWiFiHost().ClearStartupStateAndBroadcast(deviceID)
}

func (p *Pool) broadcastVoWiFiStateChange(deviceID string) {
	p.voWiFiHost().BroadcastState(deviceID)
}
