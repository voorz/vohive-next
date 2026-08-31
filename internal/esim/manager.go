package esim

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/damonto/euicc-go/bertlv"
	"github.com/damonto/euicc-go/driver"
	"github.com/damonto/euicc-go/lpa"
	sgp22 "github.com/damonto/euicc-go/v2"
	"github.com/voorz/vohive/internal/apduarbiter"
	backendpkg "github.com/voorz/vohive/internal/backend"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/internal/modem"
	"github.com/voorz/vohive/pkg/logger"
)

// 支持的 ISD-R AID 列表
var AIDs = [][]byte{
	// eSTK.me Max 专用 SE0/SE1 AID
	{0xA0, 0x65, 0x73, 0x74, 0x6B, 0x6D, 0x65, 0xFF, 0xFF, 0x49, 0x53, 0x44, 0x2D, 0x52, 0x20, 0x30}, // eSTK.me SE0
	{0xA0, 0x65, 0x73, 0x74, 0x6B, 0x6D, 0x65, 0xFF, 0xFF, 0x49, 0x53, 0x44, 0x2D, 0x52, 0x20, 0x31}, // eSTK.me SE1
	// 通用 AID
	lpa.GSMAISDRApplicationAID, // 标准 GSMA ISD-R
	{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0x00, 0x00, 0x00, 0x00, 0x89, 0x00, 0x00, 0x03, 0x00}, // eSIM.me
	{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0x00, 0x00, 0x00, 0x89, 0x00, 0x00, 0x00, 0x03, 0x00}, // eSIM.me V2
	{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0xFF, 0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x05, 0x05, 0x00}, // 5ber.eSIM
	{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0xFF, 0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x00, 0x01, 0x77}, // XeSIM
	{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x4C, 0x69, 0x6E, 0x6B, 0x73, 0x66, 0x69, 0x65, 0x6C, 0x64}, // LinksField
	{0xA0, 0x00, 0x00, 0x06, 0x28, 0x10, 0x10, 0xFF, 0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x00, 0x01, 0x00}, // GlocalMe
	// eSTK.me 旧版 AUX AID（兼容老固件）
	{0xA0, 0x65, 0x73, 0x74, 0x6B, 0x6D, 0x65, 0xFF, 0xFF, 0xFF, 0xFF, 0x49, 0x53, 0x44, 0x2D, 0x52}, // eSTK.me AUX (deprecated)
}

// eSTK.me AID 共有前缀(7 bytes): A0 65 73 74 6B 6D 65
var estkmeAIDPrefix = []byte{0xA0, 0x65, 0x73, 0x74, 0x6B, 0x6D, 0x65}

var aidOrderRank = func() map[string]int {
	rank := make(map[string]int, len(AIDs))
	for i, aid := range AIDs {
		rank[strings.ToUpper(hex.EncodeToString(aid))] = i
	}
	return rank
}()

// isESTKmeAID 判断 AID 是否属于 eSTK.me 系列
func isESTKmeAID(aid []byte) bool {
	return len(aid) >= len(estkmeAIDPrefix) && bytes.Equal(aid[:len(estkmeAIDPrefix)], estkmeAIDPrefix)
}

// predictSkuName 尝试根据 EID 前缀和固件版本预测卡片的品牌名称（如 9eSIM）
// 参考自 OpenEUICC 源码逻辑
func predictSkuName(eid string, firmware string) string {
	if eid == "" || firmware == "" {
		return ""
	}

	// 判断是否为 9eSIM 典型的 EID 前缀 (890440458467274948 / 890440452167274948等)
	if strings.HasPrefix(eid, "890440458467274948") || strings.HasPrefix(eid, "890440452167274948") {
		// 简单解析固件版本
		parts := strings.Split(firmware, ".")
		if len(parts) == 3 {
			v1, _ := strconv.Atoi(parts[0])
			v2, _ := strconv.Atoi(parts[1])
			v3, _ := strconv.Atoi(parts[2])

			// 粗略判断版本对应名称（简化自 OpenEUICC 的版本阈值）
			var verName string
			if v1 > 37 || (v1 == 37 && v2 >= 4) {
				verName = "v3.2"
			} else if v1 == 37 && v2 == 1 && v3 >= 41 {
				verName = "v3.1"
			} else if v1 == 36 && v2 >= 18 {
				verName = "v3"
			} else if v1 == 36 && v2 >= 17 && v3 >= 39 {
				verName = "v3 (beta)"
			} else if v1 == 36 && v2 == 17 && v3 >= 4 {
				verName = "v2s"
			} else if v1 == 36 && v2 >= 9 {
				verName = "v2.1"
			} else if v1 == 36 && v2 >= 7 {
				verName = "v2"
			} else if v1 < 36 {
				// 兼容可能的新规则或者我们自己测试提取的 25.x 固件
				verName = fmt.Sprintf("v%d", v1)
			}

			if verName != "" {
				return "9eSIM " + verName
			}
		}
		return "9eSIM"
	}

	return ""
}

// shouldContinueAIDScanAfterSuccess 判断在已经发现可用 AID 后是否还需要继续扫描。
// 普通 eUICC 命中首个可用 AID 后即可停止；eSTK Max 可能同时暴露 SE0/SE1，
// 因此命中 eSTK AID 后仍继续探测后续 eSTK AID。
func shouldContinueAIDScanAfterSuccess(successAIDs [][]byte, nextAID []byte) bool {
	if len(successAIDs) == 0 {
		return true
	}
	for _, a := range successAIDs {
		if isESTKmeAID(a) && isESTKmeAID(nextAID) {
			return true
		}
	}
	return false
}

func getAIDRank(aidHex string) int {
	key := strings.ToUpper(strings.TrimSpace(aidHex))
	if v, ok := aidOrderRank[key]; ok {
		return v
	}
	return 1 << 30
}

func sortEUICCProfilesStable(groups []EUICCProfiles) {
	sort.SliceStable(groups, func(i, j int) bool {
		ri := getAIDRank(groups[i].AIDHex)
		rj := getAIDRank(groups[j].AIDHex)
		if ri != rj {
			return ri < rj
		}
		if groups[i].EID != groups[j].EID {
			return groups[i].EID < groups[j].EID
		}
		return strings.ToUpper(groups[i].AIDHex) < strings.ToUpper(groups[j].AIDHex)
	})
}

func sortEUICCInfosStable(infos []EUICCInfo) {
	sort.SliceStable(infos, func(i, j int) bool {
		ri := getAIDRank(infos[i].AIDHex)
		rj := getAIDRank(infos[j].AIDHex)
		if ri != rj {
			return ri < rj
		}
		if infos[i].EID != infos[j].EID {
			return infos[i].EID < infos[j].EID
		}
		return strings.ToUpper(infos[i].AIDHex) < strings.ToUpper(infos[j].AIDHex)
	})
}

func hasReusableChipProductInfo(info *EUICCChipInfo) bool {
	if info == nil {
		return false
	}
	// 仅 SkuName / SerialNumber 来自 eSTK.me Product AID 查询（或 SkuName 由 predictSkuName 推断），
	// 可作为"已获取产品信息"的判据。
	// Firmware 不能作为判据：当 eSTK.me Product AID 查询失败时（如逻辑通道资源耗尽），
	// info.Firmware 会被标准 EUICCInfo2 的固件版本兜底填充。若据此判定缓存可复用，
	// 会永久跳过 parseESTKmeInfo，导致 SkuName/SerialNumber 永远查询不到。
	return strings.TrimSpace(info.SkuName) != "" ||
		strings.TrimSpace(info.SerialNumber) != ""
}

type EUICCSpec string

const (
	EUICCSpecUnknown EUICCSpec = ""
	EUICCSpecSGP22   EUICCSpec = "sgp22"
	EUICCSpecSGP32   EUICCSpec = "sgp32"
	EUICCSpecSGP02   EUICCSpec = "sgp02"
)

// SGP.32 and SGP.02 labels are reserved for future read-only probes. Current consumer LPA-compatible ISD-R sessions are marked SGP.22 unless a later probe proves otherwise.

const (
	euiccSpecGuessSGP22Compat   = "sgp22_compatible"
	euiccSpecConfidenceInferred = "inferred"
	aidScanPolicyFullStatic     = aidScanPolicy("full_static")
)

type aidScanPolicy string

type aidScanPlan struct {
	Policy aidScanPolicy
	AIDs   [][]byte
}

func (p aidScanPlan) CloneAIDs() [][]byte {
	return cloneAIDList(p.AIDs)
}

// EUICCInfo 单个 eUICC 的信息
type EUICCInfo struct {
	AID                    []byte    `json:"-"`
	AIDHex                 string    `json:"aid"`
	EID                    string    `json:"eid"`
	Spec                   EUICCSpec `json:"spec,omitempty"`
	SpecGuess              string    `json:"spec_guess,omitempty"`
	SpecConfidence         string    `json:"spec_confidence,omitempty"`
	FreeNvramBytes         int32     `json:"free_nvram_bytes"`       // 可用 NV 存储（字节）
	FreeNvram              string    `json:"free_nvram"`             // 可用 NV 存储（格式化）
	Firmware               string    `json:"firmware,omitempty"`     // 提取自 EUICCInfo2 / EUICCInfo1
	Manufacturer           string    `json:"manufacturer,omitempty"` // 芯片制造商（基于 EID 和 PKI 数据查询）
	Certificates           []string  `json:"certificates,omitempty"` // 支持的证书签发机构列表
	InfoSource             string    `json:"info_source,omitempty"`  // euicc_info2 / euicc_info1
	InfoVersion            string    `json:"info_version,omitempty"` // 1 / 2
	InfoError              string    `json:"info_error,omitempty"`   // 标准信息读取失败时的诊断信息
	SASAccreditationNumber string    `json:"sas_accreditation_number,omitempty"`
	DefaultSMDPAddress     string    `json:"default_smdp_address,omitempty"`
	RootSMDSAddress        string    `json:"root_ds_address,omitempty"`
}

func buildDiscoveredEUICCInfo(aid []byte, eidStr string) EUICCInfo {
	info := EUICCInfo{
		AID:            append([]byte(nil), aid...),
		AIDHex:         fmt.Sprintf("%X", aid),
		EID:            eidStr,
		SpecGuess:      euiccSpecGuessSGP22Compat,
		SpecConfidence: euiccSpecConfidenceInferred,
	}
	return info
}

// ProfileItem 单个 profile 信息
type ProfileItem struct {
	ICCID               string `json:"iccid"`
	ISDPAIDHex          string `json:"isdp_aid,omitempty"`
	Name                string `json:"name"`
	ServiceProviderName string `json:"service_provider_name"`
	State               int    `json:"state"` // 0=disabled, 1=enabled
	StateText           string `json:"state_text"`
	ClassText           string `json:"class_text,omitempty"`
	MCC                 string `json:"mcc,omitempty"`
	MNC                 string `json:"mnc,omitempty"`
	GID1                string `json:"gid1,omitempty"`
	GID2                string `json:"gid2,omitempty"`
	ProfileSizeBytes    int    `json:"profile_size_bytes,omitempty"`    // profile 实际占用字节（来自 esimstore 查询）
	ProfileSizeFormatted string `json:"profile_size_formatted,omitempty"` // 人类可读大小（如 "38.10 kb"）
	ProfileAlias         string `json:"profile_alias,omitempty"`          // profile 别名（来自 esimstore names[0]）
	ProfileName          string `json:"profile_name,omitempty"`           // SGP22 ProfileName（SM-DP+ 写入的产品名，不可改）
}

// EUICCProfiles 按 eUICC 分组的 profile 列表
type EUICCProfiles struct {
	EID      string        `json:"eid"`
	AIDHex   string        `json:"aid_hex"`
	Profiles []ProfileItem `json:"profiles"`
}

// EUICCChipInfo eUICC 芯片/卡的硬件信息
type EUICCChipInfo struct {
	EIDs         []EUICCInfo `json:"eids"`                    // 所有 EID 列表（含各自可用空间）
	SkuName      string      `json:"sku_name,omitempty"`      // 产品名称（如 "ESTKme Max"）
	SerialNumber string      `json:"serial_number,omitempty"` // 序列号（如 "T3VAMD0"）
	Firmware     string      `json:"firmware,omitempty"`      // 固件版本
}

// eSTK.me Product AID，用于查询设备名称、序列号和固件版本
var estkmeProductAID = []byte{0xA0, 0x65, 0x73, 0x74, 0x6B, 0x6D, 0x65, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x6D, 0x67, 0x74}

// Manager eSIM profile 管理器（支持多 eUICC）
type Manager struct {
	// modem 是基于 AT 命令的通道工厂（Modem 模式用），PC/SC 模式下为 nil
	modem         *modem.Manager
	backend       backendpkg.DeviceBackend
	deviceID      string
	transport     string
	controlDevice string
	imeiProvider  func(ctx context.Context) (string, error)

	// onOverviewUpdated 在 overview 缓存更新后被调用（可选），用于 SSE 推送通知
	onOverviewUpdated OverviewStateCallback

	// channelFactory 是通道工厂函数，不同模式下注入不同实现
	// Modem 模式：基于 AT 命令
	// PC/SC 模式：基于 scard.Card APDU 透传
	channelFactory          func(aid []byte) (*lpa.Client, error)
	smartCardChannelFactory func() (driver.SmartCardChannel, error)
	closeClient             func(client *lpa.Client) error

	// clearChannels 是在首轮 AID 扫描前清理逐辑通道的回调（可选）
	clearChannels func()

	// esimSizeStore 是 eSIM profile 容量查询库（可选，nil 时跳过容量补全）
	esimSizeStore *ESimSizeStore

	cacheMu                     sync.RWMutex   // 保护 chipInfoCache、overviewCache 与 discoveredEUICCs 等快照状态
	opMu                        sync.Mutex     // eSIM 硬件操作互斥（同时只允许一个写操作）
	opDone                      chan struct{}  // 写操作完成通知（替代 TryLock+Sleep 轮询）
	chipInfoCache               *EUICCChipInfo // 芯片信息缓存（硬件信息基本不变）
	overviewCache               *EsimOverview  // eSIM 总览缓存（跟随 Manager / Worker 实例）
	overviewLastErr             error
	overviewReloading           bool
	overviewGeneration          uint64
	overviewLoader              func() (*EsimOverview, error)
	profilesLoader              func() ([]EUICCProfiles, error)
	suppressOverviewReloadUntil time.Time
	discoveredEUICCs            []EUICCInfo
	sf                          *singleflight.Group

	// eUICC 扫描退避：连续失败时自动重试间隔递增，耗尽后停止自动重试。
	// 手动刷新（RefreshOverview）会重置计数器。
	scanBackoffMu    sync.Mutex
	scanBackoffCount int       // 连续失败次数
	scanBackoffUntil time.Time // 下次允许自动重试的时间；zero 表示无限制

	onBeforeSwitch       func(SwitchOperation, string) uint64              // 切卡前执行的回调，返回本次 switch token
	onAfterSwitch        func(SwitchOperation, uint64)                     // 切卡后网络就绪后执行的回调
	onSwitchFailed       func(SwitchOperation, uint64, error)              // 切卡失败后执行的回调
	onSwitchDegraded     func(SwitchOperation, uint64, SwitchPhase, error) // 切卡已接受但后处理降级回调
	onSwitchPhase        func(SwitchOperation, uint64, SwitchPhase)        // 切卡内部阶段变更回调
	switchSignal         chan string
	switchUseRefreshTrue bool

	apduArbiter          apduIdleWaiter
	postSwitchMinDelay   time.Duration
	readQueueWaitTimeout time.Duration

	// downloadCtx 是当前正在进行的下载操作的 context。
	// 如果不为 nil，由 smartCardChannelFactory 新建的 QMIChannel 会自动继承该 context，
	// 从而允许 BPP 安装阶段的长时延迟得到正确处理而不被默认超时中断。
	downloadCtx atomic.Pointer[context.Context]

	// workingAID 缓存最近一次成功命中的 ISD-R AID（对标 NekoKo _workingAidCache）。
	// 下次打开通知列表时优先尝试该 AID，避免全量扫描 9 个候选 AID。
	workingAIDMu sync.RWMutex
	workingAID   []byte
}

// ErrOperationInProgress 表示当前有写操作（下载/切换/删除）正在进行中
// 读操作（GetProfiles / GetEsimOverview）在检测到此情况时立即降级，不进入 SIM 卡通道
var ErrOperationInProgress = fmt.Errorf("eSIM 操作进行中，请稍后重试")

// ErrNoEUCCFound 表示未发现 eUICC（可能是非 eSIM 卡或未插卡）
var ErrNoEUCCFound = fmt.Errorf("未检测到 eUICC，当前卡可能不是 eSIM 卡")

type ManagerOptions struct {
	DeviceID             string
	Transport            string
	Modem                *modem.Manager
	Backend              backendpkg.DeviceBackend
	QMITransport         QMIAPDUTransport
	PCSCUSBPath          string      // PC/SC 读卡器 USB 路径（仅 transport=pcsc 时有效，运行时匹配 reader）
	PCSCSerial           string      // PC/SC 读卡器 SN（正规设备 SN 回退匹配，插拔换接口后仍可解析）
	PCSCAccessMu         *sync.Mutex // 可选：PC/SC 读卡器访问互斥锁（跨 eSIM/VoWiFi 共享）
	IMEIProvider         func(ctx context.Context) (string, error)
	OnBeforeSwitch       func(SwitchOperation, string) uint64
	OnAfterSwitch        func(SwitchOperation, uint64)
	OnSwitchFailed       func(SwitchOperation, uint64, error)
	OnSwitchDegraded     func(SwitchOperation, uint64, SwitchPhase, error)
	OnSwitchPhase        func(SwitchOperation, uint64, SwitchPhase)
	APDUArbiter          *apduarbiter.Arbiter
	PostSwitchMinDelay   time.Duration
	SwitchUseRefreshTrue bool
	OnOverviewUpdated    OverviewStateCallback // 可选：overview 缓存更新后回调（用于 SSE 推送）
	ESimSizeDBPath       string                // 可选：eSIM 容量库路径（留空则用 data/esim_sizes.db）
}

type SwitchOperation string

const (
	SwitchOperationEnableProfile  SwitchOperation = "enable_profile"
	SwitchOperationDisableProfile SwitchOperation = "disable_profile"
)

type SwitchPhase string

const (
	SwitchPhasePrepare             SwitchPhase = "prepare"
	SwitchPhaseAPDUSwitching       SwitchPhase = "apdu_switching"
	SwitchPhaseCardResetSettling   SwitchPhase = "card_reset_settling"
	SwitchPhaseIdentityRefresh     SwitchPhase = "identity_refresh"
	SwitchPhaseRuntimeRestore      SwitchPhase = "runtime_restore"
	SwitchPhaseVoWiFiRestore       SwitchPhase = "vowifi_restore"
	SwitchPhaseTransportRecovering SwitchPhase = "transport_recovering"
	SwitchPhaseReloadSkipped       SwitchPhase = "reload_skipped"
	SwitchPhaseReloadWarning       SwitchPhase = "reload_warning"
	SwitchPhaseDegraded            SwitchPhase = "degraded"
	SwitchPhaseDone                SwitchPhase = "done"
	SwitchPhaseFailed              SwitchPhase = "failed"
)

type SwitchProfileResult struct {
	SwitchToken       uint64
	Phase             SwitchPhase
	SwitchAccepted    bool
	RecoveryPending   bool
	DegradedReason    string
	PostSwitchAsync   bool
	SIMReloadWarning  string
	TargetICCID       string
	CachePatched      bool
	PowerCycleAttempt bool
}

const (
	transportAT     = "at"
	transportQMI    = "qmi"
	transportMBIM   = "mbim"
	transportPCSC   = "pcsc"
	transportCustom = "custom"
)

const (
	defaultSIMSlot                  uint8 = 1
	defaultPostSwitchMinDelay             = time.Second
	defaultReadQueueWaitTimeout           = 5 * time.Second
	writeOperationWarnThreshold           = 30 * time.Second
	postDownloadOverviewSettleDelay       = 2 * time.Second
)

var (
	switchFallbackPowerTimeout   = 20 * time.Second
	switchFallbackPowerCycleWait = 500 * time.Millisecond
)

// basicProfileTags 过滤掉耗时的 TagProfileIcon 数据（PNG等图片数据），
// 避免 ListProfile 发送几十条 APDU 以及长时间的数据传输。
var basicProfileTags = []bertlv.Tag{
	sgp22.TagICCID,
	sgp22.TagISDPAID,
	sgp22.TagProfileState,
	sgp22.TagNickname,
	sgp22.TagServiceProviderName,
	sgp22.TagProfileName,
	sgp22.TagProfileClass,
	sgp22.TagProfileOwner,
}

func listBasicProfiles(client *lpa.Client) ([]*sgp22.ProfileInfo, error) {
	if client == nil || client.APDU == nil {
		return nil, fmt.Errorf("未配置 eUICC APDU 通道")
	}
	response, err := sgp22.InvokeAPDU(client.APDU, &sgp22.ProfileInfoListRequest{
		Tags: basicProfileTags,
	})
	if err != nil {
		return nil, err
	}
	return response.ProfileList, nil
}

type simPowerController interface {
	UIMPowerOffSIM(ctx context.Context, slot uint8) error
	UIMPowerOnSIM(ctx context.Context, slot uint8) error
}

type apduIdleWaiter interface {
	WaitIdle(ctx context.Context) error
}

func normalizeTransport(in string) string {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "", transportAT:
		return transportAT
	case transportQMI:
		return transportQMI
	case transportMBIM:
		return transportMBIM
	case transportPCSC:
		return transportPCSC
	default:
		return strings.ToLower(strings.TrimSpace(in))
	}
}

type currentChannelProvider interface {
	CurrentChannel() byte
}

func newLPAClientWithChannel(ch driver.SmartCardChannel, aid []byte) (*lpa.Client, error) {
	opts := &lpa.Options{Channel: ch, AID: aid, MSS: 120}
	client, err := lpa.New(opts)
	if err != nil {
		if provider, ok := ch.(currentChannelProvider); ok {
			if current := provider.CurrentChannel(); current != 0 {
				_ = ch.CloseLogicalChannel(current)
			}
		}
		_ = ch.Disconnect()
		return nil, fmt.Errorf("创建 LPA client 失败 (AID=%X): %w", aid, err)
	}
	return client, nil
}

// NewManager 创建 eSIM 管理器（支持显式选择 AT/QMI 传输）
func NewManager(opts ManagerOptions) (*Manager, error) {
	transport := normalizeTransport(opts.Transport)
	mgr := &Manager{
		modem:                opts.Modem,
		backend:              opts.Backend,
		deviceID:             opts.DeviceID,
		transport:            transport,
		sf:                   &singleflight.Group{},
		imeiProvider:         opts.IMEIProvider,
		onBeforeSwitch:       opts.OnBeforeSwitch,
		onAfterSwitch:        opts.OnAfterSwitch,
		onSwitchFailed:       opts.OnSwitchFailed,
		onSwitchDegraded:     opts.OnSwitchDegraded,
		onSwitchPhase:        opts.OnSwitchPhase,
		onOverviewUpdated:    opts.OnOverviewUpdated,
		switchSignal:         make(chan string, 16),
		switchUseRefreshTrue: opts.SwitchUseRefreshTrue,
		opDone:               make(chan struct{}),
		postSwitchMinDelay:   defaultPostSwitchMinDelay,
		readQueueWaitTimeout: defaultReadQueueWaitTimeout,
	}
	// 仅在非 nil 时赋值，避免 typed-nil 指针进入 interface 字段后
	// 绕过 == nil 判断（PC/SC 设备无 APDUArbiter，会传入 nil 指针）
	if opts.APDUArbiter != nil {
		mgr.apduArbiter = opts.APDUArbiter
	}
	if opts.PostSwitchMinDelay > 0 {
		mgr.postSwitchMinDelay = opts.PostSwitchMinDelay
	}

	switch transport {
	case transportAT:
		if opts.Backend == nil {
			return nil, fmt.Errorf("AT 传输需要 device backend")
		}
		if opts.Modem == nil {
			return nil, fmt.Errorf("AT 传输需要 modem 管理器")
		}
		mgr.smartCardChannelFactory = func() (driver.SmartCardChannel, error) {
			return NewModemChannel(opts.Modem), nil
		}
		mgr.clearChannels = func() {
			if opts.Modem != nil {
				opts.Modem.ClearLogicalChannels()
			}
		}
	case transportQMI, transportMBIM:
		// MBIM 与 QMI 共用泛化 APDU 通道（QMIChannel）：QMI 走 QMI UIM transport，
		// MBIM 走 MBIMEx UICC transport（均实现 QMIAPDUTransport）。
		if opts.Backend == nil {
			return nil, fmt.Errorf("%s 传输需要 device backend", transport)
		}
		if opts.QMITransport == nil {
			return nil, ErrQMITransportNotAvailable
		}
		mgr.controlDevice = opts.QMITransport.ControlDevice()
		if strings.TrimSpace(mgr.controlDevice) == "" {
			return nil, ErrQMIControlDeviceMissing
		}
		mgr.smartCardChannelFactory = func() (driver.SmartCardChannel, error) {
			ch := NewQMIChannel(opts.QMITransport, 1)
			// 如果当前处于下载中，将下载的 ctx 注入给新建的通道，
			// 使底层 APDU 传输能够直接继承该 ctx 的超时/取消语义。
			if p := mgr.downloadCtx.Load(); p != nil {
				ch.SetContext(*p)
			}
			return ch, nil
		}
	case transportPCSC:
		usbPath := strings.TrimSpace(opts.PCSCUSBPath)
		sn := strings.TrimSpace(opts.PCSCSerial)
		if usbPath == "" && sn == "" {
			return nil, fmt.Errorf("PC/SC 传输需要指定 USB 路径或 SN")
		}
		// pcscChannelTracker 在 channelFactory 中保存底层 PCSCExclusiveChannel 引用，
		// 用于 closeClient 中在 CloseLogicalChannel 失败时强制 Disconnect 释放 accessMu。
		var pcscChannelTracker atomic.Pointer[PCSCExclusiveChannel]
		mgr.smartCardChannelFactory = func() (driver.SmartCardChannel, error) {
			// 用 USB 路径优先匹配，失败时按 SN 回退匹配当前模式下的 reader 名称
			readerName := ResolveReaderByUSBPath(usbPath, sn)
			if readerName == "" {
				return nil, fmt.Errorf("未找到 USB 路径 %s 或 SN %s 的读卡器", usbPath, sn)
			}
			var ch *PCSCExclusiveChannel
			var err error
			if opts.PCSCAccessMu != nil {
				ch, err = NewPCSCExclusiveChannelWithMutex(readerName, usbPath, opts.PCSCAccessMu)
			} else {
				ch, err = NewPCSCExclusiveChannel(readerName, usbPath)
			}
			if err != nil {
				return nil, fmt.Errorf("创建 PC/SC 通道失败: %w", err)
			}
			pcscChannelTracker.Store(ch)
			return ch, nil
		}
		// PC/SC 模式下 euicc-go 的 cardTransmitter.Close() 在 CloseLogicalChannel
		// 失败时（如切卡后卡片 reset 返回 6200）不会调用 Disconnect()，导致
		// PCSCExclusiveChannel 的 accessMu 锁永远不释放，后续所有 PC/SC 操作
		// （identity refresh、VoWiFi AKA 认证等）全部永久阻塞。
		// 自定义 closeClient 确保即使 CloseLogicalChannel 失败也强制 Disconnect。
		mgr.closeClient = func(client *lpa.Client) error {
			if client == nil {
				return nil
			}
			closeErr := client.Close()
			if closeErr != nil {
				// client.Close() 失败意味着 CloseLogicalChannel 报错，
				// euicc-go 不会继续调用 Disconnect()，accessMu 仍被持有。
				// 通过 tracker 获取底层 channel，强制 Disconnect 释放锁。
				ch := pcscChannelTracker.Load()
				if ch != nil {
					logger.Warn("PC/SC closeClient: Close 失败，强制 Disconnect 释放 accessMu",
						"device", opts.DeviceID,
						"close_err", closeErr)
					_ = ch.Disconnect()
					pcscChannelTracker.Store(nil)
				}
			}
			return closeErr
		}
	default:
		return nil, fmt.Errorf("不支持的 eSIM transport: %s", transport)
	}

	mgr.channelFactory = func(aid []byte) (*lpa.Client, error) {
		ch, err := mgr.newSmartCardChannel()
		if err != nil {
			return nil, err
		}
		return newLPAClientWithChannel(ch, aid)
	}
	mgr.overviewLoader = mgr.loadOverviewFresh
	mgr.profilesLoader = mgr.loadProfilesFresh

	// 初始化 eSIM 容量查询库（本地优先，不存在则从远程下载）
	if store, err := openPersistedESimSizeStore(opts.ESimSizeDBPath, euiccSizesURL, false); err != nil {
		logger.Warn("eSIM 容量库初始化失败，容量查询将跳过",
			"device", opts.DeviceID,
			"err", err)
	} else {
		mgr.esimSizeStore = store
	}

	return mgr, nil
}

type ChannelFactorySwitchCallbacks struct {
	OnBeforeSwitch   func(SwitchOperation, string) uint64
	OnAfterSwitch    func(SwitchOperation, uint64)
	OnSwitchFailed   func(SwitchOperation, uint64, error)
	OnSwitchDegraded func(SwitchOperation, uint64, SwitchPhase, error)
	OnSwitchPhase    func(SwitchOperation, uint64, SwitchPhase)
}

// NewManagerWithChannelFactoryCallbacks 创建 eSIM 管理器（通用模式，支持 PC/SC 等任意通道），
// 并暴露 token-aware 切卡回调，供调用方忽略过期的异步切卡后处理。
func NewManagerWithChannelFactoryCallbacks(
	deviceID string,
	channelFactory func(aid []byte) (*lpa.Client, error),
	clearFn func(),
	callbacks ChannelFactorySwitchCallbacks,
) *Manager {
	mgr := &Manager{
		deviceID:             deviceID,
		transport:            transportCustom,
		channelFactory:       channelFactory,
		clearChannels:        clearFn,
		sf:                   &singleflight.Group{},
		onBeforeSwitch:       callbacks.OnBeforeSwitch,
		onAfterSwitch:        callbacks.OnAfterSwitch,
		onSwitchFailed:       callbacks.OnSwitchFailed,
		onSwitchDegraded:     callbacks.OnSwitchDegraded,
		onSwitchPhase:        callbacks.OnSwitchPhase,
		switchSignal:         make(chan string, 16),
		opDone:               make(chan struct{}),
		postSwitchMinDelay:   defaultPostSwitchMinDelay,
		readQueueWaitTimeout: defaultReadQueueWaitTimeout,
	}
	mgr.overviewLoader = mgr.loadOverviewFresh
	mgr.profilesLoader = mgr.loadProfilesFresh
	return mgr
}

// NewManagerWithChannelFactory 创建 eSIM 管理器（通用模式，支持 PC/SC 等任意通道）
// channelFactory 负责创建和打开 LPA 客户端，clearFn 为可选的送前清理回调。
func NewManagerWithChannelFactory(
	deviceID string,
	channelFactory func(aid []byte) (*lpa.Client, error),
	clearFn func(),
	onBefore func(),
	onAfter func(),
) *Manager {
	var beforeWithToken func(SwitchOperation, string) uint64
	if onBefore != nil {
		beforeWithToken = func(SwitchOperation, string) uint64 {
			onBefore()
			return 0
		}
	}
	var afterWithToken func(SwitchOperation, uint64)
	if onAfter != nil {
		afterWithToken = func(SwitchOperation, uint64) {
			onAfter()
		}
	}
	return NewManagerWithChannelFactoryCallbacks(deviceID, channelFactory, clearFn, ChannelFactorySwitchCallbacks{
		OnBeforeSwitch: beforeWithToken,
		OnAfterSwitch:  afterWithToken,
	})
}

func (m *Manager) newSmartCardChannel() (driver.SmartCardChannel, error) {
	if m.smartCardChannelFactory == nil {
		return nil, fmt.Errorf("未配置 APDU 通道工厂")
	}
	return m.smartCardChannelFactory()
}

func (m *Manager) SetSmartCardChannelFactory(factory func() (driver.SmartCardChannel, error)) {
	m.smartCardChannelFactory = factory
}

func (m *Manager) SeedDiscoveredEUICCs(infos []EUICCInfo) {
	if m == nil {
		return
	}
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.discoveredEUICCs = mergeEUICCInfosLocked(m.discoveredEUICCs, infos)
}

func (m *Manager) cachedDiscoveredEUICCsWithEID() []EUICCInfo {
	if m == nil {
		return nil
	}
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	return cloneEUICCInfoListWithEID(m.discoveredEUICCs)
}

func (m *Manager) getEffectiveAIDPlan() aidScanPlan {
	return aidScanPlan{Policy: aidScanPolicyFullStatic, AIDs: cloneAIDList(AIDs)}
}

// getEffectiveAIDs 返回应被遍历的 AID 列表
func (m *Manager) getEffectiveAIDs() [][]byte {
	return m.getEffectiveAIDPlan().CloneAIDs()
}

func aidHexesForLog(aids [][]byte) []string {
	if len(aids) == 0 {
		return nil
	}
	out := make([]string, 0, len(aids))
	for _, aid := range aids {
		if len(aid) == 0 {
			continue
		}
		out = append(out, strings.ToUpper(hex.EncodeToString(aid)))
	}
	return out
}

func cloneAIDList(aids [][]byte) [][]byte {
	if len(aids) == 0 {
		return nil
	}
	out := make([][]byte, 0, len(aids))
	for _, aid := range aids {
		if len(aid) == 0 {
			continue
		}
		out = append(out, append([]byte(nil), aid...))
	}
	return out
}

func cloneEUICCInfoListWithEID(infos []EUICCInfo) []EUICCInfo {
	if len(infos) == 0 {
		return nil
	}
	out := make([]EUICCInfo, 0, len(infos))
	for _, info := range infos {
		normalized, ok := normalizeEUICCInfo(info)
		if !ok || strings.TrimSpace(normalized.EID) == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func mergeEUICCInfosLocked(existing []EUICCInfo, incoming []EUICCInfo) []EUICCInfo {
	out := cloneEUICCInfoList(existing)
	for _, info := range incoming {
		normalized, ok := normalizeEUICCInfo(info)
		if !ok {
			continue
		}
		replaced := false
		for i := range out {
			if sameEUICCIdentity(out[i], normalized) {
				out[i] = mergeEUICCInfo(out[i], normalized)
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, normalized)
		}
	}
	sortEUICCInfosStable(out)
	return out
}

func cloneEUICCInfoList(infos []EUICCInfo) []EUICCInfo {
	if len(infos) == 0 {
		return nil
	}
	out := make([]EUICCInfo, 0, len(infos))
	for _, info := range infos {
		normalized, ok := normalizeEUICCInfo(info)
		if ok {
			out = append(out, normalized)
		}
	}
	return out
}

func normalizeEUICCInfo(info EUICCInfo) (EUICCInfo, bool) {
	info.AIDHex = strings.ToUpper(strings.TrimSpace(info.AIDHex))
	info.EID = strings.TrimSpace(info.EID)
	info.SpecGuess = strings.TrimSpace(info.SpecGuess)
	info.SpecConfidence = strings.TrimSpace(info.SpecConfidence)
	if len(info.AID) == 0 && info.AIDHex != "" {
		if aid, err := hex.DecodeString(info.AIDHex); err == nil {
			info.AID = aid
		}
	}
	if len(info.AID) > 0 {
		info.AID = append([]byte(nil), info.AID...)
		if info.AIDHex == "" {
			info.AIDHex = fmt.Sprintf("%X", info.AID)
		}
	}
	return info, info.AIDHex != "" || info.EID != ""
}

func sameEUICCIdentity(a, b EUICCInfo) bool {
	aAID := strings.ToUpper(strings.TrimSpace(a.AIDHex))
	bAID := strings.ToUpper(strings.TrimSpace(b.AIDHex))
	if aAID != "" && bAID != "" && aAID == bAID {
		return true
	}
	aEID := strings.TrimSpace(a.EID)
	bEID := strings.TrimSpace(b.EID)
	return aEID != "" && bEID != "" && aEID == bEID
}

func mergeEUICCInfo(oldInfo, newInfo EUICCInfo) EUICCInfo {
	if len(newInfo.AID) > 0 {
		oldInfo.AID = append([]byte(nil), newInfo.AID...)
	}
	if strings.TrimSpace(newInfo.AIDHex) != "" {
		oldInfo.AIDHex = strings.ToUpper(strings.TrimSpace(newInfo.AIDHex))
	}
	if strings.TrimSpace(newInfo.EID) != "" {
		oldInfo.EID = strings.TrimSpace(newInfo.EID)
	}
	if newInfo.Spec != EUICCSpecUnknown {
		oldInfo.Spec = newInfo.Spec
	}
	if strings.TrimSpace(newInfo.SpecGuess) != "" {
		oldInfo.SpecGuess = strings.TrimSpace(newInfo.SpecGuess)
	}
	if strings.TrimSpace(newInfo.SpecConfidence) != "" {
		oldInfo.SpecConfidence = strings.TrimSpace(newInfo.SpecConfidence)
	}
	if newInfo.FreeNvramBytes != 0 {
		oldInfo.FreeNvramBytes = newInfo.FreeNvramBytes
	}
	if strings.TrimSpace(newInfo.FreeNvram) != "" {
		oldInfo.FreeNvram = strings.TrimSpace(newInfo.FreeNvram)
	}
	if strings.TrimSpace(newInfo.Firmware) != "" {
		oldInfo.Firmware = strings.TrimSpace(newInfo.Firmware)
	}
	if strings.TrimSpace(newInfo.Manufacturer) != "" {
		oldInfo.Manufacturer = strings.TrimSpace(newInfo.Manufacturer)
	}
	if len(newInfo.Certificates) > 0 {
		oldInfo.Certificates = append([]string(nil), newInfo.Certificates...)
	}
	if strings.TrimSpace(newInfo.InfoSource) != "" {
		oldInfo.InfoSource = strings.TrimSpace(newInfo.InfoSource)
	}
	if strings.TrimSpace(newInfo.InfoVersion) != "" {
		oldInfo.InfoVersion = strings.TrimSpace(newInfo.InfoVersion)
	}
	if strings.TrimSpace(newInfo.InfoError) != "" {
		oldInfo.InfoError = strings.TrimSpace(newInfo.InfoError)
	}
	if strings.TrimSpace(newInfo.SASAccreditationNumber) != "" {
		oldInfo.SASAccreditationNumber = strings.TrimSpace(newInfo.SASAccreditationNumber)
	}
	if strings.TrimSpace(newInfo.DefaultSMDPAddress) != "" {
		oldInfo.DefaultSMDPAddress = strings.TrimSpace(newInfo.DefaultSMDPAddress)
	}
	if strings.TrimSpace(newInfo.RootSMDSAddress) != "" {
		oldInfo.RootSMDSAddress = strings.TrimSpace(newInfo.RootSMDSAddress)
	}
	return oldInfo
}

// preCleanChannels 在遍历 AID 前统一清理一次递辑通道
func (m *Manager) preCleanChannels() {
	if m.clearChannels == nil {
		return
	}
	m.clearChannels()
}

// createLPAWithAID 用指定 AID 创建 LPA client（通过 channelFactory 统一分发）
func (m *Manager) createLPAWithAID(aid []byte) (*lpa.Client, error) {
	client, err := m.channelFactory(aid)
	if err != nil {
		return nil, fmt.Errorf("创建 LPA client 失败 (AID=%X): %w", aid, err)
	}
	logger.Info("LPA client 创建成功",
		"device", m.deviceID,
		"AID", fmt.Sprintf("%X", aid),
		"transport", m.transport,
		"control_device", m.controlDevice)
	return client, nil
}

func closeLPAClient(client *lpa.Client) (err error) {
	if client == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("关闭 LPA client panic 已恢复", "err", r)
			err = nil
		}
	}()
	return client.Close()
}

func isExpectedPostResetLPAClientCloseError(operation string, err error) bool {
	if err == nil {
		return false
	}
	switch strings.TrimSpace(operation) {
	case "switch_profile_pre_refresh",
		"switch_profile_deferred",
		"disable_profile_pre_refresh",
		"disable_profile_deferred":
	default:
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "close logical channel") && !strings.Contains(msg, "关闭逻辑通道") {
		return false
	}
	// QMI 模式：qmi_uim_card_reset 或 error=0x0030
	if strings.Contains(msg, "qmi_uim_card_reset") || strings.Contains(msg, "error=0x0030") {
		return true
	}
	// PC/SC 模式：切卡后卡片 reset，关闭逻辑通道返回 6xxx 警告 SW（如 6200）
	return strings.Contains(msg, "6200") || strings.Contains(msg, "6xxx")
}

func (m *Manager) closeLPAClientForOperation(operation string, client *lpa.Client) error {
	closeFn := closeLPAClient
	if m != nil && m.closeClient != nil {
		closeFn = m.closeClient
	}
	err := closeFn(client)
	if err != nil {
		if isExpectedPostResetLPAClientCloseError(operation, err) {
			logger.Debug("关闭 LPA client 失败，卡片 reset 后按预期忽略",
				"device", m.deviceID,
				"operation", operation,
				"err", err)
			return err
		}
		logger.Warn("关闭 LPA client 失败",
			"device", m.deviceID,
			"operation", operation,
			"err", err)
	}
	return err
}

func (m *Manager) logWriteOperationHold(operation string, started time.Time) {
	if started.IsZero() {
		return
	}
	hold := time.Since(started)
	if hold < writeOperationWarnThreshold {
		return
	}
	logger.Warn("eSIM 写操作长时间持有写锁",
		"device", m.deviceID,
		"operation", operation,
		"hold_ms", hold.Milliseconds())
}

// forEachEUICC 遍历所有可用的 eUICC，对每个唯一 EID 调用回调函数。
// 每次从静态候选 AID 重新扫描；命中可用 AID 后停止，eSTK Max 的 SE0/SE1 例外。
// 回调参数: client=已打开的 LPA 客户端, aid=当前 AID, eidStr=当前 EID 字符串
func (m *Manager) forEachEUICC(fn func(client *lpa.Client, aid []byte, eidStr string) error) error {
	// 读请求在写操作进行中采用排队等待策略，超时才返回 busy。
	writeWaitStarted := time.Now()
	if err := m.waitForNoWriteOperation(); err != nil {
		logger.Warn("eSIM 读操作等待写锁超时",
			"device", m.deviceID,
			"wait_ms", time.Since(writeWaitStarted).Milliseconds())
		return err
	}
	if waited := time.Since(writeWaitStarted); waited > 100*time.Millisecond {
		logger.Debug("eSIM 读操作已等待写锁释放",
			"device", m.deviceID,
			"wait_ms", waited.Milliseconds())
	}
	// 设备级 APDU 仲裁：等待其它 APDU 会话（如 VoWiFi 鉴权）释放，避免冲突。
	apduWaitStarted := time.Now()
	if err := m.waitForAPDUIdleForRead(); err != nil {
		logger.Warn("eSIM 读操作等待 APDU 仲裁空闲超时",
			"device", m.deviceID,
			"wait_ms", time.Since(apduWaitStarted).Milliseconds())
		return ErrOperationInProgress
	}
	if waited := time.Since(apduWaitStarted); waited > 100*time.Millisecond {
		logger.Debug("eSIM 读操作已等待 APDU 仲裁空闲",
			"device", m.deviceID,
			"wait_ms", waited.Milliseconds())
	}

	m.preCleanChannels()

	plan := m.getEffectiveAIDPlan()
	aids := plan.CloneAIDs()
	logger.Debug("准备执行 eUICC AID 扫描",
		"device", m.deviceID,
		"policy", plan.Policy,
		"candidate_count", len(aids),
		"candidate_aids", aidHexesForLog(aids))
	foundAny, err := m.doForEachEUICC(aids, fn)
	if foundAny {
		return err
	}

	logger.Warn("AID 扫描未发现 eUICC",
		"device", m.deviceID,
		"policy", plan.Policy,
		"triedCount", len(aids),
		"err", err)
	if err != nil {
		return ErrNoEUCCFound
	}
	return ErrNoEUCCFound
}

func (m *Manager) waitForNoWriteOperation() error {
	// 快路径：锁空闲则直接返回
	if m.opMu.TryLock() {
		m.opMu.Unlock()
		return nil
	}
	timeout := m.readQueueWaitTimeout
	if timeout <= 0 {
		timeout = defaultReadQueueWaitTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-m.opDone:
			// 写操作已发出完成通知，尝试确认锁已释放
			if m.opMu.TryLock() {
				m.opMu.Unlock()
				return nil
			}
		case <-timer.C:
			return ErrOperationInProgress
		}
	}
}

func (m *Manager) acquireOperationLock() error {
	if m.opMu.TryLock() {
		return nil
	}
	timeout := m.readQueueWaitTimeout
	if timeout <= 0 {
		timeout = defaultReadQueueWaitTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		done := m.opDone
		if done == nil {
			done = make(chan struct{})
		}
		select {
		case <-done:
			if m.opMu.TryLock() {
				return nil
			}
		case <-timer.C:
			return ErrOperationInProgress
		}
	}
}

func (m *Manager) lockOperation(operation string) (func(), error) {
	if err := m.acquireOperationLock(); err != nil {
		return nil, err
	}
	started := time.Now()
	return func() {
		m.logWriteOperationHold(operation, started)
		m.opMu.Unlock()
		m.notifyWriteDone()
	}, nil
}

// notifyWriteDone 通知所有等待写操作完成的读方。
// 必须在 opMu.Unlock() 之后立即调用。
func (m *Manager) notifyWriteDone() {
	// 关闭旧 channel（广播通知），并新建一个供下次写操作使用
	old := m.opDone
	m.opDone = make(chan struct{})
	if old != nil {
		close(old)
	}
}

func (m *Manager) emitSwitchPhase(operation SwitchOperation, token uint64, phase SwitchPhase) {
	if m == nil || m.onSwitchPhase == nil || phase == "" {
		return
	}
	m.onSwitchPhase(operation, token, phase)
}

func (m *Manager) waitForAPDUIdleForRead() error {
	if m == nil || m.apduArbiter == nil {
		return nil
	}
	timeout := m.readQueueWaitTimeout
	if timeout <= 0 {
		timeout = defaultReadQueueWaitTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := m.apduArbiter.WaitIdle(ctx); err != nil {
		return ErrOperationInProgress
	}
	return nil
}

// doForEachEUICC 执行一轮 AID 遍历，返回是否发现了至少一个 eUICC。
// runEUICCCallback 调用 forEachEUICC 的每-AID 回调，并把回调内部的 panic 转换为普通
// error。回调最终会触达第三方 BER-TLV 解码库（如 euicc-go），面对个别卡片返回的
// 非标准/缺字段 profile 数据时该库可能直接 panic（已知问题：缺少 profile state
// TLV 时 nil 解引用）。一旦 panic 逃出这里，会一路冒泡到 gin 的 Recovery 中间件，
// 导致整个 eSIM 总览请求返回毫无信息量的 500，前端进而把已经探测到的 eUICC 误判为
// "未检测到"。这里 recover 后按普通错误处理，使其复用既有的 per-AID 容错路径。
func (m *Manager) runEUICCCallback(aidHex, eidStr string, client *lpa.Client, aid []byte, fn func(client *lpa.Client, aid []byte, eidStr string) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("eUICC 回调 panic 已恢复",
				"device", m.deviceID,
				"AID", aidHex,
				"EID", eidStr,
				"panic", r,
				"stack", string(debug.Stack()))
			err = fmt.Errorf("eUICC 回调处理失败 (AID=%s): %v", aidHex, r)
		}
	}()
	return fn(client, aid, eidStr)
}

func (m *Manager) doForEachEUICC(aids [][]byte, fn func(client *lpa.Client, aid []byte, eidStr string) error) (bool, error) {
	seenEIDs := make(map[string]bool)
	var successAIDs [][]byte
	foundAny := false
	var lastErr error

	for _, aid := range aids {
		if !shouldContinueAIDScanAfterSuccess(successAIDs, aid) {
			logger.Debug("命中可用 AID 后停止剩余 AID 扫描",
				"device", m.deviceID,
				"next_aid", fmt.Sprintf("%X", aid),
				"success_count", len(successAIDs))
			break
		}

		err := func() error {
			aidHex := fmt.Sprintf("%X", aid)
			client, err := m.createLPAWithAID(aid)
			if err != nil {
				logger.Debug("eUICC AID 扫描阶段",
					"device", m.deviceID,
					"stage", "select_open_failed",
					"AID", aidHex,
					"err", err)
				return err
			}
			defer m.closeLPAClientForOperation("for_each_euicc", client)
			logger.Debug("eUICC AID 扫描阶段",
				"device", m.deviceID,
				"stage", "select_open_ok",
				"AID", aidHex)

			eid, err := client.EID()
			if err != nil {
				logger.Debug("eUICC AID 扫描阶段",
					"device", m.deviceID,
					"stage", "eid_failed",
					"AID", aidHex,
					"err", err)
				return err
			}

			eidStr := hex.EncodeToString(eid)
			logger.Debug("eUICC AID 扫描阶段",
				"device", m.deviceID,
				"stage", "eid_ok",
				"AID", aidHex,
				"EID", eidStr)
			if seenEIDs[eidStr] {
				logger.Debug("跳过重复 EID", "AID", aidHex, "EID", eidStr)
				return nil
			}

			seenEIDs[eidStr] = true
			successAIDs = append(successAIDs, aid)
			m.SeedDiscoveredEUICCs([]EUICCInfo{buildDiscoveredEUICCInfo(aid, eidStr)})
			foundAny = true

			return m.runEUICCCallback(aidHex, eidStr, client, aid, fn)
		}()

		if err != nil {
			lastErr = err
		}
	}

	if !foundAny {
		return false, lastErr
	}

	if lastErr != nil {
		logger.Debug("AID 扫描过程中有错误，本轮已发现 eUICC，后续读取仍会重新全量扫描",
			"device", m.deviceID,
			"success_aids_count", len(successAIDs),
			"err", lastErr)
		return true, nil
	}

	return true, nil
}

// GetEIDs 获取所有 eUICC 的 EID 列表
func (m *Manager) GetEIDs() ([]EUICCInfo, error) {
	v, err, _ := m.sf.Do("GetEIDs", func() (interface{}, error) {
		logger.Info("读取 eUICC EID",
			"device", m.deviceID,
			"source", "scan")
		var result []EUICCInfo
		if err := m.forEachEUICC(func(client *lpa.Client, aid []byte, eidStr string) error {
			euiccInfo := buildDiscoveredEUICCInfo(aid, eidStr)
			result = append(result, euiccInfo)
			logger.Info("发现 eUICC", "device", m.deviceID, "AID", euiccInfo.AIDHex, "EID", eidStr)
			return nil
		}); err != nil {
			return nil, err
		}
		m.SeedDiscoveredEUICCs(result)
		logger.Info("读取 eUICC EID 完成",
			"device", m.deviceID,
			"source", "scan",
			"count", len(result))
		return result, nil
	})

	if err != nil {
		return nil, err
	}
	return v.([]EUICCInfo), nil
}

// GetEID 获取第一个 eUICC 的 EID（向后兼容）
func (m *Manager) GetEID() (string, error) {
	eids, err := m.GetEIDs()
	if err != nil {
		return "", err
	}
	return eids[0].EID, nil
}

// GetEUICCChipInfo 获取 eUICC 芯片的硬件信息（优先返回缓存）
// 包含：所有 EID（含各自可用空间）、产品名称、序列号、固件版本
func (m *Manager) GetEUICCChipInfo(forceRefresh bool) (*EUICCChipInfo, error) {
	// 优先返回缓存（硬件信息不会变，只有可用空间会变）
	if !forceRefresh {
		m.cacheMu.RLock()
		cached := m.chipInfoCache
		m.cacheMu.RUnlock()
		if cached != nil {
			return cached, nil
		}
	}

	v, err, _ := m.sf.Do("GetEUICCChipInfo", func() (interface{}, error) {
		info := &EUICCChipInfo{}
		// fnMu 保护并发 fn 对 info.EIDs 的写操作
		var fnMu sync.Mutex

		if err := m.forEachEUICC(func(client *lpa.Client, aid []byte, eidStr string) error {
			euiccInfo := buildDiscoveredEUICCInfo(aid, eidStr)
			// 在同一 channel 中查询 EUICCInfo2——APDU 读，per-channel 并发安全
			m.parseEUICCInfo2ForEID(client, &euiccInfo)

			fnMu.Lock()
			info.EIDs = append(info.EIDs, euiccInfo)
			fnMu.Unlock()

			logger.Info("获取 eUICC 信息",
				"device", m.deviceID,
				"AID", euiccInfo.AIDHex,
				"EID", eidStr,
				"freeNvram", euiccInfo.FreeNvram)
			return nil
		}); err != nil {
			return nil, err
		}
		sortEUICCInfosStable(info.EIDs)

		// 通过 eSTK.me Product AID 获取硬件标识（SkuName/SerialNumber/Firmware）
		// 优化：硬件标识不随使用变化，若已有缓存则直接复用，跳过 6 次 APDU 开销
		m.cacheMu.RLock()
		cachedChip := m.chipInfoCache
		m.cacheMu.RUnlock()
		if hasReusableChipProductInfo(cachedChip) {
			info.SkuName = cachedChip.SkuName
			info.SerialNumber = cachedChip.SerialNumber
			info.Firmware = cachedChip.Firmware
			logger.Debug("复用 chipInfoCache 跳过 eSTK.me Product AID 查询",
				"device", m.deviceID,
				"cached_sku", cachedChip.SkuName)
		} else {
			// 首次查询或换卡后重新查询
			m.parseESTKmeInfo(info)
		}

		// 如果 eSTK.me 专有指令未返回固件版本，则回退使用从标准 EUICCInfo2 中提取到的固件版本
		if info.Firmware == "" && len(info.EIDs) > 0 {
			info.Firmware = info.EIDs[0].Firmware
		}

		// 对没有私有接口获取 SkuName 的传统白卡（例如 9eSIM）尝试硬编码预测判定
		if info.SkuName == "" && len(info.EIDs) > 0 {
			if guessedName := predictSkuName(info.EIDs[0].EID, info.Firmware); guessedName != "" {
				info.SkuName = guessedName
				logger.Info("基于特征预测了 eSIM 品牌信息", "device", m.deviceID, "guessed_sku", info.SkuName)
			}
		}

		// 写入缓存
		m.cacheMu.Lock()
		m.chipInfoCache = info
		m.cacheMu.Unlock()
		return info, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(*EUICCChipInfo), nil
}

// EsimOverview 合并的 eSIM 总览信息（芯片信息 + 按 eUICC 分组的 profiles）
type EsimOverview struct {
	ChipInfo          *EUICCChipInfo  `json:"chip_info"`          // 芯片硬件信息
	Profiles          []EUICCProfiles `json:"profiles"`           // 按 eUICC 分组的 profile 列表
	NotificationCount int             `json:"notification_count"` // 待处理通知数量（不含 autoClean 的 enable/disable）
	TotalCapacityBytes  int     `json:"total_capacity_bytes,omitempty"`  // eUICC 总容量（字节）
	UsedCapacityBytes   int     `json:"used_capacity_bytes,omitempty"`   // 已用容量（所有 profile size 之和）
	FreeCapacityBytes   int     `json:"free_capacity_bytes,omitempty"`   // 剩余容量（来自 freeNvram）
	UsagePercent        float64 `json:"usage_percent,omitempty"`         // 使用率（0-100）
	CapacityFormatted   string  `json:"capacity_formatted,omitempty"`   // 人类可读总容量（如 "7.00 MB"）
}

// parseEUICCInfo2ForEID 从标准 eUICC 信息接口解析单个 eUICC 的可用空间、固件版本、制造商和证书信息。
// 保留旧函数名以减少调用点 churn，内部会在 EUICCInfo2 失败时降级到 EUICCInfo1。
func (m *Manager) parseEUICCInfo2ForEID(client *lpa.Client, euicc *EUICCInfo) {
	m.enrichEUICCInfo(client, euicc)
}

// parseESTKmeInfo 通过 eSTK.me Product AID 获取设备名称、序列号和固件版本
func (m *Manager) parseESTKmeInfo(info *EUICCChipInfo) {
	ch, err := m.newSmartCardChannel()
	if err != nil {
		logger.Debug("当前 transport 无法创建原始 APDU 通道，跳过 Product AID 查询",
			"device", m.deviceID,
			"transport", m.transport,
			"err", err)
		return
	}
	if err := ch.Connect(); err != nil {
		logger.Debug("连接原始 APDU 通道失败，跳过 Product AID 查询",
			"device", m.deviceID,
			"transport", m.transport,
			"err", err)
		return
	}
	defer ch.Disconnect() //nolint:errcheck

	channelNum, err := ch.OpenLogicalChannel(estkmeProductAID)
	if err != nil {
		// 不是 eSTK.me 设备，跳过
		logger.Debug("非 eSTK.me 设备，跳过 Product AID 查询",
			"device", m.deviceID,
			"transport", m.transport,
			"err", err)
		return
	}
	defer ch.CloseLogicalChannel(channelNum) //nolint:errcheck

	// APDU 命令格式：CLA=00 INS=00 P1=xx P2=00 Le=00
	// P1=0x03 -> skuName, P1=0x00 -> serialNumber, P1=0x01 -> bootloader, P1=0x02 -> firmware
	readField := func(p1 byte) string {
		cmd := []byte{0x00, 0x00, p1, 0x00, 0x00}
		resp, err := ch.Transmit(cmd)
		if err != nil || len(resp) < 2 {
			return ""
		}
		// 检查 SW=9000
		if resp[len(resp)-2] != 0x90 || resp[len(resp)-1] != 0x00 {
			return ""
		}
		return string(resp[:len(resp)-2])
	}

	info.SkuName = readField(0x03)
	info.SerialNumber = readField(0x00)

	bl := readField(0x01) // bootloader version
	fw := readField(0x02) // firmware version
	if bl != "" && fw != "" {
		info.Firmware = bl + "-" + fw
	} else if fw != "" {
		info.Firmware = fw
	}

	logger.Info("获取到 eSTK.me 设备信息",
		"device", m.deviceID,
		"sku", info.SkuName,
		"serial", info.SerialNumber,
		"firmware", info.Firmware)
}

// formatBytes 将字节数格式化为人类可读的形式
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
	)
	switch {
	case b >= 1000*kb:
		return fmt.Sprintf("%.2f mb", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.2f kb", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d b", b)
	}
}

func cloneProfiles(groups []EUICCProfiles) []EUICCProfiles {
	if len(groups) == 0 {
		return nil
	}
	cloned := make([]EUICCProfiles, len(groups))
	for i, group := range groups {
		cloned[i] = EUICCProfiles{
			EID:    group.EID,
			AIDHex: group.AIDHex,
		}
		if len(group.Profiles) > 0 {
			cloned[i].Profiles = append([]ProfileItem(nil), group.Profiles...)
		}
	}
	return cloned
}

func cloneChipInfo(info *EUICCChipInfo) *EUICCChipInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	if len(info.EIDs) > 0 {
		cloned.EIDs = append([]EUICCInfo(nil), info.EIDs...)
	}
	return &cloned
}

func cloneOverview(overview *EsimOverview) *EsimOverview {
	if overview == nil {
		return nil
	}
	return &EsimOverview{
		ChipInfo:          cloneChipInfo(overview.ChipInfo),
		Profiles:          cloneProfiles(overview.Profiles),
		NotificationCount: overview.NotificationCount,
		TotalCapacityBytes: overview.TotalCapacityBytes,
		UsedCapacityBytes:  overview.UsedCapacityBytes,
		FreeCapacityBytes:  overview.FreeCapacityBytes,
		UsagePercent:       overview.UsagePercent,
		CapacityFormatted:  overview.CapacityFormatted,
	}
}

func (m *Manager) cachedOverview() *EsimOverview {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	return m.overviewCache
}

func (m *Manager) setOverviewCache(overview *EsimOverview, err error, generation uint64) {
	m.cacheMu.Lock()
	if generation != m.overviewGeneration {
		m.cacheMu.Unlock()
		return
	}
	m.overviewCache = cloneOverview(overview)
	m.overviewLastErr = err
	if overview != nil {
		m.chipInfoCache = cloneChipInfo(overview.ChipInfo)
	}
	m.cacheMu.Unlock()
	// 通知外部 overview 状态已变化（触发 SSE 推送）
	m.notifyOverviewStateChange()
}

// InvalidateOverviewCache 导出版本，供外部包（如 device）调用强制清空 overview 缓存。
// lookupProfileByICCID 从 overview 缓存中通过 ICCID 查找 profile 的 Name 和 MCC。
func (m *Manager) lookupProfileByICCID(iccid string) (name, mcc string) {
	if iccid == "" {
		return "", ""
	}
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	if m.overviewCache == nil {
		return "", ""
	}
	for _, group := range m.overviewCache.Profiles {
		for _, p := range group.Profiles {
			if p.ICCID == iccid {
				return p.Name, p.MCC
			}
		}
	}
	return "", ""
}

func (m *Manager) InvalidateOverviewCache(reason string) {
	m.invalidateOverviewCache(reason)
}

func (m *Manager) invalidateOverviewCache(reason string) {
	if m == nil {
		return
	}
	m.cacheMu.Lock()
	m.overviewGeneration++
	m.overviewCache = nil
	m.overviewLastErr = nil
	m.chipInfoCache = nil
	m.cacheMu.Unlock()
	if strings.TrimSpace(reason) != "" {
		logger.Info("eSIM 总览缓存已失效", "device", m.deviceID, "reason", reason)
	}
}

func (m *Manager) clearHardwareDiscoveryCachesLocked() {
	m.overviewGeneration++
	m.overviewCache = nil
	m.overviewLastErr = nil
	m.chipInfoCache = nil
	m.discoveredEUICCs = nil
}

func (m *Manager) shouldSuppressOverviewReload() bool {
	if m == nil {
		return false
	}
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	return !m.suppressOverviewReloadUntil.IsZero() && time.Now().Before(m.suppressOverviewReloadUntil)
}

func (m *Manager) overviewReloadDelay() time.Duration {
	if m == nil {
		return 0
	}
	m.cacheMu.RLock()
	until := m.suppressOverviewReloadUntil
	m.cacheMu.RUnlock()
	if until.IsZero() {
		return 0
	}
	delay := time.Until(until)
	if delay < 0 {
		return 0
	}
	return delay
}

func (m *Manager) waitForOverviewReloadAllowed(ctx context.Context) error {
	delay := m.overviewReloadDelay()
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) overviewReloadInProgress() bool {
	if m == nil {
		return false
	}
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	return m.overviewReloading
}

func (m *Manager) beginOverviewReloadSuppression(delay time.Duration) {
	if m == nil || delay <= 0 {
		return
	}
	m.cacheMu.Lock()
	until := time.Now().Add(delay)
	if until.After(m.suppressOverviewReloadUntil) {
		m.suppressOverviewReloadUntil = until
	}
	m.cacheMu.Unlock()
}

func (m *Manager) beginSwitchOverviewSuppression() {
	m.beginOverviewReloadSuppression(5 * time.Second)
}

func (m *Manager) patchCachedActiveProfile(targetICCID string, aidHex string) bool {
	if m == nil {
		return false
	}
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if m.overviewCache == nil {
		return false
	}
	target := normalizeICCIDValue(targetICCID)
	if target == "" {
		return false
	}
	groupKey := strings.ToUpper(strings.TrimSpace(aidHex))
	targetGroupIndex := -1
	for gi := range m.overviewCache.Profiles {
		group := &m.overviewCache.Profiles[gi]
		if groupKey != "" && strings.ToUpper(strings.TrimSpace(group.AIDHex)) != groupKey {
			continue
		}
		for pi := range group.Profiles {
			if isTargetICCIDActive(target, group.Profiles[pi].ICCID) {
				targetGroupIndex = gi
				break
			}
		}
		if targetGroupIndex >= 0 {
			break
		}
	}
	if targetGroupIndex < 0 {
		for gi := range m.overviewCache.Profiles {
			group := &m.overviewCache.Profiles[gi]
			for pi := range group.Profiles {
				if isTargetICCIDActive(target, group.Profiles[pi].ICCID) {
					targetGroupIndex = gi
					break
				}
			}
			if targetGroupIndex >= 0 {
				break
			}
		}
	}
	if targetGroupIndex < 0 {
		return false
	}

	patched := false
	for gi := range m.overviewCache.Profiles {
		group := &m.overviewCache.Profiles[gi]
		for pi := range group.Profiles {
			isTarget := isTargetICCIDActive(target, group.Profiles[pi].ICCID)
			if isTarget {
				group.Profiles[pi].State = int(sgp22.ProfileEnabled)
				group.Profiles[pi].StateText = "已启用"
				patched = true
			} else {
				group.Profiles[pi].State = int(sgp22.ProfileDisabled)
				group.Profiles[pi].StateText = "已禁用"
			}
		}
	}
	if patched {
		m.overviewLastErr = nil
	}
	return patched
}

func (m *Manager) loadOverview() (*EsimOverview, error) {
	if cached := m.cachedOverview(); cached != nil {
		return cloneOverview(cached), nil
	}
	m.cacheMu.RLock()
	generation := m.overviewGeneration
	m.cacheMu.RUnlock()
	key := fmt.Sprintf("GetEsimOverview:%d", generation)
	v, err, _ := m.sf.Do(key, func() (interface{}, error) {
		if cached := m.cachedOverview(); cached != nil {
			return cloneOverview(cached), nil
		}
		loader := m.overviewLoader
		if loader == nil {
			loader = m.loadOverviewFresh
		}
		overview, loadErr := loader()
		if loadErr != nil {
			m.setOverviewCache(nil, loadErr, generation)
			if errors.Is(loadErr, ErrNoEUCCFound) {
				m.recordScanFailure()
			}
			return nil, loadErr
		}
		m.setOverviewCache(overview, nil, generation)
		m.resetScanBackoff()
		// 异步统计通知数量，避免首次加载时 safeListNotification 的同步延迟
		m.refreshNotificationCountAsync()
		return cloneOverview(overview), nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*EsimOverview), nil
}

func (m *Manager) triggerOverviewReload(reason string) {
	if m == nil {
		return
	}
	m.cacheMu.Lock()
	if m.overviewReloading {
		m.cacheMu.Unlock()
		return
	}
	m.overviewReloading = true
	m.cacheMu.Unlock()
	go func() {
		defer func() {
			m.cacheMu.Lock()
			m.overviewReloading = false
			m.cacheMu.Unlock()
		}()
		if err := m.waitForOverviewReloadAllowed(context.Background()); err != nil {
			logger.Warn("eSIM 总览异步重载等待窗口失败", "device", m.deviceID, "reason", reason, "err", err)
			return
		}
		// 退避检查：如果连续扫描失败且未到下次重试时间，跳过自动重载
		if !m.scanBackoffAllowsAutoReload() {
			logger.Debug("eSIM 自动重载因退避策略跳过", "device", m.deviceID, "reason", reason, "backoff_count", m.scanBackoffCount)
			return
		}
		if _, err := m.loadOverview(); err != nil {
			logger.Warn("eSIM 总览异步重载失败", "device", m.deviceID, "reason", reason, "err", err)
		}
	}()
}

func (m *Manager) WarmOverviewAsync(reason string) {
	m.triggerOverviewReload(reason)
}

// scanBackoffDelays 定义连续扫描失败时的自动重试间隔。
// 第 1 次失败后等 5s，第 2 次 30s，第 3 次 60s，第 4 次 120s。
// 耗尽后退避计数器不再增长，scanBackoffAllowsAutoReload 返回 false。
var scanBackoffDelays = []time.Duration{5 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second}

// scanBackoffAllowsAutoReload 检查是否允许自动重载。
// 如果退避已耗尽（4 次连续失败），返回 false。
// 如果仍在退避窗口内，返回 false。
func (m *Manager) scanBackoffAllowsAutoReload() bool {
	if m == nil {
		return true
	}
	m.scanBackoffMu.Lock()
	defer m.scanBackoffMu.Unlock()
	if m.scanBackoffCount >= len(scanBackoffDelays) {
		return false
	}
	if m.scanBackoffUntil.IsZero() {
		return true
	}
	return time.Now().After(m.scanBackoffUntil)
}

// recordScanFailure 记录一次扫描失败，递增退避计数器并设置下次允许重试时间。
func (m *Manager) recordScanFailure() {
	if m == nil {
		return
	}
	m.scanBackoffMu.Lock()
	defer m.scanBackoffMu.Unlock()
	if m.scanBackoffCount < len(scanBackoffDelays) {
		delay := scanBackoffDelays[m.scanBackoffCount]
		m.scanBackoffUntil = time.Now().Add(delay)
		m.scanBackoffCount++
		logger.Info("eSIM 扫描退避递增",
			"device", m.deviceID,
			"failure_count", m.scanBackoffCount,
			"next_retry_in", delay.String())
	} else if m.scanBackoffCount == len(scanBackoffDelays) {
		logger.Info("eSIM 扫描退避已耗尽，不再自动重试",
			"device", m.deviceID,
			"failure_count", m.scanBackoffCount)
		m.scanBackoffCount++ // 确保只日志一次
	}
}

// resetScanBackoff 重置退避计数器（扫描成功或手动刷新时调用）。
func (m *Manager) resetScanBackoff() {
	if m == nil {
		return
	}
	m.scanBackoffMu.Lock()
	defer m.scanBackoffMu.Unlock()
	if m.scanBackoffCount > 0 {
		logger.Info("eSIM 扫描退避已重置",
			"device", m.deviceID,
			"previous_count", m.scanBackoffCount)
	}
	m.scanBackoffCount = 0
	m.scanBackoffUntil = time.Time{}
}

func buildProfileGroup(eidStr string, aid []byte, profiles []*sgp22.ProfileInfo) EUICCProfiles {
	aidHex := fmt.Sprintf("%X", aid)
	group := EUICCProfiles{
		EID:      eidStr,
		AIDHex:   aidHex,
		Profiles: make([]ProfileItem, 0, len(profiles)),
	}
	for _, p := range profiles {
		name := p.ProfileNickname
		if name == "" {
			name = p.ProfileName
		}
		stateText := "已禁用"
		if p.ProfileState == sgp22.ProfileEnabled {
			stateText = "已启用"
		}
		group.Profiles = append(group.Profiles, ProfileItem{
			ICCID:               p.ICCID.String(),
			ISDPAIDHex:          hex.EncodeToString(p.ISDPAID),
			Name:                name,
			ServiceProviderName: p.ServiceProviderName,
			State:               int(p.ProfileState),
			StateText:           stateText,
			ClassText:           p.ProfileClass.String(),
			MCC:                 p.ProfileOwner.MCC(),
			MNC:                 p.ProfileOwner.MNC(),
			GID1:                hex.EncodeToString(p.ProfileOwner.GID1),
			GID2:                hex.EncodeToString(p.ProfileOwner.GID2),
			ProfileName:         p.ProfileName,
		})
	}
	return group
}

func (m *Manager) loadProfilesFresh() ([]EUICCProfiles, error) {
	var profileGroups []EUICCProfiles
	var fnMu sync.Mutex
	if err := m.forEachEUICC(func(client *lpa.Client, aid []byte, eidStr string) error {
		aidHex := fmt.Sprintf("%X", aid)
		logger.Debug("eUICC AID 扫描阶段",
			"device", m.deviceID,
			"stage", "profiles_start",
			"AID", aidHex,
			"EID", eidStr)
		profiles, profileErr := listBasicProfiles(client)

		fnMu.Lock()
		defer fnMu.Unlock()
		if profileErr != nil {
			logger.Debug("eUICC AID 扫描阶段",
				"device", m.deviceID,
				"stage", "profiles_failed",
				"AID", aidHex,
				"EID", eidStr,
				"err", profileErr)
			profileGroups = append(profileGroups, EUICCProfiles{
				EID:      eidStr,
				AIDHex:   aidHex,
				Profiles: []ProfileItem{},
			})
			return nil
		}
		group := buildProfileGroup(eidStr, aid, profiles)
		profileGroups = append(profileGroups, group)
		logger.Debug("eUICC AID 扫描阶段",
			"device", m.deviceID,
			"stage", "profiles_ok",
			"AID", aidHex,
			"EID", eidStr,
			"profileCount", len(profiles))
		logger.Info("获取 eUICC profiles",
			"device", m.deviceID,
			"AID", aidHex,
			"EID", eidStr,
			"profileCount", len(profiles))
		return nil
	}); err != nil {
		return nil, err
	}
	sortEUICCProfilesStable(profileGroups)
	return profileGroups, nil
}

func (m *Manager) cachedEIDForAID(aidHex string) string {
	aidHex = strings.ToUpper(strings.TrimSpace(aidHex))
	if aidHex == "" {
		return ""
	}
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	if m.overviewCache != nil {
		for _, group := range m.overviewCache.Profiles {
			if strings.ToUpper(strings.TrimSpace(group.AIDHex)) == aidHex {
				return group.EID
			}
		}
	}
	if m.chipInfoCache != nil {
		for _, info := range m.chipInfoCache.EIDs {
			if strings.ToUpper(strings.TrimSpace(info.AIDHex)) == aidHex {
				return info.EID
			}
		}
	}
	return ""
}

// loadProfileGroupForAIDFresh 直接读取指定 AID 的 profiles。
// 调用方可能已经持有 opMu（例如切卡后的二次确认），因此这里不能走 forEachEUICC/RefreshProfiles。
func (m *Manager) loadProfileGroupForAIDFresh(aid []byte) (EUICCProfiles, error) {
	aidHex := fmt.Sprintf("%X", aid)
	client, err := m.createLPAWithAID(aid)
	if err != nil {
		return EUICCProfiles{AIDHex: aidHex, Profiles: []ProfileItem{}}, err
	}
	defer m.closeLPAClientForOperation("load_profiles_for_aid", client)

	eidStr := ""
	if eid, err := client.EID(); err != nil {
		logger.Debug("指定 AID 读取 profiles 前获取 EID 失败，复用缓存 EID",
			"device", m.deviceID,
			"AID", aidHex,
			"err", err)
		eidStr = m.cachedEIDForAID(aidHex)
	} else {
		eidStr = hex.EncodeToString(eid)
	}

	profiles, err := listBasicProfiles(client)
	if err != nil {
		return EUICCProfiles{EID: eidStr, AIDHex: aidHex, Profiles: []ProfileItem{}}, err
	}
	group := buildProfileGroup(eidStr, aid, profiles)
	logger.Info("获取指定 eUICC profiles",
		"device", m.deviceID,
		"AID", aidHex,
		"EID", eidStr,
		"profileCount", len(profiles))
	return group, nil
}

func (m *Manager) replaceCachedProfileGroup(group EUICCProfiles) {
	if m == nil {
		return
	}
	aidHex := strings.ToUpper(strings.TrimSpace(group.AIDHex))
	if aidHex == "" {
		return
	}
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if m.overviewCache == nil {
		m.overviewCache = &EsimOverview{}
	}
	replaced := false
	for i := range m.overviewCache.Profiles {
		if strings.ToUpper(strings.TrimSpace(m.overviewCache.Profiles[i].AIDHex)) != aidHex {
			continue
		}
		if group.EID == "" {
			group.EID = m.overviewCache.Profiles[i].EID
		}
		m.overviewCache.Profiles[i] = group
		replaced = true
		break
	}
	if !replaced {
		m.overviewCache.Profiles = append(m.overviewCache.Profiles, group)
	}
	sortEUICCProfilesStable(m.overviewCache.Profiles)
	m.overviewLastErr = nil
}

func (m *Manager) loadOverviewFresh() (*EsimOverview, error) {
	info := &EUICCChipInfo{}
	var profileGroups []EUICCProfiles
	var fnMu sync.Mutex

	if err := m.forEachEUICC(func(client *lpa.Client, aid []byte, eidStr string) error {
		euiccInfo := buildDiscoveredEUICCInfo(aid, eidStr)
		aidHex := euiccInfo.AIDHex
		m.parseEUICCInfo2ForEID(client, &euiccInfo)
		logger.Debug("eUICC AID 扫描阶段",
			"device", m.deviceID,
			"stage", "profiles_start",
			"AID", aidHex,
			"EID", eidStr)
		profiles, profileErr := listBasicProfiles(client)

		// 通知统计已移至异步（refreshNotificationCountAsync），
		// 避免首次加载时 safeListNotification + DB 查询增加 200-500ms 延迟。
		// 收集 EID 用于后续异步通知统计。
		fnMu.Lock()
		defer fnMu.Unlock()
		info.EIDs = append(info.EIDs, euiccInfo)
		if profileErr != nil {
			logger.Debug("eUICC AID 扫描阶段",
				"device", m.deviceID,
				"stage", "profiles_failed",
				"AID", aidHex,
				"EID", eidStr,
				"err", profileErr)
			profileGroups = append(profileGroups, EUICCProfiles{
				EID:      eidStr,
				AIDHex:   aidHex,
				Profiles: []ProfileItem{},
			})
			return nil
		}
		group := buildProfileGroup(eidStr, aid, profiles)
		profileGroups = append(profileGroups, group)
		logger.Debug("eUICC AID 扫描阶段",
			"device", m.deviceID,
			"stage", "profiles_ok",
			"AID", aidHex,
			"EID", eidStr,
			"profileCount", len(profiles))
		logger.Info("获取 eUICC 信息和 profiles",
			"device", m.deviceID,
			"AID", aidHex,
			"EID", eidStr,
			"freeNvram", euiccInfo.FreeNvram,
			"profileCount", len(profiles))
		return nil
	}); err != nil {
		return nil, err
	}
	sortEUICCInfosStable(info.EIDs)
	sortEUICCProfilesStable(profileGroups)

	m.cacheMu.RLock()
	cachedChip := m.chipInfoCache
	m.cacheMu.RUnlock()
	if hasReusableChipProductInfo(cachedChip) {
		info.SkuName = cachedChip.SkuName
		info.SerialNumber = cachedChip.SerialNumber
		info.Firmware = cachedChip.Firmware
		logger.Debug("复用 chipInfoCache 跳过 eSTK.me Product AID 查询",
			"device", m.deviceID,
			"cached_sku", cachedChip.SkuName)
	} else {
		m.parseESTKmeInfo(info)
	}
	if info.Firmware == "" && len(info.EIDs) > 0 {
		info.Firmware = info.EIDs[0].Firmware
	}
	if info.SkuName == "" && len(info.EIDs) > 0 {
		if guessedName := predictSkuName(info.EIDs[0].EID, info.Firmware); guessedName != "" {
			info.SkuName = guessedName
			logger.Info("基于特征预测了 eSIM 品牌信息", "device", m.deviceID, "guessed_sku", info.SkuName)
		}
	}

	overview := &EsimOverview{
		ChipInfo:          info,
		Profiles:          profileGroups,
		NotificationCount: 0, // 通知数异步统计，首次返回 0
	}
	m.enrichProfileSizes(overview)
	return overview, nil
}

// refreshNotificationCountAsync 异步统计待处理通知数量并更新 overviewCache。
// 在 loadOverviewFresh 返回后调用，避免首次加载时 safeListNotification + DB 查询的同步延迟。
func (m *Manager) refreshNotificationCountAsync() {
	if m == nil {
		return
	}
	go func() {
		count, err := m.countPendingNotifications()
		if err != nil {
			logger.Debug("异步通知统计失败",
				"device", m.deviceID,
				"err", err)
			return
		}
		m.cacheMu.Lock()
		if m.overviewCache != nil {
			m.overviewCache.NotificationCount = count
		}
		m.cacheMu.Unlock()
		logger.Debug("异步通知统计完成",
			"device", m.deviceID,
			"count", count)
	}()
}

// countPendingNotifications 统计所有 eUICC 的待处理通知数量（排除 DB 中已发送的）。
// 需要独立打开 LPA 通道，不依赖 loadOverviewFresh 的 forEachEUICC 上下文。
func (m *Manager) countPendingNotifications() (int, error) {
	var total int
	var fnMu sync.Mutex
	if err := m.forEachEUICC(func(client *lpa.Client, aid []byte, eidStr string) error {
		notifs, notifErr := safeListNotification(client)
		if notifErr != nil || len(notifs) == 0 {
			return nil
		}
		count := len(notifs)
		if eidStr != "" {
			if sentCount, sentErr := db.CountSentNotificationsByEID(eidStr); sentErr == nil && sentCount > 0 {
				sentSeqs, seqErr := db.GetSentNotificationSeqsByEID(eidStr)
				if seqErr == nil {
					sentSet := make(map[sgp22.SequenceNumber]bool, len(sentSeqs))
					for _, s := range sentSeqs {
						sentSet[sgp22.SequenceNumber(s)] = true
					}
					actualPending := 0
					for _, n := range notifs {
						if n != nil && !sentSet[n.SequenceNumber] {
							actualPending++
						}
					}
					count = actualPending
				}
			}
		}
		fnMu.Lock()
		total += count
		fnMu.Unlock()
		return nil
	}); err != nil {
		return 0, err
	}
	return total, nil
}

// GetEsimOverview 获取 eSIM 总览信息（一次遍历同时获取芯片信息和 profiles）
func (m *Manager) GetEsimOverview() (*EsimOverview, error) {
	return m.loadOverview()
}

// GetProfiles 获取所有 eUICC 按分组的 profile 列表
func (m *Manager) GetProfiles() ([]EUICCProfiles, error) {
	overview, err := m.loadOverview()
	if err != nil {
		return nil, err
	}
	return cloneProfiles(overview.Profiles), nil
}

func (m *Manager) ActiveProfileName() (string, error) {
	overview := m.cachedOverview()
	if overview == nil {
		return "", nil
	}
	for _, group := range overview.Profiles {
		for _, profile := range group.Profiles {
			if profile.State != 1 {
				continue
			}
			return strings.TrimSpace(profile.Name), nil
		}
	}
	return "", nil
}

func (m *Manager) RefreshProfiles() error {
	if m == nil {
		return nil
	}
	loader := m.profilesLoader
	if loader == nil {
		loader = m.loadProfilesFresh
	}
	profiles, err := loader()
	if err != nil {
		return err
	}
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if m.overviewCache == nil {
		m.overviewCache = &EsimOverview{}
	}
	m.overviewCache.Profiles = cloneProfiles(profiles)
	m.overviewLastErr = nil
	return nil
}

func (m *Manager) RefreshOverview() error {
	if m == nil {
		return nil
	}
	// 手动刷新：重置退避计数器，允许立即执行扫描
	m.resetScanBackoff()
	pendingReload := m.overviewReloadInProgress()
	if err := m.waitForOverviewReloadAllowed(context.Background()); err != nil {
		return err
	}
	if pendingReload || m.overviewReloadInProgress() {
		overview, err := m.loadOverview()
		if err != nil {
			return err
		}
		m.cacheMu.Lock()
		if overview == nil || overview.ChipInfo == nil {
			m.chipInfoCache = nil
		}
		m.cacheMu.Unlock()
		return nil
	}
	m.cacheMu.Lock()
	m.overviewGeneration++
	m.overviewCache = nil
	m.overviewLastErr = nil
	m.discoveredEUICCs = nil
	if !hasReusableChipProductInfo(m.chipInfoCache) {
		m.chipInfoCache = nil
	}
	m.cacheMu.Unlock()

	overview, err := m.loadOverview()
	if err != nil {
		return err
	}

	m.cacheMu.Lock()
	if overview == nil || overview.ChipInfo == nil {
		m.chipInfoCache = nil
	}
	m.cacheMu.Unlock()
	return nil
}

// findAIDForICCID 在所有 eUICC 中查找包含指定 ICCID 的 AID
func (m *Manager) findAIDForICCID(targetICCID string) ([]byte, error) {
	iccid, err := sgp22.NewICCID(targetICCID)
	if err != nil {
		return nil, NewDeleteProfileError(
			DeleteProfileErrorInvalidICCID,
			fmt.Sprintf("无效的 ICCID %q: %v", targetICCID, err),
			err,
		)
	}

	aids := m.getEffectiveAIDs()
	triedCount := 0
	var lastErr error
	for _, aid := range aids {
		found, err := func() (bool, error) {
			triedCount++
			aidHex := fmt.Sprintf("%X", aid)
			logger.Debug("查找 ICCID 所属 eUICC",
				"device", m.deviceID,
				"stage", "select_open_start",
				"ICCID", targetICCID,
				"AID", aidHex,
				"triedCount", triedCount)
			client, err := m.createLPAWithAID(aid)
			if err != nil {
				logger.Debug("查找 ICCID 所属 eUICC",
					"device", m.deviceID,
					"stage", "select_open_failed",
					"ICCID", targetICCID,
					"AID", aidHex,
					"err", err)
				return false, err
			}
			defer m.closeLPAClientForOperation("find_aid_for_iccid", client)
			logger.Debug("查找 ICCID 所属 eUICC",
				"device", m.deviceID,
				"stage", "select_open_ok",
				"ICCID", targetICCID,
				"AID", aidHex)

			logger.Debug("查找 ICCID 所属 eUICC",
				"device", m.deviceID,
				"stage", "profiles_start",
				"ICCID", targetICCID,
				"AID", aidHex)
			profiles, err := listBasicProfiles(client)
			if err != nil {
				logger.Debug("查找 ICCID 所属 eUICC",
					"device", m.deviceID,
					"stage", "profiles_failed",
					"ICCID", targetICCID,
					"AID", aidHex,
					"err", err)
				return false, err
			}
			logger.Debug("查找 ICCID 所属 eUICC",
				"device", m.deviceID,
				"stage", "profiles_ok",
				"ICCID", targetICCID,
				"AID", aidHex,
				"profileCount", len(profiles))
			for _, p := range profiles {
				if p.ICCID.String() == iccid.String() {
					return true, nil
				}
			}
			return false, nil
		}()
		if err != nil {
			lastErr = err
			continue
		}
		if found {
			logger.Info("找到 ICCID 所属 eUICC",
				"device", m.deviceID,
				"ICCID", targetICCID,
				"AID", fmt.Sprintf("%X", aid))
			return aid, nil
		}
	}
	msg := fmt.Sprintf("在所有 eUICC 中未找到 ICCID %s (triedCount=%d)", targetICCID, triedCount)
	if lastErr != nil {
		msg = fmt.Sprintf("%s，lastErr=%v", msg, lastErr)
	}
	return nil, NewDeleteProfileError(
		DeleteProfileErrorProfileNotFound,
		msg,
		nil,
	)
}

func normalizeICCIDValue(in string) string {
	v := strings.TrimSpace(in)
	v = strings.Trim(v, "\"")
	v = strings.TrimRight(v, "Ff")
	return v
}

func isTargetICCIDActive(targetICCID string, currentICCID string) bool {
	target := normalizeICCIDValue(targetICCID)
	current := normalizeICCIDValue(currentICCID)
	return target != "" && current != "" && target == current
}

func (m *Manager) backendMode() string {
	if m.backend == nil {
		return transportCustom
	}
	mode := strings.TrimSpace(m.backend.Mode())
	if mode == "" {
		return "unknown"
	}
	return mode
}

// NotifyUIMIndication 用于将 UIM refresh/slot status 指示快速反馈给切卡确认循环。
// 失败场景仍会回退到固定轮询，不依赖该信号。
func (m *Manager) NotifyUIMIndication(source string) {
	if m == nil {
		return
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "indication"
	}
	if m.switchSignal != nil {
		select {
		case m.switchSignal <- source:
		default:
		}
	}
	if m.shouldSuppressOverviewReload() {
		logger.Info("切卡窗口内跳过 eSIM 总览自动重载", "device", m.deviceID, "source", source)
		return
	}
	m.invalidateOverviewCache("uim_" + source)
	m.triggerOverviewReload("uim_" + source)
}

// NotifyModemReset 通知 eSIM 管理器模组已重置（换卡场景）。
// 清空 chipInfoCache 和 discoveredEUICCs，确保下次读取重新全量查询硬件信息。
// 换卡必然触发 modem reset，因此这是保证 chipInfo 缓存正确性的关键路径。
func (m *Manager) NotifyModemReset() {
	if m == nil {
		return
	}
	m.cacheMu.Lock()
	m.clearHardwareDiscoveryCachesLocked()
	m.cacheMu.Unlock()
	if m.shouldSuppressOverviewReload() {
		logger.Info("切卡窗口内跳过 modem reset 导致的 eSIM 自动重载", "device", m.deviceID)
		return
	}
	logger.Info("eSIM 缓存已清空（modem reset）", "device", m.deviceID)
	m.triggerOverviewReload("modem_reset")
}

func (m *Manager) NotifyModemResetDelayed(delay time.Duration) {
	if m == nil {
		return
	}
	m.cacheMu.Lock()
	m.clearHardwareDiscoveryCachesLocked()
	m.cacheMu.Unlock()
	if m.shouldSuppressOverviewReload() {
		logger.Info("切卡窗口内跳过 modem reset 导致的 eSIM 自动重载", "device", m.deviceID)
		return
	}
	logger.Info("eSIM 缓存已清空（modem reset）", "device", m.deviceID, "reload_delay", delay.String())
	if delay > 0 {
		m.beginOverviewReloadSuppression(delay)
	}
	m.triggerOverviewReload("modem_reset")
}

func (m *Manager) drainSwitchSignals() {
	if m == nil || m.switchSignal == nil {
		return
	}
	for {
		select {
		case <-m.switchSignal:
		default:
			return
		}
	}
}

func (m *Manager) waitForTargetProfileActive(ctx context.Context, targetICCID string, targetAID []byte) error {
	if m.backend == nil {
		if m.transport == transportCustom {
			return m.waitForTargetProfileActiveFromProfiles(ctx, targetICCID, targetAID)
		}
		return fmt.Errorf("eSIM manager 未配置 device backend")
	}

	targetICCID = normalizeICCIDValue(targetICCID)
	if targetICCID == "" {
		return nil
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	backendMode := m.backendMode()
	lastObservedICCID := ""
	var lastErr error

	verify := func(reason string) bool {
		currentICCID, err := m.backend.GetICCID(ctx)
		if err != nil {
			lastErr = err
			return false
		}

		lastObservedICCID = normalizeICCIDValue(currentICCID)
		if isTargetICCIDActive(targetICCID, lastObservedICCID) {
			logger.Info("检测到目标 eSIM profile 已生效",
				"device", m.deviceID,
				"backend", backendMode,
				"target", targetICCID,
				"current", lastObservedICCID,
				"reason", reason)
			return true
		}

		return false
	}

	if verify("initial") {
		return nil
	}

	switchSignal := m.switchSignal
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("等待目标 profile 生效超时 (backend=%s target=%s current=%s last_err=%v): %w",
					backendMode, targetICCID, lastObservedICCID, lastErr, ctx.Err())
			}
			return fmt.Errorf("等待目标 profile 生效超时 (backend=%s target=%s current=%s): %w",
				backendMode, targetICCID, lastObservedICCID, ctx.Err())
		case source := <-switchSignal:
			if verify("indication:" + source) {
				return nil
			}
		case <-ticker.C:
			if verify("poll") {
				return nil
			}
		}
	}
}

func (m *Manager) waitForTargetProfileActiveFromProfiles(ctx context.Context, targetICCID string, targetAID []byte) error {
	targetICCID = normalizeICCIDValue(targetICCID)
	if targetICCID == "" {
		return nil
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastObservedICCID := ""
	var lastErr error

	verify := func(reason string) bool {
		var groups []EUICCProfiles
		if len(targetAID) > 0 {
			group, err := m.loadProfileGroupForAIDFresh(targetAID)
			if err != nil {
				lastErr = err
				return false
			}
			m.replaceCachedProfileGroup(group)
			groups = []EUICCProfiles{group}
		} else {
			if err := m.RefreshProfiles(); err != nil {
				lastErr = err
				return false
			}
			overview := m.cachedOverview()
			if overview == nil {
				return false
			}
			groups = overview.Profiles
		}
		for _, group := range groups {
			for _, profile := range group.Profiles {
				if profile.State == int(sgp22.ProfileEnabled) {
					lastObservedICCID = normalizeICCIDValue(profile.ICCID)
				}
				if profile.State == int(sgp22.ProfileEnabled) && isTargetICCIDActive(targetICCID, profile.ICCID) {
					logger.Info("检测到目标 eSIM profile 已在 profiles 中生效",
						"device", m.deviceID,
						"backend", transportCustom,
						"target", targetICCID,
						"current", normalizeICCIDValue(profile.ICCID),
						"reason", reason)
					return true
				}
			}
		}
		return false
	}

	if verify("initial") {
		return nil
	}

	switchSignal := m.switchSignal
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("等待目标 profile 生效超时 (backend=%s target=%s current=%s last_err=%v): %w",
					transportCustom, targetICCID, lastObservedICCID, lastErr, ctx.Err())
			}
			return fmt.Errorf("等待目标 profile 生效超时 (backend=%s target=%s current=%s): %w",
				transportCustom, targetICCID, lastObservedICCID, ctx.Err())
		case source := <-switchSignal:
			if verify("indication:" + source) {
				return nil
			}
		case <-ticker.C:
			if verify("poll") {
				return nil
			}
		}
	}
}

func (m *Manager) waitForTargetProfileActiveWithFallback(ctx context.Context, targetICCID string, stage string, targetAID []byte) error {
	confirmErr := m.waitForTargetProfileActive(ctx, targetICCID, targetAID)
	if confirmErr == nil {
		return nil
	}

	if m.transport != transportQMI || !errors.Is(confirmErr, context.DeadlineExceeded) {
		return confirmErr
	}

	fallbackErr := m.trySIMPowerCycleAndConfirm(targetICCID)
	if fallbackErr != nil {
		return fmt.Errorf("%s: 主路径确认超时后 SIM 重载兜底失败: %w (initial=%v)", stage, fallbackErr, confirmErr)
	}

	logger.Info("主路径确认超时后，SIM 重载兜底已使目标 profile 生效",
		"device", m.deviceID,
		"target", targetICCID,
		"stage", stage)
	return nil
}

func (m *Manager) forceSIMPowerCycle(reason string) error {
	if m.transport != transportQMI {
		return nil
	}
	if m.backend == nil {
		return fmt.Errorf("eSIM manager 未配置 device backend")
	}

	controller, ok := m.backend.(simPowerController)
	if !ok {
		return fmt.Errorf("backend=%s 不支持 UIM SIM 电源控制", m.backendMode())
	}

	logger.Info("切卡后执行 SIM power cycle 强制重载卡片",
		"device", m.deviceID,
		"reason", reason,
		"slot", defaultSIMSlot)

	powerCtx, cancel := context.WithTimeout(context.Background(), switchFallbackPowerTimeout)
	defer cancel()

	var errPowerOff error
	for {
		if err := powerCtx.Err(); err != nil {
			return fmt.Errorf("UIMPowerOffSIM(slot=%d) 重试超时: %w", defaultSIMSlot, errPowerOff)
		}
		errPowerOff = controller.UIMPowerOffSIM(powerCtx, defaultSIMSlot)
		if errPowerOff == nil {
			break
		}
		logger.Warn("UIMPowerOffSIM 失败，0.5秒后重试", "device", m.deviceID, "err", errPowerOff)
		select {
		case <-powerCtx.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	time.Sleep(switchFallbackPowerCycleWait)

	var errPowerOn error
	for {
		if err := powerCtx.Err(); err != nil {
			return fmt.Errorf("UIMPowerOnSIM(slot=%d) 重试超时: %w", defaultSIMSlot, errPowerOn)
		}
		errPowerOn = controller.UIMPowerOnSIM(powerCtx, defaultSIMSlot)
		if errPowerOn == nil {
			break
		}
		logger.Warn("UIMPowerOnSIM 失败，0.5秒后重试", "device", m.deviceID, "err", errPowerOn)
		select {
		case <-powerCtx.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	time.Sleep(switchFallbackPowerCycleWait)

	return nil
}

// forceATSIMReload 针对 AT 传输通道的设备，执行 AT CFUN (ModeRFOff -> ModeOnline) 强制重载卡片，以便模块重新扫描并读取新卡身份
func (m *Manager) forceATSIMReload(reason string) error {
	if m.transport != transportAT {
		return nil
	}
	if m.backend == nil {
		return fmt.Errorf("eSIM manager 未配置 device backend")
	}

	logger.Info("切卡后执行 AT CFUN reload 强制重载卡片",
		"device", m.deviceID,
		"reason", reason)

	reloadCtx, cancel := context.WithTimeout(context.Background(), switchFallbackPowerTimeout)
	defer cancel()

	// 切换到射频关闭模式 (CFUN=4)
	if err := m.backend.SetOperatingMode(reloadCtx, backendpkg.ModeRFOff); err != nil {
		return fmt.Errorf("AT CFUN=4 SIM reload 失败: %w", err)
	}
	time.Sleep(switchFallbackPowerCycleWait)

	// 重新切换回在线模式 (CFUN=1)
	if err := m.backend.SetOperatingMode(reloadCtx, backendpkg.ModeOnline); err != nil {
		return fmt.Errorf("AT CFUN=1 SIM reload 失败: %w", err)
	}
	time.Sleep(switchFallbackPowerCycleWait)

	return nil
}

// forcePostSwitchSIMReload 统一后处理：根据设备传输协议选择相应的 SIM 重载方式（QMI 电源重启或 AT 重载）
func (m *Manager) forcePostSwitchSIMReload(reason string) error {
	switch m.transport {
	case transportQMI:
		return m.forceSIMPowerCycle(reason)
	case transportAT:
		return m.forceATSIMReload(reason)
	default:
		return nil
	}
}

type liveICCIDReader interface {
	GetICCIDLive(ctx context.Context) (string, error)
}

func normalizeICCIDForSwitchCompare(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	return strings.TrimRight(value, "F")
}

func (m *Manager) shouldForcePostSwitchSIMReload(ctx context.Context, targetICCID string) (bool, string) {
	target := normalizeICCIDForSwitchCompare(targetICCID)
	if target == "" {
		return true, "target_iccid_unavailable"
	}
	if m == nil || m.backend == nil {
		return true, "backend_unavailable"
	}
	reader, ok := m.backend.(liveICCIDReader)
	if !ok {
		return true, "identity_reader_unavailable"
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	current, err := reader.GetICCIDLive(probeCtx)
	if err != nil {
		return true, "identity_probe_failed"
	}
	if normalizeICCIDForSwitchCompare(current) == target {
		return false, "target_iccid_already_active"
	}
	return true, "target_iccid_not_active"
}

func (m *Manager) trySIMPowerCycleAndConfirm(targetICCID string) error {
	if err := m.forceSIMPowerCycle("fallback_confirm"); err != nil {
		return err
	}
	confirmCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.waitForTargetProfileActive(confirmCtx, targetICCID, nil)
}

// isExpectedCardResetSignal 判断 err 是否为切卡（refresh=true）触发的预期 eUICC 内部 RESET 信号，
// 而非真正的失败。QMI 与 MBIM 各自有自己的协议层信号（见 ErrQMIUIMCardReset / ErrMBIMUICCInvalidChannel
// 的注释），统一在此处判定，使 finalizeEnableProfileResult 与 DisableProfile 不必关心具体 transport。
func isExpectedCardResetSignal(err error) bool {
	return errors.Is(err, ErrQMIUIMCardReset) || errors.Is(err, ErrMBIMUICCInvalidChannel)
}

// isEUICCBusyTransient 判断错误是否为瞬态"eUICC 忙碌"信号。
// SGP.22 规定 ES10b 命令返回 SW=910B 表示 eUICC 忙碌，终端应等待后
// 重发同一命令；euicc-go 会把该 SW 直接报为普通错误（无哨兵值），
// 故按错误文本识别。
func isEUICCBusyTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sgp22.ErrCatBusy) {
		return true
	}
	return strings.Contains(err.Error(), "status 910B")
}

func (m *Manager) finalizeEnableProfileResult(targetICCID string, enableErr error) error {
	if enableErr == nil {
		return nil
	}
	if isExpectedCardResetSignal(enableErr) {
		logger.Info("EnableProfile 触发了 eUICC 内部 RESET（预期信号），按切卡命令已提交处理",
			"device", m.deviceID,
			"target", targetICCID)
		return nil
	}
	return fmt.Errorf("启用 profile %s 失败: %w", targetICCID, enableErr)
}

type apduSwitchBarrier interface {
	BeginBarrier(ctx context.Context, req apduarbiter.Request, policy apduarbiter.BarrierPolicy) (*apduarbiter.Barrier, error)
}

func (m *Manager) beginSwitchAPDUBarrier(ctx context.Context) (*apduarbiter.Barrier, error) {
	if m == nil || m.apduArbiter == nil {
		return nil, nil
	}
	coordinator, ok := m.apduArbiter.(apduSwitchBarrier)
	if !ok {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return coordinator.BeginBarrier(ctx, apduarbiter.Request{
		Owner: "esim_switch",
		Mode:  strings.ToUpper(m.transport),
		Class: apduarbiter.APDUClassSwitchBarrier,
	}, apduarbiter.BarrierPolicy{BlockedClasses: []apduarbiter.APDUClass{
		apduarbiter.APDUClassUSIMAKA,
		apduarbiter.APDUClassSMSC,
	}})
}

// SwitchProfile 切换到指定 ICCID 的 profile
// aidHex 可选，前端已知时直接传入可跳过全量 AID 遍历
func (m *Manager) SwitchProfile(ctx context.Context, targetICCID string, aidHex string) error {
	_, err := m.SwitchProfileWithResult(ctx, targetICCID, aidHex)
	return err
}

func (m *Manager) SwitchProfileWithResult(ctx context.Context, targetICCID string, aidHex string) (SwitchProfileResult, error) {
	const operation = SwitchOperationEnableProfile
	result := SwitchProfileResult{TargetICCID: targetICCID}
	m.opMu.Lock()
	writeStarted := time.Now()
	switchSucceeded := false
	switchStarted := false
	var switchToken uint64
	var switchFailureErr error
	defer func() {
		if !switchSucceeded {
			m.emitSwitchPhase(operation, switchToken, SwitchPhaseFailed)
		}
		m.logWriteOperationHold("switch_profile", writeStarted)
		m.opMu.Unlock()
		m.notifyWriteDone()
		if !switchSucceeded && switchStarted && m.onSwitchFailed != nil {
			m.onSwitchFailed(operation, switchToken, switchFailureErr)
		}
	}()

	iccid, err := sgp22.NewICCID(targetICCID)
	if err != nil {
		return result, fmt.Errorf("无效的 ICCID %q: %w", targetICCID, err)
	}
	m.drainSwitchSignals()

	// 1. 查找 ICCID 所属的 eUICC
	var targetAID []byte
	if aidHex != "" {
		// 前端已知 AID，直接 decode
		targetAID, err = hex.DecodeString(aidHex)
		if err != nil {
			return result, fmt.Errorf("无效的 AID hex %q: %w", aidHex, err)
		}
		logger.Info("使用前端传入的 AID", "device", m.deviceID, "AID", aidHex)
	} else {
		// 回退：遍历查找
		targetAID, err = m.findAIDForICCID(targetICCID)
		if err != nil {
			return result, err
		}
	}

	// 2. 用对应 AID 创建 LPA client
	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		return result, err
	}
	clientClosed := false
	defer func() {
		if !clientClosed {
			_ = m.closeLPAClientForOperation("switch_profile_deferred", client)
		}
	}()

	logger.Info("开始切换 eSIM profile",
		"device", m.deviceID,
		"target", targetICCID,
		"AID", fmt.Sprintf("%X", targetAID))

	// 触发切卡前的回调（通常用于主动断开 VoWiFi 防冲突）
	if m.onBeforeSwitch != nil {
		switchToken = m.onBeforeSwitch(operation, targetICCID)
		switchStarted = true
	}
	result.SwitchToken = switchToken
	result.Phase = SwitchPhaseAPDUSwitching
	m.emitSwitchPhase(operation, switchToken, SwitchPhaseAPDUSwitching)

	switchBarrier, err := m.beginSwitchAPDUBarrier(ctx)
	barrierReleased := false
	releaseSwitchBarrier := func(nextPhase SwitchPhase) {
		if switchBarrier != nil && !barrierReleased {
			switchBarrier.Release()
			barrierReleased = true
		}
		if nextPhase != "" {
			result.Phase = nextPhase
			m.emitSwitchPhase(operation, switchToken, nextPhase)
		}
	}
	defer releaseSwitchBarrier("")
	if err != nil {
		switchFailureErr = err
		return result, fmt.Errorf("等待切卡 APDU barrier 失败: %w", err)
	}

	// refresh flag 由配置控制；refresh=true 可能立即触发 UIM refresh/slot indication，
	// 必须在发 APDU 前抑制自动 overview reload。
	m.beginSwitchOverviewSuppression()

	// 4. 启用目标 profile（refresh=true 会自动禁用当前活跃的 profile）。
	// refresh=true 时 UIM card reset（QMI: ErrQMIUIMCardReset / MBIM: ErrMBIMUICCInvalidChannel）
	// 是预期信号，后续 finalize 已处理，见 isExpectedCardResetSignal。
	// CatBusy (result=5) 与 SW=910B (eUICC busy) 都是瞬态错误：卡片忙碌时
	// 等待后重试即可恢复。最多重试 3 次，间隔递增（1.5s/3s/6s）：
	// SGP.22 要求收到 910B 后等待并重发同一命令，递增间隔给卡片
	// 足够的内部事务完成时间。
	var enableErr error
	busyBackoff := []time.Duration{1500 * time.Millisecond, 3 * time.Second, 6 * time.Second}
	const maxCatBusyRetries = 3
	for attempt := 0; attempt <= maxCatBusyRetries; attempt++ {
		enableErr = client.EnableProfile(iccid, m.switchUseRefreshTrue)
		if enableErr == nil || !isEUICCBusyTransient(enableErr) {
			break
		}
		if attempt < maxCatBusyRetries {
			logger.Warn("EnableProfile 遇到 eUICC 忙碌 (CatBusy/910B)，等待后重试",
				"device", m.deviceID,
				"target", targetICCID,
				"err", enableErr,
				"attempt", fmt.Sprintf("%d/%d", attempt+1, maxCatBusyRetries))
			time.Sleep(busyBackoff[attempt])
		}
	}

	// 在模组重启前主动关闭 LPA 逻辑通道（AT+CCHC）
	if err := m.closeLPAClientForOperation("switch_profile_pre_refresh", client); err == nil {
		clientClosed = true
	}

	// 5. 给卡片一点时间完成内部刷新动作。
	time.Sleep(200 * time.Millisecond)
	releaseSwitchBarrier(SwitchPhaseCardResetSettling)

	if enableErr != nil && !isExpectedCardResetSignal(enableErr) {
		switchFailureErr = enableErr
		if err := m.finalizeEnableProfileResult(targetICCID, enableErr); err != nil {
			return result, err
		}
	}
	result.SwitchAccepted = true

	patched := m.patchCachedActiveProfile(targetICCID, fmt.Sprintf("%X", targetAID))
	result.CachePatched = patched

	logger.Info("eSIM profile 切换指令已提交，后处理将异步继续",
		"device", m.deviceID, "target", targetICCID, "cache_patched", patched, "degraded_reason", result.DegradedReason)

	// 6. 切卡后回调改为 Ready + Delay 门控，避免与 eSIM 重读/VoWiFi 鉴权冲突。
	go m.runPostSwitchRecovery(operation, switchToken, targetICCID)
	result.PostSwitchAsync = true
	result.RecoveryPending = true

	switchSucceeded = true
	return result, nil
}

// DisableProfile 禁用指定 ICCID 的 profile。
// aidHex 可选，前端已知时直接传入可跳过全量 AID 遍历。
func (m *Manager) DisableProfile(ctx context.Context, targetICCID string, aidHex string) error {
	const operation = SwitchOperationDisableProfile
	m.opMu.Lock()
	writeStarted := time.Now()
	disableSucceeded := false
	disableStarted := false
	var switchToken uint64
	var switchFailureErr error
	defer func() {
		if !disableSucceeded {
			m.emitSwitchPhase(operation, switchToken, SwitchPhaseFailed)
		}
		m.logWriteOperationHold("disable_profile", writeStarted)
		m.opMu.Unlock()
		m.notifyWriteDone()
		if !disableSucceeded && disableStarted && m.onSwitchFailed != nil {
			m.onSwitchFailed(operation, switchToken, switchFailureErr)
		}
	}()

	iccid, err := sgp22.NewICCID(targetICCID)
	if err != nil {
		return fmt.Errorf("无效的 ICCID %q: %w", targetICCID, err)
	}

	var targetAID []byte
	if aidHex != "" {
		targetAID, err = hex.DecodeString(aidHex)
		if err != nil {
			return fmt.Errorf("无效的 AID hex %q: %w", aidHex, err)
		}
	} else {
		targetAID, err = m.findAIDForICCID(targetICCID)
		if err != nil {
			return err
		}
	}

	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		return err
	}
	clientClosed := false
	defer func() {
		if !clientClosed {
			_ = m.closeLPAClientForOperation("disable_profile_deferred", client)
		}
	}()

	logger.Info("开始禁用 eSIM profile",
		"device", m.deviceID,
		"ICCID", targetICCID,
		"AID", fmt.Sprintf("%X", targetAID))

	if m.onBeforeSwitch != nil {
		switchToken = m.onBeforeSwitch(operation, "")
		disableStarted = true
	}
	m.emitSwitchPhase(operation, switchToken, SwitchPhaseAPDUSwitching)

	switchBarrier, err := m.beginSwitchAPDUBarrier(ctx)
	barrierReleased := false
	releaseSwitchBarrier := func(nextPhase SwitchPhase) {
		if switchBarrier != nil && !barrierReleased {
			switchBarrier.Release()
			barrierReleased = true
		}
		if nextPhase != "" {
			m.emitSwitchPhase(operation, switchToken, nextPhase)
		}
	}
	defer releaseSwitchBarrier("")
	if err != nil {
		switchFailureErr = err
		return fmt.Errorf("等待禁用 profile APDU barrier 失败: %w", err)
	}

	disableErr := client.DisableProfile(iccid, true)
	if err := m.closeLPAClientForOperation("disable_profile_pre_refresh", client); err == nil {
		clientClosed = true
	}

	time.Sleep(200 * time.Millisecond)
	releaseSwitchBarrier(SwitchPhaseCardResetSettling)

	if disableErr != nil {
		if isExpectedCardResetSignal(disableErr) {
			logger.Info("DisableProfile 触发了 eUICC 内部 RESET（预期信号），按禁用指令提交成功处理",
				"device", m.deviceID,
				"target", targetICCID)
		} else {
			switchFailureErr = disableErr
			return fmt.Errorf("禁用 profile %s 失败: %w", targetICCID, disableErr)
		}
	}
	m.beginSwitchOverviewSuppression()

	m.invalidateOverviewCache("disable_profile")
	m.triggerOverviewReload("disable_profile")

	logger.Info("eSIM profile 禁用指令已提交",
		"device", m.deviceID,
		"target", targetICCID)

	go m.runPostSwitchHook(operation, switchToken)

	disableSucceeded = true
	return nil
}

func (m *Manager) runPostSwitchHook(operation SwitchOperation, token uint64) {
	if m == nil || m.onAfterSwitch == nil {
		return
	}
	hookStart, postSwitchDelayMS := m.waitPostSwitchHookDelay()
	m.finishPostSwitchHook(operation, token, hookStart, postSwitchDelayMS)
}

func (m *Manager) runPostSwitchRecovery(operation SwitchOperation, token uint64, targetICCID string) {
	if m == nil {
		return
	}
	hookStart, postSwitchDelayMS := m.waitPostSwitchHookDelay()
	m.finishPostSwitchHook(operation, token, hookStart, postSwitchDelayMS)
}

func (m *Manager) waitPostSwitchHookDelay() (time.Time, int64) {
	hookStart := time.Now()
	delay := m.postSwitchMinDelay
	if delay <= 0 {
		delay = defaultPostSwitchMinDelay
	}
	time.Sleep(delay)
	return hookStart, time.Since(hookStart).Milliseconds()
}

func (m *Manager) runPostSwitchSIMReload(operation SwitchOperation, token uint64, targetICCID string) {
	shouldReload, reloadReason := m.shouldForcePostSwitchSIMReload(context.Background(), targetICCID)
	if !shouldReload {
		m.emitSwitchPhase(operation, token, SwitchPhaseReloadSkipped)
		logger.Info("切卡后目标 ICCID 已生效，跳过 SIM reload",
			"device", m.deviceID,
			"target", targetICCID,
			"reload_reason", reloadReason)
		return
	}
	if m.transport != transportQMI && m.transport != transportAT {
		return
	}
	if err := m.forcePostSwitchSIMReload("enable_profile"); err != nil {
		m.emitSwitchPhase(operation, token, SwitchPhaseReloadWarning)
		logger.Warn("切卡后 SIM reload 失败",
			"device", m.deviceID,
			"target", targetICCID,
			"reload_reason", reloadReason,
			"err", err)
		if m.onSwitchDegraded != nil {
			m.onSwitchDegraded(operation, token, SwitchPhaseReloadWarning, err)
		}
	}
}

func (m *Manager) finishPostSwitchHook(operation SwitchOperation, token uint64, hookStart time.Time, postSwitchDelayMS int64) {
	logger.Info("切卡后 hook 阶段完成",
		"device", m.deviceID,
		"operation", operation,
		"switch_token", token,
		"post_switch_delay_ms", postSwitchDelayMS,
		"hook_total_ms", time.Since(hookStart).Milliseconds())
	if m.onAfterSwitch != nil {
		m.onAfterSwitch(operation, token)
	}
}

// DownloadProgressEvent 是进度回调的事件结构
type DownloadProgressEvent struct {
	Step string // 阶段标识，如 "preflight"、"auth_client"、"auth_server"、"install"、"notify"、"done"
	Msg  string // 中文描述
	Pct  int    // 进度百分比 0-100
}

// DownloadProgressFn 是进度回调函数类型
type DownloadProgressFn func(event DownloadProgressEvent)

type SpaceDeltaDirection string

const (
	SpaceDeltaDirectionReleased SpaceDeltaDirection = "released"
	SpaceDeltaDirectionConsumed SpaceDeltaDirection = "consumed"
)

type SpaceDelta struct {
	Direction SpaceDeltaDirection `json:"direction"`
	Bytes     int64               `json:"bytes"`
}

type DeleteProfileResult struct {
	Warning     string
	WarningCode string
	SpaceDelta  *SpaceDelta
}

type DownloadProfileResult struct {
	Warning     string
	WarningCode string
	SpaceDelta  *SpaceDelta
}

type spaceDeltaOperation string

const (
	spaceDeltaOperationDelete   spaceDeltaOperation = "delete"
	spaceDeltaOperationDownload spaceDeltaOperation = "download"
)

func buildSpaceDeltaForOperation(op spaceDeltaOperation, before int32, after int32) *SpaceDelta {
	if before <= 0 || after <= 0 {
		return nil
	}

	switch op {
	case spaceDeltaOperationDelete:
		if after <= before {
			return nil
		}
		return &SpaceDelta{
			Direction: SpaceDeltaDirectionReleased,
			Bytes:     int64(after - before),
		}
	case spaceDeltaOperationDownload:
		if after >= before {
			return nil
		}
		return &SpaceDelta{
			Direction: SpaceDeltaDirectionConsumed,
			Bytes:     int64(before - after),
		}
	default:
		return nil
	}
}

func (m *Manager) readFreeNvramBytes(client *lpa.Client) int32 {
	if client == nil {
		return 0
	}
	var info EUICCInfo
	m.parseEUICCInfo2ForEID(client, &info)
	return info.FreeNvramBytes
}

func (m *Manager) readFreeNvramBytesWithRetry(client *lpa.Client, attempts int, delay time.Duration) int32 {
	for attempt := 0; attempt < attempts; attempt++ {
		if freeBytes := m.readFreeNvramBytes(client); freeBytes > 0 {
			return freeBytes
		}
		if attempt < attempts-1 {
			time.Sleep(delay)
		}
	}
	return 0
}

// NotificationItem 通知列表项（对标 NekoKo NotificationCard 数据）。
// Status 值: "pending"(未发送/卡上), "sent"(已发送), "failed"(发送失败)
type NotificationItem struct {
	SequenceNumber int64  `json:"sequence_number"`
	Event          string `json:"event"`
	ICCID          string `json:"iccid,omitempty"`
	ProfileName    string `json:"profile_name,omitempty"` // 关联 profile 的名称（手机号/卡名）
	MCC            string `json:"mcc,omitempty"`          // 关联 profile 的 MCC（用于前端国旗显示）
	Address        string `json:"address,omitempty"`
	AIDHex         string `json:"aid_hex,omitempty"`
	CanRetry       bool   `json:"can_retry"`
	Status         string `json:"status"` // pending / sent / failed
}

type NotificationErrorCode string

const (
	NotificationErrorInvalidSequence NotificationErrorCode = "INVALID_SEQUENCE"
	NotificationErrorInvalidAIDHex   NotificationErrorCode = "INVALID_AID_HEX"
	NotificationErrorNotFound        NotificationErrorCode = "NOT_FOUND"
	NotificationErrorBusy            NotificationErrorCode = "BUSY"
	NotificationErrorInternal        NotificationErrorCode = "INTERNAL"
)

type NotificationError struct {
	Code    NotificationErrorCode
	Message string
	Err     error
}

func (e *NotificationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "notification operation failed"
}

func (e *NotificationError) Unwrap() error { return e.Err }

func NewNotificationError(code NotificationErrorCode, message string, err error) error {
	return &NotificationError{Code: code, Message: message, Err: err}
}

func ClassifyNotificationError(err error) NotificationErrorCode {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrOperationInProgress) {
		return NotificationErrorBusy
	}
	var ne *NotificationError
	if errors.As(err, &ne) && ne.Code != "" {
		return ne.Code
	}
	return NotificationErrorInternal
}

func maxNotificationSequence(notifications []*sgp22.NotificationMetadata) sgp22.SequenceNumber {
	var lastSeq sgp22.SequenceNumber
	for _, notification := range notifications {
		if notification != nil && notification.SequenceNumber > lastSeq {
			lastSeq = notification.SequenceNumber
		}
	}
	return lastSeq
}

func downloadResultNotificationMetadata(result *sgp22.LoadBoundProfilePackageResponse) *sgp22.NotificationMetadata {
	if result == nil || result.Notification == nil || result.Notification.SequenceNumber <= 0 {
		return nil
	}
	return &sgp22.NotificationMetadata{SequenceNumber: result.Notification.SequenceNumber}
}

func downloadNotificationBaseline(preDownloadNotifications []*sgp22.NotificationMetadata, resultNotification *sgp22.NotificationMetadata) sgp22.SequenceNumber {
	baseline := maxNotificationSequence(preDownloadNotifications)
	if resultNotification != nil && resultNotification.SequenceNumber > 0 {
		derived := resultNotification.SequenceNumber - 1
		if derived > baseline {
			baseline = derived
		}
	}
	return baseline
}

func downloadNotificationResult(observed bool, retrieveErr error, handleErr error) DownloadProfileResult {
	if !observed {
		return DownloadProfileResult{
			Warning:     "Profile 下载完成，但通知未完全确认",
			WarningCode: "download_notification_not_observed",
		}
	}
	if retrieveErr != nil {
		return DownloadProfileResult{
			Warning:     "Profile 下载完成，但通知未完全确认",
			WarningCode: "download_notification_retrieve_failed",
		}
	}
	if handleErr != nil {
		return DownloadProfileResult{
			Warning:     "Profile 下载完成，但通知未完全确认",
			WarningCode: "download_notification_handle_failed",
		}
	}
	return DownloadProfileResult{}
}

func downloadFinalizeRecoveredResult() DownloadProfileResult {
	return DownloadProfileResult{
		Warning:     "Profile 已安装，下载会话收尾返回异常；安装通知已发送，已按成功处理",
		WarningCode: "download_installed_finalize_error_recovered",
	}
}

func safeListNotification(client *lpa.Client, filters ...sgp22.NotificationEvent) (notifications []*sgp22.NotificationMetadata, err error) {
	if client == nil {
		return nil, fmt.Errorf("LPA client 为空")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("解析通知列表响应失败: %v", r)
			notifications = nil
		}
	}()
	notifications, err = client.ListNotification(filters...)
	if err != nil {
		return nil, wrapNotificationParseError("解析通知列表响应失败", err)
	}
	return notifications, nil
}

func safeRetrieveNotificationList(client *lpa.Client, seq sgp22.SequenceNumber) (pendingNotifications []*sgp22.PendingNotification, err error) {
	if client == nil {
		return nil, fmt.Errorf("LPA client 为空")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("解析待发送通知响应失败: %v", r)
			pendingNotifications = nil
		}
	}()
	pendingNotifications, err = client.RetrieveNotificationList(seq)
	if err != nil {
		return nil, wrapNotificationParseError("解析待发送通知响应失败", err)
	}
	return pendingNotifications, nil
}

// safeRetrieveAllNotifications 使用 combined 策略一次获取全量 PendingNotification（对标 NekoKo NotificationStrategy.combined）。
// searchCriteria=nil 时卡片返回所有待处理通知，无需后续逐个 retrieve，减少 APDU 往返。
func safeRetrieveAllNotifications(client *lpa.Client) (pendingNotifications []*sgp22.PendingNotification, err error) {
	if client == nil {
		return nil, fmt.Errorf("LPA client 为空")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("解析全量通知响应失败: %v", r)
			pendingNotifications = nil
		}
	}()
	pendingNotifications, err = client.RetrieveNotificationList(nil)
	if err != nil {
		return nil, wrapNotificationParseError("解析全量通知响应失败", err)
	}
	return pendingNotifications, nil
}

func wrapNotificationParseError(prefix string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sgp22.ErrUndefined) || errors.Is(err, sgp22.ErrUnexpectedTag) {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return err
}

func isRecoverableDownloadInstallFinalizeError(err error) bool {
	if err == nil {
		return false
	}
	var bppPtr *sgp22.LoadBoundProfilePackageError
	var bppValue sgp22.LoadBoundProfilePackageError
	if errors.As(err, &bppPtr) && bppPtr != nil {
		return false
	}
	if errors.As(err, &bppValue) {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"apdu",
		"pc/sc",
		"cancel session",
		"execution error",
		"unexpected response",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func deleteNotificationResult(observed bool, retrieveErr error, handleErr error) DeleteProfileResult {
	if !observed {
		return DeleteProfileResult{
			Warning:     "Profile 已删除，但删除通知发送未完全确认",
			WarningCode: "delete_notification_not_observed",
		}
	}
	if retrieveErr != nil {
		return DeleteProfileResult{
			Warning:     "Profile 已删除，但删除通知发送未完全确认",
			WarningCode: "delete_notification_retrieve_failed",
		}
	}
	if handleErr != nil {
		return DeleteProfileResult{
			Warning:     "Profile 已删除，但删除通知发送未完全确认",
			WarningCode: "delete_notification_handle_failed",
		}
	}
	return DeleteProfileResult{}
}

func notificationEventName(event sgp22.NotificationEvent) string {
	switch event {
	case sgp22.NotificationEventInstall:
		return "install"
	case sgp22.NotificationEventEnable:
		return "enable"
	case sgp22.NotificationEventDisable:
		return "disable"
	case sgp22.NotificationEventDelete:
		return "delete"
	default:
		return "unknown"
	}
}

func buildNotificationItems(notifications []*sgp22.NotificationMetadata, aidHex string) []NotificationItem {
	items := make([]NotificationItem, 0, len(notifications))
	for _, notification := range notifications {
		if notification == nil {
			continue
		}
		item := NotificationItem{
			SequenceNumber: int64(notification.SequenceNumber),
			Event:          notificationEventName(notification.ProfileManagementOperation),
			Address:        notification.Address,
			AIDHex:         aidHex,
			CanRetry:       notification.Address != "",
		}
		if len(notification.ICCID) > 0 {
			item.ICCID = notification.ICCID.String()
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].SequenceNumber > items[j].SequenceNumber
	})
	return items
}

// buildNotificationItemsFromPending 从 combined 策略获取的 PendingNotification 列表构建 NotificationItem。
// 每个 PendingNotification 内含 NotificationMetadata，无需额外 retrieve。
// dbStatusMap 为 DB 中该 EID 的通知状态映射（seq→status int），用于合并状态。
func buildNotificationItemsFromPending(pendingNotifications []*sgp22.PendingNotification, aidHex string, dbStatusMap map[int64]int, profiles []ProfileItem) []NotificationItem {
	// 构建 ICCID → Profile 索引，用于关联卡名和国旗
	profileByICCID := make(map[string]ProfileItem, len(profiles))
	for _, p := range profiles {
		profileByICCID[p.ICCID] = p
	}

	items := make([]NotificationItem, 0, len(pendingNotifications))
	for _, pn := range pendingNotifications {
		if pn == nil || pn.Notification == nil {
			continue
		}
		meta := pn.Notification
		seq := int64(meta.SequenceNumber)

		// 合并 DB 状态：status==1(已发送) 的跳过不返回
		if dbStatus, ok := dbStatusMap[seq]; ok && dbStatus == 1 {
			continue
		}

		item := NotificationItem{
			SequenceNumber: seq,
			Event:          notificationEventName(meta.ProfileManagementOperation),
			Address:        meta.Address,
			AIDHex:         aidHex,
			CanRetry:       meta.Address != "",
			Status:         "pending",
		}
		// DB 中 status==2 → failed
		if dbStatus, ok := dbStatusMap[seq]; ok && dbStatus == 2 {
			item.Status = "failed"
		}
		if len(meta.ICCID) > 0 {
			item.ICCID = meta.ICCID.String()
			// 通过 ICCID 关联 profile 信息（卡名 + MCC）
			if p, ok := profileByICCID[item.ICCID]; ok {
				item.ProfileName = p.Name
				item.MCC = p.MCC
			}
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].SequenceNumber > items[j].SequenceNumber
	})
	return items
}

func isAutoCleanableLoadedNotification(notification *sgp22.NotificationMetadata) bool {
	if notification == nil || notification.SequenceNumber <= 0 {
		return false
	}
	switch notification.ProfileManagementOperation {
	case sgp22.NotificationEventEnable, sgp22.NotificationEventDisable:
		return true
	default:
		return false
	}
}

// isAutoCleanablePendingNotification 检查 PendingNotification 是否为可自动清理的 enable/disable 类型。
func isAutoCleanablePendingNotification(pn *sgp22.PendingNotification) bool {
	if pn == nil || pn.Notification == nil || pn.Notification.SequenceNumber <= 0 {
		return false
	}
	switch pn.Notification.ProfileManagementOperation {
	case sgp22.NotificationEventEnable, sgp22.NotificationEventDisable:
		return true
	default:
		return false
	}
}

func filterCleanedNotifications(notifications []*sgp22.NotificationMetadata, cleaned map[sgp22.SequenceNumber]bool) []*sgp22.NotificationMetadata {
	if len(cleaned) == 0 {
		return notifications
	}
	filtered := make([]*sgp22.NotificationMetadata, 0, len(notifications))
	for _, notification := range notifications {
		if notification == nil || !cleaned[notification.SequenceNumber] {
			filtered = append(filtered, notification)
		}
	}
	return filtered
}

// filterCleanedPendingNotifications 从 PendingNotification 列表中移除已清理的通知。
func filterCleanedPendingNotifications(pendingNotifications []*sgp22.PendingNotification, cleaned map[sgp22.SequenceNumber]bool) []*sgp22.PendingNotification {
	if len(cleaned) == 0 {
		return pendingNotifications
	}
	filtered := make([]*sgp22.PendingNotification, 0, len(pendingNotifications))
	for _, pn := range pendingNotifications {
		if pn == nil || pn.Notification == nil || cleaned[pn.Notification.SequenceNumber] {
			continue
		}
		filtered = append(filtered, pn)
	}
	return filtered
}

// autoCleanPendingNotifications 在当前 channel 内按设备通知设置自动处理通知（对标 NekoKo _backgroundProcess）。
// 使用 combined 策略已获取的 PendingNotification，无需重新 retrieve。
// 按事件类型(install/enable/disable/delete)查对应开关决定 shouldSend / shouldRemove / shouldDeleteWithoutSending。
// eid 参数用于备份通知内容到 DB（G6）和更新发送状态（G5）。
func (m *Manager) autoCleanPendingNotifications(client *lpa.Client, pendingNotifications []*sgp22.PendingNotification, aidHex, eid string) map[sgp22.SequenceNumber]bool {
	cleaned := make(map[sgp22.SequenceNumber]bool)

	// G8: 读取设备通知设置
	settings, _ := db.GetEsimNotificationSettings(m.deviceID)

	for _, pn := range pendingNotifications {
		if pn == nil || pn.Notification == nil {
			continue
		}
		meta := pn.Notification
		seq := meta.SequenceNumber
		eventName := notificationEventName(meta.ProfileManagementOperation)

		// G8: 按事件类型查对应开关
		shouldSend, shouldRemove, shouldDeleteWithoutSending := getNotificationActionSettings(meta.ProfileManagementOperation, settings)

		// shouldDeleteWithoutSending: 直接从卡上移除，不发送，DB status=3
		if shouldDeleteWithoutSending {
			if err := retryWithBackoff(3, 300*time.Millisecond, nil, func() error {
				err := client.RemoveNotificationFromList(seq)
				if errors.Is(err, sgp22.ErrNothingToDelete) {
					return nil
				}
				return err
			}); err != nil {
				logger.Warn("eSIM 通知直接移除失败（deleteWithoutSending）",
					"device", m.deviceID,
					"AID", aidHex,
					"sequence", seq,
					"err", err)
				continue
			}
			// DB status=3(deleted without sending)
			if eid != "" {
				iccidStr := ""
				if len(meta.ICCID) > 0 {
					iccidStr = meta.ICCID.String()
				}
				_ = db.UpdateEsimNotificationStatus(eid, int64(seq), iccidStr, 3, nil, "")
			}
			cleaned[seq] = true
			logger.Info("eSIM 通知已直接移除（不发送）",
				"device", m.deviceID,
				"AID", aidHex,
				"sequence", seq,
				"event", eventName)
			continue
		}

		// shouldSend: 发送到 RSP 服务器
		if shouldSend {
			if err := retryWithBackoff(3, 300*time.Millisecond,
				func(attempt int, wait time.Duration, err error) {
					logger.Warn("eSIM 通知自动清理发送失败，稍后重试",
						"device", m.deviceID,
						"AID", aidHex,
						"sequence", seq,
						"event", eventName,
						"attempt", fmt.Sprintf("%d/%d", attempt, 3),
						"wait_ms", wait.Milliseconds(),
						"err", err)
				},
				func() error {
					return client.HandleNotification(pn)
				},
			); err != nil {
				// G5: 发送失败 → 更新 DB status=2(failed)
				if eid != "" {
					iccidStr := ""
					if len(meta.ICCID) > 0 {
						iccidStr = meta.ICCID.String()
					}
					_ = db.UpdateEsimNotificationStatus(eid, int64(seq), iccidStr, 2, nil, err.Error())
				}
				logger.Warn("eSIM 通知自动清理失败，发送通知失败",
					"device", m.deviceID,
					"AID", aidHex,
					"sequence", seq,
					"event", eventName,
					"err", err)
				continue
			}

			// G6: 发送成功 → 备份通知内容到 DB（status=1, sent）
			if eid != "" {
				iccidStr := ""
				if len(meta.ICCID) > 0 {
					iccidStr = meta.ICCID.String()
				}
				contentBase64 := encodePendingNotificationBase64(pn)
				// 从 overview 缓存关联 profile 名称和国旗 MCC
				profileName, mcc := m.lookupProfileByICCID(iccidStr)
				record := db.EsimNotificationRecord{
					EID:                eid,
					SeqNumber:          int64(seq),
					ICCID:              iccidStr,
					ProfileName:        profileName,
					MCC:                mcc,
					Content:            contentBase64,
					Timestamp:          time.Now().UnixMilli(),
					Status:             1, // sent
					NotificationServer: meta.Address,
					NotificationType:   eventName,
				}
				if saveErr := db.SaveEsimNotification(record); saveErr != nil {
					logger.Warn("eSIM 通知 DB 备份失败",
						"device", m.deviceID,
						"AID", aidHex,
						"sequence", seq,
						"err", saveErr)
				}
			}
		}

		// shouldRemove: 发送后从卡上移除
		if shouldRemove {
			if err := retryWithBackoff(3, 300*time.Millisecond,
				func(attempt int, wait time.Duration, err error) {
					logger.Warn("eSIM 通知自动清理移除卡内通知失败，稍后重试",
						"device", m.deviceID,
						"AID", aidHex,
						"sequence", seq,
						"event", eventName,
						"attempt", fmt.Sprintf("%d/%d", attempt, 3),
						"wait_ms", wait.Milliseconds(),
						"err", err)
				},
				func() error {
					err := client.RemoveNotificationFromList(seq)
					if errors.Is(err, sgp22.ErrNothingToDelete) {
						return nil
					}
					return err
				},
			); err != nil {
				// 移除卡内失败 → 标记 deletePending=true
				if eid != "" {
					iccidStr := ""
					if len(meta.ICCID) > 0 {
						iccidStr = meta.ICCID.String()
					}
					_ = db.SetEsimNotificationDeletePending(eid, int64(seq), iccidStr, true)
				}
				logger.Warn("eSIM 通知自动清理失败，移除卡内通知失败",
					"device", m.deviceID,
					"AID", aidHex,
					"sequence", seq,
					"event", eventName,
					"err", err)
				continue
			}
		}

		// 只有实际执行了操作（发送或移除）的通知才标记为 cleaned
		// 对标 NekoKo: 只有 shouldSend/shouldRemove/deleteWithoutSending 为 true 时才处理
		if !shouldSend && !shouldRemove && !shouldDeleteWithoutSending {
			continue
		}
		cleaned[seq] = true
		logger.Info("eSIM 通知已自动清理",
			"device", m.deviceID,
			"AID", aidHex,
			"sequence", seq,
			"event", eventName)
	}
	return cleaned
}

// getNotificationActionSettings 按事件类型返回 (shouldSend, shouldRemove, shouldDeleteWithoutSending) 三元组。
// settings 为 nil 时使用默认值（对标 NekoKo AppSettings 的 notif* 字段）。
func getNotificationActionSettings(event sgp22.NotificationEvent, settings *db.EsimNotificationSettings) (bool, bool, bool) {
	if settings == nil {
		// 默认值：enable/disable 自动发送+移除，install 自动发送+移除，delete 自动发送不移除（对齐 NekoKo）
		switch event {
		case sgp22.NotificationEventEnable, sgp22.NotificationEventDisable:
			return true, true, false
		case sgp22.NotificationEventInstall:
			return true, true, false
		case sgp22.NotificationEventDelete:
			return true, false, false
		default:
			return false, false, false
		}
	}
	switch event {
	case sgp22.NotificationEventEnable:
		return settings.AutoSendEnable, settings.AutoRemoveEnable, settings.DeleteWithoutSendingEnable
	case sgp22.NotificationEventDisable:
		return settings.AutoSendDisable, settings.AutoRemoveDisable, settings.DeleteWithoutSendingDisable
	case sgp22.NotificationEventInstall:
		return settings.AutoSendInstall, settings.AutoRemoveInstall, false
	case sgp22.NotificationEventDelete:
		return settings.AutoSendDelete, settings.AutoRemoveDelete, false
	default:
		return false, false, false
	}
}

// encodePendingNotificationBase64 将 PendingNotification 序列化为 base64 字符串用于 DB 备份。
func encodePendingNotificationBase64(pn *sgp22.PendingNotification) string {
	if pn == nil || pn.PendingNotification == nil {
		return ""
	}
	data := pn.PendingNotification.Bytes()
	if len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

// listNotificationItemsWithCleanup 使用 combined 策略一次获取全量 PendingNotification（对标 NekoKo NotificationStrategy.combined）。
// 注意：此处不再同步执行 autoClean（对标 NekoKo _backgroundProcess 的延迟触发）。
// autoClean 改由 SSE 接口 ProcessNotificationsSSE 逐条异步执行，让用户先看到列表再感知处理过程。
// eid 参数用于合并 DB 中的通知状态（对标 NekoKo _refreshStatusMap + syncAndGetCount）。
func (m *Manager) listNotificationItemsWithCleanup(client *lpa.Client, aidHex, eid string) ([]NotificationItem, error) {
	pendingNotifications, err := safeRetrieveAllNotifications(client)
	if err != nil {
		return nil, err
	}

	// G3: syncNotificationsWithDB — 对比 DB 和卡上通知，清理不一致的记录
	dbStatusMap := m.syncNotificationsWithDB(eid, pendingNotifications)

	// 从 overview 缓存获取 profiles，用于关联卡名和国旗（对标 NekoKo _preloadProfileMetadata）。
	// 仅读缓存，不触发同步加载——切卡后 overview 缓存为空时 profile_name 留空，
	// 前端先展示通知列表，overview 异步加载完成后下次请求自然补全。
	var profiles []ProfileItem
	m.cacheMu.RLock()
	if m.overviewCache != nil {
		for _, group := range m.overviewCache.Profiles {
			profiles = append(profiles, group.Profiles...)
		}
	}
	m.cacheMu.RUnlock()

	// G2: 合并 DB 状态构建 NotificationItem（status==1 的不返回）
	return buildNotificationItemsFromPending(pendingNotifications, aidHex, dbStatusMap, profiles), nil
}

// syncNotificationsWithDB 对比 DB 记录和卡上通知列表，清理不一致的记录（对标 NekoKo syncAndGetCount）。
// 返回 DB 中该 EID 的通知状态映射（seq→status int），供 buildNotificationItemsFromPending 合并。
func (m *Manager) syncNotificationsWithDB(eid string, pendingNotifications []*sgp22.PendingNotification) map[int64]int {
	if eid == "" {
		return nil
	}
	// 收集卡上通知的 seq 列表
	seqsOnCard := make([]int64, 0, len(pendingNotifications))
	for _, pn := range pendingNotifications {
		if pn != nil && pn.Notification != nil {
			seqsOnCard = append(seqsOnCard, int64(pn.Notification.SequenceNumber))
		}
	}

	// 删除 DB 中 status∈(0,2) 但不在卡上的记录（卡上已不存在，未发送的记录无意义）
	if err := db.DeleteEsimUnsentNotOnCard(eid, seqsOnCard); err != nil {
		logger.Warn("eSIM 通知 DB sync 清理失败",
			"device", m.deviceID,
			"eid", eid,
			"err", err)
	}

	// 从 DB 获取该 EID 的所有通知记录，构建 statusMap
	records, err := db.GetEsimNotificationsByEID(eid)
	if err != nil {
		logger.Warn("eSIM 通知 DB 查询失败",
			"device", m.deviceID,
			"eid", eid,
			"err", err)
		return nil
	}
	statusMap := make(map[int64]int, len(records))
	for _, r := range records {
		statusMap[r.SeqNumber] = r.Status
	}
	return statusMap
}

func (m *Manager) resolveNotificationAID(aidHex string) ([]byte, error) {
	if aidHex != "" {
		targetAID, err := hex.DecodeString(aidHex)
		if err != nil {
			return nil, NewNotificationError(
				NotificationErrorInvalidAIDHex,
				fmt.Sprintf("无效的 AID hex %q: %v", aidHex, err),
				err,
			)
		}
		return targetAID, nil
	}
	return nil, nil
}

func (m *Manager) notificationCandidateAIDs() [][]byte {
	aids := m.getEffectiveAIDs()
	candidates := make([][]byte, 0, len(aids))
	for _, aid := range aids {
		candidates = append(candidates, append([]byte(nil), aid...))
	}
	return candidates
}

// getWorkingAID 返回缓存的最近一次成功命中的 ISD-R AID（对标 NekoKo getWorkingAid）。
// 返回 nil 表示无缓存，调用方应回退到全量 AID 扫描。
func (m *Manager) getWorkingAID() []byte {
	m.workingAIDMu.RLock()
	defer m.workingAIDMu.RUnlock()
	if m.workingAID == nil {
		return nil
	}
	return append([]byte(nil), m.workingAID...)
}

// setWorkingAID 缓存成功命中的 ISD-R AID（对标 NekoKo setWorkingAid）。
func (m *Manager) setWorkingAID(aid []byte) {
	m.workingAIDMu.Lock()
	defer m.workingAIDMu.Unlock()
	m.workingAID = append([]byte(nil), aid...)
}

// firstEID 返回缓存的第一个 EID（对标 NekoKo getEid），用于通知 DB 状态合并。
func (m *Manager) firstEID() string {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	for _, info := range m.discoveredEUICCs {
		if info.EID != "" {
			return info.EID
		}
	}
	return ""
}

func (m *Manager) listNotificationsForCurrentCard() ([]NotificationItem, error) {
	unlock, err := m.lockOperation("list_notifications_current_card")
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := m.waitForAPDUIdleForRead(); err != nil {
		return nil, err
	}
	m.preCleanChannels()

	items := make([]NotificationItem, 0)
	var lastErr error
	var successCount int

	// 获取 EID 用于 DB 状态合并（对标 NekoKo _runProcess 中 getEid）
	eid := m.firstEID()

	// 优先尝试缓存的 working AID（对标 NekoKo _workingAidCache）
	candidates := m.notificationCandidateAIDs()
	if cached := m.getWorkingAID(); cached != nil {
		candidates = append([][]byte{cached}, candidates...)
	}

	for _, aid := range candidates {
		createStart := time.Now()
		client, err := m.createLPAWithAID(aid)
		createMs := time.Since(createStart).Milliseconds()
		if err != nil {
			lastErr = err
			continue
		}
		aidHex := strings.ToUpper(hex.EncodeToString(aid))
		listStart := time.Now()
		aidItems, listErr := m.listNotificationItemsWithCleanup(client, aidHex, eid)
		listMs := time.Since(listStart).Milliseconds()
		closeStart := time.Now()
		closeErr := m.closeLPAClientForOperation("list_notifications_current_card", client)
		closeMs := time.Since(closeStart).Milliseconds()
		logger.Info("通知列表耗时分解",
			"device", m.deviceID,
			"AID", aidHex,
			"create_ms", createMs,
			"list_ms", listMs,
			"close_ms", closeMs,
			"close_err", closeErr,
			"items_count", len(aidItems))
		if listErr != nil {
			lastErr = listErr
			continue
		}
		successCount++
		items = append(items, aidItems...)
		// 缓存成功命中的 AID（对标 NekoKo setWorkingAid）
		m.setWorkingAID(aid)
		// 命中第一个成功 AID 后提前退出（对标 NekoKo selectAid 命中即返回）
		break
	}
	if len(items) > 0 {
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].SequenceNumber > items[j].SequenceNumber
		})
		return items, nil
	}
	if successCount > 0 {
		return []NotificationItem{}, nil
	}
	if lastErr != nil {
		return nil, NewNotificationError(NotificationErrorInternal, fmt.Sprintf("获取通知列表失败: %v", lastErr), lastErr)
	}
	return []NotificationItem{}, nil
}

func (m *Manager) ListNotifications(aidHex string) ([]NotificationItem, error) {
	targetAID, err := m.resolveNotificationAID(aidHex)
	if err != nil {
		return nil, err
	}
	if len(targetAID) == 0 {
		return m.listNotificationsForCurrentCard()
	}
	unlock, err := m.lockOperation("list_notifications")
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := m.waitForAPDUIdleForRead(); err != nil {
		return nil, err
	}
	m.preCleanChannels()
	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		return nil, NewNotificationError(NotificationErrorInternal, fmt.Sprintf("创建 LPA client 失败: %v", err), err)
	}
	eid := m.firstEID()
	items, err := m.listNotificationItemsWithCleanup(client, strings.ToUpper(hex.EncodeToString(targetAID)), eid)
	if closeErr := m.closeLPAClientForOperation("list_notifications", client); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, NewNotificationError(NotificationErrorInternal, fmt.Sprintf("获取通知列表失败: %v", err), err)
	}
	// 缓存成功命中的 AID
	m.setWorkingAID(targetAID)
	return items, nil
}

func (m *Manager) RetryNotification(sequenceNumber int64, aidHex string) error {
	m.opMu.Lock()
	writeStarted := time.Now()
	defer func() {
		m.logWriteOperationHold("retry_notification", writeStarted)
		m.opMu.Unlock()
		m.notifyWriteDone()
	}()
	if sequenceNumber <= 0 {
		return NewNotificationError(NotificationErrorInvalidSequence, fmt.Sprintf("无效的通知序号 %d", sequenceNumber), nil)
	}
	targetAID, err := m.resolveNotificationAID(aidHex)
	if err != nil {
		return err
	}
	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		return NewNotificationError(NotificationErrorInternal, fmt.Sprintf("创建 LPA client 失败: %v", err), err)
	}
	defer m.closeLPAClientForOperation("retry_notification", client)
	seq := sgp22.SequenceNumber(sequenceNumber)
	pendingNotifications, err := safeRetrieveNotificationList(client, seq)
	if err != nil {
		if errors.Is(err, sgp22.ErrUndefined) {
			return NewNotificationError(NotificationErrorNotFound, fmt.Sprintf("通知 %d 不存在", sequenceNumber), err)
		}
		return NewNotificationError(NotificationErrorInternal, fmt.Sprintf("获取待发送通知失败: %v", err), err)
	}
	if len(pendingNotifications) == 0 {
		return NewNotificationError(NotificationErrorNotFound, fmt.Sprintf("通知 %d 不存在", sequenceNumber), nil)
	}
	eid := m.firstEID()
	for _, notification := range pendingNotifications {
		if notification == nil {
			continue
		}
		if err := retryWithBackoff(3, 300*time.Millisecond, nil, func() error {
			return client.HandleNotification(notification)
		}); err != nil {
			// G5: 发送失败 → 更新 DB status=2(failed)
			if eid != "" {
				meta := notification.Notification
				iccidStr := ""
				if meta != nil && len(meta.ICCID) > 0 {
					iccidStr = meta.ICCID.String()
				}
				_ = db.UpdateEsimNotificationStatus(eid, sequenceNumber, iccidStr, 2, nil, err.Error())
			}
			return NewNotificationError(NotificationErrorInternal, fmt.Sprintf("重试发送通知失败: %v", err), err)
		}
		// G5: 发送成功 → 备份通知内容到 DB（status=1, sent）
		if eid != "" {
			meta := notification.Notification
			iccidStr := ""
			eventName := ""
			address := ""
			if meta != nil {
				if len(meta.ICCID) > 0 {
					iccidStr = meta.ICCID.String()
				}
				eventName = notificationEventName(meta.ProfileManagementOperation)
				address = meta.Address
			}
			contentBase64 := encodePendingNotificationBase64(notification)
			// 从 overview 缓存关联 profile 名称和国旗 MCC
			profileName, mcc := m.lookupProfileByICCID(iccidStr)
			record := db.EsimNotificationRecord{
				EID:                eid,
				SeqNumber:          sequenceNumber,
				ICCID:              iccidStr,
				ProfileName:        profileName,
				MCC:                mcc,
				Content:            contentBase64,
				Timestamp:          time.Now().UnixMilli(),
				Status:             1, // sent
				NotificationServer: address,
				NotificationType:   eventName,
			}
			if saveErr := db.SaveEsimNotification(record); saveErr != nil {
				logger.Warn("eSIM 通知重试 DB 备份失败",
					"device", m.deviceID,
					"sequence", sequenceNumber,
					"err", saveErr)
			}
		}
	}
	return nil
}

// DownloadProfile 下载 eSIM profile 到指定 SE
// aidHex 为目标 AID（hex 字符串），为空则使用第一个可用 AID
// smdp 为 SM-DP+ 服务器地址，matchingID 和 confirmationCode 可选
// downloadIMEI 为可选的前端指定 IMEI；为空时使用设备真实 IMEI
// progressFn 为可选进度回调，为 nil 时静默执行
// force 为 true 时跳过剩余空间不足的防炸卡保护，允许用户自担风险继续安装
func (m *Manager) DownloadProfile(ctx context.Context, aidHex, smdp, matchingID, confirmationCode, downloadIMEI string, progressFn DownloadProgressFn, force bool) (DownloadProfileResult, error) {
	report := func(step, msg string, pct int) {
		if progressFn != nil {
			progressFn(DownloadProgressEvent{Step: step, Msg: msg, Pct: pct})
		}
	}
	result := DownloadProfileResult{}
	m.opMu.Lock()
	writeStarted := time.Now()
	defer func() {
		m.logWriteOperationHold("download_profile", writeStarted)
		m.downloadCtx.Store(nil)
		m.opMu.Unlock()
		m.notifyWriteDone()
	}()
	m.downloadCtx.Store(&ctx)
	var client *lpa.Client
	var targetAID []byte
	if aidHex != "" {
		aid, err := hex.DecodeString(aidHex)
		if err != nil {
			return DownloadProfileResult{}, fmt.Errorf("无效的 AID hex %q: %w", aidHex, err)
		}
		targetAID = aid
		client, err = m.createLPAWithAID(targetAID)
		if err != nil {
			return DownloadProfileResult{}, err
		}
	} else {
		aids := m.getEffectiveAIDs()
		for _, aid := range aids {
			c, err := m.createLPAWithAID(aid)
			if err != nil {
				continue
			}
			client = c
			targetAID = append([]byte(nil), aid...)
			aidHex = fmt.Sprintf("%X", targetAID)
			break
		}
		if client == nil {
			return DownloadProfileResult{}, fmt.Errorf("未找到可用的 eUICC AID")
		}
	}
	defer func() {
		if client != nil {
			m.closeLPAClientForOperation("download_profile", client)
		}
	}()

	preDownloadNotifications, preDownloadNotificationsErr := safeListNotification(client, sgp22.NotificationEventInstall)

	report("preflight", "正在检查 eUICC 剩余空间...", 10)
	beforeFreeNvramBytes := int32(0)
	{
		checkInfo := EUICCInfo{}
		m.parseEUICCInfo2ForEID(client, &checkInfo)
		beforeFreeNvramBytes = checkInfo.FreeNvramBytes
		if checkInfo.FreeNvramBytes > 0 && checkInfo.FreeNvramBytes < 81920 {
			if !force {
				report("space_warning", fmt.Sprintf("剩余空间极度紧张（%d Bytes / %s，低于安全阈值 80KB）", checkInfo.FreeNvramBytes, checkInfo.FreeNvram), 10)
				return DownloadProfileResult{}, fmt.Errorf("SPACE_WARNING:%d:%s", checkInfo.FreeNvramBytes, checkInfo.FreeNvram)
			}
			logger.Warn("用户已确认风险，跳过防炸卡保护继续下载",
				"device", m.deviceID,
				"freeNvram", checkInfo.FreeNvram,
				"freeNvramBytes", checkInfo.FreeNvramBytes)
		} else {
			logger.Info("防炸卡预检通过", "device", m.deviceID, "freeNvram", checkInfo.FreeNvram)
		}
	}

	imei, err := m.resolveDownloadIMEI(ctx, downloadIMEI)
	if err != nil {
		return DownloadProfileResult{}, err
	}

	smdpAddr := strings.TrimSpace(smdp)
	if smdpAddr == "" {
		return DownloadProfileResult{}, fmt.Errorf("SM-DP+ 地址不能为空")
	}
	if !strings.Contains(smdpAddr, "://") {
		smdpAddr = "https://" + smdpAddr
	}
	parsedURL, err := url.Parse(smdpAddr)
	if err != nil || parsedURL.Host == "" {
		return DownloadProfileResult{}, fmt.Errorf("无效的 SM-DP+ 地址 %q", smdp)
	}

	activationCode := &lpa.ActivationCode{
		SMDP:             &url.URL{Scheme: "https", Host: parsedURL.Host},
		MatchingID:       strings.TrimSpace(matchingID),
		IMEI:             imei,
		ConfirmationCode: strings.TrimSpace(confirmationCode),
	}

	logger.Info("开始下载 eSIM profile",
		"device", m.deviceID,
		"smdp", parsedURL.Host,
		"matchingID", matchingID,
		"AID", aidHex)

	installStarted := false
	opts := &lpa.DownloadOptions{
		OnProgress: func(stage lpa.DownloadStage) {
			switch stage {
			case lpa.DownloadStageAuthenticateClient:
				report("auth_client", "正在向 SM-DP+ 进行客户端身份认证...", 30)
			case lpa.DownloadStageAuthenticateServer:
				report("auth_server", "正在向 SM-DP+ 请求 Profile 数据包...", 60)
			case lpa.DownloadStageInstall:
				installStarted = true
				report("install", "正在将 Profile 写入 eUICC...", 80)
			}
		},
	}
	downloadResult, err := client.DownloadProfile(ctx, activationCode, opts)
	if err != nil {
		downloadErr := NewDownloadProfileError(err)
		logger.Warn("下载 eSIM profile 失败",
			"device", m.deviceID,
			"smdp", parsedURL.Host,
			"matchingID", matchingID,
			"AID", aidHex,
			"freeNvram_before", beforeFreeNvramBytes,
			"error_code", downloadErr.Code,
			"bpp_command_id", downloadErr.BPPCommandID,
			"bpp_error_reason", downloadErr.BPPErrorReason,
			"details", downloadErr.Details,
			"err", err)
		if installStarted && preDownloadNotificationsErr == nil {
			report("notify", "安装结果异常，正在确认并发送下载通知...", 90)
			m.closeLPAClientForOperation("download_profile_finalize_error", client)
			client = nil
			if recovered, ok := m.recoverDownloadInstallFinalizeError(ctx, targetAID, preDownloadNotifications, err); ok {
				result = recovered
				m.invalidateOverviewCache("download_profile_recovered")
				m.beginOverviewReloadSuppression(postDownloadOverviewSettleDelay)
				m.triggerOverviewReload("download_profile_recovered")
				return result, nil
			}
		}
		m.invalidateOverviewCache("download_profile_failed")
		return DownloadProfileResult{}, downloadErr
	}

	report("notify", "正在向运营商发送下载通知...", 90)
	lastSeq := downloadNotificationBaseline(preDownloadNotifications, downloadResultNotificationMetadata(downloadResult))
	result = m.sendDownloadInstallNotification(client, lastSeq, 300*time.Millisecond)

	afterFreeNvramBytes := m.readFreeNvramBytesWithRetry(client, 3, 300*time.Millisecond)
	result.SpaceDelta = buildSpaceDeltaForOperation(spaceDeltaOperationDownload, beforeFreeNvramBytes, afterFreeNvramBytes)

	logger.Info("eSIM profile 下载完成",
		"device", m.deviceID,
		"AID", aidHex,
		"warning_code", result.WarningCode,
		"space_delta", result.SpaceDelta)

	m.invalidateOverviewCache("download_profile")
	m.beginOverviewReloadSuppression(postDownloadOverviewSettleDelay)
	m.triggerOverviewReload("download_profile")

	return result, nil
}

func (m *Manager) resolveDownloadIMEI(ctx context.Context, downloadIMEI string) (string, error) {
	if strings.TrimSpace(downloadIMEI) != "" {
		imei, err := validateDownloadIMEI(downloadIMEI)
		if err != nil {
			return "", err
		}
		return imei, nil
	}
	if m.imeiProvider != nil {
		imei, err := m.imeiProvider(ctx)
		imei = strings.TrimSpace(imei)
		if imei != "" {
			return imei, nil
		}
		if err != nil {
			logger.Warn("eSIM 下载获取设备 IMEI 失败", "device", m.deviceID, "err", err)
		}
	}
	if m.backend != nil {
		imei, err := m.backend.GetIMEI(ctx)
		imei = strings.TrimSpace(imei)
		if imei != "" {
			return imei, nil
		}
		if err != nil {
			logger.Warn("eSIM 下载获取设备 IMEI 失败",
				"device", m.deviceID,
				"backend", m.backendMode(),
				"err", err)
		}
	}
	return "", fmt.Errorf("无法获取设备 IMEI")
}

func validateDownloadIMEI(imei string) (string, error) {
	imei = strings.TrimSpace(imei)
	if len(imei) != 15 {
		return "", fmt.Errorf("无效的 IMEI：必须为 15 位数字")
	}
	for _, r := range imei {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("无效的 IMEI：必须为 15 位数字")
		}
	}
	if imei[14] != imeiLuhnCheckDigit(imei[:14]) {
		return "", fmt.Errorf("无效的 IMEI：校验位不正确")
	}
	return imei, nil
}

func imeiLuhnCheckDigit(base string) byte {
	sum := 0
	double := true
	for i := len(base) - 1; i >= 0; i-- {
		n := int(base[i] - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return byte('0' + (10-sum%10)%10)
}

// RenameProfile 修改指定 ICCID 的 eSIM profile 名称（Nickname）
// aidHex 可选，前端已知时直接传入可跳过全量 AID 遍历
func (m *Manager) RenameProfile(targetICCID string, newName string, aidHex string) error {
	m.opMu.Lock()
	writeStarted := time.Now()
	defer func() {
		m.logWriteOperationHold("rename_profile", writeStarted)
		m.opMu.Unlock()
		m.notifyWriteDone()
	}()

	iccid, err := sgp22.NewICCID(targetICCID)
	if err != nil {
		return fmt.Errorf("无效的 ICCID %q: %w", targetICCID, err)
	}

	// 1. 确定 ICCID 所属的 AID
	var targetAID []byte
	if aidHex != "" {
		targetAID, err = hex.DecodeString(aidHex)
		if err != nil {
			return fmt.Errorf("无效的 AID hex %q: %w", aidHex, err)
		}
	} else {
		// 回退：遍历查找
		targetAID, err = m.findAIDForICCID(targetICCID)
		if err != nil {
			return err
		}
	}

	// 2. 创建 LPA client
	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		return err
	}
	defer m.closeLPAClientForOperation("rename_profile", client)

	// 3. 设置新名称
	if err := client.SetNickname(iccid, newName); err != nil {
		return fmt.Errorf("修改 profile 名称失败: %w", err)
	}

	logger.Info("eSIM profile 名称修改成功",
		"device", m.deviceID,
		"ICCID", targetICCID,
		"newName", newName)
	m.invalidateOverviewCache("rename_profile")
	m.triggerOverviewReload("rename_profile")
	return nil
}

// DeleteProfile 删除指定 ICCID 的 eSIM profile
// aidHex 可选，前端已知时直接传入可跳过全量 AID 遍历
func (m *Manager) DeleteProfile(targetICCID string, aidHex string) (DeleteProfileResult, error) {
	m.opMu.Lock()
	writeStarted := time.Now()
	defer func() {
		m.logWriteOperationHold("delete_profile", writeStarted)
		m.opMu.Unlock()
		m.notifyWriteDone()
	}()

	result := DeleteProfileResult{}
	iccid, err := sgp22.NewICCID(targetICCID)
	if err != nil {
		return DeleteProfileResult{}, NewDeleteProfileError(
			DeleteProfileErrorInvalidICCID,
			fmt.Sprintf("无效的 ICCID %q: %v", targetICCID, err),
			err,
		)
	}

	var targetAID []byte
	if aidHex != "" {
		targetAID, err = hex.DecodeString(aidHex)
		if err != nil {
			return DeleteProfileResult{}, NewDeleteProfileError(
				DeleteProfileErrorInvalidAIDHex,
				fmt.Sprintf("无效的 AID hex %q: %v", aidHex, err),
				err,
			)
		}
	} else {
		targetAID, err = m.findAIDForICCID(targetICCID)
		if err != nil {
			return DeleteProfileResult{}, err
		}
	}

	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		return DeleteProfileResult{}, NewDeleteProfileError(
			DeleteProfileErrorInternal,
			fmt.Sprintf("创建 LPA client 失败: %v", err),
			err,
		)
	}
	defer m.closeLPAClientForOperation("delete_profile", client)

	logger.Info("开始删除 eSIM profile",
		"device", m.deviceID,
		"ICCID", targetICCID,
		"AID", fmt.Sprintf("%X", targetAID))

	beforeFreeNvramBytes := m.readFreeNvramBytes(client)

	var lastSeq sgp22.SequenceNumber
	currentNotifications, nerr := safeListNotification(client)
	if nerr == nil {
		for _, n := range currentNotifications {
			if n.SequenceNumber > lastSeq {
				lastSeq = n.SequenceNumber
			}
		}
	}

	if err := client.DeleteProfile(iccid); err != nil {
		return DeleteProfileResult{}, NewDeleteProfileError(
			DeleteProfileErrorInternal,
			fmt.Sprintf("删除 profile %s 失败: %v", targetICCID, err),
			err,
		)
	}

	var pendingNotifications []*sgp22.PendingNotification
	result = m.resolveDeleteNotificationResult(func() ([]*sgp22.NotificationMetadata, error) {
		return safeListNotification(client, sgp22.NotificationEventDelete)
	}, lastSeq, iccid, 300*time.Millisecond, func(seq sgp22.SequenceNumber) error {
		logger.Info("发送删除通知",
			"device", m.deviceID,
			"sequence", seq)
		return retryWithBackoff(3, 300*time.Millisecond,
			func(attempt int, wait time.Duration, err error) {
				logger.Warn("获取通知列表失败，稍后重试",
					"device", m.deviceID,
					"sequence", seq,
					"attempt", fmt.Sprintf("%d/%d", attempt, 3),
					"wait_ms", wait.Milliseconds(),
					"err", err)
			},
			func() error {
				var err error
				pendingNotifications, err = safeRetrieveNotificationList(client, seq)
				return err
			},
		)
	}, func(seq sgp22.SequenceNumber) error {
		for _, notif := range pendingNotifications {
			herr := retryWithBackoff(3, 300*time.Millisecond,
				func(attempt int, wait time.Duration, err error) {
					logger.Warn("处理删除通知失败，稍后重试",
						"device", m.deviceID,
						"sequence", seq,
						"attempt", fmt.Sprintf("%d/%d", attempt, 3),
						"wait_ms", wait.Milliseconds(),
						"err", err)
				},
				func() error {
					return client.HandleNotification(notif)
				},
			)
			if herr != nil {
				logger.Warn("处理删除通知失败，已达到重试上限",
					"device", m.deviceID,
					"sequence", seq,
					"err", herr)
				return herr
			}
		}
		return nil
	})

	afterFreeNvramBytes := m.readFreeNvramBytesWithRetry(client, 3, 300*time.Millisecond)
	result.SpaceDelta = buildSpaceDeltaForOperation(spaceDeltaOperationDelete, beforeFreeNvramBytes, afterFreeNvramBytes)

	logger.Info("eSIM profile 删除完成",
		"device", m.deviceID,
		"ICCID", targetICCID,
		"delete_ok", true,
		"notification_ok", result.WarningCode == "",
		"notification_reason", result.WarningCode,
		"space_delta", result.SpaceDelta)

	m.invalidateOverviewCache("delete_profile")
	m.triggerOverviewReload("delete_profile")

	return result, nil
}

func findMatchingDeleteNotification(notifications []*sgp22.NotificationMetadata, lastSeq sgp22.SequenceNumber, iccid sgp22.ICCID) *sgp22.NotificationMetadata {
	for _, n := range notifications {
		if n == nil {
			continue
		}
		if n.SequenceNumber > lastSeq && bytes.Equal(n.ICCID, iccid) {
			return n
		}
	}
	return nil
}

func (m *Manager) findDeleteNotificationWithWait(listFn func() ([]*sgp22.NotificationMetadata, error), lastSeq sgp22.SequenceNumber, iccid sgp22.ICCID, attempts int, delay time.Duration) (*sgp22.NotificationMetadata, bool) {
	for attempt := 0; attempt < attempts; attempt++ {
		notifications, err := listFn()
		if err == nil {
			if notification := findMatchingDeleteNotification(notifications, lastSeq, iccid); notification != nil {
				return notification, true
			}
		}
		if attempt < attempts-1 {
			time.Sleep(delay)
		}
	}
	return nil, false
}

func (m *Manager) resolveDeleteNotificationResult(listFn func() ([]*sgp22.NotificationMetadata, error), lastSeq sgp22.SequenceNumber, iccid sgp22.ICCID, delay time.Duration, retrieveFn func(seq sgp22.SequenceNumber) error, handleFn func(seq sgp22.SequenceNumber) error) DeleteProfileResult {
	notification, found := m.findDeleteNotificationWithWait(listFn, lastSeq, iccid, 3, delay)
	if !found || notification == nil {
		return deleteNotificationResult(false, nil, nil)
	}
	if err := retrieveFn(notification.SequenceNumber); err != nil {
		return deleteNotificationResult(true, err, nil)
	}
	if err := handleFn(notification.SequenceNumber); err != nil {
		return deleteNotificationResult(true, nil, err)
	}
	return DeleteProfileResult{}
}

func findMatchingNotificationAfterSeq(notifications []*sgp22.NotificationMetadata, lastSeq sgp22.SequenceNumber) *sgp22.NotificationMetadata {
	for _, n := range notifications {
		if n == nil {
			continue
		}
		if n.SequenceNumber > lastSeq {
			return n
		}
	}
	return nil
}

func (m *Manager) findNotificationWithWait(listFn func() ([]*sgp22.NotificationMetadata, error), lastSeq sgp22.SequenceNumber, attempts int, delay time.Duration) (*sgp22.NotificationMetadata, bool) {
	for attempt := 0; attempt < attempts; attempt++ {
		notifications, err := listFn()
		if err == nil {
			if notification := findMatchingNotificationAfterSeq(notifications, lastSeq); notification != nil {
				return notification, true
			}
		}
		if attempt < attempts-1 {
			time.Sleep(delay)
		}
	}
	return nil, false
}

func (m *Manager) resolveDownloadNotificationResult(listFn func() ([]*sgp22.NotificationMetadata, error), lastSeq sgp22.SequenceNumber, delay time.Duration, retrieveFn func(seq sgp22.SequenceNumber) error, handleFn func(seq sgp22.SequenceNumber) error) DownloadProfileResult {
	notification, found := m.findNotificationWithWait(listFn, lastSeq, 3, delay)
	if !found || notification == nil {
		return downloadNotificationResult(false, nil, nil)
	}
	if err := retrieveFn(notification.SequenceNumber); err != nil {
		return downloadNotificationResult(true, err, nil)
	}
	if err := handleFn(notification.SequenceNumber); err != nil {
		return downloadNotificationResult(true, nil, err)
	}
	return DownloadProfileResult{}
}

func (m *Manager) sendDownloadInstallNotification(client *lpa.Client, lastSeq sgp22.SequenceNumber, delay time.Duration) DownloadProfileResult {
	var pendingNotifications []*sgp22.PendingNotification
	return m.resolveDownloadNotificationResult(func() ([]*sgp22.NotificationMetadata, error) {
		return safeListNotification(client, sgp22.NotificationEventInstall)
	}, lastSeq, delay, func(seq sgp22.SequenceNumber) error {
		logger.Info("发送下载通知",
			"device", m.deviceID,
			"sequence", seq)
		return retryWithBackoff(3, 300*time.Millisecond,
			func(attempt int, wait time.Duration, err error) {
				logger.Warn("获取下载通知列表失败",
					"device", m.deviceID,
					"sequence", seq,
					"attempt", fmt.Sprintf("%d/%d", attempt, 3),
					"wait_ms", wait.Milliseconds(),
					"err", err)
			},
			func() error {
				var err error
				pendingNotifications, err = safeRetrieveNotificationList(client, seq)
				return err
			},
		)
	}, func(seq sgp22.SequenceNumber) error {
		for _, notification := range pendingNotifications {
			herr := retryWithBackoff(3, 300*time.Millisecond,
				func(attempt int, wait time.Duration, err error) {
					logger.Warn("处理下载通知失败，稍后重试",
						"device", m.deviceID,
						"sequence", seq,
						"attempt", fmt.Sprintf("%d/%d", attempt, 3),
						"wait_ms", wait.Milliseconds(),
						"err", err)
				},
				func() error {
					return client.HandleNotification(notification)
				},
			)
			if herr != nil {
				logger.Warn("处理下载通知失败，已达到重试上限",
					"device", m.deviceID,
					"sequence", seq,
					"err", herr)
				return herr
			}
		}
		return nil
	})
}

func (m *Manager) recoverDownloadInstallFinalizeError(ctx context.Context, targetAID []byte, preDownloadNotifications []*sgp22.NotificationMetadata, downloadErr error) (DownloadProfileResult, bool) {
	if len(targetAID) == 0 || !isRecoverableDownloadInstallFinalizeError(downloadErr) {
		return DownloadProfileResult{}, false
	}
	if err := ctx.Err(); err != nil {
		return DownloadProfileResult{}, false
	}
	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		logger.Warn("下载收尾异常后重建 LPA client 失败，无法恢复确认",
			"device", m.deviceID,
			"AID", fmt.Sprintf("%X", targetAID),
			"err", err)
		return DownloadProfileResult{}, false
	}
	defer m.closeLPAClientForOperation("download_finalize_recovery", client)

	lastSeq := downloadNotificationBaseline(preDownloadNotifications, nil)
	notificationResult := m.sendDownloadInstallNotification(client, lastSeq, 300*time.Millisecond)
	if strings.TrimSpace(notificationResult.WarningCode) != "" {
		logger.Warn("下载收尾异常后未能确认安装通知，保持下载失败",
			"device", m.deviceID,
			"AID", fmt.Sprintf("%X", targetAID),
			"warning_code", notificationResult.WarningCode,
			"err", downloadErr)
		return DownloadProfileResult{}, false
	}
	result := downloadFinalizeRecoveredResult()
	logger.Warn("下载收尾异常，但安装通知已发送，按成功处理",
		"device", m.deviceID,
		"AID", fmt.Sprintf("%X", targetAID),
		"warning_code", result.WarningCode,
		"err", downloadErr)
	return result, true
}

func retryWithBackoff(maxRetries int, baseDelay time.Duration, onRetry func(attempt int, wait time.Duration, err error), op func() error) error {
	delay := baseDelay
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := op(); err != nil {
			if attempt < maxRetries {
				if onRetry != nil {
					onRetry(attempt+1, delay, err)
				}
				time.Sleep(delay)
				delay *= 2
				continue
			}
			return err
		}
		return nil
	}
	return nil
}

// NotificationProcessEvent 是逐条处理通知时通过回调推送的 SSE 事件。
type NotificationProcessEvent struct {
	Step           string `json:"step"`            // "processing" | "sent" | "removed" | "deleted" | "failed" | "skipped" | "done"
	SequenceNumber int64  `json:"sequence_number"` // 通知序号
	Event          string `json:"event"`           // install/enable/disable/delete
	ICCID          string `json:"iccid,omitempty"`
	Message        string `json:"message"`         // 描述信息
	ProcessedCount int    `json:"processed_count"` // 已处理数
	TotalCount     int    `json:"total_count"`     // 总数
}

// ProcessNotifications 逐条处理通知（对标 NekoKo 逐条 autoClean + SSE 进度推送）。
// 每处理完一条调用 progressFn 推送事件，让前端实时感知每条通知的处理过程。
func (m *Manager) ProcessNotifications(progressFn func(NotificationProcessEvent)) error {
	// 1. 获取通知列表（不 autoClean）
	items, err := m.ListNotifications("")
	if err != nil {
		return err
	}
	if len(items) == 0 {
		progressFn(NotificationProcessEvent{Step: "done", Message: "没有待处理的通知", ProcessedCount: 0, TotalCount: 0})
		return nil
	}

	total := len(items)
	processed := 0

	// 2. 读取通知设置
	settings, _ := db.GetEsimNotificationSettings(m.deviceID)

	// 3. 获取 AID + 创建 LPA client
	targetAID, err := m.resolveNotificationAID("")
	if err != nil {
		return err
	}
	if len(targetAID) == 0 {
		candidates := m.notificationCandidateAIDs()
		if cached := m.getWorkingAID(); cached != nil {
			candidates = append([][]byte{cached}, candidates...)
		}
		if len(candidates) == 0 {
			return NewNotificationError(NotificationErrorInternal, "无可用 AID", nil)
		}
		targetAID = candidates[0]
	}

	m.opMu.Lock()
	defer func() {
		m.logWriteOperationHold("process_notifications", time.Now())
		m.opMu.Unlock()
		m.notifyWriteDone()
	}()

	if err := m.waitForAPDUIdleForRead(); err != nil {
		return err
	}
	m.preCleanChannels()
	client, err := m.createLPAWithAID(targetAID)
	if err != nil {
		return NewNotificationError(NotificationErrorInternal, fmt.Sprintf("创建 LPA client 失败: %v", err), err)
	}
	defer func() {
		_ = m.closeLPAClientForOperation("process_notifications", client)
	}()

	m.setWorkingAID(targetAID)
	eid := m.firstEID()

	// 4. 重新获取卡上的 PendingNotification（ListNotifications 返回的是 NotificationItem，
	//    需要原始 pn 用于 HandleNotification）
	pendingNotifications, err := safeRetrieveAllNotifications(client)
	if err != nil {
		return err
	}

	pendingMap := make(map[int64]*sgp22.PendingNotification, len(pendingNotifications))
	for _, pn := range pendingNotifications {
		if pn != nil && pn.Notification != nil {
			pendingMap[int64(pn.Notification.SequenceNumber)] = pn
		}
	}

	// 5. 逐条处理
	for _, item := range items {
		seq := item.SequenceNumber
		eventName := item.Event

		progressFn(NotificationProcessEvent{
			Step:           "processing",
			SequenceNumber: seq,
			Event:          eventName,
			ICCID:          item.ICCID,
			Message:        fmt.Sprintf("正在处理 #%d %s", seq, eventName),
			ProcessedCount: processed,
			TotalCount:     total,
		})

		pn, ok := pendingMap[seq]
		if !ok || pn == nil || pn.Notification == nil {
			progressFn(NotificationProcessEvent{
				Step:           "skipped",
				SequenceNumber: seq,
				Event:          eventName,
				Message:        fmt.Sprintf("#%d 不在卡上，跳过", seq),
				ProcessedCount: processed,
				TotalCount:     total,
			})
			continue
		}

		meta := pn.Notification
		shouldSend, shouldRemove, shouldDeleteWithoutSending := getNotificationActionSettings(meta.ProfileManagementOperation, settings)

		if !shouldSend && !shouldRemove && !shouldDeleteWithoutSending {
			progressFn(NotificationProcessEvent{
				Step:           "skipped",
				SequenceNumber: seq,
				Event:          eventName,
				Message:        fmt.Sprintf("#%d 设置未开启自动处理，跳过", seq),
				ProcessedCount: processed,
				TotalCount:     total,
			})
			continue
		}

		iccidStr := ""
		if len(meta.ICCID) > 0 {
			iccidStr = meta.ICCID.String()
		}

		// deleteWithoutSending
		if shouldDeleteWithoutSending {
			if err := retryWithBackoff(3, 300*time.Millisecond, nil, func() error {
				err := client.RemoveNotificationFromList(meta.SequenceNumber)
				if errors.Is(err, sgp22.ErrNothingToDelete) {
					return nil
				}
				return err
			}); err != nil {
				progressFn(NotificationProcessEvent{
					Step:           "failed",
					SequenceNumber: seq,
					Event:          eventName,
					Message:        fmt.Sprintf("#%d 移除失败: %v", seq, err),
					ProcessedCount: processed,
					TotalCount:     total,
				})
				continue
			}
			if eid != "" {
				_ = db.UpdateEsimNotificationStatus(eid, seq, iccidStr, 3, nil, "")
			}
			processed++
			progressFn(NotificationProcessEvent{
				Step:           "deleted",
				SequenceNumber: seq,
				Event:          eventName,
				ICCID:          iccidStr,
				Message:        fmt.Sprintf("#%d 已直接删除（不发送）", seq),
				ProcessedCount: processed,
				TotalCount:     total,
			})
			continue
		}

		// shouldSend
		if shouldSend {
			if err := retryWithBackoff(3, 300*time.Millisecond, nil, func() error {
				return client.HandleNotification(pn)
			}); err != nil {
				if eid != "" {
					_ = db.UpdateEsimNotificationStatus(eid, seq, iccidStr, 2, nil, err.Error())
				}
				progressFn(NotificationProcessEvent{
					Step:           "failed",
					SequenceNumber: seq,
					Event:          eventName,
					Message:        fmt.Sprintf("#%d 发送失败: %v", seq, err),
					ProcessedCount: processed,
					TotalCount:     total,
				})
				continue
			}
			if eid != "" {
				contentBase64 := encodePendingNotificationBase64(pn)
				record := db.EsimNotificationRecord{
					EID:                eid,
					SeqNumber:          seq,
					ICCID:              iccidStr,
					ProfileName:        item.ProfileName,
					MCC:                item.MCC,
					Content:            contentBase64,
					Timestamp:          time.Now().UnixMilli(),
					Status:             1,
					NotificationServer: meta.Address,
					NotificationType:   eventName,
				}
				_ = db.SaveEsimNotification(record)
			}
		}

		// shouldRemove
		if shouldRemove {
			if err := retryWithBackoff(3, 300*time.Millisecond, nil, func() error {
				err := client.RemoveNotificationFromList(meta.SequenceNumber)
				if errors.Is(err, sgp22.ErrNothingToDelete) {
					return nil
				}
				return err
			}); err != nil {
				if eid != "" {
					_ = db.SetEsimNotificationDeletePending(eid, seq, iccidStr, true)
				}
				progressFn(NotificationProcessEvent{
					Step:           "failed",
					SequenceNumber: seq,
					Event:          eventName,
					Message:        fmt.Sprintf("#%d 移除失败: %v", seq, err),
					ProcessedCount: processed,
					TotalCount:     total,
				})
				continue
			}
		}

		processed++
		action := "已发送"
		if shouldSend && shouldRemove {
			action = "已发送并移除"
		} else if !shouldSend && shouldRemove {
			action = "已移除"
		}
		progressFn(NotificationProcessEvent{
			Step:           "sent",
			SequenceNumber: seq,
			Event:          eventName,
			ICCID:          iccidStr,
			Message:        fmt.Sprintf("#%d %s", seq, action),
			ProcessedCount: processed,
			TotalCount:     total,
		})
	}

	// 6. 完成
	progressFn(NotificationProcessEvent{
		Step:           "done",
		Message:        fmt.Sprintf("全部处理完成 (%d/%d)", processed, total),
		ProcessedCount: processed,
		TotalCount:     total,
	})

	// 7. 刷新 overview 缓存
	m.invalidateOverviewCache("process_notifications")
	m.triggerOverviewReload("process_notifications")

	return nil
}

// enrichProfileSizes 为每个 profile 补上容量大小，并计算 eUICC 总容量使用率。
// 查询方式：EID 前 8 位（EUM）+ PLMN（MCC+MNC）+ SPN 三要素查询 esimstore。
// 查不到的 profile 不报错，仅跳过容量信息。
func (m *Manager) enrichProfileSizes(overview *EsimOverview) {
	if m == nil || overview == nil || m.esimSizeStore == nil {
		return
	}

	var totalUsed int
	var totalFree int

	for i := range overview.Profiles {
		group := &overview.Profiles[i]
		eid := group.EID

		// 取该 eUICC 的 freeNvram 作为剩余容量
		if overview.ChipInfo != nil {
			for _, euiccInfo := range overview.ChipInfo.EIDs {
				if euiccInfo.EID == eid && euiccInfo.FreeNvramBytes > 0 {
					totalFree += int(euiccInfo.FreeNvramBytes)
					break
				}
			}
		}

		for j := range group.Profiles {
			p := &group.Profiles[j]
			plmn := strings.TrimSpace(p.MCC) + strings.TrimSpace(p.MNC)
			if plmn == "" || strings.TrimSpace(p.ServiceProviderName) == "" {
				continue
			}
			cards, err := m.esimSizeStore.ByPLMNSPN(plmn, p.ServiceProviderName, eid)
			if err == nil && len(cards) > 0 {
				card := cards[0]
				if card.Size > 0 {
					p.ProfileSizeBytes = card.Size
					p.ProfileSizeFormatted = formatBytes(int64(card.Size))
					totalUsed += card.Size
				}
				if len(card.Names) > 0 {
					p.ProfileAlias = card.Names[0]
				}
			}
		}
	}

	// 计算总容量和使用率
	totalCapacity := totalUsed + totalFree
	if totalCapacity > 0 {
		overview.TotalCapacityBytes = totalCapacity
		overview.UsedCapacityBytes = totalUsed
		overview.FreeCapacityBytes = totalFree
		overview.UsagePercent = float64(totalUsed) / float64(totalCapacity) * 100
		overview.CapacityFormatted = formatBytes(int64(totalCapacity))
	}
}
