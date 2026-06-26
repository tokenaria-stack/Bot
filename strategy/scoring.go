package strategy

import (
	"context"
	"fmt"
	"strings"

	"trading_bot/indicators"
	"trading_bot/vector_db"
)

const (
	DefaultFeeRate = 0.0012

	scoreRSXL  = 35
	scoreRSXLL = 45
	scoreRSXS  = 35
	scoreRSXSS = 45

	scoreWozduhCross      = 35
	scoreRedCross         = 35
	scoreBreakout         = 30
	scoreFib618           = 20
	scoreExpansion        = 15
	scoreJurikBull        = 20
	scoreJurikBear        = 20
	scoreJurikRecovery    = 15
	scoreWozduhVolume     = 15
	scoreGeometryBounce   = 25
	scoreGeometryTriangle = 10
	scoreAccumulation     = 20
	scoreAOCross          = 15
	scoreMTFTrendline     = 25
	scoreHTFRSX           = 20
	scoreHTFWozduhRegime  = 15

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

// TradeEvent notifies dashboard listeners about entry or exit fills.
type TradeEvent struct {
	Side    string
	Price   float64
	BarTime int64
	Reason  string
	Kind    string // "entry" or "exit"
}

type scoreMemory interface {
	PredictWinRate(ctx context.Context, snapshot vector_db.ReportSnapshot, k uint64) (float64, int, error)
}

type markerScoreSnapshot struct {
	close           float64
	barTimeMs       int64
	falcon          FalconSignals
	volatility      VolatilityState
	divergenceScore int
	geometry        GeometryState
	fibZones        []indicators.FibZone
	rsxMarker       string
	redCrossUp      bool
	redCrossDown    bool
	jurikValue      float64
	jurikRising     bool
	wozduxSpikeUp   bool
	wozduxSpikeDown bool
	geomBounceUp    bool
	geomBounceDown  bool
	geomTriangle    bool
	adRising        bool
	adFalling       bool
	aoCrossUp       bool
	aoCrossDown     bool
}

const minScoreBars = 50

func (a *Marker) scoreSnapshot() (markerScoreSnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.klines) < minScoreBars {
		return markerScoreSnapshot{}, false
	}
	last := a.klines[len(a.klines)-1]
	return markerScoreSnapshot{
		close:           last.Close,
		barTimeMs:       last.OpenTime,
		falcon:          a.falconSignals,
		volatility:      a.volatilityState,
		divergenceScore: a.divSignal.Score,
		geometry:        a.geometryState,
		fibZones:        append([]indicators.FibZone(nil), a.fibZones...),
		rsxMarker:       a.rsxMarkers.recentTradingMarker(RSXSignalMemoryBars),
		redCrossUp:      a.redLineCrossGreenUp,
		redCrossDown:    a.redLineCrossGreenDown,
		jurikValue:      a.jurikValue,
		jurikRising:     a.jurikIsRising,
		wozduxSpikeUp:   a.wozduxVolumeSpikeUp,
		wozduxSpikeDown: a.wozduxVolumeSpikeDown,
		geomBounceUp:    a.geometryBounceUp,
		geomBounceDown:  a.geometryBounceDown,
		geomTriangle:    a.geometryTriangle,
		adRising:        a.accumulationRising,
		adFalling:       a.distributionFalling,
		aoCrossUp:       a.aoCrossZeroUp,
		aoCrossDown:     a.aoCrossZeroDown,
	}, true
}

// Calculate evaluates Marker state against the scoring matrix and returns a decision.
func (e *ScoreEngine) Calculate(marker *Marker, matrix ScoringMatrix) ScoreDecision {
	decision := ScoreDecision{
		RawAction:   WaitAction,
		FinalAction: WaitAction,
		Factors:     make(map[string]ScoreFactor),
	}
	if e == nil || marker == nil || ScoringMatrixFullyDisabledFor(matrix) {
		return decision
	}

	snap, ok := marker.scoreSnapshot()
	if !ok {
		return decision
	}

	decision.LotMod = snap.volatility.LotModifier
	decision.StopDist = snap.volatility.SafeStopDist
	if decision.LotMod <= 0 {
		decision.LotMod = 1.0
	}
	if decision.StopDist <= 0 {
		decision.StopDist = snap.volatility.ATR
	}
	if decision.StopDist <= 0 {
		decision.StopDist = snap.close * 0.002
	}

	collectScoreFactors(&decision, snap, matrix)
	if matrix.UseTrendlines {
		for key, factor := range scoreMTFFactors(marker, snap.close, snap.barTimeMs) {
			addScoreFactor(&decision, key, factor)
		}
	}
	if matrix.UseHTFOscillators {
		mergeHTFFactors(&decision, scoreHTFOscillatorFactors(marker))
	}

	for _, factor := range decision.Factors {
		switch factor.Direction {
		case BuyAction:
			decision.LongScore += factor.Score
		case SellAction:
			decision.ShortScore += factor.Score
		}
	}
	longTh := LongScoreThreshold()
	shortTh := ShortScoreThreshold()
	if decision.LongScore >= longTh && decision.LongScore > decision.ShortScore {
		decision.RawAction = BuyAction
		decision.FinalAction = BuyAction
		decision.Reason = decisionReason(decision, true)
		return decision
	}
	if decision.ShortScore >= shortTh && decision.ShortScore > decision.LongScore {
		decision.RawAction = SellAction
		decision.FinalAction = SellAction
		decision.Reason = decisionReason(decision, false)
		return decision
	}

	decision.RawAction = WaitAction
	decision.FinalAction = WaitAction
	decision.Reason = "No clear signal"
	return decision
}

// CalculateScore is a convenience wrapper around DefaultScoreEngine.Calculate.
func CalculateScore(marker *Marker, matrix ScoringMatrix) ScoreDecision {
	return DefaultScoreEngine.Calculate(marker, matrix)
}

// CalculateScoreGlobal uses the live global ScoringMatrix snapshot.
func CalculateScoreGlobal(marker *Marker) ScoreDecision {
	return DefaultScoreEngine.Calculate(marker, scoringMatrixSnapshot())
}

func addScoreFactor(decision *ScoreDecision, key string, factor ScoreFactor) {
	if factor.Score < minScoreThreshold {
		return
	}
	decision.Factors[key] = factor
}

func collectScoreFactors(decision *ScoreDecision, snap markerScoreSnapshot, m ScoringMatrix) {
	add := func(key string, factor ScoreFactor) {
		addScoreFactor(decision, key, factor)
	}

	if m.UseRSX {
		switch snap.rsxMarker {
		case "LL":
			add("RSX", ScoreFactor{Name: "RSX LL", Direction: BuyAction, Score: scoreRSXLL, Reason: "RSX LL"})
		case "L":
			add("RSX", ScoreFactor{Name: "RSX L", Direction: BuyAction, Score: scoreRSXL, Reason: "RSX L"})
		case "SS":
			add("RSX", ScoreFactor{Name: "RSX SS", Direction: SellAction, Score: scoreRSXSS, Reason: "RSX SS"})
		case "S":
			add("RSX", ScoreFactor{Name: "RSX S", Direction: SellAction, Score: scoreRSXS, Reason: "RSX S"})
		}
	}
	if m.UseWozduhCross && snap.falcon.VolCrossMarker == "lime" {
		add("WozduhCross", ScoreFactor{Name: "Wozduh cross", Direction: BuyAction, Score: scoreWozduhCross, Reason: "Wozduh cross"})
	}
	if m.UseWozduhCross && snap.falcon.VolCrossMarker == "red" {
		add("WozduhCross", ScoreFactor{Name: "Wozduh cross", Direction: SellAction, Score: scoreWozduhCross, Reason: "Wozduh cross"})
	}
	if m.UseRedCross && snap.redCrossUp {
		add("RedCross", ScoreFactor{Name: "Red×Green", Direction: BuyAction, Score: scoreRedCross, Reason: "Red×Green"})
	}
	if m.UseRedCross && snap.redCrossDown {
		add("RedCross", ScoreFactor{Name: "Red×Green", Direction: SellAction, Score: scoreRedCross, Reason: "Red×Green"})
	}
	if m.UseGeometry && snap.geometry.IsBullishBreakout {
		add("Breakout", ScoreFactor{Name: "Breakout", Direction: BuyAction, Score: scoreBreakout, Reason: "Breakout"})
	}
	if m.UseGeometry && snap.geometry.IsBearishBreakout {
		add("Breakout", ScoreFactor{Name: "Breakout", Direction: SellAction, Score: scoreBreakout, Reason: "Breakout"})
	}
	if m.UseDivergence && snap.divergenceScore > 0 {
		add("Divergence", ScoreFactor{Name: "Divergence", Direction: BuyAction, Score: snap.divergenceScore, Reason: "Divergence"})
	}
	if m.UseDivergence && snap.divergenceScore < 0 {
		add("Divergence", ScoreFactor{Name: "Divergence", Direction: SellAction, Score: -snap.divergenceScore, Reason: "Divergence"})
	}
	if m.UseFib && fib618Active(snap.fibZones) {
		add("Fib618_L", ScoreFactor{Name: "Fib 0.618", Direction: BuyAction, Score: scoreFib618, Reason: "Fib 0.618"})
		add("Fib618_S", ScoreFactor{Name: "Fib 0.618", Direction: SellAction, Score: scoreFib618, Reason: "Fib 0.618"})
	}
	if m.UseExpRegime && snap.volatility.Regime == RegimeExpansion {
		add("Expansion_L", ScoreFactor{Name: "Expansion", Direction: BuyAction, Score: scoreExpansion, Reason: "Expansion"})
		add("Expansion_S", ScoreFactor{Name: "Expansion", Direction: SellAction, Score: scoreExpansion, Reason: "Expansion"})
	}
	if m.UseJurikTrend {
		if snap.jurikRising && snap.jurikValue > 50 {
			add("JurikBull", ScoreFactor{Name: "Jurik bull", Direction: BuyAction, Score: scoreJurikBull, Reason: "Jurik bull"})
		} else if snap.jurikRising && snap.jurikValue <= 20 {
			add("JurikRecovery", ScoreFactor{Name: "Jurik recovery", Direction: BuyAction, Score: scoreJurikRecovery, Reason: "Jurik recovery"})
		} else if !snap.jurikRising && snap.jurikValue < 50 {
			add("JurikBear", ScoreFactor{Name: "Jurik bear", Direction: SellAction, Score: scoreJurikBear, Reason: "Jurik bear"})
		}
	}
	if m.UseWozduhSpike && snap.wozduxSpikeUp {
		add("WozduhSpike", ScoreFactor{Name: "Wozduh spike", Direction: BuyAction, Score: scoreWozduhVolume, Reason: "Wozduh spike"})
	}
	if m.UseWozduhSpike && snap.wozduxSpikeDown {
		add("WozduhSpike", ScoreFactor{Name: "Wozduh spike", Direction: SellAction, Score: scoreWozduhVolume, Reason: "Wozduh spike"})
	}
	if m.UseGeometryBounce && snap.geomBounceUp {
		add("GeomBounce", ScoreFactor{Name: "Geometry bounce", Direction: BuyAction, Score: scoreGeometryBounce, Reason: "Geometry bounce"})
	}
	if m.UseGeometryBounce && snap.geomBounceDown {
		add("GeomBounce", ScoreFactor{Name: "Geometry bounce", Direction: SellAction, Score: scoreGeometryBounce, Reason: "Geometry bounce"})
	}
	if m.UseGeometryTriangle && snap.geomTriangle {
		add("GeomTri_L", ScoreFactor{Name: "Geometry triangle", Direction: BuyAction, Score: scoreGeometryTriangle, Reason: "Geometry triangle"})
		add("GeomTri_S", ScoreFactor{Name: "Geometry triangle", Direction: SellAction, Score: scoreGeometryTriangle, Reason: "Geometry triangle"})
	}
	if m.UseAD && snap.adRising {
		add("Accumulation", ScoreFactor{Name: "Accumulation", Direction: BuyAction, Score: scoreAccumulation, Reason: "Accumulation"})
	}
	if m.UseAD && snap.adFalling {
		add("Distribution", ScoreFactor{Name: "Distribution", Direction: SellAction, Score: scoreAccumulation, Reason: "Distribution"})
	}
	if m.UseAOCross && snap.aoCrossUp {
		add("AOCross", ScoreFactor{Name: "AO cross up", Direction: BuyAction, Score: scoreAOCross, Reason: "AO cross up"})
	}
	if m.UseAOCross && snap.aoCrossDown {
		add("AOCross", ScoreFactor{Name: "AO cross down", Direction: SellAction, Score: scoreAOCross, Reason: "AO cross down"})
	}
}

func scoreMTFFactors(marker *Marker, close float64, barTimeMs int64) map[string]ScoreFactor {
	states := marker.MTFStates()
	if len(states) == 0 {
		return nil
	}
	out := make(map[string]ScoreFactor)
	for tf, st := range states {
		if st == nil {
			continue
		}
		for i, line := range st.TrendLines {
			if !line.IsActive {
				continue
			}
			linePrice := navigatorLinePriceAtTime(line, barTimeMs)
			if linePrice <= 0 {
				continue
			}
			key := fmt.Sprintf("MTF_%s_%d", tf, i)
			if isBullNavigatorColor(line.Color) && close >= linePrice {
				out[key] = ScoreFactor{
					Name:      "MTF " + tf + " support",
					Direction: BuyAction,
					Score:     scoreMTFTrendline,
					Reason:    "HTF support hold",
				}
			}
			if isBearNavigatorColor(line.Color) && close <= linePrice {
				out[key] = ScoreFactor{
					Name:      "MTF " + tf + " resistance",
					Direction: SellAction,
					Score:     scoreMTFTrendline,
					Reason:    "HTF resistance",
				}
			}
		}
	}
	return out
}

func scoreHTFOscillatorFactors(marker *Marker) map[string]ScoreFactor {
	states := marker.MTFStates()
	if len(states) == 0 {
		return nil
	}
	out := make(map[string]ScoreFactor)
	for tf, st := range states {
		if st == nil {
			continue
		}

		rsxKey := "RSX_" + tf
		rsxFactor := ScoreFactor{
			Name:      "HTF RSX " + tf,
			Direction: WaitAction,
			Score:     0,
			Reason:    fmt.Sprintf("HTF (%s) RSX %.1f — neutral zone", tf, st.RSXValue),
		}
		if st.RSXValue < 30 && st.RSXColor == "green" {
			rsxFactor.Direction = BuyAction
			rsxFactor.Score = scoreHTFRSX
			rsxFactor.Reason = fmt.Sprintf("Oversold HTF (%s) turning up", tf)
		} else if st.RSXValue > 70 && st.RSXColor == "red" {
			rsxFactor.Direction = SellAction
			rsxFactor.Score = scoreHTFRSX
			rsxFactor.Reason = fmt.Sprintf("Overbought HTF (%s) turning down", tf)
		}
		out[rsxKey] = rsxFactor

		wozKey := "Wozduh_" + tf
		if st.WozduhUp > 0 && st.WozduhDown > 0 {
			if st.WozduhUp > st.WozduhDown {
				out[wozKey] = ScoreFactor{
					Name:      "HTF Wozduh " + tf,
					Direction: BuyAction,
					Score:     scoreHTFWozduhRegime,
					Reason:    fmt.Sprintf("HTF (%s) Wozduh Bullish Regime", tf),
				}
			} else if st.WozduhDown > st.WozduhUp {
				out[wozKey] = ScoreFactor{
					Name:      "HTF Wozduh " + tf,
					Direction: SellAction,
					Score:     scoreHTFWozduhRegime,
					Reason:    fmt.Sprintf("HTF (%s) Wozduh Bearish Regime", tf),
				}
			} else {
				out[wozKey] = ScoreFactor{
					Name:      "HTF Wozduh " + tf,
					Direction: WaitAction,
					Score:     0,
					Reason:    fmt.Sprintf("HTF (%s) Wozduh neutral (%.1f / %.1f)", tf, st.WozduhUp, st.WozduhDown),
				}
			}
		}
	}
	return out
}

// mergeHTFFactors adds HTF telemetry factors without the generic min-score gate (score may be 0).
func mergeHTFFactors(decision *ScoreDecision, factors map[string]ScoreFactor) {
	if decision == nil || len(factors) == 0 {
		return
	}
	if decision.Factors == nil {
		decision.Factors = make(map[string]ScoreFactor, len(factors))
	}
	for key, factor := range factors {
		decision.Factors[key] = factor
	}
}

func isBullNavigatorColor(color string) bool {
	return strings.Contains(strings.ToLower(color), "089981") || strings.Contains(strings.ToLower(color), "085def")
}

func isBearNavigatorColor(color string) bool {
	return strings.Contains(strings.ToLower(color), "f23645") || strings.Contains(strings.ToLower(color), "ff5d00")
}

func fib618Active(zones []indicators.FibZone) bool {
	for _, zone := range zones {
		if zone.Ratio == 0.618 && zone.IsActive {
			return true
		}
	}
	return false
}

func decisionReason(decision ScoreDecision, long bool) string {
	direction := "SHORT"
	want := SellAction
	if long {
		direction = "LONG"
		want = BuyAction
	}
	var parts []string
	for _, f := range decision.Factors {
		if f.Direction == want && f.Name != "" {
			parts = append(parts, f.Name)
		}
	}
	if len(parts) == 0 {
		return direction + " signal"
	}
	return direction + ": " + strings.Join(parts, " + ")
}

func scoringBelowThreshold(decision ScoreDecision) bool {
	if decision.LongScore >= decision.ShortScore {
		return decision.LongScore > 0 && decision.LongScore < LongScoreThreshold()
	}
	return decision.ShortScore > 0 && decision.ShortScore < ShortScoreThreshold()
}

// TelemetryAIStatus reports Qdrant memory readiness for the dashboard.
func TelemetryAIStatus(ctx context.Context, marker *Marker, memory scoreMemory) string {
	if memory == nil || marker == nil {
		return "Offline"
	}
	if ctx == nil {
		ctx = context.Background()
	}
	winRate, count, err := memory.PredictWinRate(ctx, marker.VectorSnapshot(), aiSearchLimit)
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

// DefaultScalpFeeRate is kept as an alias for backward compatibility in backtest config.
const DefaultScalpFeeRate = DefaultFeeRate
