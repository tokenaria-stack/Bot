package strategy

// BacktestRunSettings is the settings object sent by the dashboard (POST /api/backtest/run).
// JSON keys must match web/app.js buildFinalBacktestPayload() exactly.
type BacktestRunSettings struct {
	Matrix     ScoringMatrix                  `json:"matrix"`
	Navigators map[string]NavigatorUISettings `json:"navigators"`
	Risk       *RiskSettings                  `json:"risk,omitempty"`
}

// ResolveBacktestMatrix returns the scoring matrix from the request or active defaults.
func ResolveBacktestMatrix(settings *BacktestRunSettings) ScoringMatrix {
	if settings == nil {
		return GetScoringMatrix()
	}
	m := settings.Matrix
	if ScoringMatrixFullyDisabledFor(m) {
		global := GetScoringMatrix()
		if ScoringMatrixEntrySourcesEnabledFor(global) {
			return global
		}
		return defaultScoringMatrix
	}
	return m
}

// ResolveBacktestNavigators merges settings.navigators, top-level navigators, and legacy navigator.
func ResolveBacktestNavigators(settings *BacktestRunSettings, topLevel map[string]NavigatorUISettings, legacy NavigatorUISettings) map[string]NavigatorUISettings {
	chosen := topLevel
	if settings != nil && len(settings.Navigators) > 0 {
		chosen = settings.Navigators
	}
	if len(chosen) == 0 {
		if legacy.Enabled {
			chosen = map[string]NavigatorUISettings{"price": legacy}
		} else {
			return map[string]NavigatorUISettings{}
		}
	}

	out := make(map[string]NavigatorUISettings, len(chosen))
	for pane, ui := range chosen {
		// Always route by pane key so RSX/Wozduh use indicator scales, not price klines.
		ui.Source = navigatorPaneToSource(pane)
		out[pane] = normalizeNavigatorUISettings(ui)
	}
	return out
}
