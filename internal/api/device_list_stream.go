package api

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/config"
)

// handleDeviceMgmtListStream 聚合所有设备列表状态，通过 SSE 推送给列表页。
//
// 事件:
//   - "devices": 设备列表快照（2s 间隔 + VoWiFi 状态变更时立即推送）
//   - "discovered": 设备发现完成通知（热插拔触发 RescanAndReconnect 后推送）
//
// handleDeviceMgmtListStream SSE 设备列表实时状态流
//
// @Summary      SSE 设备列表实时状态流
// @Tags         devices
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/stream [get]
// @Security     BearerAuth
func (s *Server) handleDeviceMgmtListStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	notify := c.Writer.CloseNotify()
	ctx := c.Request.Context()

	// VoWiFi 状态变更 fan-in channel
	stateChangeCh := make(chan struct{}, 1)

	// 设备发现事件 channel（热插拔触发 RescanAndReconnect 完成后收到信号）
	discoveryCh, unsubDiscovery := s.pool.SubscribeDiscoveryEvents()
	defer unsubDiscovery()

	type stateSub struct {
		un func()
	}
	var subMu sync.Mutex
	subs := make(map[string]*stateSub)

	syncStateSubs := func() {
		workers := s.pool.GetAllWorkers()
		seen := make(map[string]bool)
		for _, w := range workers {
			seen[w.ID] = true
			if _, exists := subs[w.ID]; exists {
				continue
			}
			ch, un := s.pool.SubscribeVoWiFiState(w.ID)
			id := w.ID
			go func() {
				for range ch {
					select {
					case stateChangeCh <- struct{}{}:
					default:
					}
				}
			}()
			subMu.Lock()
			subs[id] = &stateSub{un: un}
			subMu.Unlock()
		}
		// 清理已移除设备的订阅
		subMu.Lock()
		for id, sub := range subs {
			if !seen[id] {
				sub.un()
				delete(subs, id)
			}
		}
		subMu.Unlock()
	}
	syncStateSubs()
	defer func() {
		subMu.Lock()
		for _, sub := range subs {
			sub.un()
		}
		subMu.Unlock()
	}()

	sendData := func() {
		workers := s.pool.GetAllWorkers()
		managed := config.ListDevices()
		cfgByID := map[string]config.DeviceConfig{}
		for _, d := range managed {
			cfgByID[d.ID] = d
		}

		items := make([]deviceMgmtListItem, 0, len(workers))
		workerByID := make(map[string]bool)
		for _, w := range workers {
			workerByID[w.ID] = true
			cfg := w.Config
			if v, ok := cfgByID[w.ID]; ok {
				cfg = overviewDisplayConfig(w.Config, v, true)
			}
			status := w.GetCachedDeviceStatus()
			controlOnline := w.GetCachedHealthy()
		item := deviceMgmtListItem{
			ID:                     w.ID,
			Name:                   cfg.Name,
			Running:                true,
			Healthy:                controlOnline,
			ControlOnline:          controlOnline,
			PublicIP:               w.GetCachedIP(),
			PublicIPv6:             w.GetCachedIPv6(),
		Interface:              cfg.Interface,
		ESIMTransport:          config.NormalizeESIMTransport(cfg.ESIMTransport),
		PCSCReader:             cfg.PCSCReader,
		Manufacturer:           firstNonEmpty(status.Manufacturer, cfg.USBManufacturer),
		USBProduct:             cfg.USBProduct,
			SMSEnabled:             cfg.SMSEnabled,
				NetworkEnabled:         cfg.NetworkEnabled,
				FlightMode:             status.OperatingMode != nil && isFlightModeEnabled(*status.OperatingMode),
				VoWiFiEnabled:          cardPolicyVoWiFiEnabled(status.ICCID, cfg.VoWiFiEnabled),
				VoWiFiActive:           s.pool.IsVoWiFiActive(w.ID),
				VoWiFiRuntime:          s.getVoWiFiRuntimeDTO(w.ID),
				NetworkConnected:       w.NetworkConnected(),
				RegistrationStateLabel: registrationStateLabel(status.RegStatus),
Modem: deviceMgmtListModem{
Manufacturer:     status.Manufacturer,
ChipVendor:       status.ChipVendor,
Model:            status.Model,
				HardwareRevision: status.HardwareRevision,
				Operator:      status.Operator,
					NativeSPN:     status.NativeSPN,
					NativeMCC:     status.NativeMCC,
					NativeMNC:     status.NativeMNC,
					NetworkMode:   status.NetworkMode,
					NetworkDuplex: status.NetworkDuplex,
					RadioBand:     status.RadioBand,
					RadioChannel:  status.RadioChannel,
					SignalDBM:     status.SignalDBM,
					SignalSINR:    status.SignalSINR,
					IMEI:          status.IMEI,
					ICCID:         status.ICCID,
					RegStatus:     status.RegStatus,
					PSAttached:    status.PSAttached,
					OperatingMode: status.OperatingMode,
				},
			}
			s.applyLifecycleToListItem(&item, true, cfg)
			items = append(items, item)
		}

		// 离线设备
		for _, dc := range managed {
			if workerByID[dc.ID] {
				continue
			}
			pol := resolveOfflineDevicePolicy(dc.ID)
			item := deviceMgmtListItem{
				ID:                     dc.ID,
				Name:                   dc.Name,
				Running:                false,
				Healthy:                false,
				ControlOnline:          false,
				PublicIP:               "",
			Interface:              dc.Interface,
			ESIMTransport:          config.NormalizeESIMTransport(dc.ESIMTransport),
			PCSCReader:             dc.PCSCReader,
			Manufacturer:           dc.USBManufacturer,
			USBProduct:             dc.USBProduct,
			SMSEnabled:             pol.SMSEnabled,
				NetworkEnabled:         pol.NetworkEnabled,
				VoWiFiEnabled:          pol.VoWiFiEnabled,
				VoWiFiActive:           false,
				NetworkConnected:       false,
				RegistrationStateLabel: registrationStateLabel(0),
			}
			s.applyLifecycleToListItem(&item, false, dc)
			items = append(items, item)
		}

		c.SSEvent("devices", gin.H{"devices": items})
		c.Writer.Flush()
	}

	sendData()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	resyncTicker := time.NewTicker(5 * time.Second)
	defer resyncTicker.Stop()

	for {
		select {
		case <-notify:
			return
		case <-ctx.Done():
			return
		case <-s.shutdownCh:
			return
		case <-resyncTicker.C:
			syncStateSubs()
		case <-ticker.C:
			sendData()
		case <-stateChangeCh:
			sendData()
		case <-discoveryCh:
			// 热插拔触发 RescanAndReconnect 完成，通知前端刷新发现列表
			c.SSEvent("discovered", gin.H{"refresh": true})
			c.Writer.Flush()
			// 同时推送设备列表快照（设备可能已上线/离线）
			sendData()
		}
	}
}

