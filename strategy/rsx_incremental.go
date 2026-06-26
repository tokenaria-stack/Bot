package strategy

// rsxMarkerState tracks RSX pivot/divergence markers incrementally (O(1) per bar).
type rsxMarkerState struct {
	prices        []float64
	rsx           []float64
	markers       map[int]string
	latest        string
	lastPivotHigh int
	lastPivotLow  int
	lookback      int

	// TradingView rolling divergence state (rsx_div_tv.go).
	tvCloses       []float64
	tvRSX          []float64
	tvMaxClose     float64
	tvMaxRSX       float64
	tvMinClose     float64
	tvMinRSX       float64
	tvHasMax       bool
	tvHasMin       bool
	tvMaxCloseHist []float64
	tvMinCloseHist []float64

	snap rsxMarkerSnapshot
}

type rsxMarkerSnapshot struct {
	prices         []float64
	rsx            []float64
	markers        map[int]string
	latest         string
	lastPivotHigh  int
	lastPivotLow   int
	tvCloses       []float64
	tvRSX          []float64
	tvMaxClose     float64
	tvMaxRSX       float64
	tvMinClose     float64
	tvMinRSX       float64
	tvHasMax       bool
	tvHasMin       bool
	tvMaxCloseHist []float64
	tvMinCloseHist []float64
}

func newRSXMarkerState(lookback int) rsxMarkerState {
	if lookback <= 0 {
		lookback = GetRSXSettings().DivLookback
	}
	return rsxMarkerState{
		markers:       make(map[int]string),
		lastPivotHigh: -1,
		lastPivotLow:  -1,
		lookback:      lookback,
	}
}

func (s *rsxMarkerState) reset() {
	s.prices = s.prices[:0]
	s.rsx = s.rsx[:0]
	s.markers = make(map[int]string)
	s.latest = ""
	s.lastPivotHigh = -1
	s.lastPivotLow = -1
	s.tvCloses = s.tvCloses[:0]
	s.tvRSX = s.tvRSX[:0]
	s.tvMaxCloseHist = s.tvMaxCloseHist[:0]
	s.tvMinCloseHist = s.tvMinCloseHist[:0]
	s.tvHasMax = false
	s.tvHasMin = false
}

func (s *rsxMarkerState) appendBar(high, low, close, rsxVal float64) {
	settings := GetRSXSettings()
	price := RSXSourcePrice(high, low, close, settings.Source)
	s.prices = append(s.prices, price)
	s.rsx = append(s.rsx, rsxVal)

	if RSXUsesFractalDiv() {
		radius := RSXPivotRadius()
		confirmIdx := len(s.rsx) - 1 - radius
		if confirmIdx >= radius {
			s.markFractalPivotAt(confirmIdx)
		}
		macroIdx := len(s.rsx) - 1 - rsxMacroPivotRadius
		if macroIdx >= radius {
			s.tryFractalMacroOnlyMarker(macroIdx)
		}
	} else {
		s.appendBarTV(close, rsxVal)
	}

	barIdx := len(s.rsx) - 1
	if m, ok := s.markers[barIdx]; ok {
		s.latest = m
	} else {
		s.latest = ""
	}
}

func (s *rsxMarkerState) recentTradingMarker(memoryBars int) string {
	if memoryBars <= 0 {
		memoryBars = RSXSignalMemoryBars
	}
	n := len(s.rsx)
	if !RSXUsesFractalDiv() {
		n = len(s.tvRSX)
	}
	if n == 0 {
		return ""
	}
	from := n - memoryBars
	if from < 0 {
		from = 0
	}
	best := ""
	bestStrength := 0
	for i := n - 1; i >= from; i-- {
		m := s.markers[i]
		strength, ok := rsxTradingMarkerStrength[m]
		if !ok {
			continue
		}
		if strength > bestStrength {
			best = m
			bestStrength = strength
		}
	}
	return best
}

func (s *rsxMarkerState) markerAt(barIndex int) string {
	return s.markers[barIndex]
}

func (s *rsxMarkerState) SaveState() {
	s.snap.prices = append([]float64(nil), s.prices...)
	s.snap.rsx = append([]float64(nil), s.rsx...)
	s.snap.markers = make(map[int]string, len(s.markers))
	for k, v := range s.markers {
		s.snap.markers[k] = v
	}
	s.snap.latest = s.latest
	s.snap.lastPivotHigh = s.lastPivotHigh
	s.snap.lastPivotLow = s.lastPivotLow
	s.snap.tvCloses = append([]float64(nil), s.tvCloses...)
	s.snap.tvRSX = append([]float64(nil), s.tvRSX...)
	s.snap.tvMaxClose = s.tvMaxClose
	s.snap.tvMaxRSX = s.tvMaxRSX
	s.snap.tvMinClose = s.tvMinClose
	s.snap.tvMinRSX = s.tvMinRSX
	s.snap.tvHasMax = s.tvHasMax
	s.snap.tvHasMin = s.tvHasMin
	s.snap.tvMaxCloseHist = append([]float64(nil), s.tvMaxCloseHist...)
	s.snap.tvMinCloseHist = append([]float64(nil), s.tvMinCloseHist...)
}

func (s *rsxMarkerState) RestoreState() {
	s.prices = append([]float64(nil), s.snap.prices...)
	s.rsx = append([]float64(nil), s.snap.rsx...)
	s.markers = make(map[int]string, len(s.snap.markers))
	for k, v := range s.snap.markers {
		s.markers[k] = v
	}
	s.latest = s.snap.latest
	s.lastPivotHigh = s.snap.lastPivotHigh
	s.lastPivotLow = s.snap.lastPivotLow
	s.tvCloses = append([]float64(nil), s.snap.tvCloses...)
	s.tvRSX = append([]float64(nil), s.snap.tvRSX...)
	s.tvMaxClose = s.snap.tvMaxClose
	s.tvMaxRSX = s.snap.tvMaxRSX
	s.tvMinClose = s.snap.tvMinClose
	s.tvMinRSX = s.snap.tvMinRSX
	s.tvHasMax = s.snap.tvHasMax
	s.tvHasMin = s.snap.tvHasMin
	s.tvMaxCloseHist = append([]float64(nil), s.snap.tvMaxCloseHist...)
	s.tvMinCloseHist = append([]float64(nil), s.snap.tvMinCloseHist...)
}
