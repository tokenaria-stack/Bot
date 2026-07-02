package strategy

import "strings"

// BacktestRunSettings is the settings object sent by the dashboard (POST /api/backtest/run).
// JSON keys must match web/app.js buildFinalBacktestPayload() exactly.
type BacktestRunSettings struct {
	Matrix         ScoringMatrix                  `json:"matrix"`
	Navigators     map[string]NavigatorUISettings `json:"navigators"`
	Risk           *RiskSettings                  `json:"risk,omitempty"`
	SlippagePct    float64                        `json:"slippage_pct,omitempty"`
	LongThreshold  int                            `json:"longThreshold,omitempty"`
	ShortThreshold int                            `json:"shortThreshold,omitempty"`
	RSXSettings    *RSXSettings                   `json:"rsxSettings,omitempty"`
	WozduhSettings map[string]bool                `json:"wozduhSettings,omitempty"`
}

// ResolveBacktestThresholds returns isolated long/short entry thresholds for backtests.
// Does not read global dynamic thresholds — defaults to DefaultScoreThreshold when unset.
func ResolveBacktestThresholds(settings *BacktestRunSettings) (long, short int) {
	long = DefaultScoreThreshold
	short = DefaultScoreThreshold
	if settings == nil {
		return long, short
	}
	if settings.LongThreshold >= minScoreThreshold && settings.LongThreshold <= maxScoreThreshold {
		long = settings.LongThreshold
	}
	if settings.ShortThreshold >= minScoreThreshold && settings.ShortThreshold <= maxScoreThreshold {
		short = settings.ShortThreshold
	}
	return long, short
}

// ResolveBacktestRSXSettings returns clamped per-run RSX settings from the request payload.
func ResolveBacktestRSXSettings(settings *BacktestRunSettings) (RSXSettings, bool) {
	if settings == nil || settings.RSXSettings == nil {
		return RSXSettings{}, false
	}
	return NormalizeRSXSettings(*settings.RSXSettings), true
}

// ResolveBacktestSlippage returns slippage % per fill from the request or the default.
func ResolveBacktestSlippage(settings *BacktestRunSettings) float64 {
	if settings != nil && settings.SlippagePct > 0 {
		return settings.SlippagePct
	}
	return DefaultBacktestSlippagePct
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

// EnsureBacktestNavigatorsForMatrix forces navigator panes on when their scoring factors are active.
func EnsureBacktestNavigatorsForMatrix(navs map[string]NavigatorUISettings, matrix ScoringMatrix) {
	if navs == nil {
		return
	}
	if matrix.UseRSX {
		ui := navs["rsx"]
		ui.Enabled = true
		ui.Source = "RSX"
		navs["rsx"] = normalizeNavigatorUISettings(ui)
	}
	if matrix.UseWozduhCross || matrix.UseWozduhSpike || matrix.UseHTFOscillators {
		ui := navs["wozduh"]
		ui.Enabled = true
		ui.Source = "Wozduh"
		navs["wozduh"] = normalizeNavigatorUISettings(ui)
	}
	if matrix.UseTrendlines || matrix.UseHTFOscillators {
		ui := navs["price"]
		ui.Enabled = true
		navs["price"] = normalizeNavigatorUISettings(ui)
	}
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
