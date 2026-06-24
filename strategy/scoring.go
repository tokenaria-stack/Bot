package strategy

import (
	"context"
	"fmt"
	"strings"

	"trading_bot/vector_db"
)

const (
	scalpFeeMultiplier  = 3
	DefaultScalpFeeRate = 0.0012

	// RSX chart pivot / divergence markers
	scoreRSXL  = 35
	scoreRSXLL = 45
	scoreRSXS  = 35
	scoreRSXSS = 45

	// Wozduh wt11×wt22 cross dots
	scalpVolCrossScore = 35

	// Legacy Layer 3 scoring matrix
	scalpRedCrossScore         = 35
	scalpBreakoutScore         = 30
	scalpFib618Score           = 20
	scalpExpansionScore        = 15
	scalpJurikBullScore        = 20
	scalpJurikBearScore        = 20
	scalpJurikRecoveryScore    = 15
	scalpWozduxVolumeScore     = 15
	scalpGeometryBounceScore   = 25
	scalpGeometryTriangleScore = 10
	scalpAccumulationScore     = 20
	scalpAOCrossScore          = 15

	virtualTPRiskMultiple = 2.0
)

var sandboxMode bool

// SetSandboxMode toggles sandbox mode flag (legacy; Analyst pipe is pass-through in Phase 1).
func SetSandboxMode(enabled bool) {
	sandboxMode = enabled
}

// SandboxModeEnabled reports whether sandbox mode is active.
func SandboxModeEnabled() bool {
	return sandboxMode
}

const (
	aiVetoMinNeighbors = 3
	aiVetoMaxWinRate   = 0.40
	aiSearchLimit      = 5
)

// ActionType is the scalp entry verdict.
type ActionType string

const (
	BuyAction  ActionType = "BUY"
	SellAction ActionType = "SELL"
	WaitAction ActionType = "WAIT"
)

// ScalpDecision is the Layer 3 verdict produced from a market Report.
type ScalpDecision struct {
	Action     ActionType
	Score      int
	LongScore  int
	ShortScore int
	LotMod     float64
	StopDist   float64
	Reason     string
}

// TradeEvent notifies dashboard listeners about entry or exit fills.
type TradeEvent struct {
	Side    string
	Price   float64
	BarTime int64
	Reason  string
	Kind    string // "entry" or "exit"
}

// scalpMemory provides optional AI win-rate lookups (legacy telemetry).
type scalpMemory interface {
	PredictWinRate(ctx context.Context, snapshot vector_db.ReportSnapshot, k uint64) (float64, int, error)
}

// SignalInfo describes one active scoring factor that passed matrix and threshold checks.
type SignalInfo struct {
	Name      string     `json:"name"`
	Direction ActionType `json:"direction"`
	Score     int        `json:"score"`
	Reason    string     `json:"reason,omitempty"`
}

// ScoreResult is the per-signal output of ProcessScore (no aggregated score).
type ScoreResult struct {
	ActiveSignals []SignalInfo `json:"activeSignals"`
	LotMod        float64      `json:"lotMod"`
	StopDist      float64      `json:"stopDist"`
}

// ProcessScore evaluates each scoring factor against the live global ScoringMatrix.
func ProcessScore(_ context.Context, report Report, _ float64, _ scalpMemory) ScoreResult {
	return ProcessScoreForMatrix(report, scoringMatrixSnapshot())
}

// ProcessScoreForMatrix evaluates factors against an explicit matrix (e.g. backtest config).
func ProcessScoreForMatrix(report Report, matrix ScoringMatrix) ScoreResult {
	if ScoringMatrixFullyDisabledFor(matrix) {
		return ScoreResult{}
	}

	result := ScoreResult{
		ActiveSignals: collectActiveSignalsFor(report, matrix),
		LotMod:        report.Volatility.LotModifier,
		StopDist:      report.Volatility.SafeStopDist,
	}
	if result.LotMod <= 0 {
		result.LotMod = 1.0
	}
	if result.StopDist <= 0 {
		result.StopDist = report.ATR
	}
	if result.StopDist <= 0 {
		result.StopDist = report.Close * 0.002
	}
	return result
}

// ScalpDecisionFromScoreResult adapts ScoreResult to legacy ScalpDecision for execution compatibility.
func ScalpDecisionFromScoreResult(result ScoreResult, report Report) ScalpDecision {
	longScore := 0
	shortScore := 0
	for _, sig := range result.ActiveSignals {
		switch sig.Direction {
		case BuyAction:
			longScore += sig.Score
		case SellAction:
			shortScore += sig.Score
		}
	}

	bestScore := longScore
	if shortScore > bestScore {
		bestScore = shortScore
	}

	decision := ScalpDecision{
		LongScore:  longScore,
		ShortScore: shortScore,
		LotMod:     result.LotMod,
		StopDist:   result.StopDist,
	}

	longTh := LongScoreThreshold()
	shortTh := ShortScoreThreshold()

	if longScore >= longTh && longScore > shortScore {
		decision.Action = BuyAction
		decision.Score = longScore
		decision.Reason = longReason(report)
		return decision
	}

	if shortScore >= shortTh && shortScore > longScore {
		decision.Action = SellAction
		decision.Score = shortScore
		decision.Reason = shortReason(report)
		return decision
	}

	decision.Action = WaitAction
	decision.Score = bestScore
	decision.Reason = "No clear signal"
	return decision
}

func collectActiveSignalsFor(report Report, m ScoringMatrix) []SignalInfo {
	var signals []SignalInfo

	add := func(name string, dir ActionType, score int, reason string) {
		if score >= minScoreThreshold {
			signals = append(signals, SignalInfo{
				Name:      name,
				Direction: dir,
				Score:     score,
				Reason:    reason,
			})
		}
	}

	if m.UseRSX {
		switch report.RSXMarker {
		case "LL":
			add("RSX LL", BuyAction, scoreRSXLL, "RSX LL")
		case "L":
			add("RSX L", BuyAction, scoreRSXL, "RSX L")
		case "SS":
			add("RSX SS", SellAction, scoreRSXSS, "RSX SS")
		case "S":
			add("RSX S", SellAction, scoreRSXS, "RSX S")
		}
	}
	if m.UseWozduhCross && report.Falcon.VolCrossMarker == "lime" {
		add("Wozduh cross", BuyAction, scalpVolCrossScore, "Wozduh cross")
	}
	if m.UseWozduhCross && report.Falcon.VolCrossMarker == "red" {
		add("Wozduh cross", SellAction, scalpVolCrossScore, "Wozduh cross")
	}
	if m.UseRedCross && report.RedLineCrossGreenUp {
		add("Red×Green", BuyAction, scalpRedCrossScore, "Red×Green")
	}
	if m.UseRedCross && report.RedLineCrossGreenDown {
		add("Red×Green", SellAction, scalpRedCrossScore, "Red×Green")
	}
	if m.UseGeometry && report.Geometry.IsBullishBreakout {
		add("Breakout", BuyAction, scalpBreakoutScore, "Breakout")
	}
	if m.UseGeometry && report.Geometry.IsBearishBreakout {
		add("Breakout", SellAction, scalpBreakoutScore, "Breakout")
	}
	if m.UseDivergence && report.Divergence.Score > 0 {
		add("Divergence", BuyAction, report.Divergence.Score, "Divergence")
	}
	if m.UseDivergence && report.Divergence.Score < 0 {
		add("Divergence", SellAction, -report.Divergence.Score, "Divergence")
	}
	if m.UseFib && fib618Active(report) {
		add("Fib 0.618", BuyAction, scalpFib618Score, "Fib 0.618")
		add("Fib 0.618", SellAction, scalpFib618Score, "Fib 0.618")
	}
	if m.UseExpRegime && report.Volatility.Regime == RegimeExpansion {
		add("Expansion", BuyAction, scalpExpansionScore, "Expansion")
		add("Expansion", SellAction, scalpExpansionScore, "Expansion")
	}
	if m.UseJurikTrend {
		if report.JurikIsRising && report.JurikValue > 50 {
			add("Jurik bull", BuyAction, scalpJurikBullScore, "Jurik bull")
		} else if report.JurikIsRising && report.JurikValue <= 20 {
			add("Jurik recovery", BuyAction, scalpJurikRecoveryScore, "Jurik recovery")
		} else if !report.JurikIsRising && report.JurikValue < 50 {
			add("Jurik bear", SellAction, scalpJurikBearScore, "Jurik bear")
		}
	}
	if m.UseWozduhSpike && report.WozduxVolumeSpikeUp {
		add("Wozduh spike", BuyAction, scalpWozduxVolumeScore, "Wozduh spike")
	}
	if m.UseWozduhSpike && report.WozduxVolumeSpikeDown {
		add("Wozduh spike", SellAction, scalpWozduxVolumeScore, "Wozduh spike")
	}
	if m.UseGeometryBounce && report.GeometryBounceUp {
		add("Geometry bounce", BuyAction, scalpGeometryBounceScore, "Geometry bounce")
	}
	if m.UseGeometryBounce && report.GeometryBounceDown {
		add("Geometry bounce", SellAction, scalpGeometryBounceScore, "Geometry bounce")
	}
	if m.UseGeometryTriangle && report.GeometryTriangle {
		add("Geometry triangle", BuyAction, scalpGeometryTriangleScore, "Geometry triangle")
		add("Geometry triangle", SellAction, scalpGeometryTriangleScore, "Geometry triangle")
	}
	if m.UseAD && report.AccumulationRising {
		add("Accumulation", BuyAction, scalpAccumulationScore, "Accumulation")
	}
	if m.UseAD && report.DistributionFalling {
		add("Distribution", SellAction, scalpAccumulationScore, "Distribution")
	}
	if m.UseAOCross && report.AOCrossZeroUp {
		add("AO cross up", BuyAction, scalpAOCrossScore, "AO cross up")
	}
	if m.UseAOCross && report.AOCrossZeroDown {
		add("AO cross down", SellAction, scalpAOCrossScore, "AO cross down")
	}

	return signals
}

func scoringBelowThreshold(decision ScalpDecision) bool {
	if decision.LongScore >= decision.ShortScore {
		return decision.LongScore > 0 && decision.LongScore < LongScoreThreshold()
	}
	return decision.ShortScore > 0 && decision.ShortScore < ShortScoreThreshold()
}

func fib618Active(report Report) bool {
	for _, zone := range report.FibZones {
		if zone.Ratio == 0.618 && zone.IsActive {
			return true
		}
	}
	return false
}

func longReason(report Report) string {
	return signalReason("LONG", report, true)
}

func shortReason(report Report) string {
	return signalReason("SHORT", report, false)
}

func signalReason(direction string, report Report, long bool) string {
	parts := []string{}
	if long {
		appendRSXReason(&parts, report.RSXMarker, "L", "LL")
		if report.Falcon.VolCrossMarker == "lime" {
			parts = append(parts, "Wozduh cross")
		}
		if report.RedLineCrossGreenUp {
			parts = append(parts, "Red×Green")
		}
		if report.Geometry.IsBullishBreakout {
			parts = append(parts, "Breakout")
		}
		if report.Divergence.Score > 0 {
			parts = append(parts, "Divergence")
		}
		if report.AOCrossZeroUp {
			parts = append(parts, "AO cross up")
		}
	} else {
		appendRSXReason(&parts, report.RSXMarker, "S", "SS")
		if report.Falcon.VolCrossMarker == "red" {
			parts = append(parts, "Wozduh cross")
		}
		if report.RedLineCrossGreenDown {
			parts = append(parts, "Red×Green")
		}
		if report.Geometry.IsBearishBreakout {
			parts = append(parts, "Breakout")
		}
		if report.Divergence.Score < 0 {
			parts = append(parts, "Divergence")
		}
		if report.AOCrossZeroDown {
			parts = append(parts, "AO cross down")
		}
	}
	if len(parts) == 0 {
		return direction + " signal"
	}
	return direction + ": " + strings.Join(parts, " + ")
}

func appendRSXReason(parts *[]string, marker, single, strong string) {
	switch marker {
	case strong, single:
		*parts = append(*parts, "RSX "+marker)
	}
}

// TelemetryAIStatus reports Qdrant memory readiness for the dashboard.
func TelemetryAIStatus(ctx context.Context, report Report, memory scalpMemory) string {
	if memory == nil {
		return "Offline"
	}
	if ctx == nil {
		ctx = context.Background()
	}

	winRate, count, err := memory.PredictWinRate(ctx, report.VectorSnapshot(), aiSearchLimit)
	if err != nil {
		return "Error"
	}
	if count < aiVetoMinNeighbors {
		return "Learning"
	}
	if winRate < aiVetoMaxWinRate {
		return "Warning"
	}
	return "Clear"
}

// VirtualTakeProfitPrice returns the paper take-profit level for a virtual position.
func VirtualTakeProfitPrice(entrySide string, entryPrice, stopPrice float64) float64 {
	risk := entryPrice - stopPrice
	if entrySide == "SELL" {
		risk = stopPrice - entryPrice
	}
	if risk <= 0 {
		return 0
	}
	if entrySide == "BUY" {
		return entryPrice + virtualTPRiskMultiple*risk
	}
	return entryPrice - virtualTPRiskMultiple*risk
}

// FormatExitReason builds a human-readable exit reason for logs and markers.
func FormatExitReason(trigger string, side string) string {
	return fmt.Sprintf("Virtual Exit (%s): %s", side, trigger)
}
