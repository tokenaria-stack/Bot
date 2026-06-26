package strategy

import (
	"log"
	"math"
	"strings"

	"trading_bot/exchange"
)

const (
	NavigatorTrendWicks = "Wicks"
	NavigatorTrendBody  = "Body"

	NavigatorStyleSolid  = "solid"
	NavigatorStyleDashed = "dashed"
	NavigatorStyleDotted = "dotted"

	navigatorDefaultPivotRight   = 1
	navigatorDefaultMaxLineBars  = 5000
	navigatorDefaultMinHHSpacing = 5

	navigatorColorBull     = "#089981"
	navigatorColorBear     = "#f23645"
	navigatorColorWickBull = "#085def"
	navigatorColorWickBear = "#ff5d00"
)

// ChartPoint is a bar index, Unix open time (ms), and price anchor (Pine chart.point).
type ChartPoint struct {
	Index int
	Time  int64
	Price float64
}

// Valid reports whether the chart point has been initialized.
func (p ChartPoint) Valid() bool {
	return p.Index >= 0
}

// Trendline holds LuxAlgo navigator line state (Pine type bin, without drawing objects).
// Internal geometry uses bar indices (X1/X2) and per-bar Slope; Time1/Time2 are export-only.
type Trendline struct {
	X1, X2 int
	Y1, Y2 float64
	Slope  float64 // price change per bar index (Pine bn.slope)
	IsActive     bool
	Bullish      bool // true = bull/support TL (from prev PL), false = bear/resistance TL
	ControlPoint ChartPoint
	Style        string
	Color        string
}

// NavigatorSettings configures one navigator layer (Long, Medium, or Short).
type NavigatorSettings struct {
	SwingLength     int    // pivot left bars (l1/l2/l3)
	PivotRight      int    // pivot right bars (LuxAlgo default 1)
	TrendType       string // Wicks | Body
	Style           string
	BullColor       string
	BearColor       string
	WickBullColor   string
	WickBearColor   string
	MaxLineBars       int
	MinPivotSpacing   int // min bars between HH/LL vs previous same-type pivot

	MomentumEnabled bool
	MomentumBars    int
	MomentumPercent float64
	TimeHoldEnabled bool
	TimeHoldBars    int
}

// TrendlineMarker is an HH/LL label or wick-break dot.
type TrendlineMarker struct {
	Index int
	Time  int64
	Price float64
	Text  string // HH, LL, ●
	Color string
	Type  string // Label, WickBreak
}

// NavigatorEngine is an isolated LuxAlgo draw() state machine instance.
type NavigatorEngine struct {
	Settings NavigatorSettings

	Trend      int // 1 bullish (HH), -1 bearish (LL), 0 none
	PrevPH     ChartPoint
	PrevPL     ChartPoint
	Active     *Trendline // single LuxAlgo bin — one active TL per engine layer

	CompletedLines []Trendline
	Markers        []TrendlineMarker

	PendingBreakout *Trendline
	PendingType     string // Bear | Bull
	HoldCounter     int

	// Live incremental buffers (last left+right+1 bars for pivot window).
	HighBuf  []float64
	LowBuf   []float64
	CloseBuf []float64

	histHigh  []float64
	histLow   []float64
	histClose []float64
	histTimes []int64

	TrendHistory []int
}

// NavigatorLineDTO is a trendline segment for chart overlay JSON.
type NavigatorLineDTO struct {
	X1       int     `json:"x1"`
	Y1       float64 `json:"y1"`
	X2       int     `json:"x2"`
	Y2       float64 `json:"y2"`
	Time1    int64   `json:"time1"`
	Time2    int64   `json:"time2"`
	Interval string  `json:"interval,omitempty"` // chart or HTF period (e.g. 15m, 4h, 1d)
	Color    string  `json:"color"`
	Style    string  `json:"style"`
	IsActive bool    `json:"isActive,omitempty"`
	Slope    float64 `json:"slope,omitempty"`
}

// NavigatorMarkerDTO is an HH/LL label or wick-break dot for chart overlay JSON.
type NavigatorMarkerDTO struct {
	Index int     `json:"index"`
	Time  int64   `json:"time"`
	Price float64 `json:"price"`
	Text  string  `json:"text"`
	Color string  `json:"color"`
	Type  string  `json:"type"`
}

// NavigatorZoneDTO is a vertical background band between bar open times (inclusive, Unix ms).
type NavigatorZoneDTO struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Color     string `json:"color"`
}

// NavigatorResultDTO aggregates navigator lines and markers for API transport.
type NavigatorResultDTO struct {
	Lines           []NavigatorLineDTO   `json:"lines"`
	Markers         []NavigatorMarkerDTO `json:"markers"`
	BarColors       map[int64]string     `json:"barColors,omitempty"`
	BackgroundZones []NavigatorZoneDTO   `json:"backgroundZones,omitempty"`
}

// NavigatorUISettings controls multi-engine navigator runs from API/backtest UI.
// JSON keys must match web/app.js navigatorSettingsToAPI() / getNavigatorSettingsFromUI().
type NavigatorUISettings struct {
	Enabled   bool   `json:"enabled"`
	Source    string `json:"source"`    // Price, RSX, Wozduh
	TrendType string `json:"trendType"` // Wicks, Body
	Term      string `json:"term"`      // Long, Medium, Short — trend source for bar/background
	Periods   []string `json:"periods"` // MTF sync targets (e.g. 4h, 1d); empty = chart TF only
	UseLong   bool   `json:"useLong"`
	LongLen   int    `json:"longLen"`
	UseMedium bool   `json:"useMedium"`
	MediumLen int    `json:"mediumLen"`
	UseShort  bool   `json:"useShort"`
	ShortLen  int    `json:"shortLen"`

	MomentumEnabled bool    `json:"momentumEnabled"`
	MomentumBars    int     `json:"momentumBars"`
	MomentumPercent float64 `json:"momentumPercent"`
	TimeHoldEnabled bool    `json:"timeHoldEnabled"`
	TimeHoldBars    int     `json:"timeHoldBars"`

	BarColor        bool `json:"barColor"`
	BackgroundColor bool `json:"backgroundColor"`
}

const (
	navigatorSourcePrice  = "Price"
	navigatorSourceRSX    = "RSX"
	navigatorSourceWozduh = "Wozduh"

	navigatorDefaultLongLen   = 60
	navigatorDefaultMediumLen = 30
	navigatorDefaultShortLen  = 10

	navigatorPendingBear = "Bear"
	navigatorPendingBull = "Bull"

	navigatorDefaultMomentumBars    = 14
	navigatorDefaultMomentumPercent = 100.0

	navigatorMaxSlope = 100000.0
)

// RunNavigatorAggregator runs up to three navigator engines (Long/Medium/Short) and merges DTO output.
func RunNavigatorAggregator(highs, lows, closes []float64, barTimes []int64, ui NavigatorUISettings, interval string) NavigatorResultDTO {
	n := len(highs)
	if n == 0 || len(lows) != n || len(closes) != n {
		return NavigatorResultDTO{}
	}
	if len(barTimes) != n {
		barTimes = SynthesizeBarTimesMS(n, 60_000)
	}

	trendType := ui.TrendType
	if trendType == "" {
		trendType = NavigatorTrendWicks
	}

	out := NavigatorResultDTO{}
	runLayer := func(swingLen int, style string) {
		if swingLen < 1 {
			return
		}
		engine := NewNavigatorEngine(navigatorEngineSettings(ui, swingLen, style, trendType))
		engine.Execute(highs, lows, closes, barTimes)
		layer := engine.GetResultDTO()
		out.Lines = append(out.Lines, layer.Lines...)
		out.Markers = append(out.Markers, layer.Markers...)
	}

	if ui.UseLong {
		longLen := ui.LongLen
		if longLen <= 0 {
			longLen = navigatorDefaultLongLen
		}
		runLayer(longLen, NavigatorStyleSolid)
	}
	if ui.UseMedium {
		mediumLen := ui.MediumLen
		if mediumLen <= 0 {
			mediumLen = navigatorDefaultMediumLen
		}
		runLayer(mediumLen, NavigatorStyleDashed)
	}
	if ui.UseShort {
		shortLen := ui.ShortLen
		if shortLen <= 0 {
			shortLen = navigatorDefaultShortLen
		}
		runLayer(shortLen, NavigatorStyleDotted)
	}
	return tagNavigatorResultInterval(out, interval)
}

func tagNavigatorResultInterval(dto NavigatorResultDTO, interval string) NavigatorResultDTO {
	interval = normalizeNavigatorInterval(interval)
	if interval == "" {
		return dto
	}
	for i := range dto.Lines {
		dto.Lines[i].Interval = interval
	}
	return dto
}

// BuildNavigatorData routes source series and runs the multi-engine aggregator when enabled.
// htfData holds preloaded higher-TF klines keyed by period (strictly closed before chart end time).
func BuildNavigatorData(ui NavigatorUISettings, klines []exchange.Kline, rsxValues, wozduhValues []float64, interval string, htf *exchange.HTFProvider, htfData map[string][]exchange.Kline) NavigatorResultDTO {
	_ = htf
	ui = normalizeNavigatorUISettings(ui)
	if !ui.Enabled {
		return NavigatorResultDTO{}
	}

	var highs, lows, closes []float64
	switch normalizeNavigatorSource(ui.Source) {
	case navigatorSourceRSX:
		highs, lows, closes = rsxValues, rsxValues, rsxValues
	case navigatorSourceWozduh:
		highs, lows, closes = wozduhValues, wozduhValues, wozduhValues
	default:
		trendType := ui.TrendType
		if trendType == "" {
			trendType = NavigatorTrendWicks
		}
		highs, lows, _ = ExtractTrendlineData(klines, trendType)
		closes = ExtractCloses(klines)
	}
	barTimes := ExtractBarTimes(klines)
	out := RunNavigatorAggregator(highs, lows, closes, barTimes, ui, interval)
	return mergeHTFNavigatorLayers(out, ui, interval, klines, htfData)
}

// BuildHTFNavigatorLayer computes navigator lines for one HTF kline slice (walk-forward safe).
// chartKlines clips overlay geometry to the current chart window.
func BuildHTFNavigatorLayer(ui NavigatorUISettings, htfKlines []exchange.Kline, period string, chartKlines []exchange.Kline) NavigatorResultDTO {
	if len(htfKlines) < 3 {
		return NavigatorResultDTO{}
	}
	ui = normalizeNavigatorUISettings(ui)
	switch normalizeNavigatorSource(ui.Source) {
	case navigatorSourceRSX, navigatorSourceWozduh:
		return NavigatorResultDTO{}
	}
	trendType := ui.TrendType
	if trendType == "" {
		trendType = NavigatorTrendWicks
	}
	hHighs, hLows, _ := ExtractTrendlineData(htfKlines, trendType)
	hCloses := ExtractCloses(htfKlines)
	hBarTimes := ExtractBarTimes(htfKlines)
	layer := RunNavigatorAggregator(hHighs, hLows, hCloses, hBarTimes, ui, period)
	if len(chartKlines) > 0 {
		layer.Lines = ClipNavigatorLinesToChartWindow(layer.Lines, chartKlines)
		layer.Markers = clipNavigatorMarkersToChartWindow(layer.Markers, chartKlines)
	}
	return layer
}

func mergeHTFNavigatorLayers(base NavigatorResultDTO, ui NavigatorUISettings, chartInterval string, chartKlines []exchange.Kline, htfData map[string][]exchange.Kline) NavigatorResultDTO {
	if len(htfData) == 0 {
		return base
	}
	for _, period := range ui.Periods {
		period = strings.TrimSpace(period)
		if period == "" || navigatorIntervalsEqual(period, chartInterval) {
			continue
		}
		htfKlines := htfData[period]
		if len(htfKlines) < 3 {
			continue
		}
		htfLayer := BuildHTFNavigatorLayer(ui, htfKlines, period, chartKlines)
		base.Lines = append(base.Lines, htfLayer.Lines...)
		base.Markers = append(base.Markers, htfLayer.Markers...)
	}
	return base
}

func normalizeNavigatorUISettings(ui NavigatorUISettings) NavigatorUISettings {
	if !ui.Enabled {
		return ui
	}
	if !ui.UseLong && !ui.UseMedium && !ui.UseShort {
		ui.UseLong = true
		ui.UseMedium = true
		ui.UseShort = true
	}
	if ui.LongLen <= 0 {
		ui.LongLen = navigatorDefaultLongLen
	}
	if ui.MediumLen <= 0 {
		ui.MediumLen = navigatorDefaultMediumLen
	}
	if ui.ShortLen <= 0 {
		ui.ShortLen = navigatorDefaultShortLen
	}
	return ui
}

// BuildNavigatorResult runs the aggregator and optionally attaches bar/background visuals.
func BuildNavigatorResult(ui NavigatorUISettings, klines []exchange.Kline, rsxValues, wozduhValues []float64, interval string, htf *exchange.HTFProvider, htfData map[string][]exchange.Kline) NavigatorResultDTO {
	dto := BuildNavigatorData(ui, klines, rsxValues, wozduhValues, interval, htf, htfData)
	if len(klines) > 0 {
		dto.Lines = ClipNavigatorLinesToChartWindow(dto.Lines, klines)
		dto.Markers = clipNavigatorMarkersToChartWindow(dto.Markers, klines)
	}
	if !ui.BarColor && !ui.BackgroundColor {
		return dto
	}
	bars, zones := buildNavigatorVisuals(ui, klines, rsxValues, wozduhValues, interval)
	if cutFrom := navigatorMinLineStartTime(dto.Lines); cutFrom > 0 {
		zones = clipNavigatorZonesFromTime(zones, cutFrom)
		bars = clipNavigatorBarColorsFromTime(bars, cutFrom)
	}
	dto.BarColors = bars
	dto.BackgroundZones = zones
	return dto
}

func navigatorMinLineStartTime(lines []NavigatorLineDTO) int64 {
	var min int64
	for _, l := range lines {
		if l.Time1 <= 0 {
			continue
		}
		if min == 0 || l.Time1 < min {
			min = l.Time1
		}
	}
	return min
}

// ClipNavigatorLinesToChartWindow trims MTF/chart lines to the loaded chart kline window.
// Lines starting before the first chart bar are clipped in time/price; lines entirely before the window are dropped.
func ClipNavigatorLinesToChartWindow(lines []NavigatorLineDTO, klines []exchange.Kline) []NavigatorLineDTO {
	if len(klines) == 0 || len(lines) == 0 {
		return lines
	}
	minMs := navigatorChartStartMs(klines)
	maxMs := navigatorChartEndMs(klines)
	out := make([]NavigatorLineDTO, 0, len(lines))
	for _, line := range lines {
		clipped, keep := clipNavigatorLineToWindow(line, minMs, maxMs)
		if keep {
			out = append(out, clipped)
		}
	}
	return out
}

func navigatorChartEndMs(klines []exchange.Kline) int64 {
	if len(klines) == 0 {
		return 0
	}
	last := klines[len(klines)-1]
	if last.CloseTime > 0 {
		return last.CloseTime
	}
	return last.OpenTime
}

func clipNavigatorLineToWindow(line NavigatorLineDTO, minMs, maxMs int64) (NavigatorLineDTO, bool) {
	t1, t2 := line.Time1, line.Time2
	if t1 <= 0 || t2 <= 0 {
		return line, true
	}
	if t2 < minMs || t1 > maxMs {
		return line, false
	}
	if t1 < minMs {
		line.Y1 = navigatorLinePriceAtTime(line, minMs)
		line.Time1 = minMs
	}
	if t2 > maxMs {
		line.Y2 = navigatorLinePriceAtTime(line, maxMs)
		line.Time2 = maxMs
	}
	return line, true
}

func navigatorLinePriceAtTime(line NavigatorLineDTO, tMs int64) float64 {
	t1, t2 := line.Time1, line.Time2
	if t2 == t1 {
		return sanitizeFloat(line.Y1)
	}
	y := line.Y1 + (line.Y2-line.Y1)*float64(tMs-t1)/float64(t2-t1)
	return sanitizeFloat(y)
}

func clipNavigatorMarkersToChartWindow(markers []NavigatorMarkerDTO, klines []exchange.Kline) []NavigatorMarkerDTO {
	if len(klines) == 0 || len(markers) == 0 {
		return markers
	}
	minMs := navigatorChartStartMs(klines)
	out := make([]NavigatorMarkerDTO, 0, len(markers))
	for _, m := range markers {
		if m.Time > 0 && m.Time < minMs {
			continue
		}
		out = append(out, m)
	}
	return out
}

func clipNavigatorZonesFromTime(zones []NavigatorZoneDTO, fromMs int64) []NavigatorZoneDTO {
	if fromMs <= 0 || len(zones) == 0 {
		return zones
	}
	out := make([]NavigatorZoneDTO, 0, len(zones))
	for _, z := range zones {
		if z.EndTime < fromMs {
			continue
		}
		if z.StartTime < fromMs {
			z.StartTime = fromMs
		}
		out = append(out, z)
	}
	return out
}

func clipNavigatorBarColorsFromTime(bars map[int64]string, fromMs int64) map[int64]string {
	if fromMs <= 0 || len(bars) == 0 {
		return bars
	}
	out := make(map[int64]string, len(bars))
	for t, c := range bars {
		if t >= fromMs {
			out[t] = c
		}
	}
	return out
}

// BuildAllNavigators runs enabled pane configs (price, rsx, wozduh keys).
func BuildAllNavigators(panes map[string]NavigatorUISettings, symbol string, klines []exchange.Kline, rsxValues, wozduhValues []float64, interval string, htf *exchange.HTFProvider) map[string]NavigatorResultDTO {
	out := make(map[string]NavigatorResultDTO)
	if len(panes) == 0 {
		return out
	}
	startMs := navigatorChartStartMs(klines)
	maxTimeSec := navigatorMaxCloseTimeSec(klines)
	for pane, ui := range panes {
		if !ui.Enabled {
			continue
		}
		ui.Source = navigatorPaneToSource(pane)
		htfData := loadNavigatorHTFData(htf, symbol, interval, startMs, maxTimeSec, ui.Periods)
		out[pane] = BuildNavigatorResult(ui, klines, rsxValues, wozduhValues, interval, htf, htfData)
	}
	return out
}

func navigatorMaxCloseTimeSec(klines []exchange.Kline) int64 {
	if len(klines) == 0 {
		return 0
	}
	last := klines[len(klines)-1]
	if last.CloseTime > 0 {
		return last.CloseTime / 1000
	}
	return last.OpenTime / 1000
}

func navigatorChartStartMs(klines []exchange.Kline) int64 {
	if len(klines) == 0 {
		return 0
	}
	return klines[0].OpenTime
}

func normalizeNavigatorInterval(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "1M" {
		return "1M"
	}
	return strings.ToLower(raw)
}

func navigatorIntervalsEqual(a, b string) bool {
	return normalizeNavigatorInterval(a) == normalizeNavigatorInterval(b)
}

func loadNavigatorHTFData(htf *exchange.HTFProvider, symbol, chartInterval string, startMs, maxTimeSec int64, periods []string) map[string][]exchange.Kline {
	if htf == nil || symbol == "" || len(periods) == 0 {
		return nil
	}
	out := make(map[string][]exchange.Kline)
	for _, period := range periods {
		period = strings.TrimSpace(period)
		if period == "" || navigatorIntervalsEqual(period, chartInterval) {
			continue
		}
		periodKlines, err := htf.GetKlines(symbol, period, startMs)
		if err != nil || len(periodKlines) == 0 {
			continue
		}
		if maxTimeSec > 0 {
			if strict := htf.GetCandlesStrictlyBefore(symbol, period, maxTimeSec); len(strict) > 0 {
				periodKlines = strict
			}
		}
		periodKlines = sliceNavigatorKlinesFrom(periodKlines, startMs)
		if len(periodKlines) == 0 {
			continue
		}
		htf.PinKlines(symbol, period)
		out[period] = periodKlines
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sliceNavigatorKlinesFrom(klines []exchange.Kline, startMs int64) []exchange.Kline {
	if startMs <= 0 || len(klines) == 0 {
		return klines
	}
	i := 0
	for i < len(klines) && klines[i].OpenTime < startMs {
		i++
	}
	if i >= len(klines) {
		return nil
	}
	out := make([]exchange.Kline, len(klines)-i)
	copy(out, klines[i:])
	return out
}

func navigatorPaneToSource(pane string) string {
	switch pane {
	case "rsx":
		return navigatorSourceRSX
	case "wozduh":
		return navigatorSourceWozduh
	default:
		return navigatorSourcePrice
	}
}

func buildNavigatorVisuals(ui NavigatorUISettings, klines []exchange.Kline, rsxValues, wozduhValues []float64, interval string) (map[int64]string, []NavigatorZoneDTO) {
	highs, lows, closes := navigatorSeriesForSource(ui, klines, rsxValues, wozduhValues)
	if len(closes) == 0 {
		return nil, nil
	}

	trendType := ui.TrendType
	if trendType == "" {
		trendType = NavigatorTrendWicks
	}
	swingLen := navigatorTermSwingLen(ui)
	if swingLen < 1 {
		return nil, nil
	}

	barTimes := ExtractBarTimes(klines)
	engine := NewNavigatorEngine(navigatorEngineSettings(ui, swingLen, NavigatorStyleSolid, trendType))
	engine.Execute(highs, lows, closes, barTimes)
	return visualsFromTrendHistory(engine.TrendHistory, barTimes, ui.BarColor, ui.BackgroundColor)
}

func navigatorSeriesForSource(ui NavigatorUISettings, klines []exchange.Kline, rsxValues, wozduhValues []float64) (highs, lows, closes []float64) {
	switch normalizeNavigatorSource(ui.Source) {
	case navigatorSourceRSX:
		return rsxValues, rsxValues, rsxValues
	case navigatorSourceWozduh:
		return wozduhValues, wozduhValues, wozduhValues
	default:
		trendType := ui.TrendType
		if trendType == "" {
			trendType = NavigatorTrendWicks
		}
		highs, lows, _ = ExtractTrendlineData(klines, trendType)
		return highs, lows, ExtractCloses(klines)
	}
}

func navigatorTermSwingLen(ui NavigatorUISettings) int {
	switch ui.Term {
	case "Medium":
		if ui.MediumLen > 0 {
			return ui.MediumLen
		}
		return navigatorDefaultMediumLen
	case "Short":
		if ui.ShortLen > 0 {
			return ui.ShortLen
		}
		return navigatorDefaultShortLen
	default:
		if ui.LongLen > 0 {
			return ui.LongLen
		}
		return navigatorDefaultLongLen
	}
}

const (
	navigatorBarBullColor = "#08998155"
	navigatorBarBearColor = "#f2364555"
	navigatorBgBullColor  = "#08998114"
	navigatorBgBearColor  = "#f2364514"
)

func visualsFromTrendHistory(trends []int, barTimes []int64, barColor, backgroundColor bool) (map[int64]string, []NavigatorZoneDTO) {
	if len(trends) == 0 || (!barColor && !backgroundColor) {
		return nil, nil
	}
	if len(barTimes) != len(trends) {
		barTimes = SynthesizeBarTimesMS(len(trends), 60_000)
	}

	var bars map[int64]string
	if barColor {
		bars = make(map[int64]string, len(trends))
		for i, trend := range trends {
			if trend == 0 {
				continue
			}
			color := navigatorBarBullColor
			if trend < 0 {
				color = navigatorBarBearColor
			}
			bars[barTimes[i]] = color
		}
	}

	var zones []NavigatorZoneDTO
	if backgroundColor {
		start := -1
		curTrend := 0
		lastIdx := len(trends) - 1
		flush := func(endIdx int) {
			if start < 0 || curTrend == 0 || endIdx < start {
				return
			}
			color := navigatorBgBullColor
			if curTrend < 0 {
				color = navigatorBgBearColor
			}
			zones = append(zones, NavigatorZoneDTO{
				StartTime: barTimes[start],
				EndTime:   barTimes[endIdx],
				Color:     color,
			})
		}
		for i, trend := range trends {
			if trend == 0 {
				flush(i - 1)
				start = -1
				curTrend = 0
				continue
			}
			if trend != curTrend {
				flush(i - 1)
				start = i
				curTrend = trend
			}
		}
		flush(lastIdx)
	}

	return bars, zones
}

func navigatorEngineSettings(ui NavigatorUISettings, swingLen int, style, trendType string) NavigatorSettings {
	return NavigatorSettings{
		SwingLength:       swingLen,
		TrendType:         trendType,
		Style:             style,
		MomentumEnabled:   ui.MomentumEnabled,
		MomentumBars:      ui.MomentumBars,
		MomentumPercent:   ui.MomentumPercent,
		TimeHoldEnabled:   ui.TimeHoldEnabled,
		TimeHoldBars:      ui.TimeHoldBars,
	}
}

func normalizeNavigatorSource(source string) string {
	switch source {
	case navigatorSourceRSX, "rsx":
		return navigatorSourceRSX
	case navigatorSourceWozduh, "wozduh":
		return navigatorSourceWozduh
	default:
		return navigatorSourcePrice
	}
}

// ExtractCloses returns close prices aligned with kline slice indices.
func ExtractCloses(klines []exchange.Kline) []float64 {
	out := make([]float64, len(klines))
	for i, k := range klines {
		out[i] = k.Close
	}
	return out
}

// NewNavigatorEngine creates a navigator engine with LuxAlgo defaults applied.
func NewNavigatorEngine(settings NavigatorSettings) *NavigatorEngine {
	if settings.PivotRight <= 0 {
		settings.PivotRight = navigatorDefaultPivotRight
	}
	if settings.MaxLineBars <= 0 {
		settings.MaxLineBars = navigatorDefaultMaxLineBars
	}
	if settings.MinPivotSpacing <= 0 {
		settings.MinPivotSpacing = navigatorDefaultMinHHSpacing
	}
	if settings.BullColor == "" {
		settings.BullColor = navigatorColorBull
	}
	if settings.BearColor == "" {
		settings.BearColor = navigatorColorBear
	}
	if settings.WickBullColor == "" {
		settings.WickBullColor = navigatorColorWickBull
	}
	if settings.WickBearColor == "" {
		settings.WickBearColor = navigatorColorWickBear
	}
	if settings.Style == "" {
		settings.Style = NavigatorStyleSolid
	}
	return &NavigatorEngine{
		Settings: settings,
		PrevPH:   ChartPoint{Index: -1},
		PrevPL:   ChartPoint{Index: -1},
	}
}

// Execute replays LuxAlgo draw() bar-by-bar over historical series.
func (e *NavigatorEngine) Execute(highData, lowData, closeData []float64, barTimes []int64) {
	n := len(highData)
	if n == 0 || len(lowData) != n || len(closeData) != n {
		return
	}
	if len(barTimes) != n {
		barTimes = SynthesizeBarTimesMS(n, 60_000)
	}
	e.resetRun()
	for i := 0; i < n; i++ {
		e.Update(highData[i], lowData[i], closeData[i], barTimes[i], i)
	}
}

// Update processes one bar incrementally (live mode). index must equal the number of bars already processed.
func (e *NavigatorEngine) Update(newHigh, newLow, newClose float64, barTime int64, index int) {
	if index != len(e.histHigh) {
		return
	}

	left := e.Settings.SwingLength
	right := e.Settings.PivotRight
	if left < 1 {
		return
	}

	e.histHigh = append(e.histHigh, newHigh)
	e.histLow = append(e.histLow, newLow)
	e.histClose = append(e.histClose, newClose)
	e.histTimes = append(e.histTimes, barTime)
	e.pushPivotBuffers(newHigh, newLow, newClose)

	n := index
	e.extendActiveLines(n)

	if e.PendingBreakout != nil {
		e.processPendingHold(n, newClose)
	}

	highConfirmed := false
	lowConfirmed := false
	bufCap := left + right + 1
	if len(e.HighBuf) >= bufCap {
		highConfirmed = isPivotHighAt(e.HighBuf, left, left, right)
		lowConfirmed = isPivotLowAt(e.LowBuf, left, left, right)
		if highConfirmed && lowConfirmed {
			lowConfirmed = false
		}
	}

	if highConfirmed {
		e.handlePivotHigh(n, e.histHigh, e.histClose, left, right)
	} else if e.Trend < 1 && e.PendingBreakout == nil {
		e.checkBearCloseBreak(n, newHigh, newLow, newClose)
	}

	if lowConfirmed {
		e.handlePivotLow(n, e.histLow, e.histClose, left, right)
	} else if e.Trend > -1 && e.PendingBreakout == nil {
		e.checkBullCloseBreak(n, newHigh, newLow, newClose)
	}

	e.TrendHistory = append(e.TrendHistory, e.Trend)
}

func (e *NavigatorEngine) pushPivotBuffers(high, low, close float64) {
	left := e.Settings.SwingLength
	right := e.Settings.PivotRight
	cap := left + right + 1

	e.HighBuf = append(e.HighBuf, high)
	e.LowBuf = append(e.LowBuf, low)
	e.CloseBuf = append(e.CloseBuf, close)
	if len(e.HighBuf) > cap {
		e.HighBuf = e.HighBuf[len(e.HighBuf)-cap:]
		e.LowBuf = e.LowBuf[len(e.LowBuf)-cap:]
		e.CloseBuf = e.CloseBuf[len(e.CloseBuf)-cap:]
	}
}

// GetResultDTO exports completed + active lines and markers for API/chart consumers.
func (e *NavigatorEngine) GetResultDTO() NavigatorResultDTO {
	lines := make([]NavigatorLineDTO, 0, len(e.CompletedLines)+2)
	for i := range e.CompletedLines {
		lines = append(lines, e.lineToDTO(e.CompletedLines[i]))
	}
	if e.Active != nil && e.Active.IsActive {
		lines = append(lines, e.lineToDTO(*e.Active))
	}

	markers := make([]NavigatorMarkerDTO, len(e.Markers))
	for i, m := range e.Markers {
		markers[i] = e.markerToDTO(m)
	}

	return NavigatorResultDTO{Lines: lines, Markers: markers}
}

func sanitizeFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// sanitizeSlope clamps per-bar slope to a finite range (prevents vertical runaway lines).
func sanitizeSlope(slope float64) float64 {
	if math.IsNaN(slope) || math.IsInf(slope, 0) {
		return 0
	}
	if slope > navigatorMaxSlope {
		return navigatorMaxSlope
	}
	if slope < -navigatorMaxSlope {
		return -navigatorMaxSlope
	}
	return slope
}

func slopePricePerBar(y1, y2 float64, x1, x2 int) float64 {
	dx := x2 - x1
	if dx == 0 {
		return 0
	}
	return sanitizeSlope((y2 - y1) / float64(dx))
}

func linePriceAtIndex(tl *Trendline, index int) float64 {
	dx := index - tl.X1
	if dx == 0 {
		return tl.Y1
	}
	if tl.Slope != 0 {
		return tl.Y1 + tl.Slope*float64(dx)
	}
	if tl.X2 == tl.X1 {
		return tl.Y1
	}
	return tl.Y1 + (tl.Y2-tl.Y1)/float64(tl.X2-tl.X1)*float64(dx)
}

func logSuspiciousNavigatorLine(tl Trendline) {
	barSlope := slopePricePerBar(tl.Y1, tl.Y2, tl.X1, tl.X2)
	absDelta := math.Abs(tl.Y2 - tl.Y1)
	ref := math.Max(math.Abs(tl.Y1), math.Abs(tl.Y2))

	samePoint := tl.X1 == tl.X2
	hugeBarSlope := math.Abs(tl.Slope) >= navigatorMaxSlope*0.99 ||
		math.Abs(barSlope) >= navigatorMaxSlope*0.99
	hugeDelta := (ref > 0 && absDelta > ref*10) || absDelta > 1e7

	if !samePoint && !hugeBarSlope && !hugeDelta {
		return
	}
	log.Printf("[Navigator] suspicious line: x1=%d x2=%d y1=%g y2=%g slope=%g barSlope=%g",
		tl.X1, tl.X2, tl.Y1, tl.Y2, tl.Slope, barSlope)
}

func (e *NavigatorEngine) lineToDTO(tl Trendline) NavigatorLineDTO {
	y1 := sanitizeFloat(tl.Y1)
	y2 := sanitizeFloat(linePriceAtIndex(&tl, tl.X2))
	return NavigatorLineDTO{
		X1: tl.X1, Y1: y1,
		X2: tl.X2, Y2: y2,
		Time1: e.barTime(tl.X1),
		Time2: e.barTime(tl.X2),
		Color:    tl.Color,
		Style:    tl.Style,
		IsActive: tl.IsActive,
		Slope:    sanitizeSlope(tl.Slope),
	}
}

func navigatorLineToDTO(tl Trendline) NavigatorLineDTO {
	// Legacy path for tests without bar times; prefers index geometry for Y2.
	y1 := sanitizeFloat(tl.Y1)
	y2 := sanitizeFloat(linePriceAtIndex(&tl, tl.X2))
	logSuspiciousNavigatorLine(tl)
	return NavigatorLineDTO{
		X1: tl.X1, Y1: y1,
		X2: tl.X2, Y2: y2,
		Color:    tl.Color,
		Style:    tl.Style,
		IsActive: tl.IsActive,
		Slope:    sanitizeSlope(tl.Slope),
	}
}

func (e *NavigatorEngine) markerToDTO(m TrendlineMarker) NavigatorMarkerDTO {
	markerType := m.Type
	if m.Type == "Label" {
		markerType = m.Text
	}
	t := m.Time
	if t == 0 && m.Index >= 0 {
		t = e.barTime(m.Index)
	}
	return NavigatorMarkerDTO{
		Index: m.Index,
		Time:  t,
		Price: sanitizeFloat(m.Price),
		Text:  m.Text,
		Color: m.Color,
		Type:  markerType,
	}
}

// NavigatorBindBarTimesForTest attaches open times so GetResultDTO can export time1/time2 in tests.
func NavigatorBindBarTimesForTest(e *NavigatorEngine, barTimes []int64) {
	if e == nil {
		return
	}
	e.histTimes = barTimes
}

// SanitizeSlopeForTest exposes slope sanitization for unit tests.
func SanitizeSlopeForTest(slope float64) float64 {
	return sanitizeSlope(slope)
}

// NavigatorSwingHighForTest exposes LuxAlgo chH swing anchor lookup for unit tests.
func NavigatorSwingHighForTest(highs []float64, n, right int) (x int, v float64) {
	e := &NavigatorEngine{Settings: NavigatorSettings{PivotRight: right}}
	return e.swingHighSinceIndex(n, e.pivotSwingFloorBar(n, right), highs)
}

// NavigatorSwingLowForTest exposes LuxAlgo chL swing anchor lookup for unit tests.
func NavigatorSwingLowForTest(lows []float64, n, right int) (x int, v float64) {
	e := &NavigatorEngine{Settings: NavigatorSettings{PivotRight: right}}
	return e.swingLowSinceIndex(n, e.pivotSwingFloorBar(n, right), lows)
}

// BuildNavigatorPriceFromKlines runs price navigator with legacy single-layer defaults (Long only, len 10).
func BuildNavigatorPriceFromKlines(klines []exchange.Kline) NavigatorResultDTO {
	return BuildNavigatorData(NavigatorUISettings{
		Enabled:   true,
		Source:    navigatorSourcePrice,
		TrendType: NavigatorTrendWicks,
		UseLong:   true,
		LongLen:   10,
	}, klines, nil, nil, "", nil, nil)
}

func (e *NavigatorEngine) resetRun() {
	e.Trend = 0
	e.PrevPH = ChartPoint{Index: -1}
	e.PrevPL = ChartPoint{Index: -1}
	e.Active = nil
	e.CompletedLines = nil
	e.Markers = nil
	e.HighBuf = nil
	e.LowBuf = nil
	e.CloseBuf = nil
	e.histHigh = nil
	e.histLow = nil
	e.histClose = nil
	e.histTimes = nil
	e.TrendHistory = nil
	e.clearPending()
}

func (e *NavigatorEngine) barTime(index int) int64 {
	if index >= 0 && index < len(e.histTimes) {
		return e.histTimes[index]
	}
	return 0
}

func (e *NavigatorEngine) hasActiveLine() bool {
	return e.Active != nil && e.Active.IsActive
}

func (e *NavigatorEngine) hasActiveBull() bool {
	return e.hasActiveLine() && e.Active.Bullish
}

func (e *NavigatorEngine) hasActiveBear() bool {
	return e.hasActiveLine() && !e.Active.Bullish
}

func (e *NavigatorEngine) extendActiveLines(n int) {
	maxLen := e.Settings.MaxLineBars
	if !e.hasActiveLine() {
		return
	}
	if n-e.Active.X1 > maxLen {
		e.deactivateActive()
		return
	}
	e.extendNavigatorLine(e.Active, n)
}

func (e *NavigatorEngine) extendNavigatorLine(tl *Trendline, n int) {
	tl.X2 = n
	tl.Y2 = sanitizeFloat(linePriceAtIndex(tl, n))
}

func (e *NavigatorEngine) handlePivotHigh(n int, highData, closeData []float64, left, right int) {
	floorBar := e.pivotSwingFloorBar(n, right)
	x, v := e.swingHighSinceIndex(n, floorBar, highData)
	c := closeData[x]
	idxBack := n - x

	if e.Trend < 1 {
		if e.isHigherHigh(x, v, n) {
			e.Trend = 1
			e.deactivateActive()
			e.Active = e.newBullLine(e.PrevPL, n)
			e.Markers = append(e.Markers, TrendlineMarker{
				Index: x, Price: v, Text: "HH", Color: e.Settings.BullColor, Type: "Label",
			})
		} else if e.hasActiveBear() {
			e.processBearPivot(n, x, v, c, idxBack, highData, closeData)
		}
	}

	e.PrevPH = ChartPoint{Index: x, Price: v}
}

func (e *NavigatorEngine) handlePivotLow(n int, lowData, closeData []float64, left, right int) {
	floorBar := e.pivotSwingFloorBar(n, right)
	x, v := e.swingLowSinceIndex(n, floorBar, lowData)
	c := closeData[x]
	idxBack := n - x

	if e.Trend > -1 {
		if e.isLowerLow(x, v, n) {
			e.Trend = -1
			e.deactivateActive()
			e.Active = e.newBearLine(e.PrevPH, n)
			e.Markers = append(e.Markers, TrendlineMarker{
				Index: x, Price: v, Text: "LL", Color: e.Settings.BearColor, Type: "Label",
			})
		} else if e.hasActiveBull() {
			e.processBullPivot(n, x, v, c, idxBack, lowData, closeData)
		}
	}

	e.PrevPL = ChartPoint{Index: x, Price: v}
}

// pivotSwingFloorBar returns the oldest bar index still included in LuxAlgo chH/chL scan
// (Pine time[2] when right=1 → bar index n-2).
func (e *NavigatorEngine) pivotSwingFloorBar(n, right int) int {
	floor := n - right - 1
	if floor < 0 {
		return 0
	}
	return floor
}

// swingHighSinceIndex mirrors LuxAlgo chH: max high from bar n back to floorBar (index space).
func (e *NavigatorEngine) swingHighSinceIndex(n, floorBar int, highData []float64) (x int, v float64) {
	v = -math.MaxFloat64
	idx := 0
	for i := 0; i <= n; i++ {
		barIdx := n - i
		if barIdx < 0 || barIdx >= len(highData) {
			break
		}
		if highData[barIdx] > v {
			v = highData[barIdx]
			idx = i
		}
		if barIdx < floorBar {
			break
		}
	}
	return n - idx, v
}

// swingLowSinceIndex mirrors LuxAlgo chL: min low from bar n back to floorBar (index space).
func (e *NavigatorEngine) swingLowSinceIndex(n, floorBar int, lowData []float64) (x int, v float64) {
	v = math.MaxFloat64
	idx := 0
	for i := 0; i <= n; i++ {
		barIdx := n - i
		if barIdx < 0 || barIdx >= len(lowData) {
			break
		}
		if lowData[barIdx] < v {
			v = lowData[barIdx]
			idx = i
		}
		if barIdx < floorBar {
			break
		}
	}
	return n - idx, v
}

func (e *NavigatorEngine) isHigherHigh(x int, v float64, n int) bool {
	if !e.PrevPH.Valid() || v <= e.PrevPH.Price {
		return false
	}
	if x-e.PrevPH.Index <= e.Settings.MinPivotSpacing {
		return false
	}
	if e.PrevPL.Valid() && n-e.PrevPL.Index >= e.Settings.MaxLineBars {
		return false
	}
	return e.PrevPL.Valid()
}

func (e *NavigatorEngine) isLowerLow(x int, v float64, n int) bool {
	if !e.PrevPL.Valid() || v >= e.PrevPL.Price {
		return false
	}
	if x-e.PrevPL.Index <= e.Settings.MinPivotSpacing {
		return false
	}
	if e.PrevPH.Valid() && n-e.PrevPH.Index >= e.Settings.MaxLineBars {
		return false
	}
	return e.PrevPH.Valid()
}

func (e *NavigatorEngine) newBullLine(from ChartPoint, n int) *Trendline {
	cp := ChartPoint{Index: from.Index, Price: from.Price}
	return &Trendline{
		X1: from.Index, Y1: from.Price,
		X2: n, Y2: from.Price,
		Slope:        0,
		IsActive:     true,
		Bullish:      true,
		ControlPoint: cp,
		Style:        e.Settings.Style,
		Color:        e.Settings.BullColor,
	}
}

func (e *NavigatorEngine) newBearLine(from ChartPoint, n int) *Trendline {
	cp := ChartPoint{Index: from.Index, Price: from.Price}
	return &Trendline{
		X1: from.Index, Y1: from.Price,
		X2: n, Y2: from.Price,
		Slope:        0,
		IsActive:     true,
		Bullish:      false,
		ControlPoint: cp,
		Style:        e.Settings.Style,
		Color:        e.Settings.BearColor,
	}
}

func (e *NavigatorEngine) processBearPivot(n, x int, v, c float64, idxBack int, highData, closeData []float64) {
	tl := e.Active
	cp := tl.ControlPoint
	if x == cp.Index {
		return
	}

	newSlope := slopePricePerBar(cp.Price, v, cp.Index, x)
	if !(v < tl.Y1+newSlope && (v > tl.Y2+newSlope || tl.Slope == 0)) {
		return
	}

	priceLin := linePriceAtIndex(tl, x)
	if c < priceLin {
		if tl.Slope != 0 {
			e.Markers = append(e.Markers, TrendlineMarker{
				Index: x, Price: priceLin, Text: "●", Color: e.Settings.WickBearColor, Type: "WickBreak",
			})
		}
		tl.X2 = n
		tl.Y2 = sanitizeFloat(v + newSlope*float64(idxBack))
		if tl.Slope == 0 {
			e.adjustBearInnerBreaks(tl, n, x, v, idxBack, newSlope, highData, closeData)
		} else {
			tl.Slope = sanitizeSlope(newSlope)
		}
		return
	}

	e.deactivateActive()
}

func (e *NavigatorEngine) processBullPivot(n, x int, v, c float64, idxBack int, lowData, closeData []float64) {
	tl := e.Active
	cp := tl.ControlPoint
	if x == cp.Index {
		return
	}

	newSlope := slopePricePerBar(cp.Price, v, cp.Index, x)
	if !(v > tl.Y1+newSlope && (v < tl.Y2+newSlope || tl.Slope == 0)) {
		return
	}

	priceLin := linePriceAtIndex(tl, x)
	if c > priceLin {
		if tl.Slope != 0 {
			e.Markers = append(e.Markers, TrendlineMarker{
				Index: x, Price: priceLin, Text: "●", Color: e.Settings.WickBullColor, Type: "WickBreak",
			})
		}
		tl.X2 = n
		tl.Y2 = sanitizeFloat(v + newSlope*float64(idxBack))
		if tl.Slope == 0 {
			e.adjustBullInnerBreaks(tl, n, x, v, idxBack, newSlope, lowData, closeData)
		} else {
			tl.Slope = sanitizeSlope(newSlope)
		}
		return
	}

	e.deactivateActive()
}

func (e *NavigatorEngine) adjustBearInnerBreaks(tl *Trendline, n, x int, v float64, idxBack int, newSlope float64, highData, closeData []float64) {
	for {
		maxDiff := -math.MaxFloat64
		bestI := -1
		for i := 0; i <= n-tl.X1; i++ {
			barIdx := n - i
			diff := closeData[barIdx] - linePriceAtIndex(tl, barIdx)
			if diff > maxDiff {
				maxDiff = diff
				bestI = i
			}
		}
		if maxDiff > 0 {
			x1 := n - bestI
			y1 := highData[x1]
			tl.ControlPoint = ChartPoint{Index: x1, Price: y1}
			tl.X1, tl.Y1 = x1, y1
			newSlope = slopePricePerBar(y1, v, x1, x)
			tl.X2 = n
			tl.Y2 = sanitizeFloat(v + newSlope*float64(idxBack))
			tl.Slope = sanitizeSlope(newSlope)
			continue
		}
		tl.X2 = n
		tl.Y2 = sanitizeFloat(v + newSlope*float64(idxBack))
		tl.Slope = sanitizeSlope(newSlope)
		return
	}
}

func (e *NavigatorEngine) adjustBullInnerBreaks(tl *Trendline, n, x int, v float64, idxBack int, newSlope float64, lowData, closeData []float64) {
	for {
		maxDiff := -math.MaxFloat64
		bestI := -1
		for i := 0; i <= n-tl.X1; i++ {
			barIdx := n - i
			diff := linePriceAtIndex(tl, barIdx) - closeData[barIdx]
			if diff > maxDiff {
				maxDiff = diff
				bestI = i
			}
		}
		if maxDiff > 0 {
			x1 := n - bestI
			y1 := lowData[x1]
			tl.ControlPoint = ChartPoint{Index: x1, Price: y1}
			tl.X1, tl.Y1 = x1, y1
			newSlope = slopePricePerBar(y1, v, x1, x)
			tl.X2 = n
			tl.Y2 = sanitizeFloat(v + newSlope*float64(idxBack))
			tl.Slope = sanitizeSlope(newSlope)
			continue
		}
		tl.X2 = n
		tl.Y2 = sanitizeFloat(v + newSlope*float64(idxBack))
		tl.Slope = sanitizeSlope(newSlope)
		return
	}
}

func (e *NavigatorEngine) checkBearCloseBreak(n int, high, low, closePrice float64) {
	if !e.hasActiveBear() {
		return
	}
	if closePrice <= linePriceAtIndex(e.Active, n) {
		return
	}
	if !e.passesMomentumFilter(n, high, low) {
		return
	}
	if e.Settings.TimeHoldEnabled && e.Settings.TimeHoldBars > 0 {
		e.startPendingBreakout(e.Active, navigatorPendingBear)
		return
	}
	e.deactivateActive()
}

func (e *NavigatorEngine) checkBullCloseBreak(n int, high, low, closePrice float64) {
	if !e.hasActiveBull() {
		return
	}
	if closePrice >= linePriceAtIndex(e.Active, n) {
		return
	}
	if !e.passesMomentumFilter(n, high, low) {
		return
	}
	if e.Settings.TimeHoldEnabled && e.Settings.TimeHoldBars > 0 {
		e.startPendingBreakout(e.Active, navigatorPendingBull)
		return
	}
	e.deactivateActive()
}

func (e *NavigatorEngine) startPendingBreakout(tl *Trendline, pendingType string) {
	e.PendingBreakout = tl
	e.PendingType = pendingType
	e.HoldCounter = 0
}

func (e *NavigatorEngine) processPendingHold(n int, closePrice float64) {
	tl := e.PendingBreakout
	if tl == nil || !tl.IsActive {
		e.clearPending()
		return
	}

	holdBars := e.Settings.TimeHoldBars
	if holdBars <= 0 {
		holdBars = 1
	}

	linePrice := linePriceAtIndex(tl, n)
	switch e.PendingType {
	case navigatorPendingBear:
		if closePrice <= linePrice {
			e.clearPending()
			return
		}
		e.HoldCounter++
		if e.HoldCounter >= holdBars {
			e.clearPending()
			e.deactivateActive()
		}
	case navigatorPendingBull:
		if closePrice >= linePrice {
			e.clearPending()
			return
		}
		e.HoldCounter++
		if e.HoldCounter >= holdBars {
			e.clearPending()
			e.deactivateActive()
		}
	default:
		e.clearPending()
	}
}

func (e *NavigatorEngine) clearPending() {
	e.PendingBreakout = nil
	e.PendingType = ""
	e.HoldCounter = 0
}

func (e *NavigatorEngine) passesMomentumFilter(n int, high, low float64) bool {
	if !e.Settings.MomentumEnabled {
		return true
	}
	period := e.Settings.MomentumBars
	if period <= 0 {
		period = navigatorDefaultMomentumBars
	}
	pct := e.Settings.MomentumPercent
	if pct <= 0 {
		pct = navigatorDefaultMomentumPercent
	}
	atr := navigatorATR(e.histHigh, e.histLow, e.histClose, n, period)
	if atr <= 0 {
		return true
	}
	prevClose := high
	if n > 0 {
		prevClose = e.histClose[n-1]
	}
	tr := navigatorTrueRange(high, low, prevClose)
	return tr >= atr*(pct/100.0)
}

func navigatorTrueRange(high, low, prevClose float64) float64 {
	tr := high - low
	if d := math.Abs(high - prevClose); d > tr {
		tr = d
	}
	if d := math.Abs(low - prevClose); d > tr {
		tr = d
	}
	return tr
}

func navigatorATR(highs, lows, closes []float64, endIdx, period int) float64 {
	if period <= 0 || endIdx < 0 || endIdx >= len(closes) {
		return 0
	}
	start := endIdx - period + 1
	if start < 1 {
		start = 1
	}
	var sum float64
	count := 0
	for i := start; i <= endIdx; i++ {
		prevClose := closes[i-1]
		sum += navigatorTrueRange(highs[i], lows[i], prevClose)
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// NavigatorTrueRangeForTest exposes true range calculation for unit tests.
func NavigatorTrueRangeForTest(high, low, prevClose float64) float64 {
	return navigatorTrueRange(high, low, prevClose)
}

// NavigatorATRForTest exposes ATR calculation for unit tests.
func NavigatorATRForTest(highs, lows, closes []float64, endIdx, period int) float64 {
	return navigatorATR(highs, lows, closes, endIdx, period)
}

func (e *NavigatorEngine) appendCompletedLine(tl Trendline) {
	e.CompletedLines = append(e.CompletedLines, tl)
}

func (e *NavigatorEngine) deactivateActive() {
	if !e.hasActiveLine() {
		return
	}
	if e.PendingBreakout == e.Active {
		e.clearPending()
	}
	e.Active.IsActive = false
	e.appendCompletedLine(*e.Active)
	e.Active = nil
}

func (e *NavigatorEngine) deactivateBull() {
	if e.hasActiveBull() {
		e.deactivateActive()
	}
}

func (e *NavigatorEngine) deactivateBear() {
	if e.hasActiveBear() {
		e.deactivateActive()
	}
}

// DeactivateBullForTest exposes line deactivation for unit tests.
func (e *NavigatorEngine) DeactivateBullForTest() {
	e.deactivateActive()
}

// ExtractBarTimes returns kline open times (Unix ms) aligned with OHLC series.
func ExtractBarTimes(klines []exchange.Kline) []int64 {
	out := make([]int64, len(klines))
	for i, k := range klines {
		out[i] = k.OpenTime
	}
	return out
}

// SynthesizeBarTimesMS builds evenly spaced bar open times for tests or missing input.
func SynthesizeBarTimesMS(n int, stepMs int64) []int64 {
	if stepMs <= 0 {
		stepMs = 60_000
	}
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(i) * stepMs
	}
	return out
}

// ExtractTrendlineData builds high/low series and bar times for pivot detection.
func ExtractTrendlineData(klines []exchange.Kline, trendType string) (highs, lows []float64, times []int64) {
	n := len(klines)
	highs = make([]float64, n)
	lows = make([]float64, n)
	times = ExtractBarTimes(klines)

	useBody := trendType == NavigatorTrendBody
	for i, k := range klines {
		if useBody {
			highs[i] = math.Max(k.Open, k.Close)
			lows[i] = math.Min(k.Open, k.Close)
		} else {
			highs[i] = k.High
			lows[i] = k.Low
		}
	}
	return highs, lows, times
}

// FindPivots returns all pivot highs and lows confirmed with leftLen bars before
// and rightLen bars after the candidate bar (ta.pivothigh / ta.pivotlow semantics).
func FindPivots(highData, lowData []float64, barTimes []int64, leftLen, rightLen int) (pivotHighs, pivotLows []ChartPoint) {
	n := len(highData)
	if n != len(lowData) || leftLen < 1 || rightLen < 1 || n < leftLen+rightLen+1 {
		return nil, nil
	}
	if len(barTimes) != n {
		barTimes = SynthesizeBarTimesMS(n, 60_000)
	}

	for i := leftLen; i < n-rightLen; i++ {
		if isPivotHighAt(highData, i, leftLen, rightLen) {
			pivotHighs = append(pivotHighs, ChartPoint{Index: i, Time: barTimes[i], Price: highData[i]})
		}
		if isPivotLowAt(lowData, i, leftLen, rightLen) {
			pivotLows = append(pivotLows, ChartPoint{Index: i, Time: barTimes[i], Price: lowData[i]})
		}
	}
	return pivotHighs, pivotLows
}

func isPivotHighAt(data []float64, i, leftLen, rightLen int) bool {
	if i-leftLen < 0 || i+rightLen >= len(data) {
		return false
	}
	v := data[i]
	// ta.pivothigh: center >= left bars, center > right bars (allows double-top plateau on the left).
	for j := i - leftLen; j < i; j++ {
		if data[j] > v {
			return false
		}
	}
	for j := i + 1; j <= i+rightLen; j++ {
		if data[j] >= v {
			return false
		}
	}
	return true
}

func isPivotLowAt(data []float64, i, leftLen, rightLen int) bool {
	if i-leftLen < 0 || i+rightLen >= len(data) {
		return false
	}
	v := data[i]
	// ta.pivotlow: center <= left bars, center < right bars (allows double-bottom plateau on the left).
	for j := i - leftLen; j < i; j++ {
		if data[j] < v {
			return false
		}
	}
	for j := i + 1; j <= i+rightLen; j++ {
		if data[j] <= v {
			return false
		}
	}
	return true
}
