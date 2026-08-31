package esim

// esimstore.go — eSIM profile 容量查询库
//
// 数据来源（双源策略）：
//   1. 远程：https://stats-backend.euicc.info/data/euicc_sizes.json
//   2. 内置兜底：internal/esim/pki/euicc_sizes.json（联网失败时使用）
//
// 查询方式：EID 前 8 位（EUM）+ PLMN（MCC+MNC）+ SPN 三要素查询
//
// DB 策略：独立 SQLite 文件（data/esim_sizes.db），与主业务库 vohive.db 分离
//   - 写锁隔离：容量库刷新不影响主库读写
//   - 本地优先：文件存在直接读盘，不存在则从远程或内置兜底数据初始化
//
// 过期更新策略：
//   - DB meta 表记录 last_fetched（RFC3339 时间戳）
//   - 有效期一周：使用时检查时间戳，过期则后台异步拉更新
//   - 更新失败不重试：顺延有效期一周，下个周期到期再获取

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/voorz/vohive/pkg/logger"
)

// euiccSizesURL 是 euicc_sizes.json 的远程数据源地址。
const euiccSizesURL = "https://stats-backend.euicc.info/data/euicc_sizes.json"

// esimSizeRefreshInterval 是容量数据的有效期（一周）。
const esimSizeRefreshInterval = 7 * 24 * time.Hour

//go:embed pki/euicc_sizes.json
var embeddedEuiccSizesJSON []byte

// esimStoreSchema 在 Open 时幂等创建。
const esimStoreSchema = `
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT
);
CREATE TABLE IF NOT EXISTS results (
    id            INTEGER PRIMARY KEY,
    plmn          TEXT,
    spn           TEXT,
    rsp           TEXT,
    reference_size INTEGER,
    names         TEXT
);
CREATE TABLE IF NOT EXISTS eum_sizes (
    result_id INTEGER,
    eum       TEXT,
    size      INTEGER,
    PRIMARY KEY (result_id, eum)
);
CREATE INDEX IF NOT EXISTS idx_eum      ON eum_sizes(eum);
CREATE INDEX IF NOT EXISTS idx_plmn_spn ON results(plmn, spn);
`

// esimStoreSelectCols 是 results 与 eum_sizes 联表查询的列清单（顺序固定）。
const esimStoreSelectCols = `r.id, r.plmn, r.spn, r.rsp, r.reference_size, r.names, es.size, es.eum`

// ESimSizeStore 持有一个打开的容量数据库连接。
type ESimSizeStore struct {
	db      *sql.DB
	dbPath  string
	dataURL string

	refreshMu      sync.Mutex
	refreshRunning bool
}

// ESimSizeCard 是一条补全后的 eSIM profile 容量信息。
type ESimSizeCard struct {
	ID            int      `json:"id"`
	PLMN          string   `json:"plmn"`
	SPN           string   `json:"spn"`
	RSP           string   `json:"rsp,omitempty"`
	ReferenceSize int      `json:"reference_size"`
	Names         []string `json:"names,omitempty"`
	EUM           string   `json:"eum,omitempty"`
	Size          int      `json:"size"`
}

// openESimSizeStore 打开（不存在则创建）位于 dbPath 的 SQLite 容量库并完成建表。
// dataURL 为 euicc_sizes.json 的远程地址，留空则使用默认值。
func openESimSizeStore(dbPath, dataURL string) (*ESimSizeStore, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = "data/esim_sizes.db"
	}
	if strings.TrimSpace(dataURL) == "" {
		dataURL = euiccSizesURL
	}

	// 确保目录存在
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// SQLite pragmas
	for _, stmt := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, err
		}
	}

	if _, err := db.Exec(esimStoreSchema); err != nil {
		db.Close()
		return nil, err
	}

	return &ESimSizeStore{db: db, dbPath: dbPath, dataURL: dataURL}, nil
}

// Close 关闭数据库连接。
func (s *ESimSizeStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// openPersistedESimSizeStore 本地优先策略：
//   - 文件已存在：直接读盘打开，不触发下载
//   - 文件不存在：尝试从远程下载；远程失败则用内置兜底 JSON 初始化
//   - force=true：无论文件是否存在都重新下载
func openPersistedESimSizeStore(dbPath, dataURL string, force bool) (*ESimSizeStore, error) {
	if !force {
		if _, err := os.Stat(dbPath); err == nil {
			s, err := openESimSizeStore(dbPath, dataURL)
			if err == nil {
				return s, nil
			}
			// 已存在但打开失败：降级为重新创建
		}
	}
	// 尝试下载并导入
	s, err := openESimSizeStore(dbPath, dataURL)
	if err != nil {
		return nil, err
	}
	if _, err := s.refreshFromURL(); err != nil {
		logger.Warn("eSIM 容量库远程下载失败，使用内置兜底数据",
			"url", dataURL, "err", err)
		// 远程失败：用内置兜底 JSON 初始化
		if _, fallbackErr := s.importJSONBytes(embeddedEuiccSizesJSON, time.Now()); fallbackErr != nil {
			s.Close()
			return nil, fmt.Errorf("远程下载和内置兜底均失败: remote=%v, fallback=%v", err, fallbackErr)
		}
		logger.Info("eSIM 容量库已用内置兜底数据初始化", "db", dbPath)
	}
	return s, nil
}

// refreshFromURL 从远程下载 euicc_sizes.json 并全量刷新数据库。
func (s *ESimSizeStore) refreshFromURL() (int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(s.dataURL)
	if err != nil {
		return 0, fmt.Errorf("下载 %s 失败: %w", s.dataURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("下载 %s 失败: HTTP %d", s.dataURL, resp.StatusCode)
	}
	return s.importJSON(resp.Body, time.Now())
}

// rawResult / rawData 对应 euicc_sizes.json 的结构。
type rawSizeResult struct {
	PLMN          string         `json:"plmn"`
	SPN           string         `json:"serviceProviderName"`
	RSP           string         `json:"rsp"`
	Names         []string       `json:"names"`
	ReferenceSize int            `json:"reference_size"`
	EumSizes      map[string]int `json:"eum_sizes"`
}

type rawSizeData struct {
	ReferenceEUM string          `json:"reference_eum"`
	Results      []rawSizeResult `json:"results"`
}

// importJSON 从 r 读取 euicc_sizes JSON，并在一个事务内全量刷新数据库。
// fetchedAt 为本次数据获取时间，写入 meta.last_fetched。
func (s *ESimSizeStore) importJSON(r io.Reader, fetchedAt time.Time) (int, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	return s.importJSONBytes(data, fetchedAt)
}

// importJSONBytes 从字节数据导入 euicc_sizes JSON，并在一个事务内全量刷新数据库。
// fetchedAt 为本次数据获取时间，写入 meta.last_fetched。
func (s *ESimSizeStore) importJSONBytes(data []byte, fetchedAt time.Time) (int, error) {
	var raw rawSizeData
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for _, d := range []string{`DELETE FROM eum_sizes`, `DELETE FROM results`, `DELETE FROM meta`} {
		if _, err := tx.Exec(d); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('reference_eum', ?)`, raw.ReferenceEUM); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('last_fetched', ?)`, fetchedAt.Format(time.RFC3339)); err != nil {
		return 0, err
	}

	for i, row := range raw.Results {
		names, err := json.Marshal(row.Names)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(
			`INSERT INTO results(id, plmn, spn, rsp, reference_size, names) VALUES(?,?,?,?,?,?)`,
			i+1, row.PLMN, row.SPN, row.RSP, row.ReferenceSize, string(names),
		); err != nil {
			return 0, err
		}
		for eum, size := range row.EumSizes {
			if _, err := tx.Exec(
				`INSERT INTO eum_sizes(result_id, eum, size) VALUES(?,?,?)`,
				i+1, eum, size,
			); err != nil {
				return 0, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(raw.Results), nil
}

// lastFetched 返回 DB 中记录的上次数据获取时间。
// 未记录返回零值。
func (s *ESimSizeStore) lastFetched() time.Time {
	if s == nil || s.db == nil {
		return time.Time{}
	}
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'last_fetched'`).Scan(&v)
	if err != nil || v == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, v)
	return t
}

// isExpired 检查数据是否已过期（超过有效期一周）。
func (s *ESimSizeStore) isExpired() bool {
	last := s.lastFetched()
	if last.IsZero() {
		return true
	}
	return time.Since(last) >= esimSizeRefreshInterval
}

// maybeRefreshIfExpired 在数据过期时异步触发更新。
// 更新失败不重试，顺延有效期一周（写入当前时间），下个周期到期再获取。
// 该方法非阻塞，立即返回。
func (s *ESimSizeStore) maybeRefreshIfExpired() {
	if s == nil || s.db == nil {
		return
	}
	if !s.isExpired() {
		return
	}

	s.refreshMu.Lock()
	if s.refreshRunning {
		s.refreshMu.Unlock()
		return
	}
	s.refreshRunning = true
	s.refreshMu.Unlock()

	go func() {
		defer func() {
			s.refreshMu.Lock()
			s.refreshRunning = false
			s.refreshMu.Unlock()
		}()

		n, err := s.refreshFromURL()
		if err != nil {
			logger.Warn("eSIM 容量数据自动刷新失败，顺延有效期一周",
				"url", s.dataURL, "err", err)
			// 顺延有效期：写入当前时间，下个周期到期再获取
			s.updateLastFetched(time.Now())
			return
		}
		logger.Info("eSIM 容量数据自动刷新成功", "results", n, "db", s.dbPath)
	}()
}

// updateLastFetched 更新 last_fetched 时间戳（不刷新数据，仅顺延有效期）。
func (s *ESimSizeStore) updateLastFetched(t time.Time) {
	if s == nil || s.db == nil {
		return
	}
	_, _ = s.db.Exec(`INSERT INTO meta(key, value) VALUES('last_fetched', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, t.Format(time.RFC3339))
}

// sizeScanner 是 *sql.Row / *sql.Rows 共有的 Scan 接口。
type sizeScanner interface {
	Scan(dest ...interface{}) error
}

func scanESimSizeCard(src sizeScanner) (ESimSizeCard, error) {
	var c ESimSizeCard
	var namesJSON sql.NullString
	var size sql.NullInt64
	var eum sql.NullString
	if err := src.Scan(&c.ID, &c.PLMN, &c.SPN, &c.RSP, &c.ReferenceSize, &namesJSON, &size, &eum); err != nil {
		return c, err
	}
	c.Names = nil
	if namesJSON.Valid && namesJSON.String != "" {
		_ = json.Unmarshal([]byte(namesJSON.String), &c.Names)
	}
	if size.Valid {
		c.Size = int(size.Int64)
	}
	if eum.Valid {
		c.EUM = eum.String
	}
	return c, nil
}

func collectESimSizeCards(rows *sql.Rows) ([]ESimSizeCard, error) {
	defer rows.Close()
	out := make([]ESimSizeCard, 0)
	for rows.Next() {
		c, err := scanESimSizeCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// eidPrefix8 取 EID 前 8 位并归一化为大写。
func eidPrefix8(eid string) string {
	e := strings.TrimSpace(strings.ToUpper(eid))
	if len(e) > 8 {
		e = e[:8]
	}
	return e
}

// ByPLMNSPN 返回匹配 plmn+spn 的候选列表。
// 若传入 eid，则只返回该 EID 前缀（eum）下命中的记录，结果更精确。
// 查询时顺便检查数据是否过期，过期则异步触发更新。
func (s *ESimSizeStore) ByPLMNSPN(plmn, spn, eid string) ([]ESimSizeCard, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}

	// 顺便检查过期，异步触发更新
	s.maybeRefreshIfExpired()

	plmn = strings.TrimSpace(plmn)
	spn = strings.TrimSpace(spn)
	eum := eidPrefix8(eid)

	if eum != "" {
		rows, err := s.db.Query(
			`SELECT `+esimStoreSelectCols+` FROM results r JOIN eum_sizes es ON es.result_id = r.id WHERE r.plmn = ? AND r.spn = ? AND es.eum = ?`,
			plmn, spn, eum,
		)
		if err != nil {
			return nil, err
		}
		return collectESimSizeCards(rows)
	}

	// 无 EID：返回 reference_size 兜底
	rows, err := s.db.Query(
		`SELECT r.id, r.plmn, r.spn, r.rsp, r.reference_size, r.names, NULL, NULL FROM results r WHERE r.plmn = ? AND r.spn = ?`,
		plmn, spn,
	)
	if err != nil {
		return nil, err
	}
	return collectESimSizeCards(rows)
}

// LookupProfileSize 用 EID + PLMN + SPN 查询 profile 容量大小（字节）。
// 返回 (size, found)：未命中时 found=false。
// 查询时顺便检查数据是否过期，过期则异步触发更新。
func (s *ESimSizeStore) LookupProfileSize(eid, plmn, spn string) (int, bool) {
	if s == nil || s.db == nil {
		return 0, false
	}
	cards, err := s.ByPLMNSPN(plmn, spn, eid)
	if err != nil || len(cards) == 0 {
		return 0, false
	}
	// 优先返回精确匹配的 size，兜底用 reference_size
	if cards[0].Size > 0 {
		return cards[0].Size, true
	}
	if cards[0].ReferenceSize > 0 {
		return cards[0].ReferenceSize, true
	}
	return 0, false
}

// ReferenceEUM 返回数据集中的参考 EUM。
func (s *ESimSizeStore) ReferenceEUM() (string, error) {
	if s == nil || s.db == nil {
		return "", nil
	}
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'reference_eum'`).Scan(&v)
	return v, err
}

// RefreshData 手动触发从远程刷新容量数据（同步阻塞）。
func (s *ESimSizeStore) RefreshData() (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	n, err := s.refreshFromURL()
	if err != nil {
		logger.Warn("刷新 eSIM 容量数据失败", "url", s.dataURL, "err", err)
		return 0, err
	}
	logger.Info("eSIM 容量数据刷新成功", "results", n, "db", s.dbPath)
	return n, nil
}
