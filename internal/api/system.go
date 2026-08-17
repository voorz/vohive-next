package api

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/internal/updater"
	"github.com/voorz/vohive/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

// handleGetUpdateRepo 返回当前 release 源配置（无默认值）
// handleGetUpdateRepo 
//
// @Summary      GetUpdateRepo
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/update-repo [get]
// @Security     BearerAuth
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
// handleDeleteUpdateRepo 
//
// @Summary      DeleteUpdateRepo
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/update-repo [delete]
// @Security     BearerAuth
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

// handleGetVoWiFiBehavior 返回当前 VoWiFi 行为配置
// handleGetVoWiFiBehavior 
//
// @Summary      GetVoWiFiBehavior
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/vowifi-behavior [get]
// @Security     BearerAuth
func (s *Server) handleGetVoWiFiBehavior(c *gin.Context) {
	b := s.fullCfg.VoWiFi.Behavior
	c.JSON(http.StatusOK, gin.H{
		"ike_retry_count": b.IKERetryCount,
		"override_rf_off": b.OverrideRFOff,
		"rf_off_delay":    b.RFOffDelay,
	})
}

// handleUpdateVoWiFiBehavior 更新 VoWiFi 行为配置并热加载
func (s *Server) handleUpdateVoWiFiBehavior(c *gin.Context) {
	var req struct {
		IKERetryCount int  `json:"ike_retry_count"`
		OverrideRFOff bool `json:"override_rf_off"`
		RFOffDelay    int  `json:"rf_off_delay"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}
	if req.IKERetryCount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "IKE 重传次数不能为负数"})
		return
	}
	if req.RFOffDelay < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "RFOff 延迟不能为负数"})
		return
	}
	configPath := config.GetConfigPath()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "配置文件路径未初始化"})
		return
	}
	if err := config.UpdateVoWiFiBehaviorInFile(configPath, req.IKERetryCount, req.OverrideRFOff, req.RFOffDelay); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := config.ReloadFromFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "热加载配置失败: " + err.Error()})
		return
	}
	// 热更新 IKE 重传次数到运行时
	if s.pool != nil {
		s.pool.UpdateVoWiFiBehavior(req.IKERetryCount)
	}
	c.JSON(http.StatusOK, gin.H{
		"status":          "ok",
		"ike_retry_count": req.IKERetryCount,
		"override_rf_off": req.OverrideRFOff,
		"rf_off_delay":    req.RFOffDelay,
	})
}

// handleGetSMSRateLimit 返回当前短信限速配置
// handleGetSMSRateLimit 
//
// @Summary      GetSMSRateLimit
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/sms-limit [get]
// @Security     BearerAuth
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

// handleListReleases 获取 Release 列表
// handleListReleases 
//
// @Summary      ListReleases
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /system/update/releases [get]
// @Security     BearerAuth
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
// handleLocalUpdate 
//
// @Summary      LocalUpdate
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /system/update/local [post]
// @Security     BearerAuth
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

// handleGetSite 返回站点信息配置
// handleGetSite 
//
// @Summary      GetSite
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/site [get]
// @Security     BearerAuth
func (s *Server) handleGetSite(c *gin.Context) {
	cfg := config.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "配置未初始化"})
		return
	}
	site := cfg.Site
	c.JSON(http.StatusOK, gin.H{
		"name":        site.Name,
		"subtitle":    site.Subtitle,
		"has_logo":    site.LogoExt != "",
		"has_favicon": site.FaviconExt != "",
	})
}

// handleUpdateSite 更新站点名称和副标题
func (s *Server) handleUpdateSite(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Subtitle string `json:"subtitle"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}
	configPath := config.GetConfigPath()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "配置文件路径未初始化"})
		return
	}
	cfg := config.GetConfig()
	logoExt := ""
	faviconExt := ""
	if cfg != nil {
		logoExt = cfg.Site.LogoExt
		faviconExt = cfg.Site.FaviconExt
	}
	if err := config.UpdateSiteInFile(configPath, req.Name, req.Subtitle, logoExt, faviconExt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := config.ReloadFromFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "热加载配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleUploadSiteLogo 上传自定义 logo
// handleUploadSiteLogo 
//
// @Summary      UploadSiteLogo
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/site/logo [post]
// @Security     BearerAuth
func (s *Server) handleUploadSiteLogo(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "未接收到文件"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "文件缺少扩展名"})
		return
	}
	// 保存文件
	dataDir := filepath.Join("data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "创建目录失败"})
		return
	}
	logoPath := filepath.Join(dataDir, "site-logo"+ext)
	if err := c.SaveUploadedFile(file, logoPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "保存文件失败"})
		return
	}
	// 删除旧文件（如果扩展名不同）
	cfg := config.GetConfig()
	if cfg != nil && cfg.Site.LogoExt != "" && cfg.Site.LogoExt != ext {
		oldPath := filepath.Join(dataDir, "site-logo"+cfg.Site.LogoExt)
		os.Remove(oldPath)
	}
	// 更新配置
	configPath := config.GetConfigPath()
	name := "VoHive"
	subtitle := "VoWiFi 管理控制台"
	faviconExt := ""
	if cfg != nil {
		name = cfg.Site.Name
		subtitle = cfg.Site.Subtitle
		faviconExt = cfg.Site.FaviconExt
	}
	if err := config.UpdateSiteInFile(configPath, name, subtitle, ext, faviconExt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	config.ReloadFromFile()
	logger.Info("站点 logo 已更新", "ext", ext)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleUploadSiteFavicon 上传自定义 favicon
// handleUploadSiteFavicon 
//
// @Summary      UploadSiteFavicon
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/site/favicon [post]
// @Security     BearerAuth
func (s *Server) handleUploadSiteFavicon(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "未接收到文件"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "文件缺少扩展名"})
		return
	}
	dataDir := filepath.Join("data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "创建目录失败"})
		return
	}
	favPath := filepath.Join(dataDir, "site-favicon"+ext)
	if err := c.SaveUploadedFile(file, favPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "保存文件失败"})
		return
	}
	cfg := config.GetConfig()
	if cfg != nil && cfg.Site.FaviconExt != "" && cfg.Site.FaviconExt != ext {
		oldPath := filepath.Join(dataDir, "site-favicon"+cfg.Site.FaviconExt)
		os.Remove(oldPath)
	}
	configPath := config.GetConfigPath()
	name := "VoHive"
	subtitle := "VoWiFi 管理控制台"
	logoExt := ""
	if cfg != nil {
		name = cfg.Site.Name
		subtitle = cfg.Site.Subtitle
		logoExt = cfg.Site.LogoExt
	}
	if err := config.UpdateSiteInFile(configPath, name, subtitle, logoExt, ext); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	config.ReloadFromFile()
	logger.Info("站点 favicon 已更新", "ext", ext)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleServeSiteLogo 提供自定义 logo 文件（无需鉴权）
// handleServeSiteLogo 
//
// @Summary      ServeSiteLogo
// @Tags         site
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /site/logo [get]
// @Security     BearerAuth
func (s *Server) handleServeSiteLogo(c *gin.Context) {
	cfg := config.GetConfig()
	if cfg == nil || cfg.Site.LogoExt == "" {
		c.Status(http.StatusNoContent)
		return
	}
	logoPath := filepath.Join("data", "site-logo"+cfg.Site.LogoExt)
	data, err := os.ReadFile(logoPath)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	ct := mime.TypeByExtension(cfg.Site.LogoExt)
	if ct == "" {
		ct = "application/octet-stream"
	}
	c.Data(http.StatusOK, ct, data)
}

// handleServeSiteFavicon 提供自定义 favicon 文件（无需鉴权）
// handleServeSiteFavicon 
//
// @Summary      ServeSiteFavicon
// @Tags         site
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /site/favicon [get]
// @Security     BearerAuth
func (s *Server) handleServeSiteFavicon(c *gin.Context) {
	cfg := config.GetConfig()
	if cfg == nil || cfg.Site.FaviconExt == "" {
		// 回退到默认 favicon.svg
		c.Redirect(http.StatusFound, "/favicon.svg")
		return
	}
	favPath := filepath.Join("data", "site-favicon"+cfg.Site.FaviconExt)
	data, err := os.ReadFile(favPath)
	if err != nil {
		c.Redirect(http.StatusFound, "/favicon.svg")
		return
	}
	ct := mime.TypeByExtension(cfg.Site.FaviconExt)
	if ct == "" {
		ct = "image/x-icon"
	}
	c.Data(http.StatusOK, ct, data)
}

// handleGetSecurity 返回安全配置
// handleGetSecurity 
//
// @Summary      GetSecurity
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/security [get]
// @Security     BearerAuth
func (s *Server) handleGetSecurity(c *gin.Context) {
	cfg := config.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "配置未初始化"})
		return
	}
	sec := cfg.Security
	c.JSON(http.StatusOK, gin.H{
		"login_window_minutes": sec.LoginWindowMinutes,
		"login_max_attempts":   sec.LoginMaxAttempts,
		"token_ttl_hours":      sec.TokenTTLHours,
	})
}

// handleUpdateSecurity 更新安全配置并热加载
func (s *Server) handleUpdateSecurity(c *gin.Context) {
	var req struct {
		LoginWindowMinutes int `json:"login_window_minutes"`
		LoginMaxAttempts   int `json:"login_max_attempts"`
		TokenTTLHours      int `json:"token_ttl_hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}
	if req.LoginWindowMinutes <= 0 || req.LoginMaxAttempts <= 0 || req.TokenTTLHours <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数必须大于 0"})
		return
	}
	configPath := config.GetConfigPath()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "配置文件路径未初始化"})
		return
	}
	if err := config.UpdateSecurityInFile(configPath, req.LoginWindowMinutes, req.LoginMaxAttempts, req.TokenTTLHours); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := config.ReloadFromFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "热加载配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleListAPITokens 列出全部 API Token
// handleListAPITokens 
//
// @Summary      ListAPITokens
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/api-tokens [get]
// @Security     BearerAuth
func (s *Server) handleListAPITokens(c *gin.Context) {
	tokens, err := db.ListAPITokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "tokens": tokens})
}

// handleCreateAPIToken 创建新 API Token（多 Token）
func (s *Server) handleCreateAPIToken(c *gin.Context) {
	var req struct {
		Name       string `json:"name"`
		ExpiryDays int    `json:"expiry_days"` // 0 = 永不过期
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "名称不能为空"})
		return
	}

	token, err := db.CreateAPIToken(name, req.ExpiryDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var expiry int64 = 0
	if req.ExpiryDays > 0 {
		expiry = time.Now().Add(time.Duration(req.ExpiryDays) * 24 * time.Hour).Unix()
	}
	logger.Info("API Token 已创建", "ip", c.ClientIP(), "name", name, "expiry_days", req.ExpiryDays)
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"token":  token,
		"expiry": expiry,
	})
}

// handleDeleteAPIToken 删除指定 API Token（按 ID）
// handleDeleteAPIToken 
//
// @Summary      DeleteAPIToken
// @Tags         settings
// @Produce      json
// @Param        id  path      string  true  "id"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/api-tokens/{id} [delete]
// @Security     BearerAuth
func (s *Server) handleDeleteAPIToken(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "无效的 Token ID"})
		return
	}
	if err := db.DeleteAPIToken(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	logger.Info("API Token 已删除", "ip", c.ClientIP(), "id", id)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleGetServerConfig 返回服务器配置
// handleGetServerConfig 
//
// @Summary      GetServerConfig
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/server [get]
// @Security     BearerAuth
func (s *Server) handleGetServerConfig(c *gin.Context) {
	cfg := config.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "配置未初始化"})
		return
	}
	port := strings.TrimPrefix(cfg.Server.Port, ":")
	c.JSON(http.StatusOK, gin.H{
		"port":  port,
		"debug": cfg.Server.Debug,
	})
}

// handleUpdateServerConfig 更新服务器配置并热加载
func (s *Server) handleUpdateServerConfig(c *gin.Context) {
	var req struct {
		Port  string `json:"port"`
		Debug bool   `json:"debug"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}
	port := strings.TrimSpace(req.Port)
	if port == "" {
		port = "7575"
	}
	configPath := config.GetConfigPath()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "配置文件路径未初始化"})
		return
	}
	if err := config.UpdateServerConfigInFile(configPath, port, req.Debug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := config.ReloadFromFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "热加载配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "配置已保存，端口变更需重启服务生效"})
}

// handleUpdateWebCredentials 更新管理员用户名和密码
// handleUpdateWebCredentials 
//
// @Summary      UpdateWebCredentials
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /settings/web-credentials [put]
// @Security     BearerAuth
func (s *Server) handleUpdateWebCredentials(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误"})
		return
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "用户名和密码不能为空"})
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "密码处理失败"})
		return
	}
	configPath := config.GetConfigPath()
	if configPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "配置文件路径未初始化"})
		return
	}
	if err := config.UpdateWebCredentialsInFile(configPath, username, string(hashed)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "保存配置失败: " + err.Error()})
		return
	}
	// 更新内存
	s.auth.Username = username
	s.auth.Password = string(hashed)
	logger.Info("登录凭据已更新", "username", username, "ip", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "凭据已更新"})
}

// handleCheckUpdate 检查系统更新
// handleCheckUpdate 
//
// @Summary      CheckUpdate
// @Tags         settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /system/update/check [get]
// @Security     BearerAuth
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
