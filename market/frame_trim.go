package market

import (
	"trading_bot/exchange"
)

// trimKlinesToCapLocked drops oldest bars when RAM exceeds LiveKlineRAMCap.
// Parallel DataBus series are slice-trimmed in lockstep — no full replay.
func (a *Frame) trimKlinesToCapLocked() {
	if a == nil || len(a.klines) <= a.ramBarCap() {
		return
	}
	drop := len(a.klines) - a.ramBarCap()
	a.trimMarkerRAMLocked(drop)
}

// trimMarkerRAMLocked removes the oldest drop bars from klines and all aligned series.
// Caller must hold frame.mu.
func (a *Frame) trimMarkerRAMLocked(drop int) {
	if a == nil || drop <= 0 {
		return
	}
	if drop >= len(a.klines) {
		drop = len(a.klines) - 1
		if drop <= 0 {
			return
		}
	}

	a.klines = a.klines[drop:]
	a.JurikLines = trimTailLocked(a.JurikLines, drop)
	a.WozduhRed = trimTailLocked(a.WozduhRed, drop)
	a.WozduhGreen = trimTailLocked(a.WozduhGreen, drop)
	a.closeLines = trimTailLocked(a.closeLines, drop)
	a.rsxPriceLines = trimTailLocked(a.rsxPriceLines, drop)

	a.trimAnnotationsAfterDropLocked(drop)
	a.invalidatestreamingSnapLocked()
	a.alignAllDataBusToKlinesLocked()
}

func trimTailLocked[T any](series []T, drop int) []T {
	if drop <= 0 || len(series) == 0 {
		return series
	}
	if drop >= len(series) {
		return series[:0]
	}
	return series[drop:]
}

func (a *Frame) trimAnnotationsAfterDropLocked(drop int) {
	if drop <= 0 || len(a.Annotations) == 0 || len(a.klines) == 0 {
		return
	}
	minTime := exchange.ChartTimeSec(a.klines[0].OpenTime)
	out := a.Annotations[:0]
	for _, ann := range a.Annotations {
		if ann.Time >= minTime {
			out = append(out, ann)
		}
	}
	a.Annotations = out
}

func (a *Frame) invalidatestreamingSnapLocked() {
	a.streamingSnap = streamingSnapshot{}
}

// clampDataBusToKlinesLocked is an alias for alignAllDataBusToKlinesLocked (pad + truncate).
func (a *Frame) clampDataBusToKlinesLocked() {
	a.alignAllDataBusToKlinesLocked()
}
