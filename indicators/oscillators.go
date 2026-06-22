package indicators

import "math"

// RSI is a streaming Relative Strength Index using dual RMA smoothers.
type RSI struct {
	period  int
	upRMA   *RMA
	downRMA *RMA
	prevVal float64
	hasPrev bool
	value   float64
}

// NewRSI creates a streaming RSI indicator.
func NewRSI(period int) *RSI {
	if period <= 0 {
		period = 14
	}
	return &RSI{
		period:  period,
		upRMA:   NewRMA(period),
		downRMA: NewRMA(period),
	}
}

func (r *RSI) Update(val float64) float64 {
	if !r.hasPrev {
		r.prevVal = val
		r.hasPrev = true
		return r.value
	}

	diff := val - r.prevVal
	r.prevVal = val

	var up, down float64
	if diff > 0 {
		up = diff
	} else if diff < 0 {
		down = -diff
	}

	r.upRMA.Update(up)
	r.downRMA.Update(down)

	upVal := r.upRMA.Value()
	downVal := r.downRMA.Value()

	switch {
	case downVal == 0 && upVal == 0:
		r.value = 50
	case downVal == 0:
		r.value = 100
	default:
		rs := upVal / downVal
		r.value = 100 - (100 / (1 + rs))
	}

	return r.value
}

func (r *RSI) Value() float64 {
	return r.value
}

// MACD is a streaming MACD with fast/slow EMA and signal EMA.
type MACD struct {
	fastEMA    *EMA
	slowEMA    *EMA
	signalEMA  *EMA
	macdLine   float64
	signalLine float64
	hist       float64
}

// NewMACD creates a streaming MACD indicator.
func NewMACD(fastPeriod, slowPeriod, signalPeriod int) *MACD {
	if fastPeriod <= 0 {
		fastPeriod = 12
	}
	if slowPeriod <= 0 {
		slowPeriod = 26
	}
	if signalPeriod <= 0 {
		signalPeriod = 9
	}
	return &MACD{
		fastEMA:   NewEMA(fastPeriod),
		slowEMA:   NewEMA(slowPeriod),
		signalEMA: NewEMA(signalPeriod),
	}
}

func (m *MACD) Update(val float64) float64 {
	fast := m.fastEMA.Update(val)
	slow := m.slowEMA.Update(val)
	m.macdLine = fast - slow
	m.signalLine = m.signalEMA.Update(m.macdLine)
	m.hist = m.macdLine - m.signalLine
	return m.macdLine
}

func (m *MACD) Value() float64 {
	return m.macdLine
}

// Signal returns the signal line (EMA of MACD line).
func (m *MACD) Signal() float64 {
	return m.signalLine
}

// Histogram returns MACD line minus signal line.
func (m *MACD) Histogram() float64 {
	return m.hist
}

type stochCandle struct {
	high  float64
	low   float64
	close float64
}

// Stochastic is a streaming Stochastic Oscillator (%K and %D).
type Stochastic struct {
	lookback    int
	slowKPeriod int
	dPeriod     int
	ring        []stochCandle
	idx         int
	count       int
	rawK        float64
	k           float64
	d           float64
	slowKSMA    *SMA
	dSMA        *SMA
}

// NewStochastic creates a streaming Stochastic oscillator.
func NewStochastic(fastKPeriod, slowKPeriod, slowDPeriod int) *Stochastic {
	if fastKPeriod <= 0 {
		fastKPeriod = 14
	}
	if slowKPeriod <= 0 {
		slowKPeriod = 3
	}
	if slowDPeriod <= 0 {
		slowDPeriod = 3
	}
	return &Stochastic{
		lookback:    fastKPeriod,
		slowKPeriod: slowKPeriod,
		dPeriod:     slowDPeriod,
		ring:        make([]stochCandle, fastKPeriod),
		slowKSMA:    NewSMA(slowKPeriod),
		dSMA:        NewSMA(slowDPeriod),
	}
}

func (s *Stochastic) UpdateCandle(high, low, close float64) float64 {
	s.ring[s.idx] = stochCandle{high: high, low: low, close: close}
	s.idx = (s.idx + 1) % s.lookback
	if s.count < s.lookback {
		s.count++
	}

	s.rawK = s.calcRawK(close)
	s.k = s.slowKSMA.Update(s.rawK)
	s.d = s.dSMA.Update(s.k)
	return s.k
}

func (s *Stochastic) calcRawK(close float64) float64 {
	if s.count == 0 {
		return 0
	}

	n := s.count
	if n > s.lookback {
		n = s.lookback
	}

	highestHigh := math.Inf(-1)
	lowestLow := math.Inf(1)

	for i := 0; i < n; i++ {
		pos := (s.idx - n + i + s.lookback) % s.lookback
		c := s.ring[pos]
		if c.high > highestHigh {
			highestHigh = c.high
		}
		if c.low < lowestLow {
			lowestLow = c.low
		}
	}

	denom := highestHigh - lowestLow
	if denom == 0 {
		return 50
	}
	return (close - lowestLow) / denom * 100
}

func (s *Stochastic) Value() float64 {
	return s.k
}

// K returns the current %K value (slow %K when slowKPeriod > 1).
func (s *Stochastic) K() float64 {
	return s.k
}

// D returns the current %D value (SMA of %K).
func (s *Stochastic) D() float64 {
	return s.d
}

var (
	_ Indicator       = (*RSI)(nil)
	_ Indicator       = (*MACD)(nil)
	_ CandleIndicator = (*Stochastic)(nil)
)

// RSIValues calculates Relative Strength Index over a price series (batch wrapper).
func RSIValues(closePrices []float64, period int) []float64 {
	if len(closePrices) <= period || period <= 0 {
		return nil
	}

	return runIndicator(NewRSI(period), closePrices)
}

// MACDValues calculates MACD over a price series (batch wrapper).
// Returns MACD line, signal line, and histogram.
func MACDValues(closePrices []float64, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, hist []float64) {
	if len(closePrices) <= slowPeriod || slowPeriod <= 0 {
		return nil, nil, nil
	}

	m := NewMACD(fastPeriod, slowPeriod, signalPeriod)
	macd = make([]float64, len(closePrices))
	signal = make([]float64, len(closePrices))
	hist = make([]float64, len(closePrices))

	for i, v := range closePrices {
		macd[i] = m.Update(v)
		signal[i] = m.Signal()
		hist[i] = m.Histogram()
	}

	return macd, signal, hist
}

// StochValues calculates Stochastic Oscillator (%K and %D) over OHLC series (batch wrapper).
func StochValues(high, low, close []float64, fastKPeriod, slowKPeriod, slowDPeriod int) (k, d []float64) {
	if len(close) <= fastKPeriod || fastKPeriod <= 0 {
		return nil, nil
	}
	if len(high) != len(low) || len(high) != len(close) {
		return nil, nil
	}

	st := NewStochastic(fastKPeriod, slowKPeriod, slowDPeriod)
	k = make([]float64, len(close))
	d = make([]float64, len(close))

	for i := range close {
		k[i] = st.UpdateCandle(high[i], low[i], close[i])
		d[i] = st.D()
	}

	return k, d
}
