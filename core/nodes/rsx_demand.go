package nodes

// RSXWorkMask is a fixed analytical-capability bitmask for persistent Frame RSX work.
// Zero means no Jurik / TV / Fractal / DAG-ZZ fact work runs.
type RSXWorkMask uint32

const (
	NeedRSXCore RSXWorkMask = 1 << iota
	NeedRSTV
	NeedRSFractal
	NeedRSZZ
)

const RSXWorkAll = NeedRSXCore | NeedRSTV | NeedRSFractal | NeedRSZZ

var rsxPlotCore = map[string]struct{}{
	"line_rsx":        {},
	"line_rsx_signal": {},
}

var rsxFactFamily = map[string]RSXWorkMask{
	"rsx_tv_div":        NeedRSXCore | NeedRSTV,
	"rsx_tv_pivot":      NeedRSXCore | NeedRSTV,
	"rsx_fractal_div":   NeedRSXCore | NeedRSFractal,
	"rsx_fractal_pivot": NeedRSXCore | NeedRSFractal,
	"rsx_zz_div":        NeedRSXCore | NeedRSZZ,
}

// RSXWorkFromPlots unions RSXCore demand from scalar plot IDs.
func RSXWorkFromPlots(ids []string) RSXWorkMask {
	var m RSXWorkMask
	for _, id := range ids {
		if _, ok := rsxPlotCore[id]; ok {
			m |= NeedRSXCore
		}
	}
	return m
}

// RSXWorkFromFactSources unions compute-family bits from RSX fact source IDs.
func RSXWorkFromFactSources(ids []string) RSXWorkMask {
	var m RSXWorkMask
	for _, id := range ids {
		if bits, ok := rsxFactFamily[id]; ok {
			m |= bits
		}
	}
	return m
}

// RSXWorkFromClient is one WS client's contribution.
// plots nil/empty → WIRE-1 unfiltered → NeedRSXCore.
// facts == nil → omitted/null → all three families (and Core).
// facts non-nil empty → no fact-family demand.
func RSXWorkFromClient(plots []string, facts *[]string) RSXWorkMask {
	var m RSXWorkMask
	if len(plots) == 0 {
		m |= NeedRSXCore
	} else {
		m |= RSXWorkFromPlots(plots)
	}
	if facts == nil {
		m |= RSXWorkAll
	} else {
		m |= RSXWorkFromFactSources(*facts)
	}
	return m
}

// RSXClientDemand is one client's plot IDs plus optional facts pointer (nil = omitted).
type RSXClientDemand struct {
	Plots []string
	Facts *[]string
}

// RSXWorkFromClientSubscriptions ORs per-client demand for one Frame. No clients → 0.
func RSXWorkFromClientSubscriptions(clients []RSXClientDemand) RSXWorkMask {
	if len(clients) == 0 {
		return 0
	}
	var u RSXWorkMask
	for _, c := range clients {
		u |= RSXWorkFromClient(c.Plots, c.Facts)
	}
	return u
}
