package indicators

// DynamicFractal detects swing fractals with configurable left/right bar windows.
// Window size is leftBars + rightBars + 1; the center bar is confirmed after rightBars
// additional bars have arrived (zero look-ahead).
type DynamicFractal struct {
	leftBars  int
	rightBars int
	size      int
	buf       []candleHL
	idx       int
	count     int

	snapBuf   []candleHL
	snapIdx   int
	snapCount int
}

// NewDynamicFractal creates a streaming fractal detector with the given window halves.
func NewDynamicFractal(leftBars, rightBars int) *DynamicFractal {
	if leftBars <= 0 {
		leftBars = 2
	}
	if rightBars <= 0 {
		rightBars = 2
	}
	size := leftBars + rightBars + 1
	return &DynamicFractal{
		leftBars:  leftBars,
		rightBars: rightBars,
		size:      size,
		buf:       make([]candleHL, size),
	}
}

// UpdateCandle ingests a new candle and returns fractal status for the center bar.
func (d *DynamicFractal) UpdateCandle(high, low float64) FractalStatus {
	d.buf[d.idx] = candleHL{high: high, low: low}
	d.idx = (d.idx + 1) % d.size
	if d.count < d.size {
		d.count++
	}
	if d.count < d.size {
		return FractalStatus{}
	}

	centerIdx := (d.idx + d.leftBars) % d.size
	center := d.buf[centerIdx]

	status := FractalStatus{
		CenterHigh: center.high,
		CenterLow:  center.low,
	}
	if d.isUpFractal(centerIdx) {
		status.UpFractal = true
	}
	if d.isDownFractal(centerIdx) {
		status.DownFractal = true
	}
	return status
}

func (d *DynamicFractal) isUpFractal(centerIdx int) bool {
	centerHigh := d.buf[centerIdx].high
	for i := 0; i < d.size; i++ {
		if i == centerIdx {
			continue
		}
		if d.buf[i].high >= centerHigh {
			return false
		}
	}
	return true
}

func (d *DynamicFractal) isDownFractal(centerIdx int) bool {
	centerLow := d.buf[centerIdx].low
	for i := 0; i < d.size; i++ {
		if i == centerIdx {
			continue
		}
		if d.buf[i].low <= centerLow {
			return false
		}
	}
	return true
}

func (d *DynamicFractal) SaveState() {
	d.snapBuf = append(d.snapBuf[:0], d.buf...)
	d.snapIdx = d.idx
	d.snapCount = d.count
}

func (d *DynamicFractal) RestoreState() {
	if len(d.snapBuf) == d.size {
		copy(d.buf, d.snapBuf)
	}
	d.idx = d.snapIdx
	d.count = d.snapCount
}
