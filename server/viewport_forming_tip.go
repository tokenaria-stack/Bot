package server

import (
	"time"

	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server/wire"
)

// projectViewportFormingTip seeds the live forming bar into a closed-only columnar
// viewport (TradingView Model 2 / ADR-010).
//
// Contract:
//   - History Replay stays Cap-closed only (never feed forming into ReplayDAGKlines).
//   - Viewport = ClosedHistoryProjection + OptionalCurrentProjection (Frame forming tip).
//   - Only on the live edge (closed tip == Cap last-closed); deep-history windows untouched.
//   - First WS tick should OVERWRITE the same open time (deltaSec=0), not APPEND.
func (d *DashboardServer) projectViewportFormingTip(
	resp *columnarHistoryResponse,
	timeframe, binanceInterval string,
) {
	if d == nil || d.projector == nil || resp == nil || len(resp.Times) == 0 {
		return
	}
	interval := binanceInterval
	if interval == "" {
		interval = timeframe
	}
	if interval == "" {
		return
	}

	lastSec := resp.Times[len(resp.Times)-1]
	capEnd := resolveClosedBarBoundary(0, interval)
	capSec := exchange.ChartTimeSec(capEnd)
	if lastSec != capSec {
		// Off live edge (scroll-left / historical end) — do not attach now-forming tip.
		return
	}

	frame := d.frameForTimeframe(timeframe)
	if frame == nil && binanceInterval != "" && binanceInterval != timeframe {
		frame = d.frameForTimeframe(binanceInterval)
	}
	if frame == nil {
		return
	}

	raw := frame.GetKlines()
	if len(raw) == 0 {
		return
	}
	nowMs := time.Now().UnixMilli()
	tip := exchange.NormalizeKline(raw[len(raw)-1])
	if !isFormingKline(tip, nowMs) {
		return
	}
	tipSec := exchange.ChartTimeSec(tip.OpenTime)
	if tipSec <= lastSec {
		return
	}

	resp.Times = append(resp.Times, tipSec)
	resp.Candles.Open = append(resp.Candles.Open, tip.Open)
	resp.Candles.High = append(resp.Candles.High, tip.High)
	resp.Candles.Low = append(resp.Candles.Low, tip.Low)
	resp.Candles.Close = append(resp.Candles.Close, tip.Close)
	resp.Candles.Volume = append(resp.Candles.Volume, tip.Volume)

	tickPlots := d.projector.BuildTickJSON(frame.DAGTickFrame())
	absent := wire.HistoryAbsent
	if resp.Plots == nil {
		resp.Plots = map[string][]float64{}
	}
	for id, col := range resp.Plots {
		val := absent
		if tickPlots != nil {
			if v, ok := tickPlots[id]; ok {
				val = v
			}
		}
		resp.Plots[id] = append(col, val)
	}
	resp.Added = len(resp.Times)
}

func isFormingKline(k exchange.Kline, nowMs int64) bool {
	return k.CloseTime > 0 && nowMs <= k.CloseTime
}

// frameFormingTip returns Frame's forming tip kline when present (tests / diagnostics).
func frameFormingTip(frame *market.Frame, nowMs int64) (exchange.Kline, bool) {
	if frame == nil {
		return exchange.Kline{}, false
	}
	raw := frame.GetKlines()
	if len(raw) == 0 {
		return exchange.Kline{}, false
	}
	tip := exchange.NormalizeKline(raw[len(raw)-1])
	if !isFormingKline(tip, nowMs) {
		return exchange.Kline{}, false
	}
	return tip, true
}
