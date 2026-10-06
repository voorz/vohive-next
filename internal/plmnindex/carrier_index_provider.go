// Package plmnindex provides carrier index data integration.
//
// This file implements CarrierIndexProvider interface for vowifi-core's
// carrier.LookupWithIdentity, bridging the DB carrier_index table
// to the GID matching logic in vowifi-core/runtimehost/carrier.
package plmnindex

import (
	"strings"

	"github.com/voorz/vohive/internal/db"
)

// carrierIndexProvider implements carrier.CarrierIndexProvider
// by querying vohive-next's DB carrier_index table.
type carrierIndexProvider struct{}

// GetCarrierIndexRawJSON returns the raw_json field from carrier_index
// for the given PLMN key (e.g. "234-10").
func (carrierIndexProvider) GetCarrierIndexRawJSON(plmnKey string) string {
	plmnKey = strings.TrimSpace(plmnKey)
	if plmnKey == "" {
		return ""
	}
	// Try the key as-is first (it should already be normalized via PlmnKey).
	idx, err := db.GetCarrierIndex(plmnKey)
	if err != nil || idx == nil {
		return ""
	}
	return idx.RawJSON
}

// InitCarrierIndexProvider 已废弃（本地 carrier 不需要 index provider），保留为空兼容调用方。
func InitCarrierIndexProvider() {}
