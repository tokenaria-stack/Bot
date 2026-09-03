package forecast

import (
	"fmt"
	"strings"
)

// MarketKey identifies an exact market stream: venue, instrument, contract
// type, and timeframe. Symbol string alone ("BTCUSDT") is NOT a MarketKey —
// spot and perpetual BTCUSDT on the same venue are different MarketKeys
// (FORECAST-SPEC-1 §2). Any future artifact/tape/runtime bound to one
// MarketKey must REFUSE another MarketKey; never best-effort match.
type MarketKey struct {
	Venue      string // e.g. "BINANCE"
	Instrument string // e.g. "BTCUSDT"
	Contract   string // e.g. "FUTURES_PERP", "SPOT"
	Timeframe  string // e.g. "15m" — matches the existing TF catalog strings
}

// Validate reports whether all four identity components are present.
func (k MarketKey) Validate() error {
	if k.Venue == "" || k.Instrument == "" || k.Contract == "" || k.Timeframe == "" {
		return fmt.Errorf("forecast: market key requires venue, instrument, contract and timeframe (got %+v)", k)
	}
	return nil
}

// String is a readable form for logs/UI, e.g. "BINANCE:FUTURES_PERP:BTCUSDT:15m".
// It is NOT an identity encoding — comparisons must use struct equality.
func (k MarketKey) String() string {
	return strings.Join([]string{k.Venue, k.Contract, k.Instrument, k.Timeframe}, ":")
}

// SameFamily reports whether two MarketKeys share venue+instrument+contract,
// differing only in timeframe. This is the ONLY relationship that may be
// used to resolve dual-hit ambiguity with finer-timeframe history
// (FORECAST-SPEC-1 §17): never resolve one venue/contract's candles with
// another venue's or another contract type's history (e.g. never resolve
// Binance futures with Binance spot 1s).
func (k MarketKey) SameFamily(other MarketKey) bool {
	return k.Venue == other.Venue && k.Instrument == other.Instrument && k.Contract == other.Contract
}

// AnalysisRuntimeKey is the conceptual identity of one analytical-truth
// instance: an exact MarketKey plus an exact AnalysisRecipe identity.
//
//	MarketFrame  = one market/bar truth
//	AnalysisRuntime = one AnalysisRecipe identity's analytical truth
//
// Same MarketKey + same AnalysisRecipe identity → same numerical process and
// should eventually share one AnalysisRuntime. Same MarketKey + a DIFFERENT
// AnalysisRecipe identity (e.g. RSX14 vs RSX21) may run simultaneously over
// the same MarketFrame — that is deliberate isolation, not forbidden dual
// truth. Forbidden dual truth is two independent calculators claiming the
// SAME AnalysisRecipe identity while producing different state/numbers.
//
// FORECAST-SPEC-1 defines this identity only (§3/§41). No runtime
// map/registry/refcounting is implemented in this chapter. Today's
// one-DAG-per-Frame implementation remains the v1 adapter; this type must
// not be read as "one AnalysisRecipe per MarketFrame forever".
type AnalysisRuntimeKey struct {
	Market   MarketKey
	Analysis Identity
}
