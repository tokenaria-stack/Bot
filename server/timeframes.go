package server

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"trading_bot/data"
	"trading_bot/exchange"
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

var canonicalTF map[string]TimeframeSpec

func init() {
	canonicalTF = make(map[string]TimeframeSpec, 32)
	for _, e := range exchange.Catalog() {
		canonicalTF[e.Name] = specFromCatalog(e)
	}
}

func specFromCatalog(e exchange.Timeframe) TimeframeSpec {
	kind := TFRAMOnly
	binance := ""
	if e.Class == exchange.TFClassNative {
		kind = TFBinanceREST
		binance = e.Name
	}
	return TimeframeSpec{
		ID:              e.Name,
		Label:           e.Label,
		BinanceInterval: binance,
		Kind:            kind,
	}
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
	case "6h", "6hours":
		return "6h"
	case "8h", "8hours":
		return "8h"
	case "12h", "12hours":
		return "12h"
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

	var id, label string

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
			return canonicalTF["1M"], nil
		}
		id = n + "m"
		label = fmt.Sprintf("%s minute(s)", n)
	case "h", "hour", "hours":
		id = n + "h"
		label = fmt.Sprintf("%s hour(s)", n)
	case "d", "day", "days":
		id = n + "d"
		label = fmt.Sprintf("%s day(s)", n)
	case "w", "week", "weeks":
		id = n + "w"
		label = fmt.Sprintf("%s week(s)", n)
	case "month", "months":
		if n != "1" {
			return TimeframeSpec{}, fmt.Errorf("unsupported month timeframe %q", raw)
		}
		return canonicalTF["1M"], nil
	default:
		return TimeframeSpec{}, fmt.Errorf("unrecognized unit in %q", raw)
	}

	if e, ok := exchange.TimeframeByName(id); ok {
		return specFromCatalog(e), nil
	}

	return TimeframeSpec{
		ID:    id,
		Label: label,
		Kind:  TFRAMOnly,
	}, nil
}

// MenuTimeframes returns live chart menu entries (native ∪ derived views).
func MenuTimeframes() map[string][]TimeframeSpec {
	out := map[string][]TimeframeSpec{
		"MINUTES": nil,
		"HOURS":   nil,
		"DAYS":    nil,
		"SECONDS": nil,
	}
	for _, e := range exchange.LiveChart() {
		spec := specFromCatalog(e)
		switch e.MenuGroup {
		case "MINUTES", "HOURS", "DAYS", "SECONDS":
			out[e.MenuGroup] = append(out[e.MenuGroup], spec)
		}
	}
	for g, specs := range out {
		sort.Slice(specs, func(i, j int) bool {
			if specs[i].ID == "1M" {
				return false
			}
			if specs[j].ID == "1M" {
				return true
			}
			di, _ := data.IntervalDurationMs(specs[i].ID)
			dj, _ := data.IntervalDurationMs(specs[j].ID)
			return di < dj
		})
		out[g] = specs
	}
	return out
}
