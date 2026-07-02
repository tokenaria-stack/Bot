package strategy

import (
	"trading_bot/exchange"
)

// trimKlinesToCapLocked drops oldest bars when RAM exceeds LiveKlineRAMCap.
// Parallel DataBus series are slice-trimmed in lockstep — no full replay.
func (a *Marker) trimKlinesToCapLocked() {
	if a == nil || len(a.klines) <= LiveKlineRAMCap {
		return
	}
	drop := len(a.klines) - LiveKlineRAMCap
	a.trimMarkerRAMLocked(drop)
}

// trimMarkerRAMLocked removes the oldest drop bars from klines and all aligned series.
// Caller must hold analyst.mu.
func (a *Marker) trimMarkerRAMLocked(drop int) {
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
	a.chartExportPoints = trimTailLocked(a.chartExportPoints, drop)
	a.closeLines = trimTailLocked(a.closeLines, drop)
	a.rsxPriceLines = trimTailLocked(a.rsxPriceLines, drop)

	a.trimAnnotationsAfterDropLocked(drop)
	a.realignBarIndexCachesLocked(drop)
	a.invalidateLayer2SnapLocked()
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

func (a *Marker) trimAnnotationsAfterDropLocked(drop int) {
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

func (a *Marker) realignBarIndexCachesLocked(drop int) {
	if drop <= 0 {
		return
	}
	if a.cachedRSXMarkerBar >= 0 {
		a.cachedRSXMarkerBar -= drop
		if a.cachedRSXMarkerBar < 0 {
			a.cachedRSXMarkerBar = -1
			a.cachedRSXMarkerLabel = ""
		}
	}
}

func (a *Marker) invalidateLayer2SnapLocked() {
	a.layer2Snap = layer2StreamingSnapshot{}
}

// clampDataBusToKlinesLocked is an alias for alignAllDataBusToKlinesLocked (pad + truncate).
func (a *Marker) clampDataBusToKlinesLocked() {
	a.alignAllDataBusToKlinesLocked()
}
