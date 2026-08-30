package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleDeviceMgmtReloadSIM 执行 SIM 卡 power cycle（断电→上电），用于修复 UIM 状态异常。
//
// @Summary      DeviceMgmtReloadSIM
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        device_id  path      string  true  "device_id"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/{device_id}/actions/reload-sim [post]
// @Security     BearerAuth
func (s *Server) handleDeviceMgmtReloadSIM(c *gin.Context) {
	id := deviceIDParam(c)
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少 device_id"})
		return
	}

	if err := s.pool.ReloadSIM(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "SIM 卡重载完成，VoWiFi 将自动恢复"})
}
