package exchange

const parent15mMs = 15 * 60_000

// ChildSupport is VOLUME-SOURCE-ARBITRATION-1 first-step classification.
type ChildSupport string

const (
	SupportBothInternal ChildSupport = "BOTH_INTERNALLY_CONSISTENT"
	SupportRESTOnly     ChildSupport = "REST_1M_SUPPORTS_REST_15M"
	SupportVisionOnly   ChildSupport = "VISION_1M_SUPPORTS_VISION_15M"
	SupportNeither      ChildSupport = "NEITHER"
	SupportIncomplete   ChildSupport = "INCOMPLETE_CHILDREN"
)

// SumChildBase sums BaseVolume (and quote/trades) for children in [parentOT, parentOT+dur).
func SumChildBase(parentOT, dur int64, kids []VolumeAuthority) (n int, vol, quote float64, trades int64, open, high, low, close float64) {
	first := true
	for _, k := range kids {
		if k.OpenTime < parentOT || k.OpenTime >= parentOT+dur {
			continue
		}
		n++
		vol += k.Base
		quote += k.Quote
		trades += k.Trades
		if first {
			open, high, low, close = k.Open, k.High, k.Low, k.Close
			first = false
			continue
		}
		if k.High > high {
			high = k.High
		}
		if k.Low < low {
			low = k.Low
		}
		close = k.Close
	}
	return n, vol, quote, trades, open, high, low, close
}

// Classify15mChildSupport compares 15m parent v to the sum of 15 one-minute children.
func Classify15mChildSupport(rest15, vis15 float64, restKids, visKids int, restSum, visSum float64) ChildSupport {
	need := 15
	if restKids != need || visKids != need {
		if restKids != need && visKids != need {
			return SupportIncomplete
		}
	}
	restOK := restKids == need && VolumeFloatEqual(restSum, rest15)
	visOK := visKids == need && VolumeFloatEqual(visSum, vis15)
	switch {
	case restOK && visOK:
		return SupportBothInternal
	case restOK && !visOK:
		return SupportRESTOnly
	case visOK && !restOK:
		return SupportVisionOnly
	case restKids != need || visKids != need:
		return SupportIncomplete
	default:
		return SupportNeither
	}
}
