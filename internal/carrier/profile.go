package carrier

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// YAML 源配置（用户决策：GitHub 为主源，CDN fallback）。
const (
	// YAMLBaseURL 是运营商画像仓库的 GitHub Raw 根地址。
	YAMLBaseURL = "https://raw.githubusercontent.com/voorz/ios-carrier-profiles/main"
	// YAMLFallbackURL 是备用 CDN 地址。
	YAMLFallbackURL = "https://cdn.jsdelivr.net/gh/voorz/ios-carrier-profiles@main"
)

// YAMLCProfile 是 YAML 画像的结构（简化版，只取需要的字段）。
type YAMLCProfile struct {
	PLMN     string `yaml:"plmn"`
	Name     string `yaml:"name"`
	EPDG     string `yaml:"epdg"`
	PCSCF    string `yaml:"pcscf"`
	IPsec    *bool  `yaml:"ipsec"`
	AKAPref  string `yaml:"aka_preference"`
	Template string `yaml:"template_level"`
}

// YAMLFetcher 从外部源拉取运营商 YAML。
type YAMLFetcher struct {
	baseURL     string
	fallbackURL string
	cacheDir    string
	client      *http.Client
}

// NewYAMLFetcher 创建 fetcher。cacheDir 为空则用系统缓存目录。
func NewYAMLFetcher(cacheDir string) *YAMLFetcher {
	if cacheDir == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			cacheDir = filepath.Join(dir, "vohive", "carrier-profiles")
		} else {
			cacheDir = filepath.Join(os.TempDir(), "vohive-carrier-profiles")
		}
	}
	return &YAMLFetcher{
		baseURL:     YAMLBaseURL,
		fallbackURL: YAMLFallbackURL,
		cacheDir:    cacheDir,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Fetch 拉取指定 PLMN 的 YAML（先查本地缓存，再试 GitHub，最后 CDN）。
// 返回解析后的画像。注意：拉取不自动启用，需用户手动确认。
func (f *YAMLFetcher) Fetch(plmn string) (*YAMLCProfile, error) {
	// 1. 本地缓存
	if data, err := f.loadCache(plmn); err == nil {
		if p, err := parseYAML(data); err == nil {
			return p, nil
		}
	}

	// 2. GitHub 主源
	url := fmt.Sprintf("%s/%s.yaml", f.baseURL, plmn)
	if data, err := f.fetchURL(url); err == nil {
		_ = f.saveCache(plmn, data)
		return parseYAML(data)
	}

	// 3. CDN fallback
	url = fmt.Sprintf("%s/%s.yaml", f.fallbackURL, plmn)
	if data, err := f.fetchURL(url); err == nil {
		_ = f.saveCache(plmn, data)
		return parseYAML(data)
	}

	return nil, fmt.Errorf("无法拉取 PLMN %s 的运营商画像（GitHub 和 CDN 均失败）", plmn)
}

func (f *YAMLFetcher) fetchURL(url string) ([]byte, error) {
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (f *YAMLFetcher) cachePath(plmn string) string {
	return filepath.Join(f.cacheDir, plmn+".yaml")
}

func (f *YAMLFetcher) loadCache(plmn string) ([]byte, error) {
	return os.ReadFile(f.cachePath(plmn))
}

func (f *YAMLFetcher) saveCache(plmn string, data []byte) error {
	if err := os.MkdirAll(f.cacheDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(f.cachePath(plmn), data, 0644)
}

func parseYAML(data []byte) (*YAMLCProfile, error) {
	var p YAMLCProfile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("YAML 解析失败: %w", err)
	}
	return &p, nil
}

// DeriveSystemDefault 按 3GPP 规则推导系统默认配置（只读）。
// ePDG 域名按标准规则生成；非标准域名会导致 NXDOMAIN，需用户手动拉取纠正。
func DeriveSystemDefault(mcc, mnc string) *CarrierProfile {
	// MNC 补零到 3 位
	if len(mnc) < 3 {
		mnc = "00" + mnc
		if len(mnc) > 3 {
			mnc = mnc[len(mnc)-3:]
		}
	}
	epdg := fmt.Sprintf("epdg.epc.mnc%s.mcc%s.pub.3gppnetwork.org", mnc, mcc)
	// 3GPP TS 23.003: P-CSCF 域名推导规则
	pcscf := fmt.Sprintf("pcscf.epc.mnc%s.mcc%s.pub.3gppnetwork.org", mnc, mcc)
	p := CarrierProfile{
		"plmn":         PlmnKey(mcc, mnc),
		"name":         "系统默认（自动推导）",
		"epdg":         epdg,
		"template":     "system",
		"derived":      true,
		"derived_note": "按 3GPP 标准规则推导；非标准 ePDG 域名需手动拉取纠正",
		"ims": map[string]interface{}{
			"pcscf_addr": pcscf,
		},
	}
	return &p
}
