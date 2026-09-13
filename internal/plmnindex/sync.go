// Package plmnindex syncs carrier metadata from the voorz/plmn-index repository.
//
// The plmn-index repo provides an all.json file containing PLMN → operator
// metadata (brand, country, region, icon, etc.). This package fetches that
// file, parses it, and upserts the data into the carrier_index DB table.
//
// Data source: https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/all.json
package plmnindex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/pkg/logger"
	"github.com/voorz/vowifi-core/runtimehost/carrier"
)

const (
	allJSONURL    = "https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/all.json"
	allJSONMirror = "https://cdn.jsdelivr.net/gh/voorz/plmn-index@main/plmn/all.json"
	syncInterval  = 24 * time.Hour
	httpTimeout   = 30 * time.Second
)

// PlmnEntry represents one entry in all.json.
type PlmnEntry struct {
	MCC     string `json:"mcc"`
	MNC     string `json:"mnc"`
	Country struct {
		Name   string `json:"name"`
		ISO    string `json:"iso"`
		Code   string `json:"code"`
		Region string `json:"region"`
	} `json:"country"`
	Operators []struct {
		Brand     string `json:"brand"`
		Operator  string `json:"operator"`
		Status    string `json:"status"`
		Type      string `json:"type"`
		Bands     string `json:"bands"`
		Icon      string `json:"icon"`
		IconScope string `json:"icon_scope"`
		Subs      []struct {
			Brand        string   `json:"brand"`
			Names        []string `json:"names"`
			GID1         string   `json:"gid1"`
			GID2         string   `json:"gid2"`
			ProfileNames []string `json:"profile_names"`
			Icon         string   `json:"icon"`
			IconScope    string   `json:"icon_scope"`
		} `json:"subs"`
	} `json:"operators"`
}

// Sync fetches all.json and upserts carrier_index records.
// Returns the number of records upserted.
func Sync(ctx context.Context) (int, error) {
	data, err := fetchAllJSON(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch all.json: %w", err)
	}

	entries, err := parseAllJSON(data)
	if err != nil {
		return 0, fmt.Errorf("parse all.json: %w", err)
	}

	count := 0
	for _, entry := range entries {
		idx := entryToCarrierIndex(entry)
		if idx == nil {
			continue
		}
		if err := db.UpsertCarrierIndex(idx); err != nil {
			logger.Warn("plmnindex: upsert failed", "plmn", idx.PLMN, "err", err)
			continue
		}
		count++
	}

	logger.Info("plmnindex: sync complete", "count", count)
	return count, nil
}

// SyncIfNeeded checks if the index is empty or stale, and syncs if needed.
// Returns true if a sync was performed.
func SyncIfNeeded(ctx context.Context) (bool, error) {
	count, err := db.CountCarrierIndex()
	if err != nil {
		return false, err
	}
	if count > 0 {
		// Already populated; periodic sync will handle updates
		return false, nil
	}
	logger.Info("plmnindex: index empty, performing initial sync")
	_, err = Sync(ctx)
	return true, err
}

// StartPeriodicSync runs a background goroutine that syncs every 24 hours.
func StartPeriodicSync(ctx context.Context) {
	go func() {
		// Initial sync if empty
		_, err := SyncIfNeeded(ctx)
		if err != nil {
			logger.Error("plmnindex: initial sync failed", "err", err)
		}

		ticker := time.NewTicker(syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				logger.Info("plmnindex: periodic sync starting")
				if _, err := Sync(ctx); err != nil {
					logger.Error("plmnindex: periodic sync failed", "err", err)
				}
			}
		}
	}()
}

func fetchAllJSON(ctx context.Context) ([]byte, error) {
	client := &http.Client{Timeout: httpTimeout}

	for _, url := range []string{allJSONURL, allJSONMirror} {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			logger.Warn("plmnindex: fetch failed", "url", url, "err", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			logger.Warn("plmnindex: bad status", "url", url, "code", resp.StatusCode)
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		return data, nil
	}
	return nil, fmt.Errorf("all sources failed")
}

func parseAllJSON(data []byte) (map[string]PlmnEntry, error) {
	var entries map[string]PlmnEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func entryToCarrierIndex(entry PlmnEntry) *db.CarrierIndex {
	mcc := strings.TrimSpace(entry.MCC)
	mnc := strings.TrimSpace(entry.MNC)
	if mcc == "" || mnc == "" {
		return nil
	}

	// Use carrier.PlmnKey as the single source of truth for PLMN key normalization
	plmn := carrier.PlmnKey(mcc, mnc)

	// Serialize full entry as raw_json for search
	raw, _ := json.Marshal(entry)

	return &db.CarrierIndex{
		PLMN:        plmn,
		MCC:         mcc,
		MNC:         mnc,
		CountryName: entry.Country.Name,
		CountryISO:  entry.Country.ISO,
		CountryCode: entry.Country.Code,
		Region:      entry.Country.Region,
		RawJSON:     string(raw),
	}
}
