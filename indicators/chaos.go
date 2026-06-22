package indicators

import (
	"errors"

	"trading_bot/exchange"
)

const (
	DefaultAOFastPeriod = 5
	DefaultAOSlowPeriod = 34
	DefaultATRPeriod    = 14
)

// AO is a streaming Awesome Oscillator: SMA(hl2, fast) - SMA(hl2, slow).
type AO struct {
	fastSMA *SMA
	slowSMA *SMA
	value   float64
}

// NewAO creates an Awesome Oscillator indicator.
func NewAO(fastPeriod, slowPeriod int) *AO {
	if fastPeriod <= 0 {
		fastPeriod = DefaultAOFastPeriod
	}
	if slowPeriod <= 0 {
		slowPeriod = DefaultAOSlowPeriod
	}
	return &AO{
		fastSMA: NewSMA(fastPeriod),
		slowSMA: NewSMA(slowPeriod),
	}
}

// Update accepts median price (H+L)/2 and returns AO value.
func (a *AO) Update(hl2 float64) float64 {
	fast := a.fastSMA.Update(hl2)
	slow := a.slowSMA.Update(hl2)
	a.value = fast - slow
	return a.value
}

func (a *AO) Value() float64 {
	return a.value
}

var _ Indicator = (*AO)(nil)

// FractalStatus holds Williams fractal detection for the center candle in a 5-bar window.
type FractalStatus struct {
	UpFractal   bool
	DownFractal bool
	CenterHigh  float64
	CenterLow   float64
}

type candleHL struct {
	high float64
	low  float64
}

// WilliamsFractals detects Bill Williams fractals using a 5-candle ring buffer.
type WilliamsFractals struct {
	buf   [5]candleHL
	idx   int
	count int
}

// NewWilliamsFractals creates a streaming Williams fractals detector.
func NewWilliamsFractals() *WilliamsFractals {
	return &WilliamsFractals{}
}

// UpdateCandle ingests a new candle and returns fractal status for the center bar (N-2).
func (w *WilliamsFractals) UpdateCandle(high, low float64) FractalStatus {
	w.buf[w.idx] = candleHL{high: high, low: low}
	w.idx = (w.idx + 1) % 5
	if w.count < 5 {
		w.count++
	}
	if w.count < 5 {
		return FractalStatus{}
	}

	centerIdx := (w.idx + 2) % 5
	center := w.buf[centerIdx]

	status := FractalStatus{
		CenterHigh: center.high,
		CenterLow:  center.low,
	}

	if isUpFractal(w.buf, centerIdx) {
		status.UpFractal = true
	}
	if isDownFractal(w.buf, centerIdx) {
		status.DownFractal = true
	}

	return status
}

func isUpFractal(buf [5]candleHL, centerIdx int) bool {
	centerHigh := buf[centerIdx].high
	for i := range buf {
		if i == centerIdx {
			continue
		}
		if buf[i].high >= centerHigh {
			return false
		}
	}
	return true
}

func isDownFractal(buf [5]candleHL, centerIdx int) bool {
	centerLow := buf[centerIdx].low
	for i := range buf {
		if i == centerIdx {
			continue
		}
		if buf[i].low <= centerLow {
			return false
		}
	}
	return true
}

// AOValues computes Awesome Oscillator over high/low series (batch wrapper).
func AOValues(highs, lows []float64, fastPeriod, slowPeriod int) ([]float64, error) {
	if fastPeriod <= 0 {
		fastPeriod = DefaultAOFastPeriod
	}
	if slowPeriod <= 0 {
		slowPeriod = DefaultAOSlowPeriod
	}

	if len(highs) != len(lows) {
		return nil, errors.New("highs and lows length mismatch")
	}
	if len(highs) < slowPeriod {
		return nil, errors.New("not enough data for AO")
	}

	ao := NewAO(fastPeriod, slowPeriod)
	out := make([]float64, len(highs))
	for i := range highs {
		out[i] = ao.Update((highs[i] + lows[i]) / 2)
	}
	return out, nil
}

// AOValuesFromKlines is a convenience wrapper over AOValues.
func AOValuesFromKlines(klines []exchange.Kline, fastPeriod, slowPeriod int) ([]float64, error) {
	highs, lows, _ := OHLCFromKlines(klines)
	return AOValues(highs, lows, fastPeriod, slowPeriod)
}

// OHLCFromKlines extracts high, low and close series from candles.
func OHLCFromKlines(klines []exchange.Kline) (highs, lows, closes []float64) {
	highs = make([]float64, len(klines))
	lows = make([]float64, len(klines))
	closes = make([]float64, len(klines))

	for i, k := range klines {
		highs[i] = k.High
		lows[i] = k.Low
		closes[i] = k.Close
	}

	return highs, lows, closes
}

// WilliamsFractalPeaks scans klines and returns detected Williams fractals as Peak slice.
func WilliamsFractalPeaks(klines []exchange.Kline) []Peak {
	if len(klines) < 5 {
		return nil
	}

	wf := NewWilliamsFractals()
	var peaks []Peak

	for i, k := range klines {
		status := wf.UpdateCandle(k.High, k.Low)
		centerIdx := i - 2
		if centerIdx < 0 {
			continue
		}
		if status.UpFractal {
			peaks = append(peaks, Peak{
				Index: centerIdx,
				Value: status.CenterHigh,
				Type:  PeakHigh,
			})
		}
		if status.DownFractal {
			peaks = append(peaks, Peak{
				Index: centerIdx,
				Value: status.CenterLow,
				Type:  PeakLow,
			})
		}
	}

	return peaks
}
