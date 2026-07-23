package server

import (
	"time"

	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server/wire"
)

// viewportProjectionMode is the ADR-010 / B2.2 decision for projectViewportFormingTip.
type viewportProjectionMode string

const (
	viewportProjNone      viewportProjectionMode = "none"
	viewportProjAppend    viewportProjectionMode = "append"
	viewportProjOverwrite viewportProjectionMode = "overwrite"
)

// projectViewportFormingTip seeds or refreshes the live tip into a closed-only columnar
// viewport (TradingView Model 2 / ADR-010 / B2.2).
//
// Contract:
//   - History Replay stays Cap-closed only (never feed forming into ReplayDAGKlines).
//   - Viewport = ClosedHistoryProjection + OptionalCurrentProjection (Frame tip).
//   - Only on the live edge (closed tip == Cap last-closed); deep-history windows untouched.
//   - APPEND when frameOpen > historyLastOpen (new forming bar).
//   - OVERWRITE when frameOpen == historyLastOpen (same candle, live Cur vs Cap Replay).
//   - First WS tick with unchanged market must be idempotent with the projected tip.
func (d *DashboardServer) projectViewportFormingTip(
	resp *columnarHistoryResponse,
	timeframe, binanceInterval string,
) viewportProjectionMode {
	if d == nil || d.projector == nil || resp == nil || len(resp.Times) == 0 {
		return viewportProjNone
	}
	interval := binanceInterval
	if interval == "" {
		interval = timeframe
	}
	if interval == "" {
		return viewportProjNone
	}

	lastSec := resp.Times[len(resp.Times)-1]
	capEnd := resolveClosedBarBoundary(0, interval)
	capSec := exchange.ChartTimeSec(capEnd)
	if lastSec != capSec {
		// Off live edge (scroll-left / historical end) — do not attach now-forming tip.
		return viewportProjNone
	}

	frame := d.frameForTimeframe(timeframe)
	if frame == nil && binanceInterval != "" && binanceInterval != timeframe {
		frame = d.frameForTimeframe(binanceInterval)
	}
	if frame == nil {
		return viewportProjNone
	}

	raw := frame.GetKlines()
	if len(raw) == 0 {
		return viewportProjNone
	}
	nowMs := time.Now().UnixMilli()
	tip := exchange.NormalizeKline(raw[len(raw)-1])
	tipSec := exchange.ChartTimeSec(tip.OpenTime)
	forming := isFormingKline(tip, nowMs)
	tickPlots := d.projector.BuildTickJSON(frame.DAGTickFrame())

	switch {
	case tipSec > lastSec && forming:
		return d.appendFormingTip(resp, tip, tipSec, tickPlots)
	case tipSec == lastSec:
		// Same open as Cap tip: replace tip OHLC + plots with Frame live state (no new bar).
		return d.overwriteFormingTip(resp, tip, tickPlots)
	default:
		return viewportProjNone
	}
}

func (d *DashboardServer) appendFormingTip(
	resp *columnarHistoryResponse,
	tip exchange.Kline,
	tipSec int64,
	tickPlots map[string]float64,
) viewportProjectionMode {
	resp.Times = append(resp.Times, tipSec)
	resp.Candles.Open = append(resp.Candles.Open, tip.Open)
	resp.Candles.High = append(resp.Candles.High, tip.High)
	resp.Candles.Low = append(resp.Candles.Low, tip.Low)
	resp.Candles.Close = append(resp.Candles.Close, tip.Close)
	resp.Candles.Volume = append(resp.Candles.Volume, tip.Volume)
	applyTipPlotsAppend(resp, tickPlots)
	resp.Added = len(resp.Times)
	return viewportProjAppend
}

func (d *DashboardServer) overwriteFormingTip(
	resp *columnarHistoryResponse,
	tip exchange.Kline,
	tickPlots map[string]float64,
) viewportProjectionMode {
	i := len(resp.Times) - 1
	if i < 0 {
		return viewportProjNone
	}
	// Frame owns OHLC for this open — never synthesize / duplicate Close locally.
	resp.Candles.Open[i] = tip.Open
	resp.Candles.High[i] = tip.High
	resp.Candles.Low[i] = tip.Low
	resp.Candles.Close[i] = tip.Close
	resp.Candles.Volume[i] = tip.Volume
	applyTipPlotsOverwrite(resp, tickPlots)
	resp.Added = len(resp.Times)
	return viewportProjOverwrite
}

func applyTipPlotsAppend(resp *columnarHistoryResponse, tickPlots map[string]float64) {
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
}

func applyTipPlotsOverwrite(resp *columnarHistoryResponse, tickPlots map[string]float64) {
	if resp.Plots == nil || tickPlots == nil {
		return
	}
	absent := wire.HistoryAbsent
	for id, col := range resp.Plots {
		if len(col) == 0 {
			continue
		}
		val := absent
		if v, ok := tickPlots[id]; ok {
			val = v
		}
		col[len(col)-1] = val
		resp.Plots[id] = col
	}
	// Ensure Frame-only plot ids still land on the tip length.
	n := len(resp.Times)
	for id, v := range tickPlots {
		col := resp.Plots[id]
		if len(col) == n {
			col[n-1] = v
			resp.Plots[id] = col
			continue
		}
		if len(col) == 0 {
			filled := make([]float64, n)
			for i := range filled {
				filled[i] = absent
			}
			filled[n-1] = v
			resp.Plots[id] = filled
		}
	}
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