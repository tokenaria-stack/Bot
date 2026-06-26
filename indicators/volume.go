package indicators

// AD is a streaming Accumulation/Distribution line (CLV cumulative sum).
type AD struct {
	total float64
	value float64

	snapTotal float64
	snapValue float64
}

// NewAD creates an Accumulation/Distribution indicator.
func NewAD() *AD {
	return &AD{}
}

func (a *AD) UpdateCandle(high, low, close float64) float64 {
	multiplier := 0.0
	if hl := high - low; hl != 0 {
		multiplier = ((close - low) - (high - close)) / hl
	}
	a.total += multiplier
	a.value = a.total
	return a.value
}

func (a *AD) Value() float64 {
	return a.value
}

func (a *AD) SaveState() {
	a.snapTotal = a.total
	a.snapValue = a.value
}

func (a *AD) RestoreState() {
	a.total = a.snapTotal
	a.value = a.snapValue
}

var _ CandleIndicator = (*AD)(nil)

// CumSum is a running total accumulator.
type CumSum struct {
	total float64
}

// NewCumSum creates a cumulative sum accumulator.
func NewCumSum() *CumSum {
	return &CumSum{}
}

func (c *CumSum) Update(val float64) float64 {
	c.total += val
	return c.total
}

func (c *CumSum) Value() float64 {
	return c.total
}

var _ Indicator = (*CumSum)(nil)

// VolumeWeightedEMA computes EMA(price*volume) / EMA(volume).
type VolumeWeightedEMA struct {
	pvEMA *EMA
	vEMA  *EMA
	value float64

	snapValue float64
}

// NewVolumeWeightedEMA creates a volume-weighted EMA indicator.
func NewVolumeWeightedEMA(period int) *VolumeWeightedEMA {
	if period <= 0 {
		period = 14
	}
	return &VolumeWeightedEMA{
		pvEMA: NewEMA(period),
		vEMA:  NewEMA(period),
	}
}

func (v *VolumeWeightedEMA) Update(price, volume float64) float64 {
	pv := v.pvEMA.Update(price * volume)
	vol := v.vEMA.Update(volume)
	if vol == 0 {
		return v.value
	}
	v.value = pv / vol
	return v.value
}

func (v *VolumeWeightedEMA) Value() float64 {
	return v.value
}

func (v *VolumeWeightedEMA) SaveState() {
	v.snapValue = v.value
	v.pvEMA.SaveState()
	v.vEMA.SaveState()
}

func (v *VolumeWeightedEMA) RestoreState() {
	v.value = v.snapValue
	v.pvEMA.RestoreState()
	v.vEMA.RestoreState()
}
