// Package plmnindex provides carrier index data integration.
//
// This file implements CarrierIndexProvider interface for vowifi-core's
// carrier.LookupWithIdentity, bridging the DB carrier_index table
// to the GID matching logic in vowifi-core/runtimehost/carrier.
package plmnindex

import (
	"strings"

	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vowifi-core/runtimehost/carrier"
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
	idx, err := db.GetCarrierIndex(plmnKey)
	if err != nil || idx == nil {
		return ""
	}
	return idx.RawJSON
}

// InitCarrierIndexProvider injects the DB-backed carrier index provider
// into vowifi-core's carrier package. Must be called once at startup
// before any carrier.LookupWithIdentity call.
func InitCarrierIndexProvider() {
	carrier.SetCarrierIndexProvider(carrierIndexProvider{})
}
