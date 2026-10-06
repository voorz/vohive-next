package device

import (
	"context"
	"fmt"

	"github.com/voorz/vohive/pkg/smscodec"
)

func (p *Pool) GetVoWiFiApp() *ims.Client {
	return p.GetVoWiFiAppForDevice()
}

func (p *Pool) GetVoWiFiAppForDevice(deviceID ...string) *ims.Client {
	if len(deviceID) > 0 && deviceID[0] != "" {
		return p.voWiFiHost().Instance(deviceID[0])
	} else {
		for _, app := range p.voWiFiHost().Instances() {
			return app
		}
	}

	return nil
}

func (p *Pool) GetAllVoWiFiApps() map[string]*ims.Client {
	return p.voWiFiHost().Instances()
}

func (p *Pool) SendVoWiFiSMS(ctx context.Context, deviceID, to, text string) error {
	_, err := p.SendVoWiFiSMSWithResult(ctx, deviceID, to, text)
	return err
}

func (p *Pool) SendVoWiFiSMSWithResult(ctx context.Context, deviceID, to, text string) (ims.SMSResult, error) {
	return p.SendVoWiFiSMSWithOptions(ctx, deviceID, to, text, smscodec.SubmitOptions{})
}

func (p *Pool) SendVoWiFiSMSWithOptions(ctx context.Context, deviceID, to, text string, opts smscodec.SubmitOptions) (ims.SMSResult, error) {
	inst := p.voWiFiHost().Instance(deviceID)
	if inst == nil {
		return ims.SMSResult{}, fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
	}
	svc := inst.Service()
	if svc == nil {
		return ims.SMSResult{}, fmt.Errorf("设备 %s 的 VoWiFi IMS 服务未就绪", deviceID)
	}

	// 标准化路径：委托 ims.SMSModule.SendSMSWithOptions 处理 TPDU 编码 + RP-DATA 包装 + 事件分发
	msgOpts := ims.SMSRequest{Encoding: string(opts.Encoding)}
	return svc.SendSMSWithOptions(ctx, to, text, msgOpts)
}

func (p *Pool) IsVoWiFiActive(deviceID string) bool {
	return p.voWiFiHost().Active(deviceID)
}

// SendVoWiFiUSSD 通过 VoWiFi 发送 USSD 请求（首轮）。
func (p *Pool) SendVoWiFiUSSD(ctx context.Context, deviceID, command string) (*ims.USSDResult, error) {
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
func (p *Pool) ContinueVoWiFiUSSD(ctx context.Context, deviceID, sessionID, input string) (*ims.USSDResult, error) {
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

func (p *Pool) GetVoWiFiRuntimeState(deviceID string) (vowifihost.DeviceStartupState, bool) {
	return p.voWiFiHost().State(deviceID)
}

func (p *Pool) SubscribeVoWiFiState(deviceID string) (<-chan struct{}, func()) {
	return p.voWiFiHost().SubscribeState(deviceID)
}

func (p *Pool) recordVoWiFiStartupState(deviceID string, state vowifihost.DeviceStartupState) {
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
