package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

var (
	LimitsCachePath = "data/exchange_limits_cache.json"
	limitsCacheTTL  = 7 * 24 * time.Hour
)

type limitsCacheFile struct {
	FetchedAt time.Time                `json:"fetched_at"`
	Testnet   bool                     `json:"testnet"`
	Limits    map[string]SymbolLimits  `json:"limits"`
}

// loadNormalizerLimits fills the normalizer from disk cache or a single exchangeInfo fetch.
func loadNormalizerLimits(ctx context.Context, norm *Normalizer, client *futures.Client, isTestnet bool) error {
	cached, cacheAge, cacheErr := readLimitsCache(isTestnet)
	if cacheErr == nil && cacheAge < limitsCacheTTL {
		norm.ApplyLimits(cached)
		log.Printf("[Exchange] limits cache hit (%s, age %s, %d symbols)", LimitsCachePath, cacheAge.Round(time.Second), len(cached))
		return nil
	}

	fetchErr := norm.LoadLimitsFromFutures(ctx, client)
	if fetchErr == nil {
		if saveErr := writeLimitsCache(norm.LimitsSnapshot(), isTestnet); saveErr != nil {
			log.Printf("[Exchange] limits cache write failed: %v", saveErr)
		} else {
			log.Printf("[Exchange] limits refreshed from exchangeInfo (%d symbols)", len(norm.LimitsSnapshot()))
		}
		return nil
	}
	if cacheErr == nil && len(cached) > 0 {
		norm.ApplyLimits(cached)
		log.Printf("[Exchange] WARNING: exchangeInfo fetch failed (%v); using stale limits cache (age %s)", fetchErr, cacheAge.Round(time.Second))
		return nil
	}
	return fetchErr
}

func readLimitsCache(isTestnet bool) (map[string]SymbolLimits, time.Duration, error) {
	raw, err := os.ReadFile(LimitsCachePath)
	if err != nil {
		return nil, 0, err
	}
	var file limitsCacheFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, 0, err
	}
	if file.Testnet != isTestnet {
		return nil, 0, fmt.Errorf("cache testnet=%v does not match client testnet=%v", file.Testnet, isTestnet)
	}
	if len(file.Limits) == 0 {
		return nil, 0, fmt.Errorf("cache file has no limits")
	}
	if file.FetchedAt.IsZero() {
		return nil, 0, fmt.Errorf("cache file missing fetched_at")
	}
	return file.Limits, time.Since(file.FetchedAt), nil
}

func writeLimitsCache(limits map[string]SymbolLimits, isTestnet bool) error {
	if len(limits) == 0 {
		return fmt.Errorf("refusing to write empty limits cache")
	}
	if err := os.MkdirAll(filepath.Dir(LimitsCachePath), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(limitsCacheFile{
		FetchedAt: time.Now().UTC(),
		Testnet:   isTestnet,
		Limits:    limits,
	}, "", "  ")
	if err != nil {
		return err
	}
	tmp := LimitsCachePath + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, LimitsCachePath)
}
