package strategy

import "strings"

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
		return DefaultScoringMatrix()
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

// ApplyMtfOptionsToNavigators toggles higher-TF periods on the price navigator from mtfOptions.
func ApplyMtfOptionsToNavigators(navs map[string]NavigatorUISettings, mtfOptions map[string]bool) {
	if len(navs) == 0 || len(mtfOptions) == 0 {
		return
	}
	ui, ok := navs["price"]
	if !ok {
		return
	}
	periodSet := make(map[string]struct{}, len(ui.Periods)+len(mtfOptions))
	for _, p := range ui.Periods {
		p = strings.TrimSpace(p)
		if p != "" {
			periodSet[p] = struct{}{}
		}
	}
	for tf, enabled := range mtfOptions {
		tf = strings.TrimSpace(tf)
		if tf == "" {
			continue
		}
		if enabled {
			periodSet[tf] = struct{}{}
		} else {
			delete(periodSet, tf)
		}
	}
	if len(periodSet) == 0 {
		ui.Periods = nil
	} else {
		ui.Periods = make([]string, 0, len(periodSet))
		for p := range periodSet {
			ui.Periods = append(ui.Periods, p)
		}
	}
	navs["price"] = ui
}
