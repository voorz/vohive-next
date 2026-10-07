package device

import (
	"github.com/voorz/ims-go/ims"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/voorz/vohive/internal/backend"
	"github.com/voorz/vohive/internal/db"
	innersim "github.com/voorz/vohive/internal/sim"
	"github.com/voorz/vohive/internal/upstreamproxy"
	"github.com/voorz/vohive/internal/vowifihost"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vohive/pkg/mbim"
	carrier "github.com/voorz/vohive/internal/carrier"
)

type voWiFiStartContext struct {
	worker *Worker
	modem  vowifihost.Modem
	vowifihost.PreparedStart
	startedAt time.Time
}

type workerAKAProviderInput struct {
	worker   *Worker
	deviceID string
	modem    vowifihost.Modem
}

func (w workerAKAProviderInput) BackendMode() string {
	if w.worker == nil || w.worker.Backend == nil {
		return ""
	}
	return w.worker.Backend.Mode()
}

func (w workerAKAProviderInput) MBIMAKAProvider() (innersim.BackendAKAProvider, bool) {
	if w.worker == nil || w.worker.Backend == nil {
		return nil, false
	}
	provider, ok := w.worker.Backend.(interface {
		CalculateAKA(ctx context.Context, rand16, autn16 []byte) (res, ik, ck, auts []byte, err error)
	})
	if !ok || !strings.EqualFold(w.worker.Backend.Mode(), backend.BackendMBIM) {
		return nil, false
	}
	return provider, true
}

func (w workerAKAProviderInput) MBIMCapability() (*mbim.Capabilities, bool) {
	if w.worker == nil || w.worker.Backend == nil {
		return nil, false
	}
	cp, ok := w.worker.Backend.(interface{ Capability() *mbim.Capabilities })
	if !ok {
		return nil, false
	}
	c := cp.Capability()
	return c, c != nil
}

func (w workerAKAProviderInput) RuntimeModem() (innersim.ATModem, error) {
	modemIface := w.modem
	if modemIface == nil {
		var err error
		modemIface, err = BuildVoWiFiRuntimeModem(w.worker, w.deviceID)
		if err != nil {
			return nil, err
		}
	}
	modem, ok := modemIface.(innersim.ATModem)
	if !ok {
		return nil, fmt.Errorf("device %s runtime modem does not implement sim.ATModem", strings.TrimSpace(w.deviceID))
	}
	return modem, nil
}

func BuildAKAProvider(w *Worker, deviceID string) innersim.AKAProvider {
	return innersim.BuildAKAProvider(workerAKAProviderInput{
		worker:   w,
		deviceID: deviceID,
	})
}

func (p *Pool) Context() context.Context {
	if p == nil || p.ctx == nil {
		return context.Background()
	}
	return p.ctx
}

func (p *Pool) PrepareStart(deviceID, traceID, runtimeEPDGOverride string) (vowifihost.PreparedStart, error) {
	startCtx, err := p.prepareVoWiFiStartContext(deviceID, traceID, runtimeEPDGOverride)
	if err != nil {
		return vowifihost.PreparedStart{}, err
	}
	prepared := startCtx.PreparedStart
	prepared.Modem = startCtx.modem
	return prepared, nil
}

func (p *Pool) BeforeStart(deviceID string, modemIface vowifihost.Modem, proxyCfg *ims.ProxyConfig) func(context.Context, vowifihost.SessionConfig) error {
	return p.beforeVoWiFiStart(deviceID, modemIface, proxyCfg)
}

func (p *Pool) HandleStartupError(req vowifihost.StartupErrorRequest) error {
	return p.handleVoWiFiStartupError(req.TraceID, req.DeviceID, req.RuntimeEPDGOverride, req.Generation, req.StartedAt, p.GetWorker(req.DeviceID), req.State, req.Err)
}

func (p *Pool) MarkRuntimeStarted(req vowifihost.RuntimeStartedRequest) {
	w := p.GetWorker(req.DeviceID)
	if w == nil {
		return
	}
	w.smsMode = smsModeVoWiFi
	if w.Modem != nil {
		w.Modem.SetNewSMSHandler(nil)
		w.Modem.SetSMSCallback(nil)
		w.Modem.SetDisableURCRead(true)
	}
}

func (p *Pool) prepareVoWiFiStartContext(deviceID, traceID, runtimeEPDGOverride string) (voWiFiStartContext, error) {
	startCtx := voWiFiStartContext{startedAt: time.Now()}

	w := p.GetWorker(deviceID)
	if w == nil {
		return startCtx, fmt.Errorf("设备 %s 不存在", deviceID)
	}
	startCtx.worker = w

	modemIface, errModemIface := newVoWiFiModemInterface(w, deviceID)
	if errModemIface != nil {
		return startCtx, errModemIface
	}
	startCtx.modem = modemIface
	if _, ok := modemIface.(*qmiModemAdapter); ok {
		logger.Info("VoWiFi 使用 QMI 模式鉴权", "trace_id", traceID, "device", deviceID)
	}

	w.cacheMu.RLock()
	identityReady := w.state.Identity.Ready
	w.cacheMu.RUnlock()
	if !identityReady && !isPCSCDevice(w) {
		if err := w.RefreshIdentityLive(nil, "enable_vowifi"); err != nil {
			logger.Error("VoWiFi 启动前刷新当前设备身份失败",
				"trace_id", traceID,
				"device", deviceID,
				"err", err)
			return startCtx, err
		}
		p.PersistIdentityState(w)
	}

	currentStatus := w.ProjectDeviceStatus()
	logger.Info("VoWiFi 启动前读取当前设备身份",
		"trace_id", traceID,
		"device", deviceID,
		"iccid", strings.TrimSpace(currentStatus.ICCID),
		"imsi", strings.TrimSpace(currentStatus.IMSI),
		"imei", strings.TrimSpace(currentStatus.IMEI))

	startProfile, errProfile := p.buildVoWiFiStartProfile(w, traceID)
	if errProfile != nil {
		logger.Error("构建 VoWiFi 启动画像失败", "trace_id", traceID, "device", deviceID, "err", errProfile)
		return startCtx, errProfile
	}
	startCtx.Profile = startProfile

	akaProvider := innersim.BuildAKAProvider(workerAKAProviderInput{
		worker:   w,
		deviceID: deviceID,
		modem:    modemIface,
	})
	if akaProvider == nil {
		if strings.EqualFold(workerAKAProviderInput{worker: w}.BackendMode(), backend.BackendMBIM) {
			return startCtx, fmt.Errorf("设备 %s 的 MBIM 不支持 AKA(AUTH 与逻辑通道均不可用),如需 VoWiFi 请切 QMI 组态", deviceID)
		}
		return startCtx, fmt.Errorf("设备 %s 无可用 AKA provider", deviceID)
	}
	if strings.EqualFold(workerAKAProviderInput{worker: w}.BackendMode(), backend.BackendMBIM) {
		logger.Info("VoWiFi 使用 MBIM Auth(AKA) 鉴权", "trace_id", traceID, "device", deviceID)
	} else {
		logger.Info("VoWiFi 使用 APDU(AKA) 鉴权", "trace_id", traceID, "device", deviceID)
	}
	startCtx.SIM = &akaProviderAdapter{provider: akaProvider, imsi: startProfile.IMSI}

	if carrier.IsVoWiFiBlockedMCC(startProfile.MCC) {
		err := carrier.NewVoWiFiBlockedMCCError(startProfile.MCC)
		logger.Warn("VoWiFi 启动被运营商策略拦截",
			"trace_id", traceID,
			"device", deviceID,
			"mcc", formatVoWiFiPLMN3(startProfile.MCC),
			"imsi", startProfile.IMSI,
			"err", err)
		logVoWiFiFailureSummary(traceID, deviceID, "startup", "policy", err.Error(), false, 0)
		return startCtx, err
	}

	// ims-go 迁移：A4 PrepareStart 由 ims.New() 内部执行，此处不再调用 vowifi-core identity。
	// 只准备 SIM adapter 和 Profile，carrier/EPDG 解析由 ims-go 完成。
	logger.Info("VoWiFi 启动画像已准备（ims-go A4 在 ims.New 内部）",
		"trace_id", traceID,
		"device", deviceID)

	if nc := w.NetworkController(); nc != nil {
		w.restoreNetworkAfterVoWiFi = w.Config.NetworkEnabled
		logger.Info("VoWiFi 启用中，停止网络功能", "trace_id", traceID, "device", deviceID)
		if err := nc.Disconnect(); err != nil {
			logger.Warn("断开数据连接失败，继续启动 VoWiFi", "trace_id", traceID, "device", deviceID, "err", err)
		}
		w.clearCachedIP()
	}

	// PC/SC 设备无 modem/backend，跳过飞行模式切换（无原生 IMS 注册需要禁用）
	if !isPCSCDevice(w) {
		// 切卡恢复场景下设备可能已处于飞行模式，此时无需再次切换。
		// 冗余的 SetOperatingMode(LowPower) 会触发模组内部 UIM Session Close，
		// 导致 SIM 卡基础通道上的 USIM 应用选择状态丢失，使后续 AKA 认证失败（SW=6B00）。
		alreadyInFlight := false
		if opMode, opErr := w.Backend.GetOperatingMode(p.ctx); opErr == nil {
			alreadyInFlight = isFlightOperatingMode(opMode)
		}
		if strings.EqualFold(w.Backend.Mode(), backend.BackendMBIM) {
			logger.Info("MBIM 后端不支持真正的低功耗模式",
				"trace_id", traceID, "device", deviceID)
		} else if alreadyInFlight {
			logger.Info("设备已处于飞行模式，跳过冗余的飞行模式切换",
				"trace_id", traceID, "device", deviceID, "backend", w.Backend.Mode())
		} else {
			logger.Info("进入飞行模式以禁用原生 IMS 注册",
				"trace_id", traceID, "device", deviceID, "backend", w.Backend.Mode())
			if err := w.Backend.SetOperatingMode(p.ctx, backend.ModeRFOff); err != nil {
				logger.Warn("进入飞行模式失败，继续尝试建立隧道",
					"trace_id", traceID, "device", deviceID, "err", err)
			} else {
				// Wait for network stack to settle after RF-off.
				// Without this delay, mihomo may not have rebuilt its
				// routing table yet, causing the first IKE_SA_INIT
				// packet to be silently dropped (manifests as a ~95s
				// hang before reconnection succeeds).
				// Delay is configurable per carrier profile (default 5s).
				// ims-go 迁移：暂用默认值 5s（carrier profile 集成后续）
				rfOffDelay := 5 * time.Second
				if p.cfg != nil && p.cfg.VoWiFi.Behavior.OverrideRFOff && p.cfg.VoWiFi.Behavior.RFOffDelay > 0 {
					rfOffDelay = time.Duration(p.cfg.VoWiFi.Behavior.RFOffDelay) * time.Second
				}
				if rfOffDelay <= 0 {
					rfOffDelay = 5 * time.Second
				}
				logger.Info("飞行模式后等待网络栈稳定",
					"trace_id", traceID, "device", deviceID, "rf_off_delay_s", rfOffDelay)
				// 可取消的等待：响应 Pool 关闭，避免 time.Sleep 阻塞无法中断
				select {
				case <-p.ctx.Done():
					return startCtx, fmt.Errorf("等待网络栈稳定时被取消: %w", p.ctx.Err())
				case <-time.After(rfOffDelay):
				}
			}
		}
	}

	startCtx.Proxy = resolveVoWiFiCountryProxy(startProfile.MCC, traceID, deviceID)

	startCtx.NetworkMode = modemIface.GetNetworkMode()
	startCtx.StartupState = newVoWiFiSIMReadyStartupState(deviceID, string(ims.DataplaneUserspace), startCtx.NetworkMode, time.Now())
	p.recordVoWiFiStartupState(deviceID, startCtx.StartupState)
	return startCtx, nil
}

func resolveVoWiFiCountryProxy(homeMCC, traceID, deviceID string) *ims.ProxyConfig {
	proxy, countryCode, err := db.GetHomeMCCUpstreamProxy(homeMCC)
	if err != nil {
		logger.Warn("VoWiFi 启动前读取国家前置代理配置失败",
			"trace_id", traceID,
			"device", deviceID,
			"home_mcc", strings.TrimSpace(homeMCC),
			"err", err)
		return nil
	}
	if proxy == nil {
		logger.Info("VoWiFi 国家前置代理未命中，使用直连",
			"trace_id", traceID,
			"device", deviceID,
			"home_mcc", strings.TrimSpace(homeMCC),
			"proxy_country_code", countryCode,
			"mcc_table_ready", upstreamproxy.CountryTableReady(),
			"proxy_route", "direct")
		return nil
	}
	logger.Info("VoWiFi 国家前置代理已命中",
		"trace_id", traceID,
		"device", deviceID,
		"home_mcc", strings.TrimSpace(homeMCC),
		"proxy_country_code", countryCode,
		"upstream_proxy_id", proxy.ID,
		"proxy_route", "country_rule")
	return &ims.ProxyConfig{
		Addr:     proxy.Addr,
		Username: proxy.Username,
		Password: proxy.Password,
		Enabled:  proxy.Enabled,
	}
}

func (p *Pool) beforeVoWiFiStart(deviceID string, modemIface vowifihost.Modem, proxyCfg *ims.ProxyConfig) func(context.Context, vowifihost.SessionConfig) error {
	return func(startCtx context.Context, cfg vowifihost.SessionConfig) error {
		startupState := newVoWiFiSIMReadyStartupState(deviceID, cfg.DataplaneMode, modemIface.GetNetworkMode(), time.Now())
		startupState.RegStatus, startupState.RegStatusText = modemIface.GetRegStatus()
		p.recordVoWiFiStartupState(deviceID, startupState)
		if proxyCfg != nil && proxyCfg.Enabled && strings.TrimSpace(proxyCfg.Addr) != "" {
			probeRes, probeErr := upstreamproxy.ProbeSOCKS5(startCtx, upstreamproxy.ProbeConfig{
				ProxyAddr: proxyCfg.Addr,
				Username:  proxyCfg.Username,
				Password:  proxyCfg.Password,
				Timeout:   5 * time.Second,
			})
			if probeErr != nil {
				startupState.LastErrorClass = "proxy"
				startupState.LastError = probeErr.Error()
				startupState.LastReason = probeRes.FailureSummary()
				p.recordVoWiFiStartupState(deviceID, startupState)
				return fmt.Errorf("前置代理自检失败: %w", probeErr)
			}
		}
		return nil
	}
}

// akaProviderAdapter 将 ims.AKAProvider 适配为 vowifihost.SIMAdapter。
type akaProviderAdapter struct {
	provider ims.AKAProvider
	imsi     string
}

func (a *akaProviderAdapter) GetIMSI() (string, error) {
	if a.imsi != "" {
		return a.imsi, nil
	}
	return "", fmt.Errorf("IMSI not available")
}

func (a *akaProviderAdapter) CalculateAKA(rand16, autn16 []byte) (ims.AKAResult, error) {
	return a.provider.CalculateAKA(rand16, autn16)
}

func (a *akaProviderAdapter) Close() error { return nil }
