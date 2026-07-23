package server

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"time"

	"trading_bot/core"
	"trading_bot/exchange"
	"trading_bot/market"
)

const tipSSOTProbeEps = 1e-9

// TipSSOTProbeResult compares History GetWindow tip vs Live Frame tip (last closed OHLCV + RSX).
// Diagnostic only — no FE clamp, no boot-depth mutation.
type TipSSOTProbeResult struct {
	TF  string `json:"tf"`
	Now int64  `json:"nowMs"`

	HistoryBars int  `json:"historyBars"`
	FrameBars   int  `json:"frameBars"`
	HistForming bool `json:"historyHadFormingTip"`
	FrameForming bool `json:"frameHadFormingTip"`

	HistOpenTime  int64   `json:"histOpenTime"`
	FrameOpenTime int64   `json:"frameOpenTime"`
	HistO         float64 `json:"histOpen"`
	HistH         float64 `json:"histHigh"`
	HistL         float64 `json:"histLow"`
	HistC         float64 `json:"histClose"`
	HistV         float64 `json:"histVolume"`
	FrameO        float64 `json:"frameOpen"`
	FrameH        float64 `json:"frameHigh"`
	FrameL        float64 `json:"frameLow"`
	FrameC        float64 `json:"frameClose"`
	FrameV        float64 `json:"frameVolume"`

	OpenTimeMatch bool    `json:"openTimeMatch"`
	OHLCMatch     bool    `json:"ohlcMatch"`
	DeltaO        float64 `json:"deltaOpen"`
	DeltaH        float64 `json:"deltaHigh"`
	DeltaL        float64 `json:"deltaLow"`
	DeltaC        float64 `json:"deltaClose"`
	DeltaV        float64 `json:"deltaVolume"`

	HistReplayRSX  float64 `json:"histReplayRSX"`
	FrameReplayRSX float64 `json:"frameReplayRSX"`
	FrameLiveRSX   float64 `json:"frameLiveRSX"`
	ReplayRSXMatch bool    `json:"replayRSXMatch"`
	LiveVsHistRSX  float64 `json:"liveVsHistReplayRSX"`

	Verdict string `json:"verdict"`
	OK      bool   `json:"ok"`
	Reason  string `json:"reason,omitempty"`
}

// ProbeTipSSOT compares last-closed GetWindow vs Frame for one TF (Closed-bar Boundary SSOT).
// GetWindow applies CapKlineEndToLastClosed; EndTimeMs=Now is not a raw wall-clock tip.
func (d *DashboardServer) ProbeTipSSOT(ctx context.Context, tf string, candleLimit int) TipSSOTProbeResult {
	out := TipSSOTProbeResult{TF: tf, Now: time.Now().UnixMilli()}
	if d == nil {
		out.Reason = "nil dashboard"
		out.Verdict = "UNAVAILABLE"
		return out
	}
	spec, err := ResolveTimeframe(tf)
	if err != nil {
		out.Reason = err.Error()
		out.Verdict = "UNAVAILABLE"
		return out
	}
	if candleLimit <= 0 {
		candleLimit = defaultStateCandleLimit
	}
	if err := requestCtxErr(ctx); err != nil {
		out.Reason = err.Error()
		out.Verdict = "UNAVAILABLE"
		return out
	}

	win, ok := d.GetWindow(ctx, HistoryWindowQuery{
		Spec:        spec,
		EndTimeMs:   out.Now,
		CandleLimit: candleLimit,
	})
	if !ok || len(win.Klines) == 0 {
		out.Reason = "GetWindow empty"
		out.Verdict = "UNAVAILABLE"
		return out
	}
	frame := d.frameForSpec(spec)
	if frame == nil {
		out.Reason = "frame missing"
		out.Verdict = "UNAVAILABLE"
		return out
	}
	frameKlines := frame.GetKlines()
	if len(frameKlines) == 0 {
		out.Reason = "frame empty"
		out.Verdict = "UNAVAILABLE"
		return out
	}

	return compareTipSSOT(win.Klines, frameKlines, frame, market.GetRSXSettings(), out.Now, tf)
}

func (d *DashboardServer) frameForSpec(spec TimeframeSpec) *market.Frame {
	if d == nil {
		return nil
	}
	if frame, ok := d.frames[spec.ID]; ok && frame != nil {
		return frame
	}
	if spec.BinanceInterval != "" {
		if frame, ok := d.frames[spec.BinanceInterval]; ok && frame != nil {
			return frame
		}
	}
	return nil
}

// compareTipSSOT is the pure seam check (testable without HTTP).
func compareTipSSOT(
	histRaw, frameRaw []exchange.Kline,
	frame *market.Frame,
	rsx market.RSXSettings,
	nowMs int64,
	tf string,
) TipSSOTProbeResult {
	out := TipSSOTProbeResult{
		TF:          tf,
		Now:         nowMs,
		HistoryBars: len(histRaw),
		FrameBars:   len(frameRaw),
	}

	histClosed := dropFormingTip(histRaw, nowMs)
	frameClosed := dropFormingTip(frameRaw, nowMs)
	out.HistForming = len(histClosed) < len(histRaw)
	out.FrameForming = len(frameClosed) < len(frameRaw)
	if len(histClosed) == 0 || len(frameClosed) == 0 {
		out.Reason = "no closed tip after dropFormingTip"
		out.Verdict = "UNAVAILABLE"
		return out
	}

	h := histClosed[len(histClosed)-1]
	f := frameClosed[len(frameClosed)-1]
	out.HistOpenTime = exchange.EnsureUnixMillis(h.OpenTime)
	out.FrameOpenTime = exchange.EnsureUnixMillis(f.OpenTime)
	out.HistO, out.HistH, out.HistL, out.HistC, out.HistV = h.Open, h.High, h.Low, h.Close, h.Volume
	out.FrameO, out.FrameH, out.FrameL, out.FrameC, out.FrameV = f.Open, f.High, f.Low, f.Close, f.Volume
	out.OpenTimeMatch = out.HistOpenTime == out.FrameOpenTime
	out.DeltaO = h.Open - f.Open
	out.DeltaH = h.High - f.High
	out.DeltaL = h.Low - f.Low
	out.DeltaC = h.Close - f.Close
	out.DeltaV = h.Volume - f.Volume
	out.OHLCMatch = out.OpenTimeMatch &&
		math.Abs(out.DeltaO) <= tipSSOTProbeEps &&
		math.Abs(out.DeltaH) <= tipSSOTProbeEps &&
		math.Abs(out.DeltaL) <= tipSSOTProbeEps &&
		math.Abs(out.DeltaC) <= tipSSOTProbeEps &&
		math.Abs(out.DeltaV) <= tipSSOTProbeEps

	histBus := market.ReplayDAGKlines(histClosed, rsx)
	frameBus := market.ReplayDAGKlines(frameClosed, rsx)
	out.HistReplayRSX = tipRSXFromHist(histBus)
	out.FrameReplayRSX = tipRSXFromHist(frameBus)
	out.ReplayRSXMatch = floatEqualEps(out.HistReplayRSX, out.FrameReplayRSX, tipSSOTProbeEps)

	out.FrameLiveRSX = math.NaN()
	if frame != nil {
		if dag := frame.DAGTickFrame(); dag != nil {
			out.FrameLiveRSX = dag.Get(core.SlotJurikRSX)
		}
	}
	out.LiveVsHistRSX = math.Abs(out.FrameLiveRSX - out.HistReplayRSX)

	switch {
	case !out.OpenTimeMatch || !out.OHLCMatch:
		out.Verdict = "DATA_PLANE_OHLC_MISMATCH"
		out.OK = false
	case !out.ReplayRSXMatch:
		out.Verdict = "RSX_MISMATCH_ON_MATCHED_OHLC"
		out.OK = false
	default:
		out.Verdict = "DATA_PLANE_MATCH"
		out.OK = true
		if out.FrameForming && out.LiveVsHistRSX > 1e-2 {
			// Expected while live tip is forming — not a GetWindow vs Frame closed seam.
			out.Reason = "live Cur differs from closed replay (forming tip)"
		}
	}
	return out
}

func tipRSXFromHist(hist *core.HistoryBus) float64 {
	if hist == nil || hist.Count() < 1 {
		return math.NaN()
	}
	return hist.Get(core.SlotJurikRSX, 1)
}

func floatEqualEps(a, b, eps float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return math.Abs(a-b) <= eps
}

func (d *DashboardServer) logTipSSOTProbe(ctx context.Context, tf string, candleLimit int) {
	if !DebugTipSSOT() {
		return
	}
	res := d.ProbeTipSSOT(ctx, tf, candleLimit)
	log.Printf("[TipSSOT] tf=%s verdict=%s ohlc_match=%v open_time_match=%v histOT=%d frameOT=%d "+
		"ΔO=%.8g ΔH=%.8g ΔL=%.8g ΔC=%.8g ΔV=%.8g replayRSX_match=%v histRSX=%.8f frameReplayRSX=%.8f liveRSX=%.8f "+
		"histBars=%d frameBars=%d forming(h/f)=%v/%v %s",
		res.TF, res.Verdict, res.OHLCMatch, res.OpenTimeMatch, res.HistOpenTime, res.FrameOpenTime,
		res.DeltaO, res.DeltaH, res.DeltaL, res.DeltaC, res.DeltaV,
		res.ReplayRSXMatch, res.HistReplayRSX, res.FrameReplayRSX, res.FrameLiveRSX,
		res.HistoryBars, res.FrameBars, res.HistForming, res.FrameForming, res.Reason)
}

func (d *DashboardServer) handleDebugTipSSOT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !DebugTipSSOT() {
		http.Error(w, "TipSSOT probe disabled (set DEBUG_TIP_SSOT=1)", http.StatusNotFound)
		return
	}
	tf := r.URL.Query().Get("tf")
	if tf == "" {
		tf = d.tradingTimeframe
	}
	limit := parseCandleLimit(r, defaultStateCandleLimit, maxStateCandleLimit)
	res := d.ProbeTipSSOT(r.Context(), tf, limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
