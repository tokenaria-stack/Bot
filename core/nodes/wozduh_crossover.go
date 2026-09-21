package nodes

import (
	"math"

	"trading_bot/core"
)

// Presentation-only Wozduh crossover identities (WOZDUH-CROSSOVER-PAINT-1).
// Not slots, facts, FeatureSpec features, tape columns, or IndicatorFactEvent.
const (
	WozduhXoverVolEma12XEma5     = "woz_vol_rsi_ema12_x_ema5"
	WozduhXoverVwemaXEma5        = "woz_rsi_hl2_vwema_x_ema5"
	WozduhXoverVwemaXEma12       = "woz_rsi_hl2_vwema_x_ema12"
	WozduhXoverVwemaXEma5ChanMid = "woz_rsi_hl2_vwema_x_ema5_chan_mid"
)

const (
	WozduhXoverSideUp   = "up"
	WozduhXoverSideDown = "down"
)

// WozduhCrossoverPair is one approved A×B pair. Dot Y is always B.
type WozduhCrossoverPair struct {
	ID    string
	PlotA string
	PlotB string
	SlotA core.Slot
	SlotB core.Slot
}

// WozduhCrossoverPairs is the closed set of four approved pairs.
func WozduhCrossoverPairs() []WozduhCrossoverPair {
	return []WozduhCrossoverPair{
		{
			ID:    WozduhXoverVolEma12XEma5,
			PlotA: "woz_vol_rsi_ema12",
			PlotB: "woz_vol_rsi_ema5",
			SlotA: core.SlotWozduhVolRsiEma12,
			SlotB: core.SlotWozduhVolRsiEma5,
		},
		{
			ID:    WozduhXoverVwemaXEma5,
			PlotA: "woz_rsi_hl2_vwema",
			PlotB: "woz_vol_rsi_ema5",
			SlotA: core.SlotWozduhRsiHl2Vwema,
			SlotB: core.SlotWozduhVolRsiEma5,
		},
		{
			ID:    WozduhXoverVwemaXEma12,
			PlotA: "woz_rsi_hl2_vwema",
			PlotB: "woz_vol_rsi_ema12",
			SlotA: core.SlotWozduhRsiHl2Vwema,
			SlotB: core.SlotWozduhVolRsiEma12,
		},
		{
			ID:    WozduhXoverVwemaXEma5ChanMid,
			PlotA: "woz_rsi_hl2_vwema",
			PlotB: "woz_vol_rsi_ema5_chan_mid",
			SlotA: core.SlotWozduhRsiHl2Vwema,
			SlotB: core.SlotWozduhVolRsiEma5ChanMid,
		},
	}
}

// WozduhCrossoverHit is one closed-bar edge for one pair (time stamped by the projector).
type WozduhCrossoverHit struct {
	Pair string
	Side string
	Y    float64
}

// WozduhCrossoverEvent is the sparse presentation wire row.
type WozduhCrossoverEvent struct {
	Pair string  `json:"pair"`
	Side string  `json:"side"`
	Time int64   `json:"time"`
	Y    float64 `json:"y"`
}

// DetectCrossoverEdge is the Pine-style closed-bar edge trigger, including equality.
// UP: prevA <= prevB AND currA > currB. DOWN: prevA >= prevB AND currA < currB.
// Non-finite inputs yield no edge.
func DetectCrossoverEdge(prevA, prevB, currA, currB float64) string {
	if !wozduhFinite(prevA) || !wozduhFinite(prevB) || !wozduhFinite(currA) || !wozduhFinite(currB) {
		return ""
	}
	if prevA <= prevB && currA > currB {
		return WozduhXoverSideUp
	}
	if prevA >= prevB && currA < currB {
		return WozduhXoverSideDown
	}
	return ""
}

func wozduhFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func wozduhPresent(v, absent float64) bool {
	if !wozduhFinite(v) {
		return false
	}
	if absent != 0 && v == absent {
		return false
	}
	return true
}

// ScanWozduhCrossovers walks packed closed-bar plot columns. Forming tips must not be included.
func ScanWozduhCrossovers(plots map[string][]float64, times []int64, absent float64) []WozduhCrossoverEvent {
	if len(times) == 0 || plots == nil {
		return nil
	}
	pairs := WozduhCrossoverPairs()
	var out []WozduhCrossoverEvent
	for _, pair := range pairs {
		colA := plots[pair.PlotA]
		colB := plots[pair.PlotB]
		if len(colA) == 0 || len(colB) == 0 {
			continue
		}
		n := len(times)
		if len(colA) < n {
			n = len(colA)
		}
		if len(colB) < n {
			n = len(colB)
		}
		ready := false
		var prevA, prevB float64
		for i := 0; i < n; i++ {
			currA := colA[i]
			currB := colB[i]
			if !wozduhPresent(currA, absent) || !wozduhPresent(currB, absent) {
				ready = false
				continue
			}
			if ready {
				if side := DetectCrossoverEdge(prevA, prevB, currA, currB); side != "" {
					out = append(out, WozduhCrossoverEvent{
						Pair: pair.ID,
						Side: side,
						Time: times[i],
						Y:    currB,
					})
				}
			}
			prevA, prevB = currA, currB
			ready = true
		}
	}
	return out
}

func detectWozduhCrossoversFromFrame(cur *core.TickFrame, prevA, prevB *[4]float64, ready *[4]bool) []WozduhCrossoverHit {
	if cur == nil || prevA == nil || prevB == nil || ready == nil {
		return nil
	}
	pairs := WozduhCrossoverPairs()
	var hits []WozduhCrossoverHit
	for i, pair := range pairs {
		currA := cur.Get(pair.SlotA)
		currB := cur.Get(pair.SlotB)
		if !wozduhFinite(currA) || !wozduhFinite(currB) {
			ready[i] = false
			continue
		}
		if ready[i] {
			if side := DetectCrossoverEdge(prevA[i], prevB[i], currA, currB); side != "" {
				hits = append(hits, WozduhCrossoverHit{Pair: pair.ID, Side: side, Y: currB})
			}
		}
		prevA[i] = currA
		prevB[i] = currB
		ready[i] = true
	}
	return hits
}

func StampWozduhCrossoverHits(hits []WozduhCrossoverHit, timeSec int64) []WozduhCrossoverEvent {
	if len(hits) == 0 {
		return nil
	}
	out := make([]WozduhCrossoverEvent, 0, len(hits))
	for _, h := range hits {
		out = append(out, WozduhCrossoverEvent{
			Pair: h.Pair,
			Side: h.Side,
			Time: timeSec,
			Y:    h.Y,
		})
	}
	return out
}
