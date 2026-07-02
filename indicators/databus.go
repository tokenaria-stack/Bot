package indicators

// DataBus is the read-only series registry for stateless scanners.
// Implementations (e.g. strategy.Marker) own aligned per-bar indicator history.
// Series accessors must return zero-copy views; callers must not mutate returned slices.
type DataBus interface {
	JurikSeries() []float64
	WozduhRedSeries() []float64
	WozduhGreenSeries() []float64
	RSXPriceSeries() []float64
	CloseSeries() []float64
}
