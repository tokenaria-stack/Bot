package exchange

import (
	"fmt"
	"strings"
)

// TFClass is the storage / live class of a chart timeframe.
// This is a static table, not a registry or service.
type TFClass string

const (
	TFClassNative  TFClass = "native"
	TFClassDerived TFClass = "derived"
	TFClassSeconds TFClass = "seconds"
	TFClassTicks   TFClass = "ticks"
)

const (
	LiveBinanceKlineWS = "binance_kline_ws"
	LiveParentClosed   = "parent_closed" // future derived builder
	LiveAggTrade       = "aggtrade"      // future seconds / ticks
)

// Timeframe is one row of the chart timeframe catalog.
type Timeframe struct {
	Name       string
	Label      string
	Class      TFClass
	LiveSource string
	Persist    bool
	Parent     string
	MenuGroup  string // MINUTES | HOURS | DAYS; empty = hidden from live menu
}

// USD-M native klines Binance actually sells, then inactive placeholders.
// Native rows are the only live chart TFs (Frame + WS + persist).
var timeframeCatalog = []Timeframe{
	{Name: "1m", Label: "1 minute", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "MINUTES"},
	{Name: "3m", Label: "3 minutes", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "MINUTES"},
	{Name: "5m", Label: "5 minutes", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "MINUTES"},
	{Name: "15m", Label: "15 minutes", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "MINUTES"},
	{Name: "30m", Label: "30 minutes", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "MINUTES"},
	{Name: "1h", Label: "1 hour", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "HOURS"},
	{Name: "2h", Label: "2 hours", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "HOURS"},
	{Name: "4h", Label: "4 hours", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "HOURS"},
	{Name: "6h", Label: "6 hours", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "HOURS"},
	{Name: "8h", Label: "8 hours", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "HOURS"},
	{Name: "12h", Label: "12 hours", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "HOURS"},
	{Name: "1d", Label: "1 day", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "DAYS"},
	{Name: "3d", Label: "3 days", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "DAYS"},
	{Name: "1w", Label: "1 week", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "DAYS"},
	{Name: "1M", Label: "1 month", Class: TFClassNative, LiveSource: LiveBinanceKlineWS, Persist: true, MenuGroup: "DAYS"},

	// Future derived views — catalog truth only; no Frame / WS / SQLite in this slice.
	{Name: "2m", Label: "2 minutes", Class: TFClassDerived, LiveSource: LiveParentClosed, Persist: false, Parent: "1m"},
	{Name: "10m", Label: "10 minutes", Class: TFClassDerived, LiveSource: LiveParentClosed, Persist: false, Parent: "5m"},
	{Name: "45m", Label: "45 minutes", Class: TFClassDerived, LiveSource: LiveParentClosed, Persist: false, Parent: "15m"},
	{Name: "3h", Label: "3 hours", Class: TFClassDerived, LiveSource: LiveParentClosed, Persist: false, Parent: "1h"},

	{Name: "1s", Label: "1 second", Class: TFClassSeconds, LiveSource: LiveAggTrade, Persist: false},
	{Name: "5s", Label: "5 seconds", Class: TFClassSeconds, LiveSource: LiveAggTrade, Persist: false, Parent: "1s"},
	{Name: "10s", Label: "10 seconds", Class: TFClassSeconds, LiveSource: LiveAggTrade, Persist: false, Parent: "1s"},
	{Name: "15s", Label: "15 seconds", Class: TFClassSeconds, LiveSource: LiveAggTrade, Persist: false, Parent: "1s"},
	{Name: "30s", Label: "30 seconds", Class: TFClassSeconds, LiveSource: LiveAggTrade, Persist: false, Parent: "1s"},
	{Name: "45s", Label: "45 seconds", Class: TFClassSeconds, LiveSource: LiveAggTrade, Persist: false, Parent: "1s"},

	{Name: "1tick", Label: "1 tick", Class: TFClassTicks, LiveSource: LiveAggTrade, Persist: false},
	{Name: "10ticks", Label: "10 ticks", Class: TFClassTicks, LiveSource: LiveAggTrade, Persist: false},
	{Name: "100ticks", Label: "100 ticks", Class: TFClassTicks, LiveSource: LiveAggTrade, Persist: false},
	{Name: "1000ticks", Label: "1000 ticks", Class: TFClassTicks, LiveSource: LiveAggTrade, Persist: false},
}

var timeframeByName map[string]Timeframe

func init() {
	timeframeByName = make(map[string]Timeframe, len(timeframeCatalog))
	for _, e := range timeframeCatalog {
		timeframeByName[e.Name] = e
	}
}

// Catalog returns a copy of the static timeframe table.
func Catalog() []Timeframe {
	out := make([]Timeframe, len(timeframeCatalog))
	copy(out, timeframeCatalog)
	return out
}

// TimeframeByName looks up a catalog row.
func TimeframeByName(name string) (Timeframe, bool) {
	e, ok := timeframeByName[name]
	return e, ok
}

// IsNativeBinance reports whether id is an exchange-canonical USD-M kline interval.
func IsNativeBinance(id string) bool {
	e, ok := timeframeByName[id]
	return ok && e.Class == TFClassNative
}

// NativeBinanceIDs is the live Frame + WS + persist set (stable catalog order).
func NativeBinanceIDs() []string {
	out := make([]string, 0, 16)
	for _, e := range timeframeCatalog {
		if e.Class == TFClassNative {
			out = append(out, e.Name)
		}
	}
	return out
}

// NativeBinance is the native catalog rows in boot/WS order.
func NativeBinance() []Timeframe {
	out := make([]Timeframe, 0, 16)
	for _, e := range timeframeCatalog {
		if e.Class == TFClassNative {
			out = append(out, e)
		}
	}
	return out
}

// CombinedKlineStreamNames builds combined-stream kline names for every native TF.
func CombinedKlineStreamNames(symbol string) []string {
	sym := strings.ToLower(NormalizeFuturesSymbol(symbol))
	ids := NativeBinanceIDs()
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("%s@kline_%s", sym, id)
	}
	return out
}
