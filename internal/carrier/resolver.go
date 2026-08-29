package carrier

import (
	"encoding/json"

	"github.com/voorz/vohive/internal/db"
	corevcarrier "github.com/voorz/vowifi-core/runtimehost/carrier"
)

// DBProfileResolver implements corevcarrier.ProfileResolver by querying the database.
type DBProfileResolver struct{}

// LookupActiveProfile returns the active carrier profile for the given key from DB.
// Returns nil if no active profile exists.
func (r *DBProfileResolver) LookupActiveProfile(key string) (*corevcarrier.CarrierProfile, error) {
	tpl, err := db.GetActiveCarrierTemplate(key)
	if err != nil || tpl == nil || tpl.ProfileJSON == "" {
		return nil, nil
	}
	var p corevcarrier.CarrierProfile
	if err := json.Unmarshal([]byte(tpl.ProfileJSON), &p); err != nil {
		return nil, nil
	}
	return &p, nil
}

// InitProfileResolver injects the DB-backed resolver into vowifi-core.
// Must be called once at startup before any LookupWithSPN call.
func InitProfileResolver() {
	corevcarrier.SetProfileResolver(&DBProfileResolver{})
}
