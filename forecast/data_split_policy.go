package forecast

import "fmt"

const (
	DataRoleDev     = "DEV"
	DataRoleHoldout = "HOLDOUT"
)

// DefaultResearchHoldoutStartAt is the adjustable experiment wall:
// 2026-01-01 00:00:00 UTC on the 15m grid (1767225600000).
// Change this one value when a later book rolls the wall forward.
// It is not a forever calendar law and not a 2027 sentinel.
const DefaultResearchHoldoutStartAt int64 = 1767225600000

// DataSplitPolicy is the first-class development / holdout cut.
// OOF folds are compiled only inside DEV. Strategy selection must not read HOLDOUT.
type DataSplitPolicy struct {
	HoldoutStartAt int64
	Timeframe      string
}

// DefaultDataSplitPolicy is the current book wall. Swap HoldoutStartAt here
// (or pass a new policy) when the holdout year should move.
func DefaultDataSplitPolicy() DataSplitPolicy {
	return DataSplitPolicy{
		HoldoutStartAt: DefaultResearchHoldoutStartAt,
		Timeframe:      Spec2PrimaryTF,
	}
}

func (p DataSplitPolicy) Role(at int64) string {
	if at >= p.HoldoutStartAt {
		return DataRoleHoldout
	}
	return DataRoleDev
}

const DataSplitLogicHoldoutWallV1 LogicVersion = "data-split:holdout-wall-v1"

type dataSplitPayload struct {
	HoldoutStartAt int64
	Timeframe      string
}

func (p DataSplitPolicy) Identity() (Identity, error) {
	if p.HoldoutStartAt <= 0 || p.Timeframe == "" {
		return Identity{}, fmt.Errorf("forecast: DataSplitPolicy incomplete")
	}
	return NewIdentity("data-split-"+p.Timeframe, dataSplitPayload{
		HoldoutStartAt: p.HoldoutStartAt,
		Timeframe:      p.Timeframe,
	}, DataSplitLogicHoldoutWallV1)
}

func (p DataSplitPolicy) RefuseHoldoutAt(at int64) error {
	if at >= p.HoldoutStartAt {
		return fmt.Errorf("forecast: At %d is HOLDOUT (wall %d); development cannot consume it", at, p.HoldoutStartAt)
	}
	return nil
}
