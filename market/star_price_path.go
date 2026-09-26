package market

import (
	"fmt"
	"math"
	"sort"

	"trading_bot/core/nodes"
	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

const (
	PathFilled     = "filled"
	PathTruncated  = "truncated"
	PathPrimaryGap = "primary_gap"
)

const (
	MinuteFavorable         = "favorable"
	MinuteAdverse           = "adverse"
	MinuteFinerMissing      = "finer_missing"
	MinuteFinerGap          = "finer_gap"
	MinuteFinerDual         = "finer_dual"
	MinuteFinerInconsistent = "finer_inconsistent"
)

// PricePath is the raw future 15-minute price read after one Star.
// Distances, horizon, and outcome are not fields. A later recipe subtracts.
type PricePath struct {
	StarOpen  int64
	StarClose int64
	Side      string

	Signal float64
	Fill   float64
	FillOK bool
	Gap    float64
	GapOK  bool

	ATR   float64
	ATROK bool

	Bars     []PathBar
	Complete string
}

// PathBar is one successor 15-minute bar. Close is omitted.
type PathBar struct {
	OpenTime int64
	Open     float64
	High     float64
	Low      float64
}

// MinuteTouch is the 1-minute order of one unordered 15-minute bar.
// HitOpen is the minute OpenTime when one side wins. It is 0 otherwise.
type MinuteTouch struct {
	State   string
	HitOpen int64
}

// BuildPricePath reads the Star bar and its true 15-minute successors from k15.
// readBars is the copy budget, not a strategy horizon. The Star bar is not copied.
// k15 must include the causal prefix through the Star so ATR sees that history.
func BuildPricePath(star StarSnapshot, k15 []exchange.Kline, readBars int) (PricePath, error) {
	if readBars < 1 {
		return PricePath{}, fmt.Errorf("market: price path read length %d", readBars)
	}
	if star.Side != nodes.WozduhXoverSideUp && star.Side != nodes.WozduhXoverSideDown {
		return PricePath{}, fmt.Errorf("market: price path side %q", star.Side)
	}
	if star.AnchorAt <= 0 || star.AnchorAt != star.ConfirmedAt {
		return PricePath{}, fmt.Errorf("market: price path star clock %d %d", star.AnchorAt, star.ConfirmedAt)
	}
	wantClose, err := data.BarCloseTimeMs(star.AnchorAt, "15m")
	if err != nil {
		return PricePath{}, err
	}
	if star.M15.CloseTime != 0 && star.M15.CloseTime != wantClose {
		return PricePath{}, fmt.Errorf("market: price path star close %d != %d", star.M15.CloseTime, wantClose)
	}
	idx := klineIndex(k15, star.AnchorAt)
	if idx < 0 {
		return PricePath{}, fmt.Errorf("market: price path star %d not in 15m series", star.AnchorAt)
	}
	bar := k15[idx]
	if bar.CloseTime != 0 && bar.CloseTime != wantClose {
		return PricePath{}, fmt.Errorf("market: price path kline close %d != %d", bar.CloseTime, wantClose)
	}
	if !pathFinite(bar.Close) {
		return PricePath{}, fmt.Errorf("market: price path star close is not finite")
	}

	bars, complete, err := successorBars(k15, idx, readBars)
	if err != nil {
		return PricePath{}, err
	}
	out := PricePath{
		StarOpen:  star.AnchorAt,
		StarClose: wantClose,
		Side:      star.Side,
		Signal:    bar.Close,
		Complete:  complete,
		Bars:      bars,
	}
	if len(bars) > 0 {
		out.Fill = bars[0].Open
		out.FillOK = true
		out.GapOK = true
		if star.Side == nodes.WozduhXoverSideUp {
			out.Gap = out.Fill - out.Signal
		} else {
			out.Gap = out.Signal - out.Fill
		}
	}
	atr, ok, err := atrAtStar(k15[:idx+1])
	if err != nil {
		return PricePath{}, err
	}
	out.ATROK = ok
	if ok {
		out.ATR = atr
	}
	return out, nil
}

func successorBars(k15 []exchange.Kline, starIdx, readBars int) ([]PathBar, string, error) {
	prev := k15[starIdx].OpenTime
	out := make([]PathBar, 0, readBars)
	next := starIdx + 1
	for len(out) < readBars {
		expected, err := data.NextBarOpen(prev, "15m")
		if err != nil {
			return nil, "", err
		}
		if next >= len(k15) {
			return out, PathTruncated, nil
		}
		k := k15[next]
		if k.OpenTime != expected {
			return out, PathPrimaryGap, nil
		}
		if !pathFinite(k.Open) || !pathFinite(k.High) || !pathFinite(k.Low) {
			return nil, "", fmt.Errorf("market: price path non-finite bar %d", k.OpenTime)
		}
		out = append(out, PathBar{OpenTime: k.OpenTime, Open: k.Open, High: k.High, Low: k.Low})
		prev = k.OpenTime
		next++
	}
	return out, PathFilled, nil
}

func atrAtStar(prefix []exchange.Kline) (float64, bool, error) {
	high := make([]float64, len(prefix))
	low := make([]float64, len(prefix))
	cl := make([]float64, len(prefix))
	for i, k := range prefix {
		high[i], low[i], cl[i] = k.High, k.Low, k.Close
	}
	series, err := indicators.ATRSeries(indicators.CanonicalATRSpec(), high, low, cl)
	if err != nil {
		return 0, false, err
	}
	v := series[len(series)-1]
	if !pathFinite(v) || v <= 0 {
		return 0, false, nil
	}
	return v, true, nil
}

func klineIndex(k15 []exchange.Kline, open int64) int {
	i := sort.Search(len(k15), func(i int) bool { return k15[i].OpenTime >= open })
	if i < len(k15) && k15[i].OpenTime == open {
		return i
	}
	return -1
}

// ResolveMinuteTouch orders one 15-minute both-touch from contiguous 1-minute bars.
// Levels are absolute prices. The Star path is not modified.
// A winning hit time is that minute's OpenTime.
func ResolveMinuteTouch(minutes []exchange.Kline, parentOpen int64, side string, favorable, adverse float64) (MinuteTouch, error) {
	if side != nodes.WozduhXoverSideUp && side != nodes.WozduhXoverSideDown {
		return MinuteTouch{}, fmt.Errorf("market: minute touch side %q", side)
	}
	end, err := data.NextBarOpen(parentOpen, "15m")
	if err != nil {
		return MinuteTouch{}, err
	}
	at := make(map[int64]exchange.Kline, len(minutes))
	for _, k := range minutes {
		at[k.OpenTime] = k
	}
	expected := parentOpen
	for expected < end {
		next, err := data.NextBarOpen(expected, "1m")
		if err != nil {
			return MinuteTouch{}, err
		}
		if next > end {
			return MinuteTouch{}, fmt.Errorf("market: minute %d crosses 15m seam %d", expected, end)
		}
		k, ok := at[expected]
		if !ok {
			if expected == parentOpen {
				return MinuteTouch{State: MinuteFinerMissing}, nil
			}
			return MinuteTouch{State: MinuteFinerGap}, nil
		}
		fav, adv := minuteTouches(k, side, favorable, adverse)
		if fav && adv {
			return MinuteTouch{State: MinuteFinerDual}, nil
		}
		if fav {
			return MinuteTouch{State: MinuteFavorable, HitOpen: k.OpenTime}, nil
		}
		if adv {
			return MinuteTouch{State: MinuteAdverse, HitOpen: k.OpenTime}, nil
		}
		expected = next
	}
	return MinuteTouch{State: MinuteFinerInconsistent}, nil
}

func minuteTouches(k exchange.Kline, side string, favorable, adverse float64) (fav, adv bool) {
	if side == nodes.WozduhXoverSideUp {
		return k.High >= favorable, k.Low <= adverse
	}
	return k.Low <= favorable, k.High >= adverse
}

func pathFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
