package indicators

import "math"

const DefaultJurikRSXLength = 14
const DefaultRSXSignalPeriod = 9

// RSXSignalLine is SMA(RSX) — the RSX signal line (default period 9).
type RSXSignalLine struct {
	sma *SMA
}

// NewRSXSignalLine creates an RSX signal line tracker.
func NewRSXSignalLine(period int) *RSXSignalLine {
	if period <= 0 {
		period = DefaultRSXSignalPeriod
	}
	return &RSXSignalLine{sma: NewSMA(period)}
}

// Update feeds the latest RSX value and returns the current signal line value.
func (s *RSXSignalLine) Update(rsxValue float64) float64 {
	return s.sma.Update(rsxValue)
}

// Value returns the latest signal line without advancing state.
func (s *RSXSignalLine) Value() float64 {
	return s.sma.Value()
}

// Period returns the active SMA window.
func (s *RSXSignalLine) Period() int {
	if s == nil || s.sma == nil {
		return 0
	}
	return s.sma.Period()
}

// Reconfigure replaces the SMA window and clears its buffer.
func (s *RSXSignalLine) Reconfigure(period int) {
	if period <= 0 {
		period = DefaultRSXSignalPeriod
	}
	s.sma = NewSMA(period)
}

func (s *RSXSignalLine) SaveState() {
	s.sma.SaveState()
}

func (s *RSXSignalLine) RestoreState() {
	s.sma.RestoreState()
}

// JurikRSX is Mark Jurik's Relative Strength Quality Index — a noise-free RSI
// with minimal lag. Feed median price (hlc3) or any scalar stream via Update.
type JurikRSX struct {
	length int
	f18    float64
	f20    float64

	prevF8 float64

	f28, f30 float64
	f38, f40 float64
	f48, f50 float64
	f58, f60 float64
	f68, f70 float64
	f78, f80 float64

	prevF88 float64
	prevF90 float64

	value float64

	snap jurikRSXSnapshot
}

type jurikRSXSnapshot struct {
	prevF8  float64
	f28     float64
	f30     float64
	f38     float64
	f40     float64
	f48     float64
	f50     float64
	f58     float64
	f60     float64
	f68     float64
	f70     float64
	f78     float64
	f80     float64
	prevF88 float64
	prevF90 float64
	value   float64
}

// NewJurikRSX creates a Jurik RSX indicator (default length 14).
func NewJurikRSX(length int) *JurikRSX {
	if length <= 0 {
		length = DefaultJurikRSXLength
	}
	f18 := 3.0 / (float64(length) + 2.0)
	return &JurikRSX{
		length: length,
		f18:    f18,
		f20:    1.0 - f18,
	}
}

func (j *JurikRSX) Update(val float64) float64 {
	f8 := 100.0 * val
	f10 := j.prevF8
	v8 := f8 - f10

	f28 := j.f20*j.f28 + j.f18*v8
	f30 := j.f18*f28 + j.f20*j.f30
	vC := f28*1.5 - f30*0.5

	f38 := j.f20*j.f38 + j.f18*vC
	f40 := j.f18*f38 + j.f20*j.f40
	v10 := f38*1.5 - f40*0.5

	f48 := j.f20*j.f48 + j.f18*v10
	f50 := j.f18*f48 + j.f20*j.f50
	v14 := f48*1.5 - f50*0.5

	f58 := j.f20*j.f58 + j.f18*math.Abs(v8)
	f60 := j.f18*f58 + j.f20*j.f60
	v18 := f58*1.5 - f60*0.5

	f68 := j.f20*j.f68 + j.f18*v18
	f70 := j.f18*f68 + j.f20*j.f70
	v1C := f68*1.5 - f70*0.5

	f78 := j.f20*j.f78 + j.f18*v1C
	f80 := j.f18*f78 + j.f20*j.f80
	v20 := f78*1.5 - f80*0.5

	var f90Underscore float64
	switch {
	case j.prevF90 == 0:
		f90Underscore = 1
	case j.prevF88 <= j.prevF90:
		f90Underscore = j.prevF88 + 1
	default:
		f90Underscore = j.prevF90 + 1
	}

	var f88 float64
	if j.prevF90 == 0 && float64(j.length)-1 >= 5 {
		f88 = float64(j.length) - 1
	} else {
		f88 = 5
	}

	f0 := 0.0
	if f88 >= f90Underscore && f8 != f10 {
		f0 = 1
	}

	var f90 float64
	if f88 == f90Underscore && f0 == 0 {
		f90 = 0
	} else {
		f90 = f90Underscore
	}

	var v4 float64
	if f88 < f90 && v20 > 0 {
		v4 = (v14/v20 + 1) * 50
	} else {
		v4 = 50
	}

	rsx := clampRSX(v4)

	j.prevF8 = f8
	j.f28, j.f30 = f28, f30
	j.f38, j.f40 = f38, f40
	j.f48, j.f50 = f48, f50
	j.f58, j.f60 = f58, f60
	j.f68, j.f70 = f68, f70
	j.f78, j.f80 = f78, f80
	j.prevF88 = f88
	j.prevF90 = f90
	j.value = rsx

	return rsx
}

// Reconfigure replaces the RSX smoothing length and clears internal state.
func (j *JurikRSX) Reconfigure(length int) {
	if length <= 0 {
		length = DefaultJurikRSXLength
	}
	f18 := 3.0 / (float64(length) + 2.0)
	*j = JurikRSX{
		length: length,
		f18:    f18,
		f20:    1.0 - f18,
	}
}

func (j *JurikRSX) Value() float64 {
	return j.value
}

// Length returns the active Jurik RSX smoothing period.
func (j *JurikRSX) Length() int {
	if j == nil {
		return 0
	}
	return j.length
}

func (j *JurikRSX) SaveState() {
	j.snap = jurikRSXSnapshot{
		prevF8:  j.prevF8,
		f28:     j.f28,
		f30:     j.f30,
		f38:     j.f38,
		f40:     j.f40,
		f48:     j.f48,
		f50:     j.f50,
		f58:     j.f58,
		f60:     j.f60,
		f68:     j.f68,
		f70:     j.f70,
		f78:     j.f78,
		f80:     j.f80,
		prevF88: j.prevF88,
		prevF90: j.prevF90,
		value:   j.value,
	}
}

func (j *JurikRSX) RestoreState() {
	j.prevF8 = j.snap.prevF8
	j.f28 = j.snap.f28
	j.f30 = j.snap.f30
	j.f38 = j.snap.f38
	j.f40 = j.snap.f40
	j.f48 = j.snap.f48
	j.f50 = j.snap.f50
	j.f58 = j.snap.f58
	j.f60 = j.snap.f60
	j.f68 = j.snap.f68
	j.f70 = j.snap.f70
	j.f78 = j.snap.f78
	j.f80 = j.snap.f80
	j.prevF88 = j.snap.prevF88
	j.prevF90 = j.snap.prevF90
	j.value = j.snap.value
}

func clampRSX(v float64) float64 {
	if v > 100 {
		return 100
	}
	if v < 0 {
		return 0
	}
	return v
}

var _ Indicator = (*JurikRSX)(nil)

// JurikRSXValues calculates Jurik RSX over a price series (batch wrapper).
func JurikRSXValues(data []float64, length int) []float64 {
	if len(data) == 0 || length <= 0 {
		return nil
	}
	return runIndicator(NewJurikRSX(length), data)
}
