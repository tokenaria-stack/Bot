package indicators

// IndicatorFactEvent is a closed-bar knowable fact (not a trade decision).
// Times are canonical closed-bar OpenTime in Unix milliseconds.
type IndicatorFactEvent struct {
	Source      string
	Direction   string
	ConfirmedAt int64
	AnchorAt    int64
	AnchorValue float64
	AnchorPrice float64
}

const (
	FactSourceRSXTVDiv = "rsx_tv_div"

	FactDirBullish = "bullish"
	FactDirBearish = "bearish"
)
