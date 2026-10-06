package carrier

import (
	"encoding/json"

	"github.com/voorz/vohive/internal/db"
)

// DBProfileResolver 从数据库查询运营商配置。
type DBProfileResolver struct{}

// LookupActiveProfile 从 DB 返回指定 key 的生效运营商配置。
// 无生效配置时返回 nil。
func (r *DBProfileResolver) LookupActiveProfile(key string) (*CarrierProfile, error) {
	tpl, err := db.GetActiveCarrierTemplate(key)
	if err != nil || tpl == nil || tpl.ProfileJSON == "" {
		return nil, nil
	}
	var p CarrierProfile
	if err := json.Unmarshal([]byte(tpl.ProfileJSON), &p); err != nil {
		return nil, nil
	}
	return &p, nil
}

// InitProfileResolver 初始化 DB-backed resolver。
// vowifi-core 移除后不再需要注入，此函数保留为空（兼容调用方）。
func InitProfileResolver() {}
