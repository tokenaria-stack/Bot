package strategy

// dataBusLive is a scratch copy of parallel Marker series used during layer2 restore merge.
type dataBusLive struct {
	jurik    []float64
	wozRed   []float64
	wozGreen []float64
	close    []float64
	rsxPrice []float64
}

func (a *Marker) captureDataBusLiveLocked() dataBusLive {
	return dataBusLive{
		jurik:    append([]float64(nil), a.JurikLines...),
		wozRed:   append([]float64(nil), a.WozduhRed...),
		wozGreen: append([]float64(nil), a.WozduhGreen...),
		close:    append([]float64(nil), a.closeLines...),
		rsxPrice: append([]float64(nil), a.rsxPriceLines...),
	}
}

// alignAllDataBusToKlinesLocked pads or truncates every DataBus series to len(klines).
// Must run with analyst.mu held (before snap save and after trim).
func (a *Marker) alignAllDataBusToKlinesLocked() {
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

func (a *Marker) restoreDataBusFromSnapLocked(s layer2StreamingSnapshot, live dataBusLive) {
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

// batchDataBus is a static DataBus for REST/cold-path batch scans.
type batchDataBus struct {
	jurik  []float64
	red    []float64
	green  []float64
	prices []float64
	closes []float64
}

func newBatchDataBus(jurik, prices, closes []float64) *batchDataBus {
	return &batchDataBus{jurik: jurik, prices: prices, closes: closes}
}

func (b *batchDataBus) JurikSeries() []float64       { return b.jurik }
func (b *batchDataBus) WozduhRedSeries() []float64   { return b.red }
func (b *batchDataBus) WozduhGreenSeries() []float64 { return b.green }
func (b *batchDataBus) RSXPriceSeries() []float64    { return b.prices }
func (b *batchDataBus) CloseSeries() []float64       { return b.closes }

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

func (a *Marker) clearDataBusLocked() {
	a.JurikLines = a.JurikLines[:0]
	a.WozduhRed = a.WozduhRed[:0]
	a.WozduhGreen = a.WozduhGreen[:0]
	a.closeLines = a.closeLines[:0]
	a.rsxPriceLines = a.rsxPriceLines[:0]
}

func (a *Marker) AppendJurikValue(barIndex int, val float64) {
	writeBusSeries(&a.JurikLines, barIndex, len(a.klines), val)
}

func (a *Marker) AppendWozduhRed(barIndex int, val float64) {
	writeBusSeries(&a.WozduhRed, barIndex, len(a.klines), val)
}

func (a *Marker) AppendWozduhGreen(barIndex int, val float64) {
	writeBusSeries(&a.WozduhGreen, barIndex, len(a.klines), val)
}

func (a *Marker) recordDataBusBarLocked(barIndex int, sig FalconSignals) {
	n := len(a.klines)
	a.AppendJurikValue(barIndex, sig.JurikRSX)
	a.AppendWozduhRed(barIndex, sig.RedLine)
	a.AppendWozduhGreen(barIndex, sig.GreenLine)
	if barIndex < 0 || barIndex >= n {
		return
	}
	k := a.klines[barIndex]
	src := a.effectiveRSXSettings().Source
	writeBusSeries(&a.closeLines, barIndex, n, k.Close)
	writeBusSeries(&a.rsxPriceLines, barIndex, n, RSXSourcePrice(k.High, k.Low, k.Close, src))
}

func (a *Marker) JurikSeries() []float64       { return a.JurikLines }
func (a *Marker) WozduhRedSeries() []float64   { return a.WozduhRed }
func (a *Marker) WozduhGreenSeries() []float64 { return a.WozduhGreen }
func (a *Marker) CloseSeries() []float64       { return a.closeLines }
func (a *Marker) RSXPriceSeries() []float64    { return a.rsxPriceLines }
