package starlens

// Observed is one number under that field's own presence rule.
// OK false means the number is not an observation.
// A zero with OK true is a real zero.
type Observed struct {
	Value float64
	OK    bool
}

// RState is one R level of the 0.15 stop.
// NoR is not "not reached". The hit flag is false in both cases.
type RState int

const (
	NoR RState = iota
	NotReachedValidR
	Reached
)

func (s RState) String() string {
	switch s {
	case Reached:
		return "reached"
	case NotReachedValidR:
		return "not_reached"
	default:
		return "no_r"
	}
}

// StopState is whether the 0.15 stop was touched.
// NoStop is the same 47 rows that have no R.
// A false stopReached flag on those rows is not "not touched".
type StopState int

const (
	NoStop StopState = iota
	StopNotTouched
	StopTouched
)

func (s StopState) String() string {
	switch s {
	case StopTouched:
		return "touched"
	case StopNotTouched:
		return "not_touched"
	default:
		return "no_stop"
	}
}

// rState reads the stored distance and the stored hit flag.
// The hit flag alone is not a state.
func rState(distance *float64, hit bool) RState {
	if distance == nil || !(*distance > 0) {
		return NoR
	}
	if hit {
		return Reached
	}
	return NotReachedValidR
}

func stopState(distance *float64, reached bool) StopState {
	if distance == nil || !(*distance > 0) {
		return NoStop
	}
	if reached {
		return StopTouched
	}
	return StopNotTouched
}

func observedPtr(v *float64) Observed {
	if v == nil {
		return Observed{}
	}
	return Observed{Value: *v, OK: true}
}

func observedPositive(v *float64) Observed {
	if v == nil || !(*v > 0) {
		return Observed{}
	}
	return Observed{Value: *v, OK: true}
}

func observedTime(v *int64) Observed {
	if v == nil || *v == 0 {
		return Observed{}
	}
	return Observed{Value: float64(*v), OK: true}
}
