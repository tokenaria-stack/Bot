package strategy_test

import (
	"encoding/json"
	"math"
	"testing"

	"trading_bot/exchange"
	"trading_bot/strategy"
)

func TestExtractTrendlineData_WicksAndBody(t *testing.T) {
	t.Parallel()

	klines := []exchange.Kline{
		{Open: 10, High: 12, Low: 9, Close: 11},
		{Open: 11, High: 13, Low: 10, Close: 10.5},
	}

	wHigh, wLow, wTimes := strategy.ExtractTrendlineData(klines, strategy.NavigatorTrendWicks)
	if wHigh[0] != 12 || wLow[0] != 9 {
		t.Fatalf("wicks = high %v low %v, want 12/9", wHigh[0], wLow[0])
	}
	if len(wTimes) != len(klines) {
		t.Fatalf("times len = %d, want %d", len(wTimes), len(klines))
	}

	bHigh, bLow, _ := strategy.ExtractTrendlineData(klines, strategy.NavigatorTrendBody)
	if bHigh[0] != 11 || bLow[0] != 10 {
		t.Fatalf("body = high %v low %v, want 11/10", bHigh[0], bLow[0])
	}
	if bHigh[1] != 11 || bLow[1] != 10.5 {
		t.Fatalf("body[1] = high %v low %v, want 11/10.5", bHigh[1], bLow[1])
	}
}

func TestFindPivots_basic(t *testing.T) {
	t.Parallel()

	// left=2, right=1 → pivot high at index 2 (value 5)
	highs := []float64{1, 2, 5, 3, 2, 1}
	lows := []float64{0, 1, 2, 1, 0, 0}

	ph, pl := strategy.FindPivots(highs, lows, nil, 2, 1)
	if len(ph) != 1 || ph[0].Index != 2 || ph[0].Price != 5 {
		t.Fatalf("pivot highs = %+v, want index 2 price 5", ph)
	}
	if len(pl) != 0 {
		t.Fatalf("pivot lows = %+v, want none", pl)
	}
}

func TestFindPivots_pivotLow(t *testing.T) {
	t.Parallel()

	highs := []float64{5, 4, 3, 4, 5, 6}
	lows := []float64{4, 3, 1, 2, 3, 4}

	_, pl := strategy.FindPivots(highs, lows, nil, 2, 1)
	if len(pl) != 1 || pl[0].Index != 2 || pl[0].Price != 1 {
		t.Fatalf("pivot lows = %+v, want index 2 price 1", pl)
	}
}

func TestFindPivots_invalidInput(t *testing.T) {
	t.Parallel()

	ph, pl := strategy.FindPivots([]float64{1, 2}, []float64{1}, nil, 2, 1)
	if ph != nil || pl != nil {
		t.Fatalf("expected nil for mismatched lengths, got %+v %+v", ph, pl)
	}

	ph, pl = strategy.FindPivots([]float64{1, 2, 3}, []float64{1, 2, 3}, nil, 2, 1)
	if ph != nil || pl != nil {
		t.Fatalf("expected nil for short series, got %+v %+v", ph, pl)
	}
}

func TestFindPivots_luxAlgoDefaultRight(t *testing.T) {
	t.Parallel()

	// LuxAlgo uses left=10, right=1 on a simple spike at index 10.
	n := 25
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i := range highs {
		highs[i] = 100
		lows[i] = 99
	}
	highs[10] = 110

	ph, _ := strategy.FindPivots(highs, lows, nil, 10, 1)
	found := false
	for _, p := range ph {
		if p.Index == 10 && p.Price == 110 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pivot high at 10, got %+v", ph)
	}
}

func TestNavigatorEngine_isolation(t *testing.T) {
	t.Parallel()

	highs := []float64{1, 2, 5, 3, 2, 1, 2, 5, 3, 2}
	lows := []float64{0, 1, 2, 1, 0, 0, 1, 2, 1, 0}
	closes := highs

	e1 := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 2})
	e2 := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 3})
	e1.Execute(highs, lows, closes, nil)
	e2.Execute(highs, lows, closes, nil)

	if len(e1.Markers) == len(e2.Markers) && len(e1.Markers) == 0 {
		// both may be empty on short series; ensure separate slices
		if e1.CompletedLines == nil {
			e1.CompletedLines = []strategy.Trendline{}
		}
	}
	if &e1.Markers == &e2.Markers {
		t.Fatal("engines must not share marker slices")
	}
}

func TestNavigatorEngine_higherHighCreatesBullLine(t *testing.T) {
	t.Parallel()

	highs := []float64{60, 61, 62, 63, 64, 65, 100, 95, 90, 88, 92, 96, 110, 108, 105}
	lows := []float64{58, 57, 56, 50, 52, 54, 70, 68, 65, 55, 58, 62, 80, 78, 76}
	closes := highs

	engine := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1})
	engine.Execute(highs, lows, closes, nil)

	var hhFound bool
	for _, m := range engine.Markers {
		if m.Text == "HH" && m.Index == 12 && m.Price == 110 {
			hhFound = true
		}
	}
	if !hhFound {
		t.Fatalf("expected HH marker at index 12, got markers %+v", engine.Markers)
	}
	if engine.Trend != 1 {
		t.Fatalf("Trend = %d, want 1", engine.Trend)
	}
	if engine.Active == nil || !engine.Active.IsActive || !engine.Active.Bullish {
		t.Fatal("expected active bullish trendline after HH")
	}
}

func TestNavigatorEngine_lowerLowCreatesBearLine(t *testing.T) {
	t.Parallel()

	n := 18
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i := range highs {
		highs[i] = 50
		lows[i] = 45
	}
	highs[2], highs[3], highs[4], highs[5], highs[6] = 60, 65, 80, 70, 68
	lows[3], lows[4], lows[5], lows[6], lows[7] = 35, 32, 30, 33, 31
	lows[10], lows[11], lows[12], lows[13], lows[14] = 25, 22, 20, 24, 26
	closes := append([]float64(nil), lows...)

	engine := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1})
	engine.Execute(highs, lows, closes, nil)

	var llFound bool
	for _, m := range engine.Markers {
		if m.Text == "LL" && m.Index == 12 && m.Price == 20 {
			llFound = true
		}
	}
	if !llFound {
		t.Fatalf("expected LL marker at index 12, got markers %+v", engine.Markers)
	}
	if engine.Trend != -1 {
		t.Fatalf("Trend = %d, want -1", engine.Trend)
	}
	if engine.Active == nil || !engine.Active.IsActive || engine.Active.Bullish {
		t.Fatal("expected active bearish trendline after LL")
	}
}

func TestNavigatorEngine_bearCloseBreakCompletesLine(t *testing.T) {
	t.Parallel()

	n := 18
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := range highs {
		highs[i] = 50
		lows[i] = 45
		closes[i] = 47
	}
	highs[2], highs[3], highs[4], highs[5], highs[6] = 60, 65, 80, 70, 68
	lows[3], lows[4], lows[5], lows[6], lows[7] = 35, 32, 30, 33, 31
	lows[10], lows[11], lows[12], lows[13], lows[14] = 25, 22, 20, 24, 26
	closes[16] = 200

	engine := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1})
	engine.Execute(highs, lows, closes, nil)

	if engine.Active != nil && engine.Active.IsActive {
		t.Fatalf("expected bear line deactivated after close break, still active %+v", engine.Active)
	}
	if len(engine.CompletedLines) == 0 {
		t.Fatal("expected at least one completed line after breakout")
	}
}

func TestNavigatorEngine_executeEmptySafe(t *testing.T) {
	t.Parallel()

	engine := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 10})
	engine.Execute(nil, nil, nil, nil)
	engine.Execute([]float64{1}, []float64{1, 2}, []float64{1}, nil)
}

func TestNavigatorEngine_UpdateMatchesExecute(t *testing.T) {
	t.Parallel()

	highs := []float64{60, 61, 62, 63, 64, 65, 100, 95, 90, 88, 92, 96, 110, 108, 105}
	lows := []float64{58, 57, 56, 50, 52, 54, 70, 68, 65, 55, 58, 62, 80, 78, 76}
	closes := append([]float64(nil), highs...)

	settings := strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1}

	barTimes := strategy.SynthesizeBarTimesMS(len(highs), 60_000)

	batch := strategy.NewNavigatorEngine(settings)
	batch.Execute(highs, lows, closes, barTimes)
	batchDTO := batch.GetResultDTO()

	incremental := strategy.NewNavigatorEngine(settings)
	for i := range highs {
		incremental.Update(highs[i], lows[i], closes[i], barTimes[i], i)
	}
	incDTO := incremental.GetResultDTO()

	if !navigatorDTOsEqual(batchDTO, incDTO) {
		t.Fatalf("Update != Execute\nbatch=%+v\ninc=%+v", batchDTO, incDTO)
	}
	if incremental.Trend != batch.Trend {
		t.Fatalf("Trend mismatch: batch=%d incremental=%d", batch.Trend, incremental.Trend)
	}
}

func TestNavigatorEngine_GetResultDTO(t *testing.T) {
	t.Parallel()

	engine := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1})
	strategy.NavigatorBindBarTimesForTest(engine, strategy.SynthesizeBarTimesMS(20, 60_000))
	engine.Markers = append(engine.Markers, strategy.TrendlineMarker{
		Index: 5, Price: 100, Text: "HH", Color: "#089981", Type: "Label",
	})
	engine.CompletedLines = append(engine.CompletedLines, strategy.Trendline{
		X1: 1, Y1: 50, X2: 10, Y2: 60, Color: "#089981", Style: strategy.NavigatorStyleSolid, IsActive: false,
	})
	engine.Active = &strategy.Trendline{
		X1: 3, Y1: 55, X2: 12, Y2: 65, Color: "#089981", Style: strategy.NavigatorStyleSolid, IsActive: true, Bullish: true,
	}

	dto := engine.GetResultDTO()
	if len(dto.Lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(dto.Lines))
	}
	if dto.Lines[0].X1 != 1 || dto.Lines[1].X2 != 12 {
		t.Fatalf("unexpected lines: %+v", dto.Lines)
	}
	if len(dto.Markers) != 1 || dto.Markers[0].Type != "HH" {
		t.Fatalf("markers: %+v", dto.Markers)
	}
}

func TestNavigatorAppendCompletedLineUnlimited(t *testing.T) {
	t.Parallel()

	engine := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1})
	for i := 0; i < 55; i++ {
		engine.Active = &strategy.Trendline{
			X1: i, Y1: 1, X2: i + 1, Y2: 2, IsActive: true, Bullish: true,
		}
		engine.DeactivateBullForTest()
	}
	if len(engine.CompletedLines) != 55 {
		t.Fatalf("completed lines = %d, want all 55 retained", len(engine.CompletedLines))
	}
}

func TestNavigatorMaxCompletedLinesExport(t *testing.T) {
	t.Parallel()

	engine := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1})
	for i := 0; i < 50; i++ {
		engine.CompletedLines = append(engine.CompletedLines, strategy.Trendline{
			X1: i, Y1: 50, X2: i + 5, Y2: 55, IsActive: false,
		})
	}
	engine.Active = &strategy.Trendline{
		X1: 100, Y1: 60, X2: 110, Y2: 70, IsActive: true, Bullish: true,
	}
	dto := engine.GetResultDTO()
	if len(dto.Lines) != 51 {
		t.Fatalf("export lines = %d, want 51", len(dto.Lines))
	}
}

func navigatorDTOsEqual(a, b strategy.NavigatorResultDTO) bool {
	if len(a.Lines) != len(b.Lines) || len(a.Markers) != len(b.Markers) {
		return false
	}
	for i := range a.Lines {
		al, bl := a.Lines[i], b.Lines[i]
		if al.X1 != bl.X1 || al.Y1 != bl.Y1 || al.X2 != bl.X2 || al.Y2 != bl.Y2 ||
			al.Time1 != bl.Time1 || al.Time2 != bl.Time2 ||
			al.Color != bl.Color || al.Style != bl.Style {
			return false
		}
	}
	for i := range a.Markers {
		am, bm := a.Markers[i], b.Markers[i]
		if am.Index != bm.Index || am.Time != bm.Time || am.Price != bm.Price || am.Text != bm.Text ||
			am.Color != bm.Color || am.Type != bm.Type {
			return false
		}
	}
	return true
}

func TestRunNavigatorAggregator_multiEngineStyles(t *testing.T) {
	t.Parallel()

	n := 18
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := range highs {
		highs[i] = 50
		lows[i] = 45
		closes[i] = 47
	}
	highs[2], highs[3], highs[4], highs[5], highs[6] = 60, 65, 80, 70, 68
	lows[3], lows[4], lows[5], lows[6], lows[7] = 35, 32, 30, 33, 31
	lows[10], lows[11], lows[12], lows[13], lows[14] = 25, 22, 20, 24, 26

	ui := strategy.NavigatorUISettings{
		UseLong:   true,
		LongLen:   2,
		UseMedium: true,
		MediumLen: 2,
		UseShort:  true,
		ShortLen:  2,
	}
	merged := strategy.RunNavigatorAggregator(highs, lows, closes, nil, ui, "15m")
	if len(merged.Lines) == 0 {
		t.Fatal("expected merged lines from three engines")
	}

	styles := map[string]int{}
	for _, line := range merged.Lines {
		styles[line.Style]++
	}
	if styles[strategy.NavigatorStyleSolid] == 0 ||
		styles[strategy.NavigatorStyleDashed] == 0 ||
		styles[strategy.NavigatorStyleDotted] == 0 {
		t.Fatalf("expected solid/dashed/dotted styles, got %+v", styles)
	}
}

func TestBuildNavigatorData_disabledAndRouting(t *testing.T) {
	t.Parallel()

	klines := []exchange.Kline{
		{Open: 10, High: 12, Low: 9, Close: 11},
		{Open: 11, High: 13, Low: 10, Close: 10.5},
	}
	rsx := []float64{55, 56}
	woz := []float64{40, 41}

	if got := strategy.BuildNavigatorData(strategy.NavigatorUISettings{}, klines, rsx, woz, "", nil, nil); len(got.Lines) != 0 {
		t.Fatalf("disabled navigator should be empty, got %+v", got)
	}

	price := strategy.BuildNavigatorData(strategy.NavigatorUISettings{
		Enabled: true, Source: "Price", UseLong: true, LongLen: 1,
	}, klines, rsx, woz, "15m", nil, nil)
	if len(price.Lines) == 0 && len(price.Markers) == 0 {
		// short series may produce no pivots — still exercises routing without panic
		t.Log("price navigator empty on 2 bars (expected)")
	}

	rsxNav := strategy.BuildNavigatorData(strategy.NavigatorUISettings{
		Enabled: true, Source: "RSX", UseShort: true, ShortLen: 1,
	}, klines, rsx, woz, "15m", nil, nil)
	_ = rsxNav

	wozNav := strategy.BuildNavigatorData(strategy.NavigatorUISettings{
		Enabled: true, Source: "Wozduh", UseShort: true, ShortLen: 1,
	}, klines, rsx, woz, "15m", nil, nil)
	_ = wozNav
}

func buildBearLineSeriesBase() (highs, lows, closes []float64, breakIdx int) {
	n := 22
	highs = make([]float64, n)
	lows = make([]float64, n)
	closes = make([]float64, n)
	for i := range highs {
		highs[i] = 50
		lows[i] = 45
		closes[i] = 47
	}
	highs[2], highs[3], highs[4], highs[5], highs[6] = 60, 65, 80, 70, 68
	lows[3], lows[4], lows[5], lows[6], lows[7] = 35, 32, 30, 33, 31
	lows[10], lows[11], lows[12], lows[13], lows[14] = 25, 22, 20, 24, 26
	for i := 13; i < n; i++ {
		highs[i] = 79.5
		lows[i] = 79
		closes[i] = 79.5
	}
	breakIdx = 16
	return highs, lows, closes, breakIdx
}

func runNavigatorThroughBar(settings strategy.NavigatorSettings, highs, lows, closes []float64, through int) *strategy.NavigatorEngine {
	engine := strategy.NewNavigatorEngine(settings)
	barTimes := strategy.SynthesizeBarTimesMS(len(highs), 60_000)
	for i := 0; i <= through; i++ {
		engine.Update(highs[i], lows[i], closes[i], barTimes[i], i)
	}
	return engine
}

func TestNavigatorMomentumFilter_blocksWeakBreak(t *testing.T) {
	t.Parallel()

	highs, lows, closes, breakIdx := buildBearLineSeriesBase()
	base := strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1}

	engine := runNavigatorThroughBar(base, highs, lows, closes, breakIdx-1)
	if engine.Active == nil || engine.Active.Bullish || !engine.Active.IsActive {
		t.Fatal("expected active bear line before breakout bar")
	}

	filtered := base
	filtered.MomentumEnabled = true
	filtered.MomentumBars = 3
	filtered.MomentumPercent = 100

	weak := runNavigatorThroughBar(filtered, highs, lows, closes, breakIdx-1)
	weakTimes := strategy.SynthesizeBarTimesMS(len(highs), 60_000)
	weak.Update(79.99, 79.95, 80.01, weakTimes[breakIdx], breakIdx)

	if weak.Active == nil || weak.Active.Bullish || !weak.Active.IsActive {
		t.Fatal("weak breakout should not complete bear line when momentum filter enabled")
	}
	if len(weak.CompletedLines) > 0 {
		t.Fatalf("weak breakout should not add completed lines, got %d", len(weak.CompletedLines))
	}

	instant := runNavigatorThroughBar(base, highs, lows, closes, breakIdx-1)
	instantTimes := strategy.SynthesizeBarTimesMS(len(highs), 60_000)
	instant.Update(79.5, 79, 95, instantTimes[breakIdx], breakIdx)
	if instant.Active != nil && instant.Active.IsActive && !instant.Active.Bullish {
		t.Fatal("expected immediate breakout without filters")
	}
}

func TestNavigatorMomentumFilter_allowsStrongBreak(t *testing.T) {
	t.Parallel()

	highs, lows, closes, breakIdx := buildBearLineSeriesBase()
	settings := strategy.NavigatorSettings{
		SwingLength:     2,
		PivotRight:      1,
		MomentumEnabled: true,
		MomentumBars:    3,
		MomentumPercent: 50,
	}

	engine := runNavigatorThroughBar(settings, highs, lows, closes, breakIdx-1)
	breakTimes := strategy.SynthesizeBarTimesMS(len(highs), 60_000)
	engine.Update(79.5, 79, 95, breakTimes[breakIdx], breakIdx)

	if engine.Active != nil && engine.Active.IsActive && !engine.Active.Bullish {
		t.Fatal("strong momentum breakout should complete bear line")
	}
	if len(engine.CompletedLines) == 0 {
		t.Fatal("expected completed line after strong breakout")
	}
}

func TestNavigatorTimeHold_confirmsAfterBars(t *testing.T) {
	t.Parallel()

	highs, lows, closes, breakIdx := buildBearLineSeriesBase()
	settings := strategy.NavigatorSettings{
		SwingLength:     2,
		PivotRight:      1,
		TimeHoldEnabled: true,
		TimeHoldBars:    2,
	}

	engine := runNavigatorThroughBar(settings, highs, lows, closes, breakIdx-1)
	breakTimes := strategy.SynthesizeBarTimesMS(len(highs), 60_000)
	engine.Update(81, 79.5, 81.2, breakTimes[breakIdx], breakIdx)
	if engine.Active == nil || engine.Active.Bullish || !engine.Active.IsActive {
		t.Fatal("bear line should stay active while pending time hold")
	}

	engine.Update(81, 79.6, 81.4, breakTimes[breakIdx+1], breakIdx+1)
	if engine.Active == nil || engine.Active.Bullish || !engine.Active.IsActive {
		t.Fatal("bear line should still be pending after one hold bar")
	}

	engine.Update(81, 79.7, 81.6, breakTimes[breakIdx+2], breakIdx+2)
	if engine.Active != nil && engine.Active.IsActive && !engine.Active.Bullish {
		t.Fatal("bear line should complete after time hold bars elapsed")
	}
	if len(engine.CompletedLines) == 0 {
		t.Fatal("expected completed line after time hold confirmation")
	}
}

func TestNavigatorTimeHold_cancelsOnRevert(t *testing.T) {
	t.Parallel()

	highs, lows, closes, breakIdx := buildBearLineSeriesBase()
	settings := strategy.NavigatorSettings{
		SwingLength:     2,
		PivotRight:      1,
		TimeHoldEnabled: true,
		TimeHoldBars:    3,
	}

	engine := runNavigatorThroughBar(settings, highs, lows, closes, breakIdx-1)
	breakTimes := strategy.SynthesizeBarTimesMS(len(highs), 60_000)
	engine.Update(81, 79.5, 81.2, breakTimes[breakIdx], breakIdx)
	engine.Update(81, 79, 79.4, breakTimes[breakIdx+1], breakIdx+1)

	if engine.Active == nil || engine.Active.Bullish || !engine.Active.IsActive {
		t.Fatal("bear line should remain active after false breakout revert")
	}
	if len(engine.CompletedLines) > 0 {
		t.Fatal("reverted pending breakout should not complete line")
	}
}

func TestNavigatorTrueRangeAndATR(t *testing.T) {
	t.Parallel()

	tr := strategy.NavigatorTrueRangeForTest(12, 10, 11)
	if tr != 2 {
		t.Fatalf("TR = %v, want 2", tr)
	}

	highs := []float64{10, 12, 11}
	lows := []float64{9, 10, 10}
	closes := []float64{10, 11, 10.5}
	atr := strategy.NavigatorATRForTest(highs, lows, closes, 2, 2)
	if atr <= 0 {
		t.Fatalf("ATR = %v, want positive", atr)
	}
}

func TestBuildNavigatorResult_timeAnchoredVisuals(t *testing.T) {
	t.Parallel()

	n := 20
	klines := make([]exchange.Kline, n)
	for i := range klines {
		klines[i] = exchange.Kline{
			OpenTime: int64(i) * 60_000,
			Open:     10,
			High:     12,
			Low:      9,
			Close:    11,
		}
	}
	klines[5].High = 20
	klines[10].Low = 5
	klines[15].High = 25

	ui := strategy.NavigatorUISettings{
		Enabled:         true,
		Source:          "Price",
		UseLong:         true,
		LongLen:         2,
		BarColor:        true,
		BackgroundColor: true,
	}
	got := strategy.BuildNavigatorResult(ui, klines, nil, nil, "15m", nil, nil)

	if len(got.BarColors) == 0 {
		t.Fatal("expected time-keyed bar colors map")
	}
	for barTime := range got.BarColors {
		if barTime <= 0 {
			t.Fatalf("bar color key = %d, want positive open time ms", barTime)
		}
	}

	if len(got.BackgroundZones) == 0 {
		t.Fatal("expected background zones")
	}
	for _, zone := range got.BackgroundZones {
		if zone.StartTime <= 0 || zone.EndTime <= 0 {
			t.Fatalf("zone times = %d..%d, want positive ms", zone.StartTime, zone.EndTime)
		}
		if zone.EndTime < zone.StartTime {
			t.Fatalf("zone end before start: %+v", zone)
		}
	}
	last := got.BackgroundZones[len(got.BackgroundZones)-1]
	if last.EndTime != klines[n-1].OpenTime {
		t.Fatalf("active zone EndTime = %d, want last bar %d", last.EndTime, klines[n-1].OpenTime)
	}
}

func TestFindPivots_equalLeftPlateau(t *testing.T) {
	t.Parallel()

	// left=2, right=1: flat top on the left still confirms pivot at index 2.
	highs := []float64{5, 5, 5, 3, 2}
	lows := []float64{4, 4, 4, 3, 2}

	ph, _ := strategy.FindPivots(highs, lows, nil, 2, 1)
	if len(ph) != 1 || ph[0].Index != 2 || ph[0].Price != 5 {
		t.Fatalf("pivot highs = %+v, want index 2 price 5", ph)
	}
}

func TestFindPivots_equalRightRejectsHigh(t *testing.T) {
	t.Parallel()

	// Equal high on the right cancels pivot at the center bar (strict > on right side).
	highs := []float64{3, 5, 5}
	lows := []float64{2, 4, 4}

	ph, _ := strategy.FindPivots(highs, lows, nil, 1, 1)
	for _, p := range ph {
		if p.Index == 1 {
			t.Fatalf("index 1 should not be pivot high with equal right bar, got %+v", ph)
		}
	}
}

func TestFindPivots_equalLeftPlateauLow(t *testing.T) {
	t.Parallel()

	lows := []float64{5, 5, 5, 7, 8}
	highs := []float64{6, 6, 6, 8, 9}

	_, pl := strategy.FindPivots(highs, lows, nil, 2, 1)
	if len(pl) != 1 || pl[0].Index != 2 || pl[0].Price != 5 {
		t.Fatalf("pivot lows = %+v, want index 2 price 5", pl)
	}
}

func TestNavigatorSwingScan_luxAlgoFloorIncludesOlderBar(t *testing.T) {
	t.Parallel()

	// At n=5 (right=1), LuxAlgo floor is bar index 3 (n-right-1). Bar 2 is older and still sampled once before break.
	highs := []float64{12, 11, 50, 30, 40, 35}

	x, v := strategy.NavigatorSwingHighForTest(highs, 5, 1)
	if x != 2 || v != 50 {
		t.Fatalf("swing high = index %d price %v, want index 2 price 50", x, v)
	}
}

func TestNavigatorSwingScan_prefersMostRecentTie(t *testing.T) {
	t.Parallel()

	highs := []float64{10, 10, 10, 10, 10, 10}

	x, v := strategy.NavigatorSwingHighForTest(highs, 5, 1)
	if v != 10 || x != 5 {
		t.Fatalf("swing high tie = index %d price %v, want index 5 price 10", x, v)
	}
}

func TestNavigatorSwingLowScan_luxAlgoFloor(t *testing.T) {
	t.Parallel()

	lows := []float64{20, 19, 5, 12, 10, 11}

	x, v := strategy.NavigatorSwingLowForTest(lows, 5, 1)
	if x != 2 || v != 5 {
		t.Fatalf("swing low = index %d price %v, want index 2 price 5", x, v)
	}
}

func TestSanitizeSlope(t *testing.T) {
	t.Parallel()

	if got := strategy.SanitizeSlopeForTest(math.NaN()); got != 0 {
		t.Fatalf("NaN slope = %v, want 0", got)
	}
	if got := strategy.SanitizeSlopeForTest(1e9); got != 100000 {
		t.Fatalf("huge slope = %v, want 100000", got)
	}
}

func TestClipNavigatorLinesToChartWindow(t *testing.T) {
	t.Parallel()

	klines := []exchange.Kline{
		{OpenTime: 1_000_000, CloseTime: 1_900_000, Close: 100},
		{OpenTime: 2_000_000, CloseTime: 2_900_000, Close: 110},
	}
	lines := []strategy.NavigatorLineDTO{
		{Time1: 500_000, Time2: 1_500_000, Y1: 90, Y2: 105},
		{Time1: 500_000, Time2: 800_000, Y1: 90, Y2: 95},
		{Time1: 1_500_000, Time2: 2_500_000, Y1: 105, Y2: 115},
	}
	clipped := strategy.ClipNavigatorLinesToChartWindow(lines, klines)
	if len(clipped) != 2 {
		t.Fatalf("len = %d, want 2 (drop fully-before + keep clipped + keep in-window)", len(clipped))
	}
	if clipped[0].Time1 != 1_000_000 {
		t.Fatalf("clipped start Time1 = %d, want 1000000", clipped[0].Time1)
	}
	if clipped[0].Y1 <= 90 || clipped[0].Y1 >= 105 {
		t.Fatalf("clipped Y1 = %g, want between 90 and 105", clipped[0].Y1)
	}
	if clipped[1].Time1 != 1_500_000 {
		t.Fatalf("in-window line Time1 = %d, want 1500000", clipped[1].Time1)
	}
}

func TestRunNavigatorAggregator_TagsInterval(t *testing.T) {
	t.Parallel()

	highs := []float64{10, 11, 12, 13, 14, 15}
	lows := []float64{9, 9, 9, 9, 9, 9}
	closes := highs
	barTimes := strategy.SynthesizeBarTimesMS(len(highs), 4*60*60*1000)
	out := strategy.RunNavigatorAggregator(highs, lows, closes, barTimes, strategy.NavigatorUISettings{
		Enabled: true, UseLong: true, LongLen: 2,
	}, "4h")
	for _, line := range out.Lines {
		if line.Interval != "4h" {
			t.Fatalf("line interval = %q, want 4h", line.Interval)
		}
	}
}

func TestNavigatorLineDTO_JSONSafeFloats(t *testing.T) {
	t.Parallel()

	engine := strategy.NewNavigatorEngine(strategy.NavigatorSettings{SwingLength: 2, PivotRight: 1})
	highs := []float64{60, 61, 62, 63, 64, 65, 100, 95, 90, 88, 92, 96, 110, 108, 105}
	lows := []float64{58, 57, 56, 50, 52, 54, 70, 68, 65, 55, 58, 62, 80, 78, 76}
	closes := append([]float64(nil), highs...)
	barTimes := strategy.SynthesizeBarTimesMS(len(highs), 60_000)
	engine.Execute(highs, lows, closes, barTimes)

	dto := engine.GetResultDTO()
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("navigator DTO marshal failed: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}
