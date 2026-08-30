package device

import (
	"context"
	"fmt"
	"strings"
	"time"

	qmimanager "github.com/voorz/quectel-qmi-go/pkg/manager"
)

func waitForCondition(ctx context.Context, interval time.Duration, check func() bool) error {
	if check() {
		return nil
	}
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if check() {
				return nil
			}
		}
	}
}

func (p *Pool) waitWorkerReady(deviceID string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()
	return waitForCondition(waitCtx, 200*time.Millisecond, func() bool {
		w := p.GetWorker(deviceID)
		if w == nil {
			return false
		}
		return w.IsDeviceHealthy()
	})
}

func (p *Pool) waitRadioRecoveryReady(deviceID string, timeout time.Duration) error {
	w := p.GetWorker(deviceID)
	if w == nil {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	waitCtx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	// 先等待 QMI core 完全就绪（包括 modem reset 后的恢复）。
	if coreWaiter, ok := w.Backend.(interface {
		WaitCoreReady(context.Context) error
	}); ok {
		if err := coreWaiter.WaitCoreReady(waitCtx); err != nil {
			return fmt.Errorf("等待设备 %s QMI Core 恢复就绪失败: %w", deviceID, err)
		}
	}

	if b, ok := w.Backend.(interface {
		GetUIMReadiness(context.Context) (qmimanager.UIMReadiness, error)
	}); ok {
		return waitForCondition(waitCtx, 500*time.Millisecond, func() bool {
			rdy, err := b.GetUIMReadiness(waitCtx)
			if err != nil {
				return false
			}
			return rdy.Reason == qmimanager.UIMReadinessReady
		})
	}
	if w.QMICore != nil {
		return w.QMICore.WaitIdentityReady(waitCtx)
	}
	if w.Modem != nil {
		if !w.Modem.WaitReady(timeout) {
			return context.DeadlineExceeded
		}
		return nil
	}
	return nil
}

func (p *Pool) waitQMICoreReady(deviceID string, timeout time.Duration) error {
	w := p.GetWorker(deviceID)
	if w == nil {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	waitCtx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	// 先等待 QMI core 完全就绪（包括 modem reset 后的恢复），
	// 避免在 core recovery 期间 UIM session 不可用时启动 VoWiFi。
	if coreWaiter, ok := w.Backend.(interface {
		WaitCoreReady(context.Context) error
	}); ok {
		if err := coreWaiter.WaitCoreReady(waitCtx); err != nil {
			return fmt.Errorf("等待设备 %s QMI Core 恢复就绪失败: %w", deviceID, err)
		}
	}

	if b, ok := w.Backend.(interface {
		GetUIMReadiness(context.Context) (qmimanager.UIMReadiness, error)
	}); ok {
		return waitForCondition(waitCtx, 500*time.Millisecond, func() bool {
			rdy, err := b.GetUIMReadiness(waitCtx)
			if err != nil {
				return false
			}
			return rdy.Reason == qmimanager.UIMReadinessReady
		})
	}
	if w.QMICore != nil {
		return w.QMICore.WaitIdentityReady(waitCtx)
	}
	return nil
}

func (p *Pool) WaitQMICoreReady(deviceID string, timeout time.Duration) error {
	return p.waitQMICoreReady(deviceID, timeout)
}

func (p *Pool) waitQMIControlReady(deviceID string, timeout time.Duration) error {
	w := p.GetWorker(deviceID)
	if w == nil {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}
	waitCtx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	if b, ok := w.Backend.(interface {
		GetUIMReadiness(context.Context) (qmimanager.UIMReadiness, error)
	}); ok {
		return waitForCondition(waitCtx, 500*time.Millisecond, func() bool {
			rdy, err := b.GetUIMReadiness(waitCtx)
			if err != nil {
				return false
			}
			return rdy.ControlReady
		})
	}
	if w.QMICore != nil {
		return w.QMICore.WaitControlReady(waitCtx)
	}
	return nil
}

func (p *Pool) WaitQMIControlReady(deviceID string, timeout time.Duration) error {
	return p.waitQMIControlReady(deviceID, timeout)
}

func (p *Pool) WaitWorkerReady(deviceID string, timeout time.Duration) error {
	return p.waitWorkerReady(deviceID, timeout)
}

func (p *Pool) WorkerExists(deviceID string) bool {
	return p.GetWorker(deviceID) != nil
}

// EUICCAvailable 返回 eUICC 可用状态的三态指针，供 VoWiFi 启动门控使用。
func (p *Pool) EUICCAvailable(deviceID string) *bool {
	w := p.GetWorker(deviceID)
	if w == nil || w.EsimMgr == nil {
		return nil
	}
	return w.EsimMgr.EUICCAvailable()
}

func (p *Pool) IsSwitching(deviceID string) bool {
	return p.IsESIMSwitching(deviceID)
}

// IsVoWiFiDesired checks whether VoWiFi should be running for this device
// based on the current card policy. Used by OnTunnelDown to avoid triggering
// recovery after the user has disabled VoWiFi.
func (p *Pool) IsVoWiFiDesired(deviceID string) bool {
	if p == nil {
		return false
	}
	w := p.GetWorker(deviceID)
	if w == nil {
		return false
	}
	iccid := strings.TrimSpace(w.CurrentICCID())
	if iccid == "" {
		status := w.ProjectDeviceStatus()
		iccid = strings.TrimSpace(status.ICCID)
	}
	if iccid == "" {
		return false
	}
	p.mu.RLock()
	resolver := p.policyResolver
	p.mu.RUnlock()
	if resolver == nil {
		return false
	}
	pol, err := resolver.Resolve(iccid)
	if err != nil {
		return false
	}
	return pol.VoWiFiEnabled
}

// enableVoWiFiWhenReady waits for readiness, then submits enable through the lifecycle controller.
// Do not call this from controller run paths that already hold the per-device lifecycle mutex.
func (p *Pool) enableVoWiFiWhenReady(deviceID string, timeout time.Duration, reason string) error {
	if err := p.waitQMICoreReady(deviceID, timeout); err != nil {
		return fmt.Errorf("等待设备 %s 身份就绪失败(%s): %w", deviceID, reason, err)
	}
	if err := p.waitWorkerReady(deviceID, timeout); err != nil {
		return fmt.Errorf("等待设备 %s 就绪失败(%s): %w", deviceID, reason, err)
	}
	return p.EnableVoWiFi(deviceID)
}

func (p *Pool) EnableVoWiFi(deviceID string) error {
	if p.IsESIMSwitching(deviceID) {
		return fmt.Errorf("设备 %s 正在切卡，暂不允许启动 VoWiFi", deviceID)
	}
	return p.voWiFiHost().Enable(p.ctx, deviceID)
}
