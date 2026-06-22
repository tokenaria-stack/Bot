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

// SetSandboxMode toggles sandbox (risk bypass lives in RiskManager; scoring is unchanged).
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

// scalpMemory provides optional AI win-rate lookups (used by RiskManager).
type scalpMemory interface {
	PredictWinRate(ctx context.Context, snapshot vector_db.ReportSnapshot, k uint64) (float64, int, error)
}

// EvaluateScalpSignal sums all scoring factors; vetoes are handled by RiskManager.
func EvaluateScalpSignal(_ context.Context, report Report, _ float64, _ scalpMemory) ScalpDecision {
	if ScoringMatrixFullyDisabled() {
		return ScalpDecision{Action: WaitAction, Reason: "Matrix disabled"}
	}

	longScore := scoreLong(report)
	shortScore := scoreShort(report)

	bestScore := longScore
	if shortScore > bestScore {
		bestScore = shortScore
	}

	decision := ScalpDecision{
		LongScore:  longScore,
		ShortScore: shortScore,
		LotMod:     report.Volatility.LotModifier,
		StopDist:   report.Volatility.SafeStopDist,
	}
	if decision.LotMod <= 0 {
		decision.LotMod = 1.0
	}
	if decision.StopDist <= 0 {
		decision.StopDist = report.ATR
	}
	if decision.StopDist <= 0 {
		decision.StopDist = report.Close * 0.002
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

func scoringBelowThreshold(decision ScalpDecision) bool {
	if decision.LongScore >= decision.ShortScore {
		return decision.LongScore > 0 && decision.LongScore < LongScoreThreshold()
	}
	return decision.ShortScore > 0 && decision.ShortScore < ShortScoreThreshold()
}

func scoreLong(report Report) int {
	m := scoringMatrixSnapshot()
	score := 0

	if m.UseRSX {
		switch report.RSXMarker {
		case "LL":
			score += scoreRSXLL
		case "L":
			score += scoreRSXL
		}
	}
	if m.UseWozduhCross && report.Falcon.VolCrossMarker == "lime" {
		score += scalpVolCrossScore
	}
	if m.UseRedCross && report.RedLineCrossGreenUp {
		score += scalpRedCrossScore
	}
	if m.UseGeometry && report.Geometry.IsBullishBreakout {
		score += scalpBreakoutScore
	}
	if m.UseDivergence && report.Divergence.Score > 0 {
		score += report.Divergence.Score
	}
	if m.UseFib && fib618Active(report) {
		score += scalpFib618Score
	}
	if m.UseExpRegime && report.Volatility.Regime == RegimeExpansion {
		score += scalpExpansionScore
	}
	if m.UseJurikTrend {
		if report.JurikIsRising && report.JurikValue > 50 {
			score += scalpJurikBullScore
		} else if report.JurikIsRising && report.JurikValue <= 20 {
			score += scalpJurikRecoveryScore
		}
	}
	if m.UseWozduhSpike && report.WozduxVolumeSpikeUp {
		score += scalpWozduxVolumeScore
	}
	if m.UseGeometryBounce && report.GeometryBounceUp {
		score += scalpGeometryBounceScore
	}
	if m.UseGeometryTriangle && report.GeometryTriangle {
		score += scalpGeometryTriangleScore
	}
	if m.UseAD && report.AccumulationRising {
		score += scalpAccumulationScore
	}
	if m.UseAOCross && report.AOCrossZeroUp {
		score += scalpAOCrossScore
	}

	return score
}

func scoreShort(report Report) int {
	m := scoringMatrixSnapshot()
	score := 0

	if m.UseRSX {
		switch report.RSXMarker {
		case "SS":
			score += scoreRSXSS
		case "S":
			score += scoreRSXS
		}
	}
	if m.UseWozduhCross && report.Falcon.VolCrossMarker == "red" {
		score += scalpVolCrossScore
	}
	if m.UseRedCross && report.RedLineCrossGreenDown {
		score += scalpRedCrossScore
	}
	if m.UseGeometry && report.Geometry.IsBearishBreakout {
		score += scalpBreakoutScore
	}
	if m.UseDivergence && report.Divergence.Score < 0 {
		score += -report.Divergence.Score
	}
	if m.UseFib && fib618Active(report) {
		score += scalpFib618Score
	}
	if m.UseExpRegime && report.Volatility.Regime == RegimeExpansion {
		score += scalpExpansionScore
	}
	if m.UseJurikTrend && !report.JurikIsRising && report.JurikValue < 50 {
		score += scalpJurikBearScore
	}
	if m.UseWozduhSpike && report.WozduxVolumeSpikeDown {
		score += scalpWozduxVolumeScore
	}
	if m.UseGeometryBounce && report.GeometryBounceDown {
		score += scalpGeometryBounceScore
	}
	if m.UseGeometryTriangle && report.GeometryTriangle {
		score += scalpGeometryTriangleScore
	}
	if m.UseAD && report.DistributionFalling {
		score += scalpAccumulationScore
	}
	if m.UseAOCross && report.AOCrossZeroDown {
		score += scalpAOCrossScore
	}

	return score
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
