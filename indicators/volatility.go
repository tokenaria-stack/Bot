package indicators

// BollingerBands is a streaming Bollinger Bands indicator (SMA + rolling stdev).
type BollingerBands struct {
	sma    *SMA
	stdev  *RollingStDev
	multUp float64
	multDn float64
	middle float64

	snapMiddle float64
}

// NewBollingerBands creates a Bollinger Bands indicator.
func NewBollingerBands(period int, devUp, devDn float64) *BollingerBands {
	if period <= 0 {
		period = 20
	}
	if devUp <= 0 {
		devUp = 2
	}
	if devDn <= 0 {
		devDn = devUp
	}
	return &BollingerBands{
		sma:    NewSMA(period),
		stdev:  NewRollingStDev(period),
		multUp: devUp,
		multDn: devDn,
	}
}

func (b *BollingerBands) Update(val float64) float64 {
	b.middle = b.sma.Update(val)
	b.stdev.Update(val)
	return b.middle
}

func (b *BollingerBands) Value() float64 {
	return b.middle
}

// Bands returns upper, middle, and lower band values.
func (b *BollingerBands) Bands() (upper, middle, lower float64) {
	middle = b.middle
	stdev := b.stdev.Value()
	upper = middle + stdev*b.multUp
	lower = middle - stdev*b.multDn
	return upper, middle, lower
}

func (b *BollingerBands) SaveState() {
	b.sma.SaveState()
	b.stdev.SaveState()
	b.snapMiddle = b.middle
}

func (b *BollingerBands) RestoreState() {
	b.sma.RestoreState()
	b.stdev.RestoreState()
	b.middle = b.snapMiddle
}

var _ Indicator = (*BollingerBands)(nil)

// BollingerBandsValues calculates Bollinger Bands over a price series (batch wrapper).
func BollingerBandsValues(closePrices []float64, period int, devUp, devDn float64) (upper, middle, lower []float64) {
	if len(closePrices) < period || period <= 0 {
		return nil, nil, nil
	}

	bb := NewBollingerBands(period, devUp, devDn)
	upper = make([]float64, len(closePrices))
	middle = make([]float64, len(closePrices))
	lower = make([]float64, len(closePrices))

	for i, v := range closePrices {
		middle[i] = bb.Update(v)
		upper[i], _, lower[i] = bb.Bands()
	}

	return upper, middle, lower
}

// ATR is a streaming Average True Range using Wilder's RMA smoothing.
type ATR struct {
	period    int
	rma       *RMA
	prevClose float64
	hasPrev   bool
	value     float64

	snapPrevClose float64
	snapHasPrev   bool
	snapValue     float64
}

// NewATR creates an ATR indicator for the given period.
func NewATR(period int) *ATR {
	if period <= 0 {
		period = 14
	}
	return &ATR{
		period: period,
		rma:    NewRMA(period),
	}
}

func (a *ATR) UpdateCandle(high, low, close float64) float64 {
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

func (a *ATR) Value() float64 {
	return a.value
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

// ATRValues calculates Average True Range over OHLC series (batch wrapper).
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
