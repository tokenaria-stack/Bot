package strategy

import (
	"context"
	"fmt"
	"log"
	"time"

	"trading_bot/exchange"
)

const (
	BacktestInitialCapital     = 10000.0
	DefaultBacktestSlippagePct = 0.03 // 0.03% per fill
)

// BacktestMinBars returns the minimum candle count required before replay in backtests.
func BacktestMinBars() int {
	return IndicatorWarmupBars
}

const (
	backtestPadMinDailyDays   = 60
	backtestPadExtraDailyDays = 30
	backtestPadMaxDailyDays   = 365
	backtestPadExtraWeekly    = 4
	backtestPadExtraMonthly   = 2
)

// BacktestPadStartDays returns how many calendar days to extend start backward for coarse TFs.
func BacktestPadStartDays(binanceInterval string, have, need int) int {
	if have >= need {
		return 0
	}
	missing := need - have
	switch binanceInterval {
	case "1d":
		pad := missing + backtestPadExtraDailyDays
		if pad < backtestPadMinDailyDays {
			pad = backtestPadMinDailyDays
		}
		if pad > backtestPadMaxDailyDays {
			pad = backtestPadMaxDailyDays
		}
		return pad
	case "1w":
		weeks := missing + backtestPadExtraWeekly
		return weeks * 7
	case "1M":
		months := missing + backtestPadExtraMonthly
		return months * 30
	default:
		return 0
	}
}

// PadBacktestStartMs shifts start backward when coarse intervals lack enough candles.
// Returns new start ms and whether padding was applied.
func PadBacktestStartMs(binanceInterval string, startMs, endMs int64, candleCount int) (int64, bool) {
	padDays := BacktestPadStartDays(binanceInterval, candleCount, IndicatorWarmupBars)
	if padDays <= 0 {
		return startMs, false
	}
	newStart := startMs - int64(padDays)*24*time.Hour.Milliseconds()
	if newStart < 0 {
		newStart = 0
	}
	if newStart >= startMs {
		return startMs, false
	}
	return newStart, true
}

// BacktestConfig configures a historical chart replay run.
// Trading/scoring knobs were purged in Core 5.0 Phase F.
type BacktestConfig struct {
	Symbol         string
	Interval       string
	Navigator      NavigatorUISettings
	Navigators     map[string]NavigatorUISettings
	HTF            *exchange.HTFProvider
	RSXSettings    *RSXSettings
	WozduhPrefs    map[string]bool
	SimOnly        bool
	SkipNavigators bool
}

// BacktestChartPoint is one candle + full indicator snapshot for the backtest chart.
// Score fields are contract sockets: zero until a strategy engine is plugged back in.
type BacktestChartPoint struct {
	Time   int64
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64

	Jurik          float64
	RSX            float64
	RSXSignal      float64
	RsiPrice       float64
	EmaRsi         float64
	RsiRsi         float64
	RsiHl2         float64
	RsiVolFast     float64
	RsiVolSlow     float64
	MacdRsi        float64
	RsiAd          float64
	RsiHl2Vol      float64
	VolCrossMarker string
	VolChanMid     float64
	VolChanUp      float64
	VolChanDn      float64
	PriceChanMid   float64
	PriceChanUp    float64
	PriceChanDn    float64
	Color          string
	Marker         string
	VolumeSpikeUp   bool
	VolumeSpikeDown bool

	LongScore   int                    `json:"longScore"`
	ShortScore  int                    `json:"shortScore"`
	RawAction   string                 `json:"rawAction"`
	FinalAction string                 `json:"finalAction"`
	IsVetoed    bool                   `json:"isVetoed"`
	VetoReason  string                 `json:"vetoReason,omitempty"`
	Factors     map[string]ScoreFactor `json:"factors,omitempty"`
}

// BacktestSimPoint carries indicator/marker fields only (no OHLC) for slim sim wire responses.
type BacktestSimPoint struct {
	Time            int64   `json:"time"`
	Jurik           float64 `json:"jurik,omitempty"`
	RSX             float64 `json:"rsx,omitempty"`
	RSXSignal       float64 `json:"rsxSignal,omitempty"`
	RsiVolFast      float64 `json:"rsiVolFast,omitempty"`
	RsiVolSlow      float64 `json:"rsiVolSlow,omitempty"`
	VolCrossMarker  string  `json:"volCrossMarker,omitempty"`
	Color           string  `json:"color,omitempty"`
	Marker          string  `json:"marker,omitempty"`
	VolumeSpikeUp   bool    `json:"volumeSpikeUp,omitempty"`
	VolumeSpikeDown bool    `json:"volumeSpikeDown,omitempty"`
}

// BacktestTradeResult is one completed round-trip trade (contract socket, Core 5.x).
type BacktestTradeResult struct {
	Time            int64
	EntryTime       int64
	Side            string
	EntryPrice      float64
	ExitPrice       float64
	StopLossPrice   float64
	ExitReason      string
	EntryReason     string   `json:"entryReason,omitempty"`
	FactorsSnapshot []string `json:"factorsSnapshot,omitempty"`
	StrategySource  string   `json:"strategySource,omitempty"`
	ActiveFactors   []string `json:"activeFactors,omitempty"`
	SignalKind      string   `json:"signalKind,omitempty"`
	EntryScore      float64  `json:"entryScore,omitempty"`
	PnL             float64
	Duration        string
}

// BacktestEquityPoint is a balance snapshot for the equity curve.
type BacktestEquityPoint struct {
	Time  int64
	Value float64
}

// BacktestRunResult aggregates replay output. Trade/metric fields are contract
// sockets kept for the dashboard wire format; they stay empty until Core 5.x.
type BacktestRunResult struct {
	TotalTrades    int
	WinRate        float64
	NetProfit      float64
	ProfitFactor   float64
	MaxDrawdown    float64
	RecoveryFactor float64
	Cancelled      bool
	Trades         []BacktestTradeResult
	EquityCurve    []BacktestEquityPoint
	ChartData      []BacktestChartPoint
	SimData        []BacktestSimPoint
	NavigatorData  NavigatorResultDTO
	Navigators     map[string]NavigatorResultDTO
	Annotations    []ChartAnnotation
}

// BacktestEngine replays historical candles through the indicator pipeline (chart-only).
type BacktestEngine struct {
	cfg BacktestConfig
}

// NewBacktestEngine creates a backtest replay runner.
func NewBacktestEngine(cfg BacktestConfig) *BacktestEngine {
	return &BacktestEngine{cfg: cfg}
}

// Run replays historical candles through the indicator pipeline and assembles chart output.
// When ctx is cancelled the loop stops early and returns partial results with Cancelled=true.
func (e *BacktestEngine) Run(ctx context.Context, candles []exchange.Candle) (*BacktestRunResult, error) {
	if len(candles) == 0 {
		return nil, fmt.Errorf("no candles to backtest")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	log.Printf("[Backtest] Processing %d candles (chart replay, trading purged in Core 5.0)", len(candles))

	replayCfg := StreamingReplayConfigFromBacktest(e.cfg)
	replay := RunStreamingReplay(ctx, candlesToKlines(candles), replayCfg)

	if replay.Cancelled {
		if e.cfg.SimOnly {
			log.Printf("[Backtest] Cancelled (%d sim points)", len(replay.SimPoints))
		} else {
			log.Printf("[Backtest] Cancelled (%d chart points)", len(replay.ChartPoints))
		}
	}

	chartPts := replay.ChartPoints
	simPts := replay.SimPoints
	if e.cfg.SimOnly {
		if len(simPts) >= IndicatorWarmupBars {
			simPts = simPts[IndicatorWarmupBars-1:]
		}
	} else if len(chartPts) >= IndicatorWarmupBars {
		chartPts = chartPts[IndicatorWarmupBars-1:]
	}

	return e.assembleRunResult(
		chartPts, simPts,
		replay.HistKlines, replay.HistRSX, replay.HistWozduh,
		replay.Cancelled, replay.Annotations,
	), nil
}

func (e *BacktestEngine) assembleRunResult(
	chartData []BacktestChartPoint,
	simData []BacktestSimPoint,
	histKlines []exchange.Kline,
	histRSX, histWozduh []float64,
	cancelled bool,
	annotations []ChartAnnotation,
) *BacktestRunResult {
	var navigators map[string]NavigatorResultDTO
	var navData NavigatorResultDTO
	if !e.cfg.SkipNavigators {
		navigators = BuildAllNavigators(e.cfg.Navigators, e.cfg.Symbol, histKlines, histRSX, histWozduh, e.cfg.Interval, e.cfg.HTF)
		if len(navigators) > 0 {
			if priceNav, ok := navigators["price"]; ok {
				navData = priceNav
			} else {
				for _, v := range navigators {
					navData = v
					break
				}
			}
		} else if e.cfg.Navigator.Enabled {
			startMs := navigatorChartStartMs(histKlines)
			maxTimeSec := navigatorMaxCloseTimeSec(histKlines)
			htfData := loadNavigatorHTFData(e.cfg.HTF, e.cfg.Symbol, e.cfg.Interval, startMs, maxTimeSec, e.cfg.Navigator.Periods)
			navData = BuildNavigatorResult(e.cfg.Navigator, histKlines, histRSX, histWozduh, e.cfg.Interval, e.cfg.HTF, htfData)
			navigators = map[string]NavigatorResultDTO{"price": navData}
		}
	}

	var finalChartData []BacktestChartPoint
	var finalSimData []BacktestSimPoint
	if e.cfg.SimOnly {
		finalSimData = simData
	} else {
		finalChartData = chartData
	}

	return &BacktestRunResult{
		Cancelled:     cancelled,
		ChartData:     finalChartData,
		SimData:       finalSimData,
		NavigatorData: navData,
		Navigators:    navigators,
		Annotations:   annotations,
	}
}

func backtestChartPointToSim(pt BacktestChartPoint) BacktestSimPoint {
	return BacktestSimPoint{
		Time:            pt.Time,
		Jurik:           pt.Jurik,
		RSX:             pt.RSX,
		RSXSignal:       pt.RSXSignal,
		RsiVolFast:      pt.RsiVolFast,
		RsiVolSlow:      pt.RsiVolSlow,
		VolCrossMarker:  pt.VolCrossMarker,
		Color:           pt.Color,
		Marker:          pt.Marker,
		VolumeSpikeUp:   pt.VolumeSpikeUp,
		VolumeSpikeDown: pt.VolumeSpikeDown,
	}
}

func populateBacktestPointFromFalcon(pt *BacktestChartPoint, sig FalconSignals, prevBlue float64, prevReady bool) {
	pt.RsiPrice = sig.RsiPrice
	pt.EmaRsi = sig.EmaRsi
	pt.RsiRsi = sig.RsiRsi
	pt.RsiHl2 = sig.RsiHl2
	pt.RsiVolFast = sig.RsiVolFast
	pt.RsiVolSlow = sig.RsiVolSlow
	pt.MacdRsi = sig.MacdRsi
	pt.RsiAd = sig.RsiAd
	pt.RsiHl2Vol = sig.RsiHl2Vol
	pt.VolCrossMarker = sig.VolCrossMarker
	pt.VolChanMid = sig.VolChanMid
	pt.VolChanUp = sig.VolChanUp
	pt.VolChanDn = sig.VolChanDn
	pt.PriceChanMid = sig.PriceChanMid
	pt.PriceChanUp = sig.PriceChanUp
	pt.PriceChanDn = sig.PriceChanDn
	if prevReady {
		pt.VolumeSpikeUp = DetectWozduxVolumeSpikeUp(prevBlue, sig.BlueLine, sig.RedLine)
		pt.VolumeSpikeDown = DetectWozduxVolumeSpikeDown(prevBlue, sig.BlueLine, sig.RedLine)
	}
}

func populateBacktestPointFromMarker(pt *BacktestChartPoint, marker *Marker, prevBlue float64, prevReady bool) {
	falcon := marker.FalconSnapshot()
	pt.RSX = falcon.JurikRSX
	pt.Jurik = falcon.JurikRSX
	pt.RSXSignal = falcon.JurikRSXSignal
	populateBacktestPointFromFalcon(pt, falcon, prevBlue, prevReady)
}

// candlesToKlines converts backtest candles to klines for the unified RSX pipeline.
func candlesToKlines(candles []exchange.Candle) []exchange.Kline {
	out := make([]exchange.Kline, len(candles))
	for i, c := range candles {
		out[i] = exchange.Kline{
			OpenTime:  c.OpenTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			CloseTime: c.CloseTime,
		}
	}
	return out
}

// ParseBacktestDateRange converts YYYY-MM-DD strings to inclusive millisecond bounds (UTC).
func ParseBacktestDateRange(startDate, endDate string) (int64, int64, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0, 0, fmt.Errorf("parse start date: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return 0, 0, fmt.Errorf("parse end date: %w", err)
	}
	if end.Before(start) {
		return 0, 0, fmt.Errorf("end date before start date")
	}
	end = end.Add(24*time.Hour - time.Millisecond)
	return start.UTC().UnixMilli(), end.UTC().UnixMilli(), nil
}
