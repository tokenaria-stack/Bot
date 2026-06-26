package strategy

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
