package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/voorz/vohive/internal/api"
	carrierconfig "github.com/voorz/vohive/internal/carrier"
	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/internal/device"
	"github.com/voorz/vohive/internal/esim"
	"github.com/voorz/vohive/internal/notify"
	"github.com/voorz/vohive/internal/plmnindex"
	proxyserver "github.com/voorz/vohive/internal/proxy/server"
	"github.com/voorz/vohive/internal/proxy/traffic"
	"github.com/voorz/vohive/internal/sipgw"
	"github.com/voorz/vohive/internal/upstreamproxy"
	"github.com/voorz/vohive/internal/voice"
	"github.com/voorz/vowifi-core/runtimehost/carrier"
	"github.com/voorz/vowifi-core/runtimehost/voicehost"

	"github.com/voorz/vohive/internal/web"
	"github.com/voorz/vohive/pkg/logger"

	"github.com/voorz/sipgo/sip"
)

// voiceBus 全局变量（在 sipRegistrar 回调中创建，在 apiServer 初始化时注入）
var _voiceBus *voice.Bus

// @title           VoHive Main API
// @version         1.0
// @description     VoHive 主服务控制面 API — 自动生成 OpenAPI 文档 POC
// @BasePath        /api
// @securityDefinitions.apikey BearerAuth
// @in                          header
// @name                        Authorization
// @description                 "Bearer <token>"
func main() {
	// sipgo 日志将在主日志系统初始化后接入（见下方 logger.Setup 之后）
	// 绕过 sipgo 底层硬编码的 UDP MTU 限制（默认 1500），
	// 防止由于包含 APNs/FCM 推送 Token 的超长 Contact URI 导致 UDP 发送直接报错。
	// 大包会自动在 IP 层被切片(IP Fragmentation)。
	sip.UDPMTUSize = 65535
	// Parse flags
	var configPath string
	var backendOnly bool
	flag.StringVar(&configPath, "c", "config/config.yaml", "config file path")
	flag.BoolVar(&backendOnly, "backend-only", false, "run as backend-only (disable embedded web UI)")
	flag.Parse()

	// 1. 加载配置
	if err := config.InitGlobalManager(configPath); err != nil {
		log.Fatalf("初始化配置管理器失败: %v", err)
	}
	cfg := config.GetConfig()

	// 2. 初始化日志
	logger.Setup(logger.LogConfig{
		Debug:    cfg.Server.Debug,
		Filename: "logs/app.log",
	})
	// 将内置 slog 重定向到已就绪的系统日志框架
	slog.SetDefault(slog.New(logger.NewSlogHandler(logger.ZapLogger())))
	// 将 sipgo 日志接入主日志系统（zap），开启 SIP 传输层/事务层调试日志
	sip.SetDefaultLogger(slog.New(logger.NewSlogHandler(logger.ZapLogger())))
	sip.SIPDebug = true
	logger.Info("VoHive 模组管理器启动中...")

	// 根据 config 设置 PC/SC 读卡器驱动模式（usbfs=内置USBFS直连, pcscd=系统pcscd服务）
	// 配置为空时自动检测：如果 pcscd 服务正在运行则使用 pcsc 模式，否则使用 usbfs
	mode := strings.ToLower(strings.TrimSpace(cfg.Server.PcscDriverMode))
	if mode == "" {
		// 自动检测 pcscd 服务状态
		if out, err := exec.Command("systemctl", "is-active", "pcscd").Output(); err == nil && strings.TrimSpace(string(out)) == "active" {
			mode = "pcscd"
		} else {
			mode = "usbfs"
		}
		logger.Info("配置未指定 pcsc_driver_mode，自动检测: " + mode)
	}
	if mode == "pcscd" || mode == "pcsc" {
		esim.SetPcscTransport(esim.PCSCTransportPCSC)
		logger.Info("PC/SC 驱动模式: pcscd（原生驱动）")
	} else {
		esim.SetPcscTransport(esim.PCSCTransportUSBFS)
		logger.Info("PC/SC 驱动模式: usbfs（内置驱动）")
	}

	go func() {
		disclaimer := `
╔══════════════════════════════════════════════════════════════════════╗
║【免责与使用声明】														 
║1. 本软件仅供个人技术测试与研究交流，严禁任何商业用途。
║2. 严禁将本软件用于任何非法或违规场景。
║3. 本软件涉及底层通信操作，因测试产生的硬件、资费或网络风险由用户自行承担。
║4. 作者不对使用本软件造成的任何直接或间接损失负责。
╚══════════════════════════════════════════════════════════════════════╝`
		logger.Warn(disclaimer)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			logger.Warn(disclaimer)
		}
	}()

	loadResult, err := carrier.LoadCarrierOverrides("")
	if err != nil {
		carrier.ClearCarrierOverrides()
		logger.Warn("加载 carrier_overrides 失败，回退内置运营商配置",
			"path", loadResult.Path,
			"err", err)
	} else if loadResult.Missing {
		//logger.Info("carrier_overrides 文件不存在，使用内置运营商配置", "path", loadResult.Path)
	} else {
		logger.Info("carrier_overrides 已加载", "path", loadResult.Path, "entries", loadResult.Count)
	}

	// 3. 初始化数据库
	dbPath := "data/vohive.db"
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	dbResolvedPath := dbPath
	if absPath, err := filepath.Abs(dbPath); err == nil {
		dbResolvedPath = absPath
	}
	logger.Info("数据库已初始化", "path", dbPath, "resolved_path", dbResolvedPath)

	// 从 DB 加载活跃的运营商用户配置到 profiles 内存覆盖
	if err := carrierconfig.LoadActiveOverrides(); err != nil {
		logger.Warn("加载运营商用户配置失败，使用系统默认", "err", err)
	}

	// 启动 plmn-index 同步（首次空库时拉取，之后每 24h 定期同步）
	plmnindex.StartPeriodicSync(context.Background())
	countryResult := upstreamproxy.InitCountryTable(context.Background(), upstreamproxy.CountryTableOptions{
		CachePath: upstreamproxy.DefaultCountryTableCachePath,
	})
	if countryResult.Err != nil {
		logger.Warn("MCC/MNC 国家表不可用，VoWiFi 国家代理规则将按未知国家直连",
			"path", countryResult.CachePath,
			"source_url", countryResult.SourceURL,
			"source", countryResult.Source,
			"err", countryResult.Err)
	} else {
		logger.Info("MCC/MNC 国家表已加载",
			"path", countryResult.CachePath,
			"source", countryResult.Source,
			"rows", countryResult.RowCount,
			"countries", countryResult.Countries)
	}
	go func() {
		need, err := db.NeedBackfillSMSContacts()
		if err != nil {
			logger.Error("短信联系人回填检查失败", "err", err)
			return
		}
		if !need {
			return
		}
		logger.Info("开始短信联系人回填")
		if err := db.BackfillSMSPeerAndContacts(1000); err != nil {
			logger.Error("短信联系人回填失败", "err", err)
			return
		}
		logger.Info("短信联系人回填完成")
	}()

	// 4. 初始化设备池

	pool := device.NewPool(cfg)

	pool.SetPolicyResolver(db.CardPolicyResolver{})

	// 6. 初始化代理实例管理器
	proxyMgr := proxyserver.NewManager()
	logger.Info("代理实例管理器已初始化")

	// 7. 初始化语音网关与软电话 Registrar
	// voiceGW 始终创建，用于管理 VoWiFi Agent（SimulateCall 等）。
	// SIP Registrar（Linphone 软电话接入）仅在 voice_gateway.sip.listen 非空时启用。
	var sipRegistrar *sipgw.Registrar
	var notifyMgr *notify.Manager
	voiceGW := voicehost.NewGateway()
	pool.SetVoiceGateway(voiceGW)

	if err := voiceGW.Start(context.Background()); err != nil {
		logger.Error("语音网关启动失败", "err", err)
	} else {
		logger.Info("语音网关已启动")

		// SIP Registrar（软电话）：开箱即用，默认监听 5060
		{
			// 从 DB 加载语音网关配置（首次启动 db 层自动创建并生成授权码）
			vg := db.GetOrCreateVoiceGateway(cfg.Web.Username)
			sipgwCfg := sipgw.Config{
				Enabled: true,
				SIP: sipgw.SIPConfig{
					Listen:    "0.0.0.0:5060",
					Transport: "udp",
					Realm:     "vohive.local",
					WSListen:  "0.0.0.0:5061",
				},
				Media: sipgw.MediaConfig{
					RTPPortMin: 10000,
					RTPPortMax: 20000,
					Codecs:     []string{"PCMU/8000", "PCMA/8000"},
				},
			}
			if vg != nil {
				if vg.SIPListen != "" {
					sipgwCfg.SIP.Listen = vg.SIPListen
				}
				if vg.SIPTransport != "" {
					sipgwCfg.SIP.Transport = vg.SIPTransport
				}
				if vg.SIPRealm != "" {
					sipgwCfg.SIP.Realm = vg.SIPRealm
				}
				sipgwCfg.SIP.ExternalIP = vg.SIPExternalIP
				if vg.WSListen != "" {
					sipgwCfg.SIP.WSListen = vg.WSListen
				}
				sipgwCfg.SIP.WSSListen = vg.WSSListen
				sipgwCfg.SIP.WSSCertFile = vg.WSSCertFile
				sipgwCfg.SIP.WSSKeyFile = vg.WSSKeyFile
				sipgwCfg.User = sipgw.UserConfig{
					Username: vg.Username,
					Password: vg.Password,
					DeviceID: vg.DeviceID,
				}
				if vg.RTPPortMin > 0 {
					sipgwCfg.Media.RTPPortMin = vg.RTPPortMin
				}
				if vg.RTPPortMax > 0 {
					sipgwCfg.Media.RTPPortMax = vg.RTPPortMax
				}
				if vg.Codecs != "" {
					var codecs []string
					if json.Unmarshal([]byte(vg.Codecs), &codecs) == nil && len(codecs) > 0 {
						sipgwCfg.Media.Codecs = codecs
					}
				}
				sipgwCfg.LinphonePush.LinphoneUser = vg.LinphoneUser
				sipgwCfg.LinphonePush.LinphonePassword = vg.LinphonePassword
			}
			// 默认值：开箱即用
			if sipgwCfg.SIP.Listen == "" {
				sipgwCfg.SIP.Listen = "0.0.0.0:5060"
			}
			if sipgwCfg.SIP.WSListen == "" {
				sipgwCfg.SIP.WSListen = "0.0.0.0:5061"
			}
			if sipgwCfg.SIP.Transport == "" {
				sipgwCfg.SIP.Transport = "udp"
			}
			if sipgwCfg.SIP.Realm == "" {
				sipgwCfg.SIP.Realm = "vohive.local"
			}
			if sipgwCfg.Media.RTPPortMin == 0 {
				sipgwCfg.Media.RTPPortMin = 10000
			}
			if sipgwCfg.Media.RTPPortMax == 0 {
				sipgwCfg.Media.RTPPortMax = 20000
			}
			// SIP 用户由用户自行在设置页配置
			if sipgwCfg.User.Username == "" && sipgwCfg.User.Password == "" {
				logger.Info("SIP 网关无用户配置，请在设置页添加 SIP 用户")
			}

			var err error
			sipRegistrar, err = sipgw.NewRegistrar(sipgwCfg)
			if err != nil {
				logger.Error("Registrar 初始化失败", "err", err)
			} else {
				voiceBus := voice.NewBus()
				_voiceBus = voiceBus
				pool.SetVoWiFiCallEventPublisher(voiceBus)

				voiceGW.SetClientAdapter(sipRegistrar)
				pool.SetVoWiFiSIPRegistrar(sipRegistrar)

				// SIP 回调统一由 pool.SetSIPRegistrar 注册：
				// onInvite/onBye/onCancel 路由逻辑 + voiceBus 事件追踪均在 pool 内处理。
				// 以下仅注册 pool 不管的 PRACK/ACK 等直接转发到 voiceGW。
				sipRegistrar.SetOnPrack(voiceGW.HandleClientPrack)
				sipRegistrar.SetOnAck(voiceGW.HandleClientAck)

				pool.SetSIPRegistrar(sipRegistrar)

				if err := sipRegistrar.Start(context.Background()); err != nil {
					logger.Error("Registrar 启动失败", "err", err)
				} else {
					logger.Info("软电话 Registrar 已启动", "listen", sipgwCfg.SIP.Listen, "user", sipgwCfg.User.Username)
				}
			}
		}

		// 通知管理器初始化
		var err error
		notifyMgr, err = notify.NewManager(cfg, pool)
		if err != nil {
			logger.Warn("通知管理器初始化异常", "err", err)
		} else {
			pool.SetNotifier(notifyMgr)
			voiceGW.SetNotifier(notifyMgr)
		}

	}

	// 5. 启动工作器 (代理, 短信, 健康检查)
	_ = pool.StartAll()

	trafficSampler := traffic.New(traffic.Options{Pool: pool, Mgr: proxyMgr})
	trafficSampler.Start()
	realtimeTraffic := traffic.NewRealtimeManager(traffic.RealtimeOptions{Pool: pool})

	// 7. 启动 API 服务器
	// 准备静态文件系统
	var staticFS http.FileSystem
	if backendOnly {
		logger.Info("启用纯后端模式（未挂载前端静态资源）")
	} else {
		distFS, err := web.GetFS()
		if err != nil {
			log.Fatalf("无法加载嵌入的 Web 文件: %v", err)
		}
		staticFS = http.FS(distFS)
	}

	// 通知管理器如果在上方未初始化，在这里兜底（防止 VoiceGW 未配置/启动的场景）
	if notifyMgr == nil {
		var err error
		notifyMgr, err = notify.NewManager(cfg, pool)
		if err != nil {
			logger.Warn("通知管理器初始化异常", "err", err)
		} else {
			pool.SetNotifier(notifyMgr)
			if voiceGW != nil {
				voiceGW.SetNotifier(notifyMgr)
			}
		}
	}

	apiServer := api.New(cfg, pool, staticFS, proxyMgr, voiceGW, notifyMgr, configPath)
	apiServer.SetRealtimeTraffic(realtimeTraffic)

	// Linphone 推送通知器
	if sipRegistrar != nil {
		apiServer.SetPushNotifier(sipRegistrar)
	}

	// 通话事件总线（在 sipRegistrar 初始化时创建）
	if _voiceBus != nil {
		apiServer.SetVoiceBus(_voiceBus)

		// 通话结束时自动写入 voice_history
		_voiceBus.OnEvent(func(event voice.CallEvent) {
			if event.State != voice.CallStateEnded {
				return
			}
			if event.Type == voice.CallTypeOutgoing && event.Duration == 0 {
				// 去电取消，不记录
				return
			}
			now := time.Now()
			record := &db.VoiceHistory{
				DeviceID:  event.DeviceID,
				Peer:      event.Number,
				Number:    event.Number,
				Type:      string(event.Type),
				Direction: string(event.Direction),
				Duration:  event.Duration,
				CallID:    event.CallID,
				Timestamp: now,
			}
			if event.EndedAt != nil {
				record.Timestamp = *event.EndedAt
			}
			if err := db.CreateVoiceHistory(record); err != nil {
				logger.Warn("写入通话记录失败", "err", err, "call_id", event.CallID)
			}
		})
	}

	syncProxyConfigs := func(reason, deviceID string) {
		if err := apiServer.SyncProxyConfigs(); err != nil {
			logger.Warn("同步代理配置失败", "reason", reason, "device_id", deviceID, "err", err)
			return
		}
		logger.Debug("代理配置已同步", "reason", reason, "device_id", deviceID)
	}
	pool.OnDataConnected(func(deviceID string) {
		syncProxyConfigs("qmi_data_connected", deviceID)
	})

	// 启动后同步代理配置到实例管理器
	go func() {
		time.Sleep(500 * time.Millisecond) // 等待 API 服务器初始化
		syncProxyConfigs("startup", "")
	}()

	apiErrCh := make(chan error, 1)
	go func() {
		if err := apiServer.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			apiErrCh <- err
		}
	}()

	logger.Info("所有服务已启动")

	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// 8. 等待关闭信号
	var sig os.Signal
	select {
	case sig = <-quit:
		logger.Info("收到关闭信号", "signal", sig.String())
	case err := <-apiErrCh:
		logger.Error("API 服务器失败", "err", err)
	}
	logger.Info("正在优雅关闭所有服务...")

	// 9. 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("关闭 API 服务器时出错", "err", err)
		}

		if notifyMgr != nil {
			notifyMgr.Close()
		}

		trafficSampler.Stop()

		if err := proxyMgr.Shutdown(shutdownCtx); err != nil {
			logger.Error("关闭代理实例时出错", "err", err)
		}

		// 关闭语音网关与软电话 Registrar
		if voiceGW != nil {
			if err := voiceGW.Stop(); err != nil {
				logger.Error("关闭语音网关时出错", "err", err)
			}
		}
		if sipRegistrar != nil {
			if err := sipRegistrar.Stop(); err != nil {
				logger.Error("关闭 Registrar 时出错", "err", err)
			}
		}

		if err := pool.Shutdown(); err != nil {
			logger.Error("关闭工作器池时出错", "err", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-quit:
	case <-time.After(12 * time.Second):
		logger.Warn("关闭超时，强制退出")
	}

	logger.Info("再见!")
}
