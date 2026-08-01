package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/internal/global"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/minio/selfupdate"
	"golang.org/x/mod/semver"
)

// ErrDisabled is returned when in-app binary updates are disabled.
// The original vohive-next build keeps update functionality enabled;
// this var exists for API compatibility with the synchronous error-handling path.
var ErrDisabled = errors.New("in-app binary updates are disabled for this source-integrated build")

type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	Assets      []Asset `json:"assets"`
	PublishedAt string  `json:"published_at"`
	PreRelease  bool    `json:"prerelease"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type UpdateInfo struct {
	HasUpdate   bool   `json:"has_update"`
	CurrentVer  string `json:"current_version"`
	LatestVer   string `json:"latest_version"`
	ReleaseNote string `json:"release_note"`
	IsDocker    bool   `json:"is_docker"`
	Error       string `json:"error,omitempty"`
}

// resolveRepo 从全局配置读取 release 源，未配置时返回空字符串。
func resolveRepo() (owner, name string, configured bool) {
	if cfg := config.GetConfig(); cfg != nil {
		if s := strings.TrimSpace(cfg.UpdateRepo.Owner); s != "" {
			owner = s
		}
		if s := strings.TrimSpace(cfg.UpdateRepo.Name); s != "" {
			name = s
		}
	}
	configured = owner != "" && name != ""
	return
}

// ReleaseListItem 是 ListReleases 返回的单个 Release 摘要。
// 只暴露前端需要的字段，避免泄露完整 Asset 列表。
type ReleaseListItem struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
	PreRelease  bool   `json:"prerelease"`
}

// ListReleases 获取所有 Release 列表（最多 30 条，GitHub API 默认分页）
func ListReleases() ([]ReleaseListItem, error) {
	repoOwner, repoName, configured := resolveRepo()
	if !configured {
		return nil, errors.New("Release 仓库未配置，请先在设置中配置仓库地址")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=30", repoOwner, repoName)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github api failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	items := make([]ReleaseListItem, 0, len(releases))
	for _, r := range releases {
		items = append(items, ReleaseListItem{
			TagName:     r.TagName,
			Name:        r.Name,
			PublishedAt: r.PublishedAt,
			Body:        r.Body,
			PreRelease:  r.PreRelease,
		})
	}
	return items, nil
}

// CheckUpdate 检查是否有新版本
func CheckUpdate() (*UpdateInfo, error) {
	repoOwner, repoName, configured := resolveRepo()
	if !configured {
		return &UpdateInfo{
			Error: "Release 仓库未配置，请先在设置中配置仓库地址",
		}, nil
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github api failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	currentVersion := global.Version
	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}
	latestVersion := release.TagName
	if !strings.HasPrefix(latestVersion, "v") {
		latestVersion = "v" + latestVersion
	}

	// 使用 semver 比较版本
	hasUpdate := false
	if semver.IsValid(currentVersion) && semver.IsValid(latestVersion) {
		if semver.Compare(currentVersion, latestVersion) < 0 {
			hasUpdate = true
		}
	} else {
		// 如果本地或线上不是标准 semver (比如 unknown, dev 等)，可以尝试直接不等即提示更新
		if currentVersion != latestVersion {
			hasUpdate = true
		}
	}

	isDocker := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		isDocker = true
	}

	return &UpdateInfo{
		HasUpdate:   hasUpdate,
		CurrentVer:  currentVersion,
		LatestVer:   latestVersion,
		ReleaseNote: release.Body,
		IsDocker:    isDocker,
	}, nil
}

// ApplyUpdate 获取最新 release 并下载对应架构的二进制进行自我替换
func ApplyUpdate() error {
	repoOwner, repoName, configured := resolveRepo()
	if !configured {
		return errors.New("Release 仓库未配置，请先在设置中配置仓库地址")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	return applyUpdateFromURL(apiURL, "")
}

// ApplyUpdateByTag 按 tag 下载指定 Release 的二进制并自我替换
func ApplyUpdateByTag(tag string) error {
	repoOwner, repoName, configured := resolveRepo()
	if !configured {
		return errors.New("Release 仓库未配置，请先在设置中配置仓库地址")
	}
	if tag == "" {
		return errors.New("tag 不能为空")
	}

	// GitHub API 按 tag 获取单个 Release
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", repoOwner, repoName, tag)
	return applyUpdateFromURL(apiURL, tag)
}

// applyUpdateFromURL 从 GitHub API URL 获取 Release 信息，下载匹配架构的二进制并替换
func applyUpdateFromURL(apiURL string, tag string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api returned status: %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to decode release info: %w", err)
	}

	// 拼接对应的 asset name。例如: vohive_v1.0.0_linux_amd64
	targetGoos := runtime.GOOS
	targetGoarch := runtime.GOARCH
	if targetGoarch == "arm" {
		targetGoarch = "armv7" // 根据 Makefile 中的定义，vohive 编的 arm 是 armv7
	}

	binaryName := "vohive"
	assetPrefix := fmt.Sprintf("%s_%s_%s_%s", binaryName, release.TagName, targetGoos, targetGoarch)

	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, assetPrefix) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no matching asset found for architecture %s_%s", targetGoos, targetGoarch)
	}

	logger.Info("开始下载更新", "url", downloadURL, "tag", release.TagName)

	// 下载二进制
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", dlResp.StatusCode)
	}

	// 执行替换
	err = selfupdate.Apply(dlResp.Body, selfupdate.Options{})
	if err != nil {
		// 回滚
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback failed: %v, original error: %w", rerr, err)
		}
		return fmt.Errorf("update failed: %w", err)
	}

	logger.Info("应用更新成功，正在准备重启...", "tag", release.TagName)

	// 延迟退出以便接口能返回成功响应
	go func() {
		time.Sleep(2 * time.Second)
		logger.Info("进程发出关闭信号以应用更新")
		if process, err := os.FindProcess(os.Getpid()); err == nil {
			process.Signal(syscall.SIGTERM)
		} else {
			os.Exit(0)
		}
	}()

	return nil
}

// ApplyLocalUpdate 从本地文件 reader 替换当前二进制
func ApplyLocalUpdate(r io.Reader) error {
	err := selfupdate.Apply(r, selfupdate.Options{})
	if err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback failed: %v, original error: %w", rerr, err)
		}
		return fmt.Errorf("update failed: %w", err)
	}

	logger.Info("本地二进制更新成功，正在准备重启...")

	go func() {
		time.Sleep(2 * time.Second)
		logger.Info("进程发出关闭信号以应用更新")
		if process, err := os.FindProcess(os.Getpid()); err == nil {
			process.Signal(syscall.SIGTERM)
		} else {
			os.Exit(0)
		}
	}()

	return nil
}
