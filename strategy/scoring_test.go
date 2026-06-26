package strategy

import (
	"testing"
)

func allEnabledScoringMatrix() ScoringMatrix {
	return ScoringMatrix{
		UseRSX:              true,
		UseWozduhCross:      true,
		UseRedCross:         true,
		UseGeometry:         true,
		UseGeometryBounce:   true,
		UseGeometryTriangle: true,
		UseTrendlines:       true,
		UseDivergence:       true,
		UseFib:              true,
		UseExpRegime:        true,
		UseJurikTrend:       true,
		UseWozduhSpike:      true,
		UseAD:               true,
		UseAOCross:          true,
	}
}

func testVolatilityOK() VolatilityState {
	return VolatilityState{ATR: 1.0, Regime: RegimeExpansion, LotModifier: 1.0, SafeStopDist: 1.0}
}

func testMarkerWithFlags(t *testing.T, mutate func(*Marker)) *Marker {
	t.Helper()
	klines := makeSyntheticKlines(60)
	m := NewMarker(klines, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	m.mu.Lock()
	mutate(m)
	m.mu.Unlock()
	return m
}

func scoreDecision(t *testing.T, m *Marker, matrix ScoringMatrix) ScoreDecision {
	t.Helper()
	return DefaultScoreEngine.Calculate(m, matrix)
}

func TestScoreEngine_BuyThreshold(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	m := testMarkerWithFlags(t, func(marker *Marker) {
		marker.rsxMarkers.markers[59] = "LL"
		marker.falconSignals.VolCrossMarker = "lime"
		marker.redLineCrossGreenUp = true
		marker.geometryState.IsBullishBreakout = true
		marker.volatilityState = testVolatilityOK()
	})
	decision := scoreDecision(t, m, allEnabledScoringMatrix())
	if decision.LongScore < LongScoreThreshold() {
		t.Fatalf("longScore = %d, want >= %d", decision.LongScore, LongScoreThreshold())
	}
}

func TestScoreEngine_ShortThreshold(t *testing.T) {
	SetScoringMatrix(allEnabledScoringMatrix())
	t.Cleanup(ResetScoringMatrix)

	m := testMarkerWithFlags(t, func(marker *Marker) {
		marker.rsxMarkers.markers[59] = "SS"
		marker.falconSignals.VolCrossMarker = "red"
		marker.redLineCrossGreenDown = true
		marker.geometryState.IsBearishBreakout = true
		marker.volatilityState = testVolatilityOK()
	})
	decision := scoreDecision(t, m, allEnabledScoringMatrix())
	if decision.ShortScore < ShortScoreThreshold() {
		t.Fatalf("shortScore = %d, want >= %d", decision.ShortScore, ShortScoreThreshold())
	}
}

func TestScoreEngine_DisabledMatrixWaits(t *testing.T) {
	m := testMarkerWithFlags(t, func(marker *Marker) {
		marker.rsxMarkers.markers[59] = "LL"
	})
	decision := scoreDecision(t, m, ScoringMatrix{})
	if decision.FinalAction != WaitAction {
		t.Fatalf("FinalAction = %q, want WAIT", decision.FinalAction)
	}
}

func TestScoreEngine_MTFTrendlinesFactor(t *testing.T) {
	m := testMarkerWithFlags(t, func(marker *Marker) {
		marker.mtfStates = map[string]*HTFState{
			"4h": {
				TrendLines: []NavigatorLineDTO{{
					Interval: "4h",
					Time1:    1,
					Time2:    9_999_999_999_999,
					Y1:       90,
					Y2:       100,
					Color:    navigatorColorBull,
					IsActive: true,
				}},
			},
		}
	})
	matrix := ScoringMatrix{UseTrendlines: true}
	decision := scoreDecision(t, m, matrix)
	if len(decision.Factors) == 0 {
		t.Fatal("expected MTF trendline factor")
	}
}

func TestScoreEngine_HTFOscillatorsFactor(t *testing.T) {
	m := testMarkerWithFlags(t, func(marker *Marker) {
		marker.mtfStates = map[string]*HTFState{
			"4h": {
				Interval:   "4h",
				RSXValue:   25,
				RSXColor:   "green",
				WozduhUp:   55,
				WozduhDown: 45,
			},
		}
	})
	matrix := ScoringMatrix{UseHTFOscillators: true}
	decision := scoreDecision(t, m, matrix)
	if decision.Factors["RSX_4h"].Direction != BuyAction {
		t.Fatalf("RSX_4h factor = %+v, want BUY", decision.Factors["RSX_4h"])
	}
	if decision.Factors["Wozduh_4h"].Direction != BuyAction {
		t.Fatalf("Wozduh_4h factor = %+v, want BUY", decision.Factors["Wozduh_4h"])
	}
	if decision.Factors["RSX_4h"].Score != scoreHTFRSX {
		t.Fatalf("RSX_4h score = %d, want %d", decision.Factors["RSX_4h"].Score, scoreHTFRSX)
	}
}

func TestScoreEngine_UsesExplicitMatrix(t *testing.T) {
	ResetScoringMatrix()
	t.Cleanup(ResetScoringMatrix)

	m := testMarkerWithFlags(t, func(marker *Marker) {
		marker.falconSignals.VolCrossMarker = "lime"
		marker.volatilityState = testVolatilityOK()
	})
	enabled := ScoringMatrix{UseWozduhCross: true, UseExpRegime: true}
	local := scoreDecision(t, m, enabled)
	if len(local.Factors) == 0 {
		t.Fatal("expected active factors from explicit matrix")
	}
	global := CalculateScoreGlobal(m)
	if len(global.Factors) != 0 {
		t.Fatalf("global matrix should be disabled, got %d factors", len(global.Factors))
	}
}
