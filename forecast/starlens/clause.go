package starlens

import "fmt"

// Clause kinds. They are not one equality test.
const (
	KindContinuous = "continuous"
	KindR          = "r"
	KindStatus     = "status"
	KindSide       = "side"
	KindStop       = "stop"
)

// Comparisons for a continuous threshold.
const (
	CmpGT  = ">"
	CmpGTE = ">="
	CmpLT  = "<"
	CmpLTE = "<="
)

// Clause is one typed restriction.
// A continuous bound of zero is a real bound, so the field is always stored.
type Clause struct {
	Kind   string  `json:"kind"`
	Field  string  `json:"field,omitempty"`
	Cmp    string  `json:"cmp,omitempty"`
	Bound  float64 `json:"bound"`
	Level  int     `json:"level,omitempty"`
	State  string  `json:"state,omitempty"`
	Status string  `json:"status,omitempty"`
	Side   string  `json:"side,omitempty"`
}

// Continuous is a threshold on one outcome field.
// Rows that are not observable under that field's rule fail it.
func Continuous(field, cmp string, bound float64) Clause {
	return Clause{Kind: KindContinuous, Field: field, Cmp: cmp, Bound: bound}
}

// REvent is 1R / 0.15, 2R / 0.15, or 3R / 0.15.
// state is "reached" or "not_reached".
// NoR satisfies neither.
func REvent(level int, state string) Clause {
	return Clause{Kind: KindR, Level: level, State: state}
}

// Status is one stored status or one discovery word.
func Status(status string) Clause {
	return Clause{Kind: KindStatus, Status: status}
}

// Side is "up" or "down".
func Side(side string) Clause {
	return Clause{Kind: KindSide, Side: side}
}

// StopEvent is "touched" or "not_touched" for the 0.15 stop.
// NoStop satisfies neither.
func StopEvent(state string) Clause {
	return Clause{Kind: KindStop, State: state}
}

func (c Clause) validate() error {
	switch c.Kind {
	case KindContinuous:
		if _, ok := knownField(c.Field); !ok {
			return fmt.Errorf("starlens: continuous field %q", c.Field)
		}
		switch c.Cmp {
		case CmpGT, CmpGTE, CmpLT, CmpLTE:
		default:
			return fmt.Errorf("starlens: comparison %q", c.Cmp)
		}
	case KindR:
		if c.Level < 1 || c.Level > 3 {
			return fmt.Errorf("starlens: R level %d", c.Level)
		}
		if c.State != "reached" && c.State != "not_reached" {
			return fmt.Errorf("starlens: R state %q", c.State)
		}
	case KindStatus:
		if _, err := statusMatches(c.Status, "SURVIVED_WINDOW"); err != nil {
			return err
		}
	case KindSide:
		if c.Side != "up" && c.Side != "down" {
			return fmt.Errorf("starlens: side %q", c.Side)
		}
	case KindStop:
		if c.State != "touched" && c.State != "not_touched" {
			return fmt.Errorf("starlens: stop state %q", c.State)
		}
	default:
		return fmt.Errorf("starlens: clause kind %q", c.Kind)
	}
	return nil
}

func knownField(field string) (struct{}, bool) {
	switch field {
	case FieldEntryPrice, FieldMFEPrice, FieldMAEPrice, FieldMFEATR, FieldMAEATR,
		FieldMFEPercent, FieldMAEPercent, FieldStopPrice, FieldStopDistance,
		FieldStopDistanceATR, FieldR:
		return struct{}{}, true
	default:
		return struct{}{}, false
	}
}

// key groups clauses that are the same picture.
// Every clause on field X is X's own clause.
func (c Clause) key() string {
	switch c.Kind {
	case KindContinuous:
		return "n:" + c.Field
	case KindR:
		return fmt.Sprintf("r:%d", c.Level)
	case KindStatus:
		return "status"
	case KindSide:
		return "side"
	case KindStop:
		return "stop"
	default:
		return "unknown"
	}
}

func statusMatches(want, stored string) (bool, error) {
	switch want {
	case "survived":
		return stored == "SURVIVED_WINDOW", nil
	case "stop first":
		return stored == "STOP_FIRST", nil
	case "unordered":
		return stored == "UNORDERED_BAR", nil
	case "no reading":
		switch stored {
		case "SURVIVED_WINDOW", "STOP_FIRST", "UNORDERED_BAR":
			return false, nil
		default:
			return true, nil
		}
	case "SURVIVED_WINDOW", "STOP_FIRST", "UNORDERED_BAR", "INCOMPLETE_WINDOW",
		"PATH_GAP", "NO_STRUCTURE", "NO_ATR", "NO_ENTRY", "INVALID_GEOMETRY":
		return stored == want, nil
	default:
		return false, fmt.Errorf("starlens: status %q", want)
	}
}

func (c Clause) holds(row Row) (bool, error) {
	switch c.Kind {
	case KindContinuous:
		n, ok := row.Number(c.Field)
		if !ok {
			return false, fmt.Errorf("starlens: continuous field %q", c.Field)
		}
		if !n.OK {
			return false, nil
		}
		switch c.Cmp {
		case CmpGT:
			return n.Value > c.Bound, nil
		case CmpGTE:
			return n.Value >= c.Bound, nil
		case CmpLT:
			return n.Value < c.Bound, nil
		case CmpLTE:
			return n.Value <= c.Bound, nil
		default:
			return false, fmt.Errorf("starlens: comparison %q", c.Cmp)
		}
	case KindR:
		state, err := row.level(c.Level)
		if err != nil {
			return false, err
		}
		switch c.State {
		case "reached":
			return state == Reached, nil
		case "not_reached":
			return state == NotReachedValidR, nil
		default:
			return false, fmt.Errorf("starlens: R state %q", c.State)
		}
	case KindStatus:
		return statusMatches(c.Status, row.Status)
	case KindSide:
		return row.Side == c.Side, nil
	case KindStop:
		switch c.State {
		case "touched":
			return row.Stop == StopTouched, nil
		case "not_touched":
			return row.Stop == StopNotTouched, nil
		default:
			return false, fmt.Errorf("starlens: stop state %q", c.State)
		}
	default:
		return false, fmt.Errorf("starlens: clause kind %q", c.Kind)
	}
}
