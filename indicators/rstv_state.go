package indicators

import (
	"errors"
	"math"
)

// ErrRSTVInvalidInput means UpdateClosed refused the bar and left state unchanged.
var ErrRSTVInvalidInput = errors.New("rstv: invalid closed input")

const (
	RSTVFamilyDiv   = "div"
	RSTVFamilyPivot = "pivot"
)

// RSTVEvent is a neutral Everget fact (no market/Frame/tape types).
type RSTVEvent struct {
	Family      string
	Direction   string
	AnchorAt    int64
	ConfirmedAt int64
	AnchorValue float64
	AnchorPrice float64
}

// RSTVEvents is a fixed-capacity result (at most 2 div + 2 pivot).
type RSTVEvents struct {
	Events [4]RSTVEvent
	Count  uint8
}

func (e RSTVEvents) Facts() []IndicatorFactEvent {
	if e.Count == 0 {
		return nil
	}
	out := make([]IndicatorFactEvent, 0, e.Count)
	for i := 0; i < int(e.Count); i++ {
		out = append(out, e.Events[i].Fact())
	}
	return out
}

func (ev RSTVEvent) Fact() IndicatorFactEvent {
	src := FactSourceRSXTVDiv
	if ev.Family == RSTVFamilyPivot {
		src = FactSourceRSXTVPivot
	}
	return IndicatorFactEvent{
		Source:      src,
		Direction:   ev.Direction,
		ConfirmedAt: ev.ConfirmedAt,
		AnchorAt:    ev.AnchorAt,
		AnchorValue: ev.AnchorValue,
		AnchorPrice: ev.AnchorPrice,
	}
}

func (e *RSTVEvents) push(ev RSTVEvent) {
	if e.Count >= 4 {
		return
	}
	e.Events[e.Count] = ev
	e.Count++
}

// RSTVState is the single closed-bar Everget transducer for rsx_tv_div and rsx_tv_pivot.
// It consumes authoritative RSX; it never computes Jurik.
type RSTVState struct {
	lookback int

	ring    []float64
	ringN   int
	ringPos int

	max, min, maxRSI, minRSI float64
	hasMax, hasMin           bool

	max1, max2     float64
	min1, min2     float64
	close1, close2 float64
	rsx1, rsx2     float64
	at1, at2       int64

	maxRSI1, maxRSI2, maxRSI3 float64
	minRSI1, minRSI2, minRSI3 float64
	rsiHist                   int

	bars    int
	lastAt  int64
	started bool
}

// NewRSTVState constructs a zero machine. lookback is immutable; <=0 uses DefaultRSXLookback.
func NewRSTVState(lookback int) *RSTVState {
	if lookback <= 0 {
		lookback = DefaultRSXLookback
	}
	return &RSTVState{
		lookback: lookback,
		ring:     make([]float64, lookback),
	}
}

func (s *RSTVState) Lookback() int {
	if s == nil {
		return 0
	}
	return s.lookback
}

func (s *RSTVState) LastOpenTime() int64 {
	if s == nil || !s.started {
		return 0
	}
	return s.lastAt
}

func (s *RSTVState) Started() bool {
	return s != nil && s.started
}

// UpdateClosed applies one confirmed bar. Invalid input returns ErrRSTVInvalidInput
// and does not mutate state.
func (s *RSTVState) UpdateClosed(openTime int64, close, rsx float64) (RSTVEvents, error) {
	var out RSTVEvents
	if s == nil {
		return out, ErrRSTVInvalidInput
	}
	if math.IsNaN(close) || math.IsInf(close, 0) || math.IsNaN(rsx) || math.IsInf(rsx, 0) {
		return out, ErrRSTVInvalidInput
	}
	if s.started && openTime <= s.lastAt {
		return out, ErrRSTVInvalidInput
	}

	s.pushRSX(rsx)
	hb := s.highestAgo()
	lb := s.lowestAgo()

	if hb == 0 || !s.hasMax {
		s.max = close
		s.maxRSI = rsx
		s.hasMax = true
	}
	if lb == 0 || !s.hasMin {
		s.min = close
		s.minRSI = rsx
		s.hasMin = true
	}
	if close > s.max {
		s.max = close
	}
	if rsx > s.maxRSI {
		s.maxRSI = rsx
	}
	if close < s.min {
		s.min = close
	}
	if rsx < s.minRSI {
		s.minRSI = rsx
	}

	if s.rsiHist >= 3 {
		// Pine: max_rsi == max_rsi[2] and max_rsi[2] != max_rsi[3]
		if s.maxRSI == s.maxRSI2 && s.maxRSI2 != s.maxRSI3 {
			out.push(RSTVEvent{
				Family:      RSTVFamilyPivot,
				Direction:   FactDirPivotHigh,
				AnchorAt:    s.at2,
				ConfirmedAt: openTime,
				AnchorValue: s.rsx2,
				AnchorPrice: s.close2,
			})
		}
		if s.minRSI == s.minRSI2 && s.minRSI2 != s.minRSI3 {
			out.push(RSTVEvent{
				Family:      RSTVFamilyPivot,
				Direction:   FactDirPivotLow,
				AnchorAt:    s.at2,
				ConfirmedAt: openTime,
				AnchorValue: s.rsx2,
				AnchorPrice: s.close2,
			})
		}
	}

	if s.bars >= 2 {
		if s.max1 > s.max2 && s.rsx1 < s.maxRSI && rsx <= s.rsx1 {
			out.push(RSTVEvent{
				Family:      RSTVFamilyDiv,
				Direction:   FactDirBearish,
				AnchorAt:    s.at1,
				ConfirmedAt: openTime,
				AnchorValue: s.rsx1,
				AnchorPrice: s.close1,
			})
		}
		if s.min1 < s.min2 && s.rsx1 > s.minRSI && rsx >= s.rsx1 {
			out.push(RSTVEvent{
				Family:      RSTVFamilyDiv,
				Direction:   FactDirBullish,
				AnchorAt:    s.at1,
				ConfirmedAt: openTime,
				AnchorValue: s.rsx1,
				AnchorPrice: s.close1,
			})
		}
	}

	// Deterministic order: div bear, div bull, pivot high, pivot low.
	s.reorderEvents(&out)

	s.max2, s.max1 = s.max1, s.max
	s.min2, s.min1 = s.min1, s.min
	s.close2, s.close1 = s.close1, close
	s.rsx2, s.rsx1 = s.rsx1, rsx
	s.at2, s.at1 = s.at1, openTime
	s.maxRSI3, s.maxRSI2, s.maxRSI1 = s.maxRSI2, s.maxRSI1, s.maxRSI
	s.minRSI3, s.minRSI2, s.minRSI1 = s.minRSI2, s.minRSI1, s.minRSI
	if s.rsiHist < 4 {
		s.rsiHist++
	}
	s.bars++
	s.lastAt = openTime
	s.started = true
	return out, nil
}

func (s *RSTVState) reorderEvents(out *RSTVEvents) {
	if out.Count <= 1 {
		return
	}
	var ordered [4]RSTVEvent
	n := 0
	take := func(family, dir string) {
		for i := 0; i < int(out.Count); i++ {
			ev := out.Events[i]
			if ev.Family == family && ev.Direction == dir {
				ordered[n] = ev
				n++
			}
		}
	}
	take(RSTVFamilyDiv, FactDirBearish)
	take(RSTVFamilyDiv, FactDirBullish)
	take(RSTVFamilyPivot, FactDirPivotHigh)
	take(RSTVFamilyPivot, FactDirPivotLow)
	*out = RSTVEvents{Count: uint8(n)}
	copy(out.Events[:], ordered[:n])
}

func (s *RSTVState) pushRSX(rsx float64) {
	s.ring[s.ringPos] = rsx
	s.ringPos++
	if s.ringPos == s.lookback {
		s.ringPos = 0
	}
	if s.ringN < s.lookback {
		s.ringN++
	}
}

func (s *RSTVState) ringAtOldestToNewest(j int) float64 {
	start := s.ringPos - s.ringN
	if start < 0 {
		start += s.lookback
	}
	return s.ring[(start+j)%s.lookback]
}

// highestAgo matches full-scan highestBarsAgo (left-to-right, strict >, init at newest).
func (s *RSTVState) highestAgo() int {
	n := s.ringN
	if n == 0 {
		return 0
	}
	i := n - 1
	bestIdx := i
	bestVal := s.ringAtOldestToNewest(i)
	for j := 0; j < n; j++ {
		v := s.ringAtOldestToNewest(j)
		if v > bestVal {
			bestVal = v
			bestIdx = j
		}
	}
	return i - bestIdx
}

func (s *RSTVState) lowestAgo() int {
	n := s.ringN
	if n == 0 {
		return 0
	}
	i := n - 1
	bestIdx := i
	bestVal := s.ringAtOldestToNewest(i)
	for j := 0; j < n; j++ {
		v := s.ringAtOldestToNewest(j)
		if v < bestVal {
			bestVal = v
			bestIdx = j
		}
	}
	return i - bestIdx
}

// ReplayRSTVFacts walks aligned closed series through one RSTVState.
func ReplayRSTVFacts(opens []int64, closes, rsx []float64, lookback int) []IndicatorFactEvent {
	facts, _ := ReplayRSTV(opens, closes, rsx, lookback)
	return facts
}

// ReplayRSTV returns facts and the machine after the last accepted bar.
func ReplayRSTV(opens []int64, closes, rsx []float64, lookback int) ([]IndicatorFactEvent, *RSTVState) {
	st := NewRSTVState(lookback)
	n := len(rsx)
	if n == 0 || len(closes) != n || len(opens) != n {
		return nil, st
	}
	out := make([]IndicatorFactEvent, 0)
	for i := 0; i < n; i++ {
		evs, err := st.UpdateClosed(opens[i], closes[i], rsx[i])
		if err != nil {
			continue
		}
		out = append(out, evs.Facts()...)
	}
	if len(out) == 0 {
		return nil, st
	}
	return out, st
}

func filterRSTVFacts(all []IndicatorFactEvent, source string) []IndicatorFactEvent {
	if len(all) == 0 {
		return nil
	}
	out := make([]IndicatorFactEvent, 0)
	for _, ev := range all {
		if ev.Source == source {
			out = append(out, ev)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
