package market

// dataBusLive is a scratch copy of parallel Frame series used during streaming restore merge.
type dataBusLive struct {
	jurik    []float64
	wozRed   []float64
	wozGreen []float64
	close    []float64
	rsxPrice []float64
}

func (a *Frame) captureDataBusLiveLocked() dataBusLive {
	return dataBusLive{
		jurik:    append([]float64(nil), a.JurikLines...),
		wozRed:   append([]float64(nil), a.WozduhRed...),
		wozGreen: append([]float64(nil), a.WozduhGreen...),
		close:    append([]float64(nil), a.closeLines...),
		rsxPrice: append([]float64(nil), a.rsxPriceLines...),
	}
}

// alignAllDataBusToKlinesLocked pads or truncates every DataBus series to len(klines).
// Must run with frame.mu held (before snap save and after trim).
func (a *Frame) alignAllDataBusToKlinesLocked() {
	n := len(a.klines)
	if n == 0 {
		a.clearDataBusLocked()
		return
	}
	a.JurikLines = alignFloatSeriesToLen(a.JurikLines, n, 0)
	a.WozduhRed = alignFloatSeriesToLen(a.WozduhRed, n, 0)
	a.WozduhGreen = alignFloatSeriesToLen(a.WozduhGreen, n, 0)
	a.closeLines = alignFloatSeriesToLen(a.closeLines, n, 0)
	a.rsxPriceLines = alignFloatSeriesToLen(a.rsxPriceLines, n, 0)
}

func alignFloatSeriesToLen(series []float64, n int, fill float64) []float64 {
	if n <= 0 {
		return series[:0]
	}
	if len(series) == n {
		return series
	}
	if len(series) > n {
		return series[:n]
	}
	out := make([]float64, n)
	copy(out, series)
	for i := len(series); i < n; i++ {
		out[i] = fill
	}
	return out
}

func tailAlignToN[T any](src []T, n int) []T {
	if n <= 0 {
		return nil
	}
	if len(src) == n {
		return append([]T(nil), src...)
	}
	if len(src) > n {
		return append([]T(nil), src[len(src)-n:]...)
	}
	return append([]T(nil), src...)
}

func mergeSnapFloats(live, snap []float64, n int) []float64 {
	out := make([]float64, n)
	snap = tailAlignToN(snap, n)
	copy(out, snap)
	for i := len(snap); i < n; i++ {
		if i < len(live) {
			out[i] = live[i]
		}
	}
	return out
}

func (a *Frame) restoreDataBusFromSnapLocked(s streamingSnapshot, live dataBusLive) {
	n := len(a.klines)
	if n == 0 {
		a.clearDataBusLocked()
		return
	}
	a.JurikLines = mergeSnapFloats(live.jurik, s.jurikLines, n)
	a.WozduhRed = mergeSnapFloats(live.wozRed, s.wozduhRed, n)
	a.WozduhGreen = mergeSnapFloats(live.wozGreen, s.wozduhGreen, n)
	a.closeLines = alignFloatSeriesToLen(live.close, n, 0)
	a.rsxPriceLines = alignFloatSeriesToLen(live.rsxPrice, n, 0)
	a.alignAllDataBusToKlinesLocked()
}

func writeBusSeries(series *[]float64, barIndex, klinesLen int, val float64) {
	if barIndex < 0 || barIndex >= klinesLen {
		return
	}
	targetLen := barIndex + 1
	for len(*series) < targetLen {
		*series = append(*series, 0)
	}
	(*series)[barIndex] = val
	if len(*series) > klinesLen {
		*series = (*series)[:klinesLen]
	}
}

func (a *Frame) clearDataBusLocked() {
	a.JurikLines = a.JurikLines[:0]
	a.WozduhRed = a.WozduhRed[:0]
	a.WozduhGreen = a.WozduhGreen[:0]
	a.closeLines = a.closeLines[:0]
	a.rsxPriceLines = a.rsxPriceLines[:0]
}

func (a *Frame) AppendJurikValue(barIndex int, val float64) {
	writeBusSeries(&a.JurikLines, barIndex, len(a.klines), val)
}

func (a *Frame) AppendWozduhRed(barIndex int, val float64) {
	writeBusSeries(&a.WozduhRed, barIndex, len(a.klines), val)
}

func (a *Frame) AppendWozduhGreen(barIndex int, val float64) {
	writeBusSeries(&a.WozduhGreen, barIndex, len(a.klines), val)
}

func (a *Frame) JurikSeries() []float64       { return a.JurikLines }
func (a *Frame) WozduhRedSeries() []float64   { return a.WozduhRed }
func (a *Frame) WozduhGreenSeries() []float64 { return a.WozduhGreen }
func (a *Frame) CloseSeries() []float64       { return a.closeLines }
func (a *Frame) RSXPriceSeries() []float64    { return a.rsxPriceLines }
