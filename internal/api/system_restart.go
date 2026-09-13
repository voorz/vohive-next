package api

import (
	"bytes"
	"net/http"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/pkg/logger"
)

// runSystemctl 执行 systemctl 命令并捕获 stderr 输出。
// 优先使用 sudo（非 root 场景），失败时回退到不带 sudo。
func runSystemctl(action string) error {
	// 尝试 sudo systemctl（非 root 用户场景）
	cmd := exec.Command("sudo", "systemctl", action, "vohive")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		logger.Error("sudo systemctl 失败，尝试不带 sudo",
			"action", action,
			"err", err,
			"stderr", stderrStr,
		)

		// 回退：不带 sudo 重试（root 用户场景，或已配置 polkit）
		cmd2 := exec.Command("systemctl", action, "vohive")
		var stderr2 bytes.Buffer
		cmd2.Stderr = &stderr2

		if err2 := cmd2.Run(); err2 != nil {
			stderr2Str := stderr2.String()
			logger.Error("systemctl 最终失败",
				"action", action,
				"err", err2,
				"stderr", stderr2Str,
			)
			return err2
		}

		logger.Info("systemctl（不带 sudo）成功", "action", action)
		return nil
	}

	logger.Info("sudo systemctl 成功", "action", action)
	return nil
}

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

		if err := runSystemctl("restart"); err != nil {
			logger.Error("重启 vohive 服务失败", "err", err)
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

		if err := runSystemctl("stop"); err != nil {
			logger.Error("停止 vohive 服务失败", "err", err)
			return
		}
	}()
}
