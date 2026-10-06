package vowifihost

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/voorz/vohive/pkg/logger"
)

const defaultDesiredRecoverReason = "desired_reconcile"

type DesiredRecoverRequest struct {
	DeviceID     string
	Reason       string
	OverrideEPDG string
	Generation   uint64
	Now          time.Time
	OnResult     func(deviceID, reason string, err error)
}

func (m *Manager) DesiredRecoverable(deviceID string) bool {
	if m == nil {
		return false
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false
	}
	store := m.RuntimeStore()
	return !store.Active(deviceID) && !store.Starting(deviceID)
}

func (m *Manager) ScheduleDesiredRecover(ctx context.Context, req DesiredRecoverRequest) bool {
	if m == nil {
		return false
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return false
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = defaultDesiredRecoverReason
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	if !m.DesiredRecoverable(deviceID) {
		return false
	}
	if !m.BeginDesiredRecover(deviceID, now) {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}

	logger.Warn("VoWiFi 目标态恢复开始", "event", "VOWIFI_DESIRED_RECOVER", "device", deviceID, "reason", reason)
	go func() {
		// 修复：panic 恢复，避免 goroutine 静默死亡导致无日志、无状态更新。
		// （2026-10-07 生产故障：Recover 中的 panic 会导致 goroutine 消失，
		//  前端看到 recover_failed 但 last_error 为空。）
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("VoWiFi 目标态恢复 panic: %v", r)
				logger.Error("VoWiFi 目标态恢复发生 panic", "event", "VOWIFI_DESIRED_RECOVER_PANIC",
					"device", deviceID, "reason", reason, "panic", r, "stack", string(debug.Stack()))
				m.MarkDesiredRecoverFailed(deviceID, time.Now(), err)
				if req.OnResult != nil {
					req.OnResult(deviceID, reason, err)
				}
			}
		}()
		err := m.Recover(ctx, LifecycleRecoverRequest{
			DeviceID:     deviceID,
			Reason:       reason,
			OverrideEPDG: req.OverrideEPDG,
			Generation:   req.Generation,
		})
		if req.OnResult != nil {
			req.OnResult(deviceID, reason, err)
		} else if err != nil {
			if errors.Is(err, ErrUIMUnavailable) {
				// UIM 门控失败：设置 60s cooldown，不走正常递增退避
				m.SetDesiredRecoverCooldown(deviceID, uimGateCooldown)
				logger.Warn("VoWiFi 启动门控：USIM 逻辑通道状态异常，60s 后重试",
					"event", "VOWIFI_UIM_GATE_BLOCKED",
					"device", deviceID, "reason", reason)
			} else {
				m.MarkDesiredRecoverFailed(deviceID, time.Now(), err)
				logger.Warn("VoWiFi 目标态恢复失败，已设置退避", "event", "VOWIFI_DESIRED_RECOVER_FAILED", "device", deviceID, "reason", reason, "err", err)
			}
		} else {
			// Recover returned nil but the tunnel has not yet been confirmed
			// (runtimehost.Start is async). Set a cooldown to prevent tight
			// recover loops when the tunnel immediately fails.
			m.SetDesiredRecoverCooldown(deviceID, 10*time.Second)
			m.RecordStartupState(deviceID, DeviceStartupState{
				DeviceID:   deviceID,
				Phase:      "recover_pending",
				LastReason: "VoWiFi 启动中，等待隧道确认",
				UpdatedAt:  time.Now(),
			})
		}
	}()
	return true
}
