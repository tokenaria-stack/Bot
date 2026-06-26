package indicators

// SMA is a Simple Moving Average with O(1) memory via a ring buffer.
type SMA struct {
	period int
	buf    []float64
	idx    int
	count  int
	sum    float64
	value  float64

	snapIdx   int
	snapCount int
	snapSum   float64
	snapValue float64
	snapBuf   []float64
}

// NewSMA creates an SMA indicator for the given window size.
func NewSMA(period int) *SMA {
	if period <= 0 {
		return &SMA{period: 1, buf: make([]float64, 1)}
	}
	return &SMA{
		period: period,
		buf:    make([]float64, period),
	}
}

func (s *SMA) Update(val float64) float64 {
	if s.count >= s.period {
		s.sum -= s.buf[s.idx]
	} else {
		s.count++
	}

	s.buf[s.idx] = val
	s.sum += val
	s.idx = (s.idx + 1) % s.period

	if s.count < s.period {
		s.value = s.sum / float64(s.count)
	} else {
		s.value = s.sum / float64(s.period)
	}
	return s.value
}

func (s *SMA) Value() float64 {
	return s.value
}

func (s *SMA) SaveState() {
	s.snapIdx = s.idx
	s.snapCount = s.count
	s.snapSum = s.sum
	s.snapValue = s.value
	if cap(s.snapBuf) < len(s.buf) {
		s.snapBuf = make([]float64, len(s.buf))
	}
	s.snapBuf = s.snapBuf[:len(s.buf)]
	copy(s.snapBuf, s.buf)
}

func (s *SMA) RestoreState() {
	s.idx = s.snapIdx
	s.count = s.snapCount
	s.sum = s.snapSum
	s.value = s.snapValue
	copy(s.buf, s.snapBuf)
}

// EMA is an Exponential Moving Average with O(1) memory.
type EMA struct {
	period      int
	alpha       float64
	initialized bool
	value       float64

	snapInitialized bool
	snapValue       float64
}

// NewEMA creates an EMA indicator (alpha = 2 / (N + 1)).
func NewEMA(period int) *EMA {
	if period <= 0 {
		period = 1
	}
	return &EMA{
		period: period,
		alpha:  2.0 / float64(period+1),
	}
}

func (e *EMA) Update(val float64) float64 {
	if !e.initialized {
		e.value = val
		e.initialized = true
		return e.value
	}

	e.value = val*e.alpha + e.value*(1-e.alpha)
	return e.value
}

func (e *EMA) Value() float64 {
	return e.value
}

func (e *EMA) SaveState() {
	e.snapInitialized = e.initialized
	e.snapValue = e.value
}

func (e *EMA) RestoreState() {
	e.initialized = e.snapInitialized
	e.value = e.snapValue
}

// RMA is Wilder's Running Moving Average (alpha = 1 / N), used by TradingView RSI.
type RMA struct {
	period      int
	alpha       float64
	initialized bool
	value       float64

	snapInitialized bool
	snapValue       float64
}

// NewRMA creates an RMA indicator (alpha = 1 / N).
func NewRMA(period int) *RMA {
	if period <= 0 {
		period = 1
	}
	return &RMA{
		period: period,
		alpha:  1.0 / float64(period),
	}
}

func (r *RMA) Update(val float64) float64 {
	if !r.initialized {
		r.value = val
		r.initialized = true
		return r.value
	}

	r.value = val*r.alpha + r.value*(1-r.alpha)
	return r.value
}

func (r *RMA) Value() float64 {
	return r.value
}

func (r *RMA) SaveState() {
	r.snapInitialized = r.initialized
	r.snapValue = r.value
}

func (r *RMA) RestoreState() {
	r.initialized = r.snapInitialized
	r.value = r.snapValue
}

var (
	_ Indicator = (*SMA)(nil)
	_ Indicator = (*EMA)(nil)
	_ Indicator = (*RMA)(nil)
)

// SMAValues calculates Simple Moving Average over a price series (batch wrapper).
func SMAValues(closePrices []float64, period int) []float64 {
	if len(closePrices) < period || period <= 0 {
		return nil
	}

	s := NewSMA(period)
	out := make([]float64, len(closePrices))
	for i, v := range closePrices {
		out[i] = s.Update(v)
	}
	return out
}

// EMAValues calculates Exponential Moving Average over a price series (batch wrapper).
func EMAValues(closePrices []float64, period int) []float64 {
	if len(closePrices) < period || period <= 0 {
		return nil
	}

	e := NewEMA(period)
	out := make([]float64, len(closePrices))
	for i, v := range closePrices {
		out[i] = e.Update(v)
	}
	return out
}

// runIndicator feeds a series through a streaming indicator.
func runIndicator(ind Indicator, values []float64) []float64 {
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = ind.Update(v)
	}
	return out
}
