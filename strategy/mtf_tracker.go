package strategy

import (
	"strings"
	"time"

	"trading_bot/exchange"
)

// MinHTFPrefetchBars is the minimum number of closed HTF bars loaded before walk-forward MTF.
const MinHTFPrefetchBars = 100

// HTFState holds walk-forward higher-TF navigator output at a simulation tick.
type HTFState struct {
	Interval      string
	TrendLines    []NavigatorLineDTO
	Markers       []NavigatorMarkerDTO
	LastUpdateSec int64
	CandleCount   int
	RSXValue      float64
	RSXColor      string // "green" / "red" / "neutral"
	WozduhUp      float64 // HTF wt11 (RsiVolFast)
	WozduhDown    float64 // HTF wt22 (RsiVolSlow)
}

// WalkForwardMTFTracker advances HTF navigator state only when an HTF bar closes.
type WalkForwardMTFTracker struct {
	provider      *exchange.HTFProvider
	symbol        string
	chartInterval string
	navigatorUI   NavigatorUISettings
	activeTFs     []string
	states        map[string]*HTFState
	nextUpdateSec map[string]int64
	chartStartMs  int64
}

// NewWalkForwardMTFTracker creates a tracker for the given HTF periods (price navigator settings).
func NewWalkForwardMTFTracker(provider *exchange.HTFProvider, symbol, chartInterval string, navigatorUI NavigatorUISettings, tfs []string) *WalkForwardMTFTracker {
	return &WalkForwardMTFTracker{
		provider:      provider,
		symbol:        symbol,
		chartInterval: chartInterval,
		navigatorUI:   normalizeNavigatorUISettings(navigatorUI),
		activeTFs:     append([]string(nil), tfs...),
		states:        make(map[string]*HTFState),
		nextUpdateSec: make(map[string]int64),
	}
}

// CollectWalkForwardMTFPeriods returns enabled HTF periods from the price navigator config.
func CollectWalkForwardMTFPeriods(navigators map[string]NavigatorUISettings, chartInterval string) []string {
	ui, ok := navigators["price"]
	if !ok || !ui.Enabled || len(ui.Periods) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ui.Periods))
	var out []string
	for _, period := range ui.Periods {
		period = strings.TrimSpace(period)
		if period == "" || navigatorIntervalsEqual(period, chartInterval) {
			continue
		}
		if _, dup := seen[period]; dup {
			continue
		}
		seen[period] = struct{}{}
		out = append(out, period)
	}
	return out
}

// SetChartStartMs sets the earliest chart open time for HTF prefetch.
func (t *WalkForwardMTFTracker) SetChartStartMs(startMs int64) {
	if t == nil {
		return
	}
	t.chartStartMs = startMs
}

// Prefetch loads and pins HTF caches before the simulation loop.
func (t *WalkForwardMTFTracker) Prefetch() {
	if t == nil || t.provider == nil || t.symbol == "" {
		return
	}
	for _, tf := range t.activeTFs {
		startMs := htfPrefetchStartMs(t.chartStartMs, tf)
		klines, err := t.provider.GetKlines(t.symbol, tf, startMs)
		if err != nil || len(klines) == 0 {
			continue
		}
		t.provider.PinKlines(t.symbol, tf)
	}
}

func htfPrefetchStartMs(chartStartMs int64, interval string) int64 {
	intervalSec := exchange.ParseIntervalToSeconds(interval)
	if intervalSec <= 0 {
		return chartStartMs
	}
	intervalMs := intervalSec * 1000
	minDepthMs := int64(MinHTFPrefetchBars) * intervalMs
	nowMs := time.Now().UnixMilli()
	earliest := nowMs - minDepthMs
	if chartStartMs <= 0 || chartStartMs > earliest {
		return earliest
	}
	return chartStartMs
}

// Update recalculates HTF navigator layers when an HTF close boundary is crossed.
func (t *WalkForwardMTFTracker) Update(currentTickSec int64, chartKlines []exchange.Kline) {
	if t == nil || t.provider == nil || len(t.activeTFs) == 0 || currentTickSec <= 0 {
		return
	}

	for _, tf := range t.activeTFs {
		nextUpdate := t.nextUpdateSec[tf]
		if nextUpdate > 0 && currentTickSec < nextUpdate {
			continue
		}

		candles := t.provider.GetCandlesStrictlyBefore(t.symbol, tf, currentTickSec)
		if len(candles) == 0 {
			t.nextUpdateSec[tf] = currentTickSec + 60
			continue
		}

		layer := BuildHTFNavigatorLayer(t.navigatorUI, candles, tf, chartKlines)
		rsx, rsxColor, wozUp, wozDown := evaluateHTFOscillators(candles)
		t.states[tf] = &HTFState{
			Interval:      tf,
			TrendLines:    append([]NavigatorLineDTO(nil), layer.Lines...),
			Markers:       append([]NavigatorMarkerDTO(nil), layer.Markers...),
			LastUpdateSec: currentTickSec,
			CandleCount:   len(candles),
			RSXValue:      rsx,
			RSXColor:      rsxColor,
			WozduhUp:      wozUp,
			WozduhDown:    wozDown,
		}

		intervalSec := exchange.ParseIntervalToSeconds(tf)
		lastCloseSec := htfLastClosedBarSec(candles[len(candles)-1], intervalSec)
		if intervalSec > 0 {
			t.nextUpdateSec[tf] = lastCloseSec + intervalSec
		} else {
			t.nextUpdateSec[tf] = currentTickSec + 60
		}
	}
}

func htfLastClosedBarSec(last exchange.Kline, intervalSec int64) int64 {
	if last.CloseTime > 0 {
		return last.CloseTime / 1000
	}
	if intervalSec > 0 {
		return last.OpenTime/1000 + intervalSec
	}
	return last.OpenTime / 1000
}

// GetState returns the latest walk-forward state for one HTF period.
func (t *WalkForwardMTFTracker) GetState(tf string) *HTFState {
	if t == nil {
		return nil
	}
	return t.states[tf]
}

// States returns the cached walk-forward HTF state map (read-only).
// The map is replaced only on HTF close boundaries inside Update; intra-bar ticks reuse the same pointer.
func (t *WalkForwardMTFTracker) States() map[string]*HTFState {
	if t == nil || len(t.states) == 0 {
		return nil
	}
	return t.states
}

// evaluateHTFOscillators runs an isolated FalconEngine over strictly-closed HTF candles.
func evaluateHTFOscillators(candles []exchange.Kline) (rsx float64, rsxColor string, wozUp, wozDown float64) {
	if len(candles) == 0 {
		return 0, "neutral", 0, 0
	}
	falcon := NewFalconEngine()
	var prevRSX float64
	var last FalconSignals
	for i, c := range candles {
		last = falcon.Evaluate(c.High, c.Low, c.Close, c.Volume)
		if i == len(candles)-1 {
			rsx = last.JurikRSX
			rsxColor = rsxColorLabel(RSXColor(last.JurikRSX, prevRSX))
			wozUp = last.RsiVolFast
			wozDown = last.RsiVolSlow
		}
		prevRSX = last.JurikRSX
	}
	return rsx, rsxColor, wozUp, wozDown
}

func rsxColorLabel(color string) string {
	switch color {
	case RSXColorGreen:
		return "green"
	case RSXColorRed:
		return "red"
	default:
		return "neutral"
	}
}
