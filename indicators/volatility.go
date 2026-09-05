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
