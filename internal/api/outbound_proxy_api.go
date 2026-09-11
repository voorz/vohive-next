package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/pkg/logger"
)

// 出站代理 API handlers（Phase 4）
//
// 这 5 个端点是对 Phase 2 outbound_proxy_reconcile.go 中业务方法的 HTTP 封装。
// 章程锚点：enable/disable 需同步更新 cardpolicy.OutboundProxyEnabled（跟卡走）。

// handleOutboundProxyEnable 开启出站代理
//
// @Summary      开启设备出站代理
// @Description  更新 cardpolicy OutboundProxyEnabled=true → 创建代理实例 → SyncProxyConfigs
// @Tags         outbound-proxy
// @Produce      json
// @Param        device_id  path      string  true  "设备 ID"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      500  {object}  map[string]interface{}  "内部错误"
// @Router       /devices/{device_id}/outbound-proxy/enable [post]
// @Security     BearerAuth
func (s *Server) handleOutboundProxyEnable(c *gin.Context) {
	deviceID := deviceIDParam(c)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}

	// 通过 patchCardPolicyForDevice 获取当前 ICCID 并更新 cardpolicy
	iccid, applied, err := s.patchCardPolicyForDevice(deviceID, func(p *db.CardPolicy) {
		p.OutboundProxyEnabled = true
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if !applied {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "设备当前无 ICCID，无法开启出站代理"})
		return
	}

	// 创建代理实例
	if err := s.EnsureOutboundProxyForDevice(deviceID, iccid); err != nil {
		logger.Error("开启出站代理失败", "device", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "创建代理实例失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "出站代理已开启",
		"iccid":   iccid,
	})
}

// handleOutboundProxyDisable 关闭出站代理
//
// @Summary      关闭设备出站代理
// @Description  更新 cardpolicy OutboundProxyEnabled=false → 销毁代理实例 → SyncProxyConfigs
// @Tags         outbound-proxy
// @Produce      json
// @Param        device_id  path      string  true  "设备 ID"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      500  {object}  map[string]interface{}  "内部错误"
// @Router       /devices/{device_id}/outbound-proxy/disable [post]
// @Security     BearerAuth
func (s *Server) handleOutboundProxyDisable(c *gin.Context) {
	deviceID := deviceIDParam(c)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}

	// 更新 cardpolicy
	_, applied, err := s.patchCardPolicyForDevice(deviceID, func(p *db.CardPolicy) {
		p.OutboundProxyEnabled = false
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if !applied {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "设备当前无 ICCID，无法关闭出站代理"})
		return
	}

	// 销毁代理实例（同时取消暴露的前置代理）
	if err := s.DestroyOutboundProxyForDevice(deviceID, ""); err != nil {
		logger.Error("关闭出站代理失败", "device", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "销毁代理实例失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "出站代理已关闭",
	})
}

// handleOutboundProxyStatus 查询出站代理状态
//
// @Summary      查询设备出站代理状态
// @Description  返回实例信息、运行状态、连入数量、op_ready、是否暴露为前置代理
// @Tags         outbound-proxy
// @Produce      json
// @Param        device_id  path      string  true  "设备 ID"
// @Success      200  {object}  map[string]interface{}  "出站代理状态"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      500  {object}  map[string]interface{}  "内部错误"
// @Router       /devices/{device_id}/outbound-proxy [get]
// @Security     BearerAuth
func (s *Server) handleOutboundProxyStatus(c *gin.Context) {
	deviceID := deviceIDParam(c)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}

	status, err := s.GetOutboundProxyStatus(deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// handleOutboundProxyExposeUpstream 暴露为前置代理
//
// @Summary      将出站代理暴露为前置代理
// @Description  将出站代理信息写入 upstream_proxies 表并自动绑定国家规则
// @Tags         outbound-proxy
// @Accept       json
// @Produce      json
// @Param        device_id    path      string  true  "设备 ID"
// @Param        country_code body      string  false "国家代码（可选，不传则不绑定国家规则）"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      500  {object}  map[string]interface{}  "内部错误"
// @Router       /devices/{device_id}/outbound-proxy/expose-upstream [post]
// @Security     BearerAuth
func (s *Server) handleOutboundProxyExposeUpstream(c *gin.Context) {
	deviceID := deviceIDParam(c)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}

	var req struct {
		CountryCode string `json:"country_code"`
	}
	_ = c.ShouldBindJSON(&req)

	// 获取设备当前 ICCID
	iccid := s.pool.CurrentICCIDForDevice(deviceID)
	if iccid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "设备当前无 ICCID"})
		return
	}

	if err := s.ExposeOutboundProxyAsUpstream(deviceID, iccid, req.CountryCode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "已暴露为前置代理",
	})
}

// handleOutboundProxyUnexposeUpstream 取消暴露前置代理
//
// @Summary      取消将出站代理暴露为前置代理
// @Description  从 upstream_proxies 表删除出站代理的派生入口
// @Tags         outbound-proxy
// @Produce      json
// @Param        device_id  path      string  true  "设备 ID"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      500  {object}  map[string]interface{}  "内部错误"
// @Router       /devices/{device_id}/outbound-proxy/unexpose-upstream [post]
// @Security     BearerAuth
func (s *Server) handleOutboundProxyUnexposeUpstream(c *gin.Context) {
	deviceID := deviceIDParam(c)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}

	// 获取设备当前 ICCID（用于精确匹配实例）
	iccid := s.pool.CurrentICCIDForDevice(deviceID)

	if err := s.UnexposeOutboundProxyAsUpstream(deviceID, iccid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "已取消暴露前置代理",
	})
}
