package server

import (
	"fmt"
	"regexp"
	"strings"
)

// TimeframeKind classifies how history for a timeframe is sourced.
type TimeframeKind int

const (
	TFBinanceREST TimeframeKind = iota
	TFRAMOnly
)

// TimeframeSpec describes a resolved dashboard timeframe.
type TimeframeSpec struct {
	ID              string
	Label           string
	BinanceInterval string
	Kind            TimeframeKind
}

var binanceIntervals = map[string]string{
	"1m": "1m", "2m": "2m", "3m": "3m", "5m": "5m", "15m": "15m", "30m": "30m",
	"1h": "1h", "2h": "2h", "4h": "4h", "6h": "6h", "8h": "8h", "12h": "12h",
	"1d": "1d", "3d": "3d", "1w": "1w", "1M": "1M",
}

var canonicalTF = map[string]TimeframeSpec{
	"1tick":   {ID: "1tick", Label: "1 tick", Kind: TFRAMOnly},
	"10ticks": {ID: "10ticks", Label: "10 ticks", Kind: TFRAMOnly},
	"100ticks": {ID: "100ticks", Label: "100 ticks", Kind: TFRAMOnly},
	"1000ticks": {ID: "1000ticks", Label: "1000 ticks", Kind: TFRAMOnly},
	"1s":  {ID: "1s", Label: "1 second", Kind: TFRAMOnly},
	"5s":  {ID: "5s", Label: "5 seconds", Kind: TFRAMOnly},
	"10s": {ID: "10s", Label: "10 seconds", Kind: TFRAMOnly},
	"15s": {ID: "15s", Label: "15 seconds", Kind: TFRAMOnly},
	"30s": {ID: "30s", Label: "30 seconds", Kind: TFRAMOnly},
	"45s": {ID: "45s", Label: "45 seconds", Kind: TFRAMOnly},
	"1m":  {ID: "1m", Label: "1 minute", BinanceInterval: "1m", Kind: TFBinanceREST},
	"3m":  {ID: "3m", Label: "3 minutes", BinanceInterval: "3m", Kind: TFBinanceREST},
	"5m":  {ID: "5m", Label: "5 minutes", BinanceInterval: "5m", Kind: TFBinanceREST},
	"10m": {ID: "10m", Label: "10 minutes", Kind: TFRAMOnly},
	"15m": {ID: "15m", Label: "15 minutes", BinanceInterval: "15m", Kind: TFBinanceREST},
	"30m": {ID: "30m", Label: "30 minutes", BinanceInterval: "30m", Kind: TFBinanceREST},
	"45m": {ID: "45m", Label: "45 minutes", Kind: TFRAMOnly},
	"1h":  {ID: "1h", Label: "1 hour", BinanceInterval: "1h", Kind: TFBinanceREST},
	"2h":  {ID: "2h", Label: "2 hours", BinanceInterval: "2h", Kind: TFBinanceREST},
	"3h":  {ID: "3h", Label: "3 hours", Kind: TFRAMOnly},
	"4h":  {ID: "4h", Label: "4 hours", BinanceInterval: "4h", Kind: TFBinanceREST},
	"1d":  {ID: "1d", Label: "1 day", BinanceInterval: "1d", Kind: TFBinanceREST},
	"1w":  {ID: "1w", Label: "1 week", BinanceInterval: "1w", Kind: TFBinanceREST},
	"1M":  {ID: "1M", Label: "1 month", BinanceInterval: "1M", Kind: TFBinanceREST},
}

var customTFRe = regexp.MustCompile(`(?i)^(\d+)\s*(tick|ticks|t|s|sec|second|seconds|m|min|minute|minutes|h|hour|hours|d|day|days|w|week|weeks|M|month|months)?$`)

// ResolveTimeframe maps a UI or custom timeframe string to a spec.
func ResolveTimeframe(raw string) (TimeframeSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return canonicalTF["1m"], nil
	}

	if spec, ok := canonicalTF[raw]; ok {
		return spec, nil
	}

	key := normalizeTFKey(raw)
	if spec, ok := canonicalTF[key]; ok {
		return spec, nil
	}

	return parseCustomTimeframe(raw)
}

// ResolveBacktestInterval maps UI interval strings to a Binance-backed spec for backtest runs.
func ResolveBacktestInterval(raw string) (TimeframeSpec, error) {
	spec, err := ResolveTimeframe(raw)
	if err != nil {
		return TimeframeSpec{}, err
	}
	if spec.Kind == TFBinanceREST && spec.BinanceInterval != "" {
		return spec, nil
	}
	return TimeframeSpec{}, fmt.Errorf("interval %q not supported for backtest", raw)
}

func normalizeTFKey(raw string) string {
	k := strings.TrimSpace(raw)
	lower := strings.ToLower(k)

	switch k {
	case "3M", "6M", "12M":
		return k
	}

	switch lower {
	case "1h", "1hour":
		return "1h"
	case "2h", "2hours":
		return "2h"
	case "3h", "3hours":
		return "3h"
	case "4h", "4hours":
		return "4h"
	case "1d", "d", "day", "1D", "D":
		return "1d"
	case "1w", "w", "week", "1W", "W":
		return "1w"
	case "1m":
		return "1m"
	case "1M", "M":
		return "1M"
	case "3m":
		return "3m"
	case "m":
		return "1m"
	case "month":
		return "1M"
	}

	if spec, ok := canonicalTF[lower]; ok {
		return spec.ID
	}
	if spec, ok := canonicalTF[k]; ok {
		return spec.ID
	}

	return lower
}

func parseCustomTimeframe(raw string) (TimeframeSpec, error) {
	m := customTFRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return TimeframeSpec{}, fmt.Errorf("unrecognized timeframe %q", raw)
	}

	n := m[1]
	unit := strings.ToLower(m[2])
	if unit == "" {
		unit = "m"
	}

	var id, label, binance string
	kind := TFRAMOnly

	switch unit {
	case "tick", "ticks", "t":
		if n != "1" {
			id = n + "ticks"
		} else {
			id = "1tick"
		}
		label = fmt.Sprintf("%s tick(s)", n)
	case "s", "sec", "second", "seconds":
		id = n + "s"
		label = fmt.Sprintf("%s second(s)", n)
	case "m", "min", "minute", "minutes":
		if strings.HasSuffix(strings.TrimSpace(raw), "M") {
			if n != "1" {
				return TimeframeSpec{}, fmt.Errorf("unsupported month timeframe %q", raw)
			}
			id = "1M"
			label = "1 month"
			binance = "1M"
			kind = TFBinanceREST
			break
		}
		id = n + "m"
		label = fmt.Sprintf("%s minute(s)", n)
		if iv, ok := binanceIntervals[id]; ok {
			binance = iv
			kind = TFBinanceREST
		}
	case "h", "hour", "hours":
		id = n + "h"
		label = fmt.Sprintf("%s hour(s)", n)
		if iv, ok := binanceIntervals[id]; ok {
			binance = iv
			kind = TFBinanceREST
		}
	case "d", "day", "days":
		id = n + "d"
		label = fmt.Sprintf("%s day(s)", n)
		if iv, ok := binanceIntervals[id]; ok {
			binance = iv
			kind = TFBinanceREST
		}
	case "w", "week", "weeks":
		id = n + "w"
		label = fmt.Sprintf("%s week(s)", n)
		if iv, ok := binanceIntervals[id]; ok {
			binance = iv
			kind = TFBinanceREST
		}
	case "month", "months":
		if n != "1" {
			return TimeframeSpec{}, fmt.Errorf("unsupported month timeframe %q", raw)
		}
		id = "1M"
		label = "1 month"
		binance = "1M"
		kind = TFBinanceREST
	default:
		return TimeframeSpec{}, fmt.Errorf("unrecognized unit in %q", raw)
	}

	return TimeframeSpec{
		ID:              id,
		Label:           label,
		BinanceInterval: binance,
		Kind:            kind,
	}, nil
}

// MenuTimeframes returns all predefined menu entries grouped for the UI catalog.
func MenuTimeframes() map[string][]TimeframeSpec {
	return map[string][]TimeframeSpec{
		"TICKS":    {canonicalTF["1tick"], canonicalTF["10ticks"], canonicalTF["100ticks"], canonicalTF["1000ticks"]},
		"SECONDS":  {canonicalTF["1s"], canonicalTF["5s"], canonicalTF["10s"], canonicalTF["15s"], canonicalTF["30s"], canonicalTF["45s"]},
		"MINUTES":  {canonicalTF["1m"], canonicalTF["2m"], canonicalTF["3m"], canonicalTF["5m"], canonicalTF["10m"], canonicalTF["15m"], canonicalTF["30m"], canonicalTF["45m"]},
		"HOURS":    {canonicalTF["1h"], canonicalTF["2h"], canonicalTF["3h"], canonicalTF["4h"]},
		"DAYS":     {canonicalTF["1d"], canonicalTF["1w"], canonicalTF["1M"]},
	}
}
