package market

import "strings"

// BacktestRunSettings is the settings object sent by the dashboard (POST /api/backtest/run).
// JSON keys must match the web backtest payload. Matrix/risk/thresholds were purged in Phase F.
type BacktestRunSettings struct {
	Navigators     map[string]NavigatorUISettings `json:"navigators"`
	SlippagePct    float64                        `json:"slippage_pct,omitempty"`
	RSXSettings    *RSXSettings                   `json:"rsxSettings,omitempty"`
	WozduhSettings map[string]bool                `json:"wozduhSettings,omitempty"`
	SimOnly        bool                           `json:"simOnly"`        // If true, server omits OHLC from wire response
	SkipNavigators bool                           `json:"skipNavigators"` // If true, skip histRSX/histWozduh and navigator geometry
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
