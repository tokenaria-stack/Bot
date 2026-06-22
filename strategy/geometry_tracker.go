package strategy

import "trading_bot/indicators"

const geometryVolSMAPeriod = 20

// GeometryState summarizes active trendlines, touches, and breakout signals.
type GeometryState struct {
	TriangleKind       string
	ResistanceTouches  int
	SupportTouches     int
	ResistanceBreakout bool
	SupportBreakout    bool
	IsBullishBreakout  bool // volume-confirmed break above resistance trendline
	IsBearishBreakout  bool // volume-confirmed break below support trendline
	BounceUp           bool // rejection bounce off support trendline
	BounceDown         bool // rejection bounce off resistance trendline
	BreakoutStrength   int
}

type geometryTracker struct {
	highSwings []indicators.Peak
	lowSwings  []indicators.Peak
	resistance indicators.Trendline
	support    indicators.Trendline
	hasResist  bool
	hasSupport bool
	volSma     *indicators.SMA
}

func newGeometryTracker() *geometryTracker {
	return &geometryTracker{
		volSma: indicators.NewSMA(geometryVolSMAPeriod),
	}
}

func (g *geometryTracker) reset() {
	g.highSwings = nil
	g.lowSwings = nil
	g.hasResist = false
	g.hasSupport = false
	g.volSma = indicators.NewSMA(geometryVolSMAPeriod)
}

func (g *geometryTracker) onSwingNode(barIndex int, node indicators.ZigZagNode) {
	peak := indicators.Peak{
		Index: barIndex,
		Value: node.Price,
	}
	if node.IsHigh {
		peak.Type = indicators.PeakHigh
		g.highSwings = appendSwingPeak(g.highSwings, peak, 2)
		if len(g.highSwings) >= 2 {
			g.resistance = indicators.NewTrendline(g.highSwings[len(g.highSwings)-2], g.highSwings[len(g.highSwings)-1], true)
			_, _ = g.resistance.Equation()
			g.hasResist = g.highSwings[len(g.highSwings)-2].Index != g.highSwings[len(g.highSwings)-1].Index
		}
		return
	}

	peak.Type = indicators.PeakLow
	g.lowSwings = appendSwingPeak(g.lowSwings, peak, 2)
	if len(g.lowSwings) >= 2 {
		g.support = indicators.NewTrendline(g.lowSwings[len(g.lowSwings)-2], g.lowSwings[len(g.lowSwings)-1], false)
		_, _ = g.support.Equation()
		g.hasSupport = g.lowSwings[len(g.lowSwings)-2].Index != g.lowSwings[len(g.lowSwings)-1].Index
	}
}

func (g *geometryTracker) updateBar(barIndex int, high, low, close, open, volume, atr float64) GeometryState {
	state := GeometryState{}
	avgVolume := g.volSma.Update(volume)

	if g.hasResist {
		g.resistance.UpdateTouches(barIndex, high, low, atr)
		state.ResistanceTouches = g.resistance.Touches
		if detectResistanceBounce(g.resistance, barIndex, high, low, close, open, atr) {
			state.BounceDown = true
		}
		if ok, strength := g.resistance.CheckBreakout(barIndex, close, open, volume, avgVolume, true); ok {
			state.ResistanceBreakout = true
			state.IsBullishBreakout = true
			state.BreakoutStrength = strength
		}
	}

	if g.hasSupport {
		g.support.UpdateTouches(barIndex, high, low, atr)
		state.SupportTouches = g.support.Touches
		if detectSupportBounce(g.support, barIndex, high, low, close, open, atr) {
			state.BounceUp = true
		}
		if ok, strength := g.support.CheckBreakout(barIndex, close, open, volume, avgVolume, false); ok {
			state.SupportBreakout = true
			state.IsBearishBreakout = true
			if strength > state.BreakoutStrength {
				state.BreakoutStrength = strength
			}
		}
	}

	if g.hasResist && g.hasSupport {
		state.TriangleKind = indicators.DetectTriangle(g.resistance, g.support)
	}

	return state
}

func detectSupportBounce(tl indicators.Trendline, barIndex int, _, low, close, open, atr float64) bool {
	if atr <= 0 {
		return false
	}
	lineVal := tl.ValueAt(barIndex)
	tolerance := 0.5 * atr
	touched := low <= lineVal+tolerance && low >= lineVal-tolerance
	return touched && close >= lineVal && close > open
}

func detectResistanceBounce(tl indicators.Trendline, barIndex int, high, _, close, open, atr float64) bool {
	if atr <= 0 {
		return false
	}
	lineVal := tl.ValueAt(barIndex)
	tolerance := 0.5 * atr
	touched := high >= lineVal-tolerance && high <= lineVal+tolerance
	return touched && close <= lineVal && close < open
}

func appendSwingPeak(peaks []indicators.Peak, peak indicators.Peak, maxKeep int) []indicators.Peak {
	peaks = append(peaks, peak)
	if len(peaks) > maxKeep {
		peaks = peaks[len(peaks)-maxKeep:]
	}
	return peaks
}
