package starlens

import "fmt"

// Continuous fields stored on the outcome row.
// ATR and percent are different fields. R is the 0.15 entry-to-stop distance.
const (
	FieldEntryPrice      = "entryPrice"
	FieldMFEPrice        = "mfePrice"
	FieldMAEPrice        = "maePrice"
	FieldMFEATR          = "mfeAtr"
	FieldMAEATR          = "maeAtr"
	FieldMFEPercent      = "mfePercent"
	FieldMAEPercent      = "maePercent"
	FieldStopPrice       = "stopPrice"
	FieldStopDistance    = "stopDistance"
	FieldStopDistanceATR = "stopDistanceAtr"
	FieldR               = "r"
)

// Row is one Star after the outcome reader has applied each field's own rule.
// Times are stored so a later chapter can show them.
// This chapter does not subtract them into a duration.
type Row struct {
	Index      int
	DecisionAt int64
	Side       string
	Status     string

	EntryPrice      Observed
	MFEPrice        Observed
	MAEPrice        Observed
	MFEATR          Observed
	MAEATR          Observed
	MFEPercent      Observed
	MAEPercent      Observed
	StopPrice       Observed
	StopDistance    Observed
	StopDistanceATR Observed
	R               Observed

	R1 RState
	R2 RState
	R3 RState

	Stop StopState

	MFEAt  Observed
	MAEAt  Observed
	StopAt Observed
	At1R   Observed
	At2R   Observed
	At3R   Observed

	// coords is the 191 Schema 3 + Matrix projection. It is not a Schema 3 row.
	coords map[string]Observed
}

func (r Row) Ref() StarRef {
	return StarRef{Index: r.Index, DecisionAt: r.DecisionAt, Side: r.Side}
}

// Number returns one continuous field.
// The bool is false when the name is not an outcome field.
// OK on the value is that field's own presence rule.
func (r Row) Number(field string) (Observed, bool) {
	switch field {
	case FieldEntryPrice:
		return r.EntryPrice, true
	case FieldMFEPrice:
		return r.MFEPrice, true
	case FieldMAEPrice:
		return r.MAEPrice, true
	case FieldMFEATR:
		return r.MFEATR, true
	case FieldMAEATR:
		return r.MAEATR, true
	case FieldMFEPercent:
		return r.MFEPercent, true
	case FieldMAEPercent:
		return r.MAEPercent, true
	case FieldStopPrice:
		return r.StopPrice, true
	case FieldStopDistance:
		return r.StopDistance, true
	case FieldStopDistanceATR:
		return r.StopDistanceATR, true
	case FieldR:
		return r.R, true
	default:
		if _, ok := continuousIDs[field]; !ok {
			return Observed{}, false
		}
		if r.coords == nil {
			return Observed{}, true
		}
		return r.coords[field], true
	}
}

func (r Row) level(n int) (RState, error) {
	switch n {
	case 1:
		return r.R1, nil
	case 2:
		return r.R2, nil
	case 3:
		return r.R3, nil
	default:
		return 0, fmt.Errorf("starlens: R level %d", n)
	}
}
