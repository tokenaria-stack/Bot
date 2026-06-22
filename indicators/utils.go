package indicators

import (
	"math"

	"trading_bot/exchange"
)

// ExtractPrices extracts OHLCV price series from klines for indicator input.
func ExtractPrices(klines []exchange.Kline) (open, high, low, close, volume []float64) {
	open = make([]float64, 0, len(klines))
	high = make([]float64, 0, len(klines))
	low = make([]float64, 0, len(klines))
	close = make([]float64, 0, len(klines))
	volume = make([]float64, 0, len(klines))

	for _, k := range klines {
		open = append(open, k.Open)
		high = append(high, k.High)
		low = append(low, k.Low)
		close = append(close, k.Close)
		volume = append(volume, k.Volume)
	}

	return open, high, low, close, volume
}

// RollingSum maintains the sum of the last N values with O(1) memory.
type RollingSum struct {
	period int
	buf    []float64
	idx    int
	count  int
	sum    float64
	value  float64
}

// NewRollingSum creates a rolling sum over a fixed window.
func NewRollingSum(period int) *RollingSum {
	if period <= 0 {
		period = 1
	}
	return &RollingSum{
		period: period,
		buf:    make([]float64, period),
	}
}

func (r *RollingSum) Update(val float64) float64 {
	if r.count >= r.period {
		r.sum -= r.buf[r.idx]
	} else {
		r.count++
	}

	r.buf[r.idx] = val
	r.sum += val
	r.idx = (r.idx + 1) % r.period
	r.value = r.sum
	return r.value
}

func (r *RollingSum) Value() float64 {
	return r.value
}

// RollingStDev maintains population standard deviation over the last N values.
type RollingStDev struct {
	period int
	buf    []float64
	idx    int
	count  int
	sum    float64
	sumSq  float64
	value  float64
}

// NewRollingStDev creates a rolling standard deviation indicator.
func NewRollingStDev(period int) *RollingStDev {
	if period <= 0 {
		period = 1
	}
	return &RollingStDev{
		period: period,
		buf:    make([]float64, period),
	}
}

func (r *RollingStDev) Update(val float64) float64 {
	if r.count >= r.period {
		old := r.buf[r.idx]
		r.sum -= old
		r.sumSq -= old * old
	} else {
		r.count++
	}

	r.buf[r.idx] = val
	r.sum += val
	r.sumSq += val * val
	r.idx = (r.idx + 1) % r.period

	if r.count < 2 {
		r.value = 0
		return r.value
	}

	n := float64(r.count)
	mean := r.sum / n
	variance := (r.sumSq / n) - mean*mean
	if variance < 0 {
		variance = 0
	}
	r.value = math.Sqrt(variance)
	return r.value
}

func (r *RollingStDev) Value() float64 {
	return r.value
}

var (
	_ Indicator = (*RollingSum)(nil)
	_ Indicator = (*RollingStDev)(nil)
)
