package indicators

// IndicatorFactEvent is a closed-bar knowable fact (not a trade decision).
// Times are canonical closed-bar OpenTime in Unix milliseconds.
type IndicatorFactEvent struct {
	Source      string
	Direction   string
	Pattern     string
	ConfirmedAt int64
	AnchorAt    int64
	AnchorValue float64
	AnchorPrice float64
}

const (
	FactSourceRSXTVDiv   = "rsx_tv_div"
	FactSourceRSXTVPivot = "rsx_tv_pivot"
	FactSourceRSXZZDiv   = "rsx_zz_div"

	FactDirBullish   = "bullish"
	FactDirBearish   = "bearish"
	FactDirPivotHigh = "high"
	FactDirPivotLow  = "low"

	FactPatternRegular = "regular"
	FactPatternHidden  = "hidden"
)
