package api

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/pkg/logger"
)

// pcscDriverStatus 表示 PC/SC 驱动检测状态
type pcscDriverStatus struct {
	PcscdInstalled   bool   `json:"pcscd_installed"`
	LibccidInstalled bool  `json:"libccid_installed"`
	PcscdActive      bool   `json:"pcscd_active"`
	AllReady         bool   `json:"all_ready"`
	Message          string `json:"message"`
}

// checkPkgInstalled 通过 dpkg -s 检测包是否已安装
func checkPkgInstalled(pkg string) bool {
	cmd := exec.Command("dpkg", "-s", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "Status: install ok installed")
}

// checkPcscdActive 检测 pcscd 服务是否正在运行
func checkPcscdActive() bool {
	cmd := exec.Command("systemctl", "is-active", "pcscd")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "active"
}

// getPcscDriverStatus 汇总 PC/SC 驱动状态
func getPcscDriverStatus() pcscDriverStatus {
	pcscdInstalled := checkPkgInstalled("pcscd")
	libccidInstalled := checkPkgInstalled("libccid")
	pcscdActive := checkPcscdActive()

	status := pcscDriverStatus{
		PcscdInstalled:   pcscdInstalled,
		LibccidInstalled: libccidInstalled,
		PcscdActive:      pcscdActive,
		AllReady:         pcscdInstalled && libccidInstalled && pcscdActive,
	}

	if status.AllReady {
		status.Message = "PC/SC 驱动已就绪"
	} else if !pcscdInstalled && !libccidInstalled {
		status.Message = "PC/SC 驱动未安装"
	} else if !pcscdActive {
		status.Message = "PC/SC 驱动已安装但服务未运行"
	} else {
		status.Message = "PC/SC 驱动不完整"
	}

	return status
}

// handleGetPcscDriverStatus GET /api/system/pcsc-driver
func (s *Server) handleGetPcscDriverStatus(c *gin.Context) {
	status := getPcscDriverStatus()
	c.JSON(200, status)
}

// handleInstallPcscDriver POST /api/system/pcsc-driver/install
func (s *Server) handleInstallPcscDriver(c *gin.Context) {
	logger.Info("收到 PC/SC 驱动安装请求", "ip", c.ClientIP())

	// 1. apt-get install -y libccid pcscd
	installCmd := exec.Command("apt-get", "install", "-y", "libccid", "pcscd")
	installCmd.Env = append(installCmd.Env, "DEBIAN_FRONTEND=noninteractive")
	if output, err := installCmd.CombinedOutput(); err != nil {
		logger.Error("PC/SC 驱动安装失败", "err", err, "output", string(output))
		c.JSON(500, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("安装失败: %v", err),
		})
		return
	}

	// 2. systemctl enable --now pcscd
	enableCmd := exec.Command("systemctl", "enable", "--now", "pcscd")
	if output, err := enableCmd.CombinedOutput(); err != nil {
		logger.Error("pcscd 启用失败", "err", err, "output", string(output))
		c.JSON(500, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("pcscd 启用失败: %v", err),
		})
		return
	}

	// 3. 等待 pcscd 就绪
	time.Sleep(2 * time.Second)

	// 4. 重新检测状态
	status := getPcscDriverStatus()
	logger.Info("PC/SC 驱动安装完成", "all_ready", status.AllReady)
	c.JSON(200, gin.H{
		"status": "ok",
		"result": status,
	})
}
