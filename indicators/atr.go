package indicators

import (
	"fmt"
	"math"
)

// ATR constitution (ATR-TRUTH-1):
//
//	Same ATRSpec => same transition law.
//	Bit-identical ATR additionally requires the same prior state
//	OR the same ordered closed initialization history.
//
// Canonical law atr:wilder-rma-first-tr-v1 is first-TR-seeded Wilder RMA.
// It is NOT SMA-of-N seed / TradingView ta.atr initialization.
// Period is not a reconstruction-history guarantee.

// ATRMethod names the smoothing family. v1 supports only Wilder RMA.
type ATRMethod string

const ATRMethodWilderRMA ATRMethod = "wilder_rma"

// ATRLogicWilderRMAFirstTRV1 is the canonical ATR logic identity.
const ATRLogicWilderRMAFirstTRV1 = "atr:wilder-rma-first-tr-v1"

// ATRSpec is a resolved semantic value object (no standalone digest).
// Period+Method+Logic identify one transition law — not one live instance state.
type ATRSpec struct {
	Period int
	Method ATRMethod
	Logic  string
}

// CanonicalATRSpec returns the default resolved v1 spec (period 14).
func CanonicalATRSpec() ATRSpec {
	return ATRSpec{
		Period: DefaultATRPeriod,
		Method: ATRMethodWilderRMA,
		Logic:  ATRLogicWilderRMAFirstTRV1,
	}
}

// ValidateATRSpec refuses unresolved/hidden-default specs.
func ValidateATRSpec(s ATRSpec) error {
	if s.Period <= 0 {
		return fmt.Errorf("indicators: ATRSpec Period must be > 0")
	}
	if s.Method != ATRMethodWilderRMA {
		return fmt.Errorf("indicators: unsupported ATR Method %q", s.Method)
	}
	if s.Logic != ATRLogicWilderRMAFirstTRV1 {
		return fmt.Errorf("indicators: unsupported ATR Logic %q", s.Logic)
	}
	return nil
}

// ATR is the canonical streaming Average True Range (Wilder RMA, first-TR seed).
type ATR struct {
	spec      ATRSpec
	period    int
	rma       *RMA
	prevClose float64
	hasPrev   bool
	value     float64

	snapPrevClose float64
	snapHasPrev   bool
	snapValue     float64
}

// NewATR creates a legacy trusted-input ATR. Period<=0 still defaults to 14
// (ZigZag/volatility callers). Forecast/LabelSet must use NewATRFromSpec.
func NewATR(period int) *ATR {
	if period <= 0 {
		period = DefaultATRPeriod
	}
	return mustATR(ATRSpec{
		Period: period,
		Method: ATRMethodWilderRMA,
		Logic:  ATRLogicWilderRMAFirstTRV1,
	})
}

// NewATRFromSpec constructs canonical ATR. Invalid spec is refused (no Period=0 default).
func NewATRFromSpec(spec ATRSpec) (*ATR, error) {
	if err := ValidateATRSpec(spec); err != nil {
		return nil, err
	}
	return mustATR(spec), nil
}

func mustATR(spec ATRSpec) *ATR {
	return &ATR{
		spec:   spec,
		period: spec.Period,
		rma:    NewRMA(spec.Period),
	}
}

func (a *ATR) Spec() ATRSpec {
	if a == nil {
		return ATRSpec{}
	}
	return a.spec
}

// Ready is true after the first successful closed update (v1 law).
func (a *ATR) Ready() bool {
	return a != nil && a.hasPrev
}

func (a *ATR) Value() float64 {
	if a == nil {
		return 0
	}
	return a.value
}

func (a *ATR) clone() *ATR {
	rmaCopy := *a.rma
	out := *a
	out.rma = &rmaCopy
	return &out
}

func validateATRCandle(high, low, close float64) error {
	if math.IsNaN(high) || math.IsInf(high, 0) ||
		math.IsNaN(low) || math.IsInf(low, 0) ||
		math.IsNaN(close) || math.IsInf(close, 0) {
		return fmt.Errorf("indicators: ATR candle is not finite")
	}
	if high < low {
		return fmt.Errorf("indicators: ATR High < Low")
	}
	return nil
}

// apply is the single Wilder first-TR transition. Callers must already trust
// or have validated the candle. Commits state.
func (a *ATR) apply(high, low, close float64) float64 {
	var tr float64
	if !a.hasPrev {
		tr = high - low
		a.hasPrev = true
	} else {
		tr = trueRange(high, low, a.prevClose)
	}
	a.prevClose = close
	a.value = a.rma.Update(tr)
	return a.value
}

// UpdateCandle is the legacy trusted-input door (ZigZag/volatility). Same
// transition as UpdateClosed; does not validate or refuse.
func (a *ATR) UpdateCandle(high, low, close float64) float64 {
	return a.apply(high, low, close)
}

// UpdateClosed is the canonical checked closed-bar update. Invalid input
// refuses and does not mutate IIR state.
func (a *ATR) UpdateClosed(high, low, close float64) (float64, error) {
	if a == nil || a.rma == nil {
		return 0, fmt.Errorf("indicators: nil ATR")
	}
	if err := ValidateATRSpec(a.spec); err != nil {
		return a.value, err
	}
	if err := validateATRCandle(high, low, close); err != nil {
		return a.value, err
	}
	next := a.clone()
	v := next.apply(high, low, close)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return a.value, fmt.Errorf("indicators: ATR result is not finite")
	}
	a.rma = next.rma
	a.prevClose = next.prevClose
	a.hasPrev = next.hasPrev
	a.value = next.value
	return a.value, nil
}

func (a *ATR) SaveState() {
	a.rma.SaveState()
	a.snapPrevClose = a.prevClose
	a.snapHasPrev = a.hasPrev
	a.snapValue = a.value
}

func (a *ATR) RestoreState() {
	a.rma.RestoreState()
	a.prevClose = a.snapPrevClose
	a.hasPrev = a.snapHasPrev
	a.value = a.snapValue
}

var _ CandleIndicator = (*ATR)(nil)

func trueRange(high, low, prevClose float64) float64 {
	hl := high - low
	hc := abs(high - prevClose)
	lc := abs(low - prevClose)
	return max3(hl, hc, lc)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func max3(a, b, c float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

// ATRSeries is the canonical O(N) batch helper: NewATRFromSpec + UpdateClosed
// per closed bar. Not ATRValues.
func ATRSeries(spec ATRSpec, high, low, close []float64) ([]float64, error) {
	if len(high) != len(low) || len(high) != len(close) {
		return nil, fmt.Errorf("indicators: ATRSeries OHLC length mismatch")
	}
	atr, err := NewATRFromSpec(spec)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(close))
	for i := range close {
		v, err := atr.UpdateClosed(high[i], low[i], close[i])
		if err != nil {
			return nil, fmt.Errorf("indicators: ATRSeries bar %d: %w", i, err)
		}
		out[i] = v
	}
	return out, nil
}

// ATRValues is a legacy batch shape (nil when len<=period). Frame fractal
// filtering depends on that contract. NOT the canonical LabelSet series API.
func ATRValues(high, low, close []float64, period int) []float64 {
	if len(close) <= period || period <= 0 {
		return nil
	}
	if len(high) != len(low) || len(high) != len(close) {
		return nil
	}

	atr := NewATR(period)
	out := make([]float64, len(close))
	for i := range close {
		out[i] = atr.UpdateCandle(high[i], low[i], close[i])
	}
	return out
}
