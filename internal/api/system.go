package api

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/internal/updater"
	"github.com/voorz/vohive/pkg/logger"
)

var errNotFound = errors.New("not found")

// handleGetUpdateRepo 返回当前 release 源配置（无默认值）
func (s *Server) handleGetUpdateRepo(c *gin.Context) {
	cfg := config.GetConfig()
	owner := ""
	name := ""
	if cfg != nil {
		owner = cfg.UpdateRepo.Owner
		name = cfg.UpdateRepo.Name
	}
	c.JSON(http.StatusOK, gin.H{"owner": owner, "name": name})
}

// handleUpdateUpdateRepo 更新 release 源配置并热加载
func (s *Server) handleUpdateUpdateRepo(c *gin.Context) {
	var req struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}
	owner := strings.TrimSpace(req.Owner)
	name := strings.TrimSpace(req.Name)
	if owner == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "owner 和 name 不能为空"})
		return
	}
	configPath := config.GetConfigPath()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "配置文件路径未初始化"})
		return
	}
	if err := config.UpdateUpdateRepoInFile(configPath, owner, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := config.ReloadFromFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "热加载配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "owner": owner, "name": name})
}

// handleDeleteUpdateRepo 删除 release 源配置并热加载
func (s *Server) handleDeleteUpdateRepo(c *gin.Context) {
	configPath := config.GetConfigPath()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "配置文件路径未初始化"})
		return
	}
	if err := config.DeleteUpdateRepoInFile(configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := config.ReloadFromFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "热加载配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleGetSMSRateLimit 返回当前短信限速配置
func (s *Server) handleGetSMSRateLimit(c *gin.Context) {
	hourly, daily := s.smsRateLimiter().Limits()
	c.JSON(http.StatusOK, gin.H{"hourly_limit": hourly, "daily_limit": daily})
}

// handleUpdateSMSRateLimit 更新短信限速配置并热加载
func (s *Server) handleUpdateSMSRateLimit(c *gin.Context) {
	var req struct {
		HourlyLimit int `json:"hourly_limit"`
		DailyLimit  int `json:"daily_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}
	if req.HourlyLimit <= 0 || req.DailyLimit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "限制值必须大于 0"})
		return
	}
	if req.DailyLimit < req.HourlyLimit {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "每日限制不能小于每小时限制"})
		return
	}
	configPath := config.GetConfigPath()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "配置文件路径未初始化"})
		return
	}
	if err := config.UpdateSMSRateLimitInFile(configPath, req.HourlyLimit, req.DailyLimit); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := config.ReloadFromFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "热加载配置失败: " + err.Error()})
		return
	}
	s.smsRateLimiter().UpdateLimits(req.HourlyLimit, req.DailyLimit)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "hourly_limit": req.HourlyLimit, "daily_limit": req.DailyLimit})
}

// resolveUninstallTargets 计算自毁流程需要清理的数据目录和配置文件路径。
// 配置文件路径必须来自运行时实际加载的路径（config.GetConfigPath()），
// 不能假定其固定位于进程工作目录下的 "config" 子目录——
// OpenWrt 部署通过 -c 显式传入 /etc/vohive/config.yaml，与工作目录无关，
// 用硬编码相对路径删除会删错地方（实际等于什么都没删）。
// configPath 为空（配置管理器未初始化）时不返回任何配置文件路径，避免误删。
func resolveUninstallTargets(configPath string) (dataDir string, configFile string) {
	return "data", configPath
}

// detectServiceStopCommands 根据当前部署形态返回应执行的"停止 + 禁用自启"命令。
// systemd 的 Restart=always 和 OpenWrt procd 的 respawn 都只在进程
// "非主动" 退出时才会重新拉起；只要在自毁前显式请求服务管理器停止/禁用，
// 即使后续删除可执行文件失败（例如只读 squashfs），也不会被重新拉起。
// 仅靠"删掉自己导致 exec 失败"这种副作用来阻止重启是不可靠的。
func detectServiceStopCommands(lookPath func(string) (string, error), statFile func(string) bool) [][]string {
	var cmds [][]string
	if statFile("/etc/init.d/vohive") {
		cmds = append(cmds, []string{"/etc/init.d/vohive", "disable"})
		cmds = append(cmds, []string{"/etc/init.d/vohive", "stop"})
		return cmds
	}
	if _, err := lookPath("systemctl"); err == nil {
		cmds = append(cmds, []string{"systemctl", "disable", "--now", "vohive"})
	}
	return cmds
}

// handleListReleases 获取 Release 列表
func (s *Server) handleListReleases(c *gin.Context) {
	releases, err := updater.ListReleases()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"releases": releases})
}

// handleApplyUpdateByTag 按 tag 下载指定 Release 并应用更新
func (s *Server) handleApplyUpdateByTag(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "tag 不能为空"})
		return
	}
	if err := updater.ApplyUpdateByTag(tag); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, updater.ErrDisabled) {
			status = http.StatusConflict
		} else {
			logger.Error("按 tag 应用更新失败", "tag", tag, "err", err)
		}
		c.JSON(status, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "正在后台下载更新，系统稍后将自动重启..."})
}

// handleLocalUpdate 上传本地二进制文件进行更新
func (s *Server) handleLocalUpdate(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "未接收到文件: " + err.Error()})
		return
	}

	logger.Info("收到本地二进制上传", "filename", file.Filename, "size", file.Size)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "打开上传文件失败: " + err.Error()})
		return
	}
	defer src.Close()

	if err := updater.ApplyLocalUpdate(src); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, updater.ErrDisabled) {
			status = http.StatusConflict
		} else {
			logger.Error("本地二进制更新失败", "err", err)
		}
		c.JSON(status, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "正在后台安装更新，系统稍后将自动重启..."})
}

// handleCheckUpdate 检查系统更新
func (s *Server) handleCheckUpdate(c *gin.Context) {
	info, err := updater.CheckUpdate()
	if err != nil {
		logger.Error("检查系统更新失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

// handleApplyUpdate 应用系统更新
func (s *Server) handleApplyUpdate(c *gin.Context) {
	if err := updater.ApplyUpdate(); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, updater.ErrDisabled) {
			status = http.StatusConflict
		} else {
			logger.Error("应用更新失败", "err", err)
		}
		c.JSON(status, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "正在后台下载更新，系统稍后将自动重启..."})
}

// handleUninstall 自毁/卸载接口，用于用户拒绝免责声明时
// 必须登录后才能调用——这是真正会删数据、删配置、删自身可执行文件并
// os.Exit(0) 的破坏性操作，绝不能允许未鉴权请求触发。
func (s *Server) handleUninstall(c *gin.Context) {
	if !s.isAuthenticatedRequest(c, time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":     "error",
			"code":       "unauthorized",
			"message":    "未授权",
			"request_id": requestID(c),
		})
		return
	}

	logger.Warn("用户拒绝了免责声明，正在触发自毁/卸载逻辑")
	c.JSON(http.StatusOK, gin.H{"message": "正在卸载软件..."})

	// 在后台异步执行自毁，以免请求无法返回
	go func() {
		time.Sleep(1 * time.Second)

		// 先主动通知服务管理器停止 + 禁用自启，确保即使后面的文件删除
		// 失败（只读文件系统等），systemd Restart=always / procd respawn
		// 也不会把进程重新拉起来。命令异步触发(Start 不 Wait)，
		// 避免对"停止自己"这条命令的等待造成死锁。
		for _, args := range detectServiceStopCommands(exec.LookPath, fileExists) {
			cmd := exec.Command(args[0], args[1:]...)
			if err := cmd.Start(); err != nil {
				logger.Warn("通知服务管理器停止失败", "cmd", args, "err", err)
				continue
			}
			go cmd.Wait()
		}

		dataDir, configFile := resolveUninstallTargets(config.GetConfigPath())

		if err := os.RemoveAll(dataDir); err != nil {
			logger.Warn("清理数据目录失败", "dir", dataDir, "err", err)
		}
		if configFile != "" {
			if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
				logger.Warn("清理配置文件失败", "file", configFile, "err", err)
			}
		}
		if executable, err := os.Executable(); err == nil {
			if err := os.Remove(executable); err != nil {
				logger.Warn("删除可执行文件失败", "file", executable, "err", err)
			}
		}

		logger.Warn("自毁流程结束，退出进程")
		os.Exit(0)
	}()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
