package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/db"
)

type enabledPatchRequest struct {
	Enabled *bool `json:"enabled"`
}

type networkPatchRequest struct {
	Enabled   *bool  `json:"enabled"`
	IPVersion string `json:"ip_version"`
	APN       string `json:"apn"`
}

// handleDeviceNetworkPatch
//
// @Summary      DeviceNetworkPatch
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        device_id  path      string  true  "device_id"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/{device_id}/network [patch]
// @Security     BearerAuth
func (s *Server) handleDeviceNetworkPatch(c *gin.Context) {
	var req networkPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "enabled 为必填项"})
		return
	}

	deviceID := deviceIDParam(c)

	if *req.Enabled {
		// 仅落库 IP/APN（不提前改 NetworkEnabled，避免 StartNetwork 失败时 DB 与实际状态不符）
		// NetworkEnabled 的落库由 handleDeviceMgmtStartNetwork 根据 StartNetwork 结果决定
		ipVersion := strings.TrimSpace(req.IPVersion)
		apn := strings.TrimSpace(req.APN)
		_, _, _ = s.patchCardPolicyForDevice(deviceID, func(p *db.CardPolicy) {
			if ipVersion != "" {
				p.IPVersion = ipVersion
			}
			p.APN = apn
		})
		// 同步 worker 配置中的 IP/APN（NetworkEnabled 由 handleDeviceMgmtStartNetwork 落库后同步）
		s.pool.SetWorkerNetworkPolicy(deviceID, false, ipVersion, apn)
		s.handleDeviceMgmtStartNetwork(c, ipVersion, apn)
		return
	}

	// enabled=false：落库 network_enabled=false
	s.patchCardPolicyForDevice(deviceID, func(p *db.CardPolicy) {
		p.NetworkEnabled = false
	})
	s.handleDeviceMgmtStopNetwork(c)
}

// handleDeviceVoWiFiPatch
//
// @Summary      DeviceVoWiFiPatch
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        device_id  path      string  true  "device_id"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/{device_id}/vowifi [patch]
// @Security     BearerAuth
func (s *Server) handleDeviceVoWiFiPatch(c *gin.Context) {
	var req enabledPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "enabled 为必填项"})
		return
	}

	deviceID := deviceIDParam(c)

	if *req.Enabled {
		// 落库：仅置 vowifi_enabled=true。不碰 airplane_enabled——它是用户的纯飞行
		// 意图，作为关闭 VoWiFi 后的回退依据；VoWiFi 接管射频由运行时投影派生。
		_, applied, _ := s.patchCardPolicyForDevice(deviceID, vowifiEnablePolicyMutation)
		// 同步 w.Config，使概览即时切到 VoWiFi 模式面板（EnableVoWiFi 不碰 Config）。
		s.pool.SetWorkerVoWiFiPolicy(deviceID, true)
		s.handleVoWiFiEnable(c)
		// PC/SC 设备启动 VoWiFi 前 ICCID 可能尚未刷新，导致上面 patchCardPolicyForDevice 跳过。
		// VoWiFi 启动后 ICCID 已写入 worker 身份缓存，补写一次确保卡策略表一致。
		if !applied {
			s.patchCardPolicyForDevice(deviceID, vowifiEnablePolicyMutation)
		}
		return
	}

	// 落库：仅清 vowifi_enabled=false，保留 airplane_enabled（用户飞行意图）。
	// 关闭 VoWiFi 后 DisableVoWiFi 会按当前卡策略重投影：之前是飞行则回飞行，否则回在线。
	s.patchCardPolicyForDevice(deviceID, vowifiDisablePolicyMutation)
	s.pool.SetWorkerVoWiFiPolicy(deviceID, false)
	s.handleVoWiFiDisable(c)
}

// vowifiEnablePolicyMutation 开 VoWiFi 的落库副作用：只置 vowifi，飞行意图保持不变。
func vowifiEnablePolicyMutation(p *db.CardPolicy) { p.VoWiFiEnabled = true }

// vowifiDisablePolicyMutation 关 VoWiFi 的落库副作用：只清 vowifi，保留用户飞行意图以便回退。
func vowifiDisablePolicyMutation(p *db.CardPolicy) { p.VoWiFiEnabled = false }
