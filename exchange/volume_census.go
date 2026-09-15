package exchange

import "time"

// StoredVolume is one archive row used in census join.
type StoredVolume struct {
	OpenTime int64
	Volume   float64
}

// CensusCounts is VOLUME-INGEST-1 classification tallies.
type CensusCounts struct {
	Stored         int `json:"stored"`
	Compared       int `json:"compared"`
	NoAuthority    int `json:"no_authority"`
	AuthorityExtra int `json:"authority_extra"`
	TotalBase      int `json:"TOTAL_BASE"`
	TakerBuyBase   int `json:"TAKER_BUY_BASE"`
	TotalQuote     int `json:"TOTAL_QUOTE"`
	TakerBuyQuote  int `json:"TAKER_BUY_QUOTE"`
	ZeroMissing    int `json:"ZERO_OR_MISSING"`
	MatchNone      int `json:"MATCH_NONE"`
}

func (c *CensusCounts) addClass(cl VolumeClass) {
	switch cl {
	case VolumeClassTotalBase:
		c.TotalBase++
	case VolumeClassTakerBuyBase:
		c.TakerBuyBase++
	case VolumeClassTotalQuote:
		c.TotalQuote++
	case VolumeClassTakerBuyQuote:
		c.TakerBuyQuote++
	case VolumeClassZeroMissing:
		c.ZeroMissing++
	default:
		c.MatchNone++
	}
}

func (c CensusCounts) Dirty() bool {
	return c.TakerBuyBase > 0 || c.TotalQuote > 0 || c.TakerBuyQuote > 0 || c.MatchNone > 0
}

func (c CensusCounts) CaseBOnly() bool {
	return c.TakerBuyBase > 0 && c.TotalQuote == 0 && c.TakerBuyQuote == 0 && c.MatchNone == 0
}

// CensusSample is one classified mismatch for the VOLUME-INGEST-1 forensic appendix.
type CensusSample struct {
	OpenTime     int64       `json:"open_time"`
	Stored       float64     `json:"stored"`
	Base         float64     `json:"v"`
	TakerBuyBase float64     `json:"V"`
	Class        VolumeClass `json:"class"`
}

func JoinVolumeCensus(stored []StoredVolume, auth []VolumeAuthority) CensusCounts {
	c, _ := JoinVolumeCensusWithSamples(stored, auth, 0)
	return c
}

func JoinVolumeCensusWithSamples(stored []StoredVolume, auth []VolumeAuthority, maxPerClass int) (CensusCounts, []CensusSample) {
	am := make(map[int64]VolumeAuthority, len(auth))
	for _, a := range auth {
		am[a.OpenTime] = a
	}
	sm := make(map[int64]struct{}, len(stored))
	var z CensusCounts
	z.Stored = len(stored)
	perClass := map[VolumeClass]int{}
	var samples []CensusSample
	for _, s := range stored {
		sm[s.OpenTime] = struct{}{}
		a, ok := am[s.OpenTime]
		if !ok {
			z.NoAuthority++
			continue
		}
		z.Compared++
		cl := ClassifyStoredVolume(s.Volume, a)
		z.addClass(cl)
		if maxPerClass > 0 && cl != VolumeClassTotalBase && perClass[cl] < maxPerClass {
			perClass[cl]++
			samples = append(samples, CensusSample{
				OpenTime: s.OpenTime, Stored: s.Volume, Base: a.Base, TakerBuyBase: a.TakerBuyBase, Class: cl,
			})
		}
	}
	for ot := range am {
		if _, ok := sm[ot]; !ok {
			z.AuthorityExtra++
		}
	}
	return z, samples
}

// YearOfOpenTime is UTC calendar year of a bar open.
func YearOfOpenTime(openMs int64) int {
	return time.UnixMilli(openMs).UTC().Year()
}

// SplitCensusByYear joins globally then reports per UTC year of stored bars.
func SplitCensusByYear(stored []StoredVolume, auth []VolumeAuthority) map[int]CensusCounts {
	am := make(map[int64]VolumeAuthority, len(auth))
	for _, a := range auth {
		am[a.OpenTime] = a
	}
	by := map[int][]StoredVolume{}
	for _, s := range stored {
		y := YearOfOpenTime(s.OpenTime)
		by[y] = append(by[y], s)
	}
	out := map[int]CensusCounts{}
	for y, rows := range by {
		// Authority extras are not split by year in this helper; compared/no_auth are.
		var z CensusCounts
		z.Stored = len(rows)
		for _, s := range rows {
			a, ok := am[s.OpenTime]
			if !ok {
				z.NoAuthority++
				continue
			}
			z.Compared++
			z.addClass(ClassifyStoredVolume(s.Volume, a))
		}
		out[y] = z
	}
	return out
}
