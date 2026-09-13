package api

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	proxytraffic "github.com/voorz/vohive/internal/proxy/traffic"
)

// dashboardAggregatedTraffic 是 /dashboard/overview/stream 推送给前端的聚合流量快照。
type dashboardAggregatedTraffic struct {
	RXBPS        int64    `json:"rx_bps"`
	TXBPS        int64    `json:"tx_bps"`
	RXDeltaBytes int64    `json:"rx_delta_bytes"`
	TXDeltaBytes int64    `json:"tx_delta_bytes"`
	TotalRXBytes uint64   `json:"total_rx_bytes"`
	TotalTXBytes uint64   `json:"total_tx_bytes"`
	DeviceCount  int      `json:"device_count"`
	ActiveDevs   []string `json:"active_devices,omitempty"`
	Timestamp    string   `json:"timestamp"`
	Status       string   `json:"status"`
}

// handleDashboardOverviewStream 聚合所有在线设备实时流量，通过 SSE 推送给仪表盘。
//
// 事件:
//   - "traffic": 聚合流量快照（~1s 间隔）
//   - "overview": 设备计数/连接数概览（10s 间隔）
//
// handleDashboardOverviewStream SSE 仪表盘聚合实时流量
//
// @Summary      SSE 仪表盘聚合实时流量
// @Tags         dashboard
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /dashboard/overview/stream [get]
// @Security     BearerAuth
func (s *Server) handleDashboardOverviewStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	notify := c.Writer.CloseNotify()
	ctx := c.Request.Context()

	// ---- 聚合状态 ----
	var mu sync.Mutex
	latest := make(map[string]proxytraffic.RealtimeSnapshot) // deviceID → 最新快照

	// ---- 订阅管理 ----
	type sub struct {
		ch <-chan proxytraffic.RealtimeSnapshot
		un func()
		id string
	}
	var subMu sync.Mutex
	subs := make(map[string]*sub)
	cancelFanin := make(chan struct{})

	// fan-in goroutine：把每个 device channel 的快照写入 latest map
	startSub := func(id string) {
		subMu.Lock()
		if _, exists := subs[id]; exists {
			subMu.Unlock()
			return
		}
		subMu.Unlock()

		ch, un := s.trafficRT.Subscribe(ctx, id)
		s := &sub{ch: ch, un: un, id: id}

		subMu.Lock()
		subs[id] = s
		subMu.Unlock()

		go func() {
			for snap := range ch {
				mu.Lock()
				latest[snap.DeviceID] = snap
				mu.Unlock()
			}
		}()
	}

	stopSub := func(id string) {
		subMu.Lock()
		if sp, ok := subs[id]; ok {
			delete(subs, id)
			subMu.Unlock()
			sp.un()
			return
		}
		subMu.Unlock()
	}

	stopAllSubs := func() {
		subMu.Lock()
		for _, sp := range subs {
			sp.un()
		}
		subs = nil
		subMu.Unlock()
	}

	// 初始订阅所有在线且网络已连接的设备
	syncSubscriptions := func() {
		workers := s.pool.GetAllWorkers()
		onlineIDs := make(map[string]bool)
		for _, w := range workers {
			if !w.NetworkConnected() {
				continue
			}
			onlineIDs[w.ID] = true
		}

		// 订阅新上线的设备
		for id := range onlineIDs {
			startSub(id)
		}
		// 取消已离线设备的订阅
		subMu.Lock()
		for id := range subs {
			if !onlineIDs[id] {
				subMu.Unlock()
				stopSub(id)
				subMu.Lock()
			}
		}
		subMu.Unlock()
	}

	syncSubscriptions()

	// 定时重新同步订阅（设备上下线）
	resyncTicker := time.NewTicker(5 * time.Second)
	defer resyncTicker.Stop()

	// 聚合推送 ticker（~1s）
	trafficTicker := time.NewTicker(1 * time.Second)
	defer trafficTicker.Stop()

	// overview 推送 ticker（10s）
	overviewTicker := time.NewTicker(10 * time.Second)
	defer overviewTicker.Stop()

	// 初始推送一条 overview
	sendOverview := func() {
		workers := s.pool.GetAllWorkers()
		online := 0
		connCount := 0
		for _, w := range workers {
			if w.NetworkConnected() {
				online++
			}
			if w.Proxy != nil {
				if stats := w.Proxy.GetStats(); stats != nil {
					connCount += int(stats["active_conns"])
				}
			}
		}
		perf := s.hostStats.perf()
		c.SSEvent("overview", gin.H{
			"device_count":     len(workers),
			"online_count":     online,
			"connection_count": connCount,
			"host_perf":        perf,
		})
		c.Writer.Flush()
	}
	sendOverview()

	for {
		select {
		case <-notify:
			close(cancelFanin)
			stopAllSubs()
			return
		case <-ctx.Done():
			close(cancelFanin)
			stopAllSubs()
			return
		case <-s.shutdownCh:
			close(cancelFanin)
			stopAllSubs()
			return
		case <-resyncTicker.C:
			syncSubscriptions()
		case <-trafficTicker.C:
			// 聚合所有设备的最新快照
			mu.Lock()
			var agg dashboardAggregatedTraffic
			agg.ActiveDevs = make([]string, 0, len(latest))
			allOK := true
			for id, snap := range latest {
				if snap.Status != proxytraffic.RealtimeStatusOK {
					if snap.Status == proxytraffic.RealtimeStatusError {
						allOK = false
					}
					continue
				}
				agg.RXBPS += snap.RXBPS
				agg.TXBPS += snap.TXBPS
				agg.RXDeltaBytes += snap.RXDeltaBytes
				agg.TXDeltaBytes += snap.TXDeltaBytes
				agg.TotalRXBytes += snap.TotalRXBytes
				agg.TotalTXBytes += snap.TotalTXBytes
				agg.ActiveDevs = append(agg.ActiveDevs, id)
			}
			mu.Unlock()

			agg.DeviceCount = len(agg.ActiveDevs)
			agg.Timestamp = time.Now().UTC().Format(time.RFC3339)
			if agg.DeviceCount == 0 {
				agg.Status = "waiting_sample"
			} else if allOK {
				agg.Status = "ok"
			} else {
				agg.Status = "ok" // 部分设备错误仍报 ok
			}

			c.SSEvent("traffic", agg)
			c.Writer.Flush()

			// 宿主机性能数据（1秒间隔，与 traffic 同步）
			c.SSEvent("host_perf", s.hostStats.perf())
			c.Writer.Flush()
		case <-overviewTicker.C:
			sendOverview()
		}
	}
}
