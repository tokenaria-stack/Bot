package market

import (
	"math"

	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

// ZZDivSwingSample is a confirmed ZigZag swing after BarIndex has been resolved
// to time and RSX. Divergence state after this point must not use BarIndex.
type ZZDivSwingSample struct {
	AnchorAt int64
	IsHigh   bool
	Price    float64
	RSX      float64
}

// ZZDivFactCollector emits rsx_zz_div on newly confirmed same-type swing pairs.
// Closed bars only. Independent of DAG score slots.
type ZZDivFactCollector struct {
	lastHigh          ZZDivSwingSample
	lastLow           ZZDivSwingSample
	hasHigh           bool
	hasLow            bool
	hasProcessed      bool
	lastProcessedAt   int64
	lastProcessedHigh bool
	classifyCalls     int
}

// ClassifyCalls is the number of four-geometry classifications (tests).
func (c *ZZDivFactCollector) ClassifyCalls() int {
	if c == nil {
		return 0
	}
	return c.classifyCalls
}

// ObserveClosed consumes the newest EventRing swing after a closed TickUpdate.
// BarIndex is used only to resolve OpenTime and RSX lookback, then discarded.
func (c *ZZDivFactCollector) ObserveClosed(bus *core.Bus, klines []exchange.Kline, confirmIdx int) (indicators.IndicatorFactEvent, bool) {
	if c == nil || bus == nil || bus.Events == nil || bus.Hist == nil {
		return indicators.IndicatorFactEvent{}, false
	}
	if confirmIdx < 0 || confirmIdx >= len(klines) {
		return indicators.IndicatorFactEvent{}, false
	}
	swings := bus.Events.GetLast(1)
	if len(swings) == 0 {
		return indicators.IndicatorFactEvent{}, false
	}
	latest := swings[0]
	if latest.BarIndex < 0 || latest.BarIndex >= len(klines) {
		return indicators.IndicatorFactEvent{}, false
	}
	anchorAt := klines[latest.BarIndex].OpenTime
	if c.hasProcessed && c.lastProcessedAt == anchorAt && c.lastProcessedHigh == latest.IsHigh {
		return indicators.IndicatorFactEvent{}, false
	}
	c.hasProcessed = true
	c.lastProcessedAt = anchorAt
	c.lastProcessedHigh = latest.IsHigh

	rsx := histRSXFromNewest(bus.Hist, confirmIdx, latest.BarIndex)
	if math.IsNaN(rsx) {
		return indicators.IndicatorFactEvent{}, false
	}
	sample := ZZDivSwingSample{
		AnchorAt: anchorAt,
		IsHigh:   latest.IsHigh,
		Price:    latest.Price,
		RSX:      rsx,
	}

	var prev ZZDivSwingSample
	var havePrev bool
	if sample.IsHigh {
		if c.hasHigh {
			prev, havePrev = c.lastHigh, true
		}
		c.lastHigh = sample
		c.hasHigh = true
	} else {
		if c.hasLow {
			prev, havePrev = c.lastLow, true
		}
		c.lastLow = sample
		c.hasLow = true
	}
	if !havePrev {
		return indicators.IndicatorFactEvent{}, false
	}

	c.classifyCalls++
	dir, pattern, ok := indicators.ClassifyZigZagDivergence(
		indicators.ZigZagSwingSample{IsHigh: prev.IsHigh, Price: prev.Price, RSX: prev.RSX},
		indicators.ZigZagSwingSample{IsHigh: sample.IsHigh, Price: sample.Price, RSX: sample.RSX},
	)
	if !ok {
		return indicators.IndicatorFactEvent{}, false
	}
	return indicators.ZigZagDivFact(
		dir,
		pattern,
		klines[confirmIdx].OpenTime,
		sample.AnchorAt,
		sample.RSX,
		sample.Price,
	)
}

// histRSXFromNewest reads Jurik RSX after Hist.Advance: Get(1) is the confirm bar.
func histRSXFromNewest(hist *core.HistoryBus, confirmIdx, swingBar int) float64 {
	if hist == nil || confirmIdx < swingBar {
		return math.NaN()
	}
	return hist.Get(core.SlotJurikRSX, confirmIdx-swingBar+1)
}
