package market

// DefaultLiveNavigatorPanes returns dashboard default navigator settings for live trading.
func DefaultLiveNavigatorPanes() map[string]NavigatorUISettings {
	return map[string]NavigatorUISettings{
		"price": {
			Enabled:   true,
			Source:    navigatorSourcePrice,
			TrendType: NavigatorTrendWicks,
			UseLong:   true,
			LongLen:   60,
			UseMedium: true,
			MediumLen: 30,
			UseShort:  true,
			ShortLen:  10,
		},
	}
}

// ResolveNavigatorPanes normalizes pane maps for live navigator settings.
func ResolveNavigatorPanes(chosen map[string]NavigatorUISettings, legacy NavigatorUISettings) map[string]NavigatorUISettings {
	if len(chosen) == 0 {
		if legacy.Enabled {
			chosen = map[string]NavigatorUISettings{"price": legacy}
		} else {
			return map[string]NavigatorUISettings{}
		}
	}
	out := make(map[string]NavigatorUISettings, len(chosen))
	for pane, ui := range chosen {
		ui.Source = navigatorPaneToSource(pane)
		out[pane] = normalizeNavigatorUISettings(ui)
	}
	return out
}
