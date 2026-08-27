package api

import (
	"net/http"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/pkg/logger"
)

// handleSystemRestart 重启 vohive 服务（systemctl restart vohive）
//
// 通过 goroutine 延迟 1 秒执行 systemctl restart，
// 确保 HTTP 响应能正常返回给客户端。
//
// @Summary      SystemRestart
// @Description  Restart the vohive systemd service
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "重启指令已发送"
// @Failure      500  {object}  map[string]interface{}  "服务不可用"
// @Router       /system/restart [post]
// @Security     BearerAuth
func (s *Server) handleSystemRestart(c *gin.Context) {
	logger.Info("收到服务重启请求", "ip", c.ClientIP())

	// 先响应客户端，再异步执行重启
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "服务即将重启，请在 5 秒后刷新页面",
	})

	// 异步延迟执行，确保响应已发送
	go func() {
		time.Sleep(1 * time.Second)
		logger.Info("正在重启 vohive 服务...")

		// 尝试 systemctl restart vohive
		cmd := exec.Command("systemctl", "restart", "vohive")
		if err := cmd.Run(); err != nil {
			logger.Error("systemctl restart vohive 失败", "err", err)
			return
		}
		// 如果 systemctl 成功，当前进程会被 systemd 终止并重启
		// 不会执行到这里
	}()
}

// handleSystemStop 停止 vohive 服务（systemctl stop vohive）
//
// 通过 goroutine 延迟 1 秒执行 systemctl stop，
// 确保 HTTP 响应能正常返回给客户端。
//
// @Summary      SystemStop
// @Description  Stop the vohive systemd service
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "停止指令已发送"
// @Failure      500  {object}  map[string]interface{}  "服务不可用"
// @Router       /system/stop [post]
// @Security     BearerAuth
func (s *Server) handleSystemStop(c *gin.Context) {
	logger.Info("收到服务停止请求", "ip", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "服务即将停止",
	})

	go func() {
		time.Sleep(1 * time.Second)
		logger.Info("正在停止 vohive 服务...")

		cmd := exec.Command("systemctl", "stop", "vohive")
		if err := cmd.Run(); err != nil {
			logger.Error("systemctl stop vohive 失败", "err", err)
			return
		}
	}()
}
