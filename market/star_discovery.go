package market

import (
	"math"
	"sort"
)

// Discovery law, fixed before any coordinate is inspected.
//
// The 2026 wall is 1767225600000. Stars at or after that time are
// in the join and are not used to place slices or to name a region.
//
// A coordinate is sliced only where its own OK flag is true.
// Five slices. The edges are the 20, 40, 60, and 80 percent
// ranks of the discovery-period observations. The same edges
// are not refit per coordinate family and the count of slices
// is not searched.
//
// An OK-false value is its own group. It is not slice zero.
// A stored 0 with OK false is missing. A real 0 with OK true
// stays in whatever slice its rank gives it.
//
// No coordinate is combined with another. No region is named
// from these slices. Naming would require a cutoff chosen
// after seeing the tables.
const (
	DiscoveryHoldoutAt = 1767225600000
	DiscoverySlices    = 5
	DiscoveryWindow    = 96
	DiscoveryBarMs     = int64(15 * 60 * 1000)
)

// PathFact is the three measurements for one Star.
// SurvivalOK false means the frozen stop was not a measurement
// on that Star. FavorableOK false means the ATR excursion is
// not a full-window reading. Zero is not used as the missing test.
type PathFact struct {
	DecisionAt  int64
	Side        string
	StopClass   string
	SurvivalOK  bool
	Survived    bool
	Favorable   float64
	FavorableOK bool
	Adverse     float64
	AdverseOK   bool
}

// ClassifyPath reads one outcome row.
// SURVIVED_WINDOW survived the stop through the window.
// STOP_FIRST and UNORDERED_BAR reached the stop. Unordered is
// not called survival.
// Every other status has no survival reading.
// Favorable and adverse are full-window ATR excursions.
// They stay absent when the ATR excursion is absent or the
// window is incomplete. Travel after the stop remains inside
// the excursion. R levels are not read.
func ClassifyPath(status string, excursionOK, atrExcursion bool, mfeATR, maeATR float64) (class string, survivalOK, survived, excursion bool) {
	switch status {
	case "SURVIVED_WINDOW":
		class, survivalOK, survived = "survived", true, true
	case "STOP_FIRST":
		class, survivalOK = "stop_first", true
	case "UNORDERED_BAR":
		class, survivalOK = "unordered", true
	default:
		class = "absent"
	}
	excursion = excursionOK && atrExcursion && status != "INCOMPLETE_WINDOW" && status != "PATH_GAP"
	if excursion && (!starFinite(mfeATR) || !starFinite(maeATR)) {
		excursion = false
	}
	return class, survivalOK, survived, excursion
}

// PathSummary is a count and a set of percentile readings.
// The percentiles are descriptions. They are not thresholds.
type PathSummary struct {
	Stars          int
	SurvivalOK     int
	Survived       int
	StopFirst      int
	Unordered      int
	SurvivalAbsent int
	FavorableOK    int
	AdverseOK      int
	FavorableP10   float64
	FavorableP25   float64
	FavorableP50   float64
	FavorableP75   float64
	FavorableP90   float64
	AdverseP10     float64
	AdverseP25     float64
	AdverseP50     float64
	AdverseP75     float64
	AdverseP90     float64
}

// SummarizePath describes one population. It does not drop a Star.
func SummarizePath(facts []PathFact) PathSummary {
	var s PathSummary
	s.Stars = len(facts)
	fav := make([]float64, 0, len(facts))
	adv := make([]float64, 0, len(facts))
	for _, f := range facts {
		if !f.SurvivalOK {
			s.SurvivalAbsent++
		} else {
			s.SurvivalOK++
			switch f.StopClass {
			case "survived":
				s.Survived++
			case "stop_first":
				s.StopFirst++
			case "unordered":
				s.Unordered++
			default:
				s.SurvivalAbsent++
				s.SurvivalOK--
			}
		}
		if f.FavorableOK {
			s.FavorableOK++
			fav = append(fav, f.Favorable)
		}
		if f.AdverseOK {
			s.AdverseOK++
			adv = append(adv, f.Adverse)
		}
	}
	s.FavorableP10, s.FavorableP25, s.FavorableP50, s.FavorableP75, s.FavorableP90 = percentiles(fav)
	s.AdverseP10, s.AdverseP25, s.AdverseP50, s.AdverseP75, s.AdverseP90 = percentiles(adv)
	return s
}

func percentiles(xs []float64) (p10, p25, p50, p75, p90 float64) {
	if len(xs) == 0 {
		return 0, 0, 0, 0, 0
	}
	sorted := append([]float64(nil), xs...)
	sort.Float64s(sorted)
	return rank(sorted, 0.10), rank(sorted, 0.25), rank(sorted, 0.50), rank(sorted, 0.75), rank(sorted, 0.90)
}

func rank(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	i := int(math.Floor(float64(n-1) * p))
	if i < 0 {
		i = 0
	}
	if i >= n {
		i = n - 1
	}
	return sorted[i]
}

func SliceEdges(xs []float64) (edges [4]float64, ok bool) {
	if len(xs) < DiscoverySlices {
		return edges, false
	}
	sorted := append([]float64(nil), xs...)
	sort.Float64s(sorted)
	edges[0] = rank(sorted, 0.20)
	edges[1] = rank(sorted, 0.40)
	edges[2] = rank(sorted, 0.60)
	edges[3] = rank(sorted, 0.80)
	return edges, true
}

// SliceIndex places one observation. OK false is not a slice.
func SliceIndex(edges [4]float64, v float64, ok bool) (idx int, in bool) {
	if !ok {
		return 0, false
	}
	for i, e := range edges {
		if v <= e {
			return i, true
		}
	}
	return DiscoverySlices - 1, true
}

// EpisodeRuns groups Stars that follow the previous Star inside the
// 96-bar window. The run is a description of overlap. It is not a
// feature and it does not delete a Star.
type EpisodeRuns struct {
	Runs           int
	MaxRun         int
	MedianRun      int
	StarsAfterPrev int
}

func DescribeEpisodes(times []int64) EpisodeRuns {
	var out EpisodeRuns
	if len(times) == 0 {
		return out
	}
	window := int64(DiscoveryWindow) * DiscoveryBarMs
	lengths := make([]int, 0, 64)
	length := 1
	out.Runs = 1
	for i := 1; i < len(times); i++ {
		if times[i] > times[i-1] && times[i]-times[i-1] <= window {
			length++
			out.StarsAfterPrev++
		} else {
			lengths = append(lengths, length)
			length = 1
			out.Runs++
		}
	}
	lengths = append(lengths, length)
	out.MaxRun = lengths[0]
	for _, n := range lengths {
		if n > out.MaxRun {
			out.MaxRun = n
		}
	}
	sorted := append([]int(nil), lengths...)
	sort.Ints(sorted)
	out.MedianRun = sorted[(len(sorted)-1)/2]
	return out
}

// CoordObservation is one Schema 3 or Matrix number under its frozen OK rule.
type CoordObservation struct {
	Name  string
	Value float64
	OK    bool
}

// RawObservations is the 142 schema-3 numbers in a fixed order.
// VWEMA, orange mid, and the channel bounds are observations when
// the bar is present and the number is finite. ValuesOK is not the gate.
func RawObservations(row StarSnapshotV3) []CoordObservation {
	out := make([]CoordObservation, 0, 142)
	out = appendTF(out, "M15", row.M15)
	out = appendTF(out, "H1", row.H1)
	out = appendTF(out, "H4", row.H4)
	out = appendTF(out, "D1", row.D1)
	out = append(out, flagObs("RSX", row.RSX, row.RSXOK))
	out = append(out, flagObs("RSXSlope", row.RSXSlope, row.RSXSlopeOK))
	out = append(out, flagObs("RSXAccel", row.RSXAccel, row.RSXAccelOK))
	out = append(out, flagObs("Signal", row.Signal, row.SignalOK))
	out = append(out, flagObs("SignalSlope", row.SignalSlope, row.SignalSlopeOK))
	out = append(out, flagObs("RSXMinusSignal", row.RSXMinusSignal, row.RSXMinusOK))
	out = appendRSX(out, "H1RSX", row.H1, row.H1RSX)
	out = appendRSX(out, "H4RSX", row.H4, row.H4RSX)
	out = appendRSX(out, "D1RSX", row.D1, row.D1RSX)
	out = append(out, flagObs("TVAge", float64(row.TVAge), row.TVAgeOK))
	return out
}

func RelationObservations(rel StarRelations) []CoordObservation {
	names := RelationNames()
	values := rel.Relations()
	out := make([]CoordObservation, len(values))
	for i := range values {
		out[i] = CoordObservation{Name: names[i], Value: values[i].Value, OK: values[i].OK}
	}
	return out
}

func appendTF(dst []CoordObservation, name string, tf StarTF) []CoordObservation {
	p := tf.Present
	dst = append(dst, finiteObs(name+".Vwema", tf.Vwema, p))
	dst = append(dst, finiteObs(name+".ChanMid", tf.ChanMid, p))
	dst = append(dst, finiteObs(name+".ChanUp", tf.ChanUp, p))
	dst = append(dst, finiteObs(name+".ChanDn", tf.ChanDn, p))
	dst = append(dst, flagObs(name+".Slope", tf.Slope, p && tf.SlopeOK))
	dst = append(dst, flagObs(name+".Width", tf.Width, p && tf.WidthOK))
	dst = append(dst, flagObs(name+".WidthChange", tf.WidthChange, p && tf.WidthChangeOK))
	dst = append(dst, flagObs(name+".Distance", tf.Distance, p && tf.DistanceOK))
	dst = append(dst, flagObs(name+".MidSlope", tf.MidSlope, p && tf.MidSlopeOK))
	dst = append(dst, flagObs(name+".MidAccel", tf.MidAccel, p && tf.MidAccelOK))
	dst = append(dst, flagObs(name+".Ema5", tf.Ema5, p && tf.Ema5OK))
	dst = append(dst, flagObs(name+".Ema5Slope", tf.Ema5Slope, p && tf.Ema5SlopeOK))
	dst = append(dst, flagObs(name+".Ema5Accel", tf.Ema5Accel, p && tf.Ema5AccelOK))
	dst = append(dst, flagObs(name+".Ema12", tf.Ema12, p && tf.Ema12OK))
	dst = append(dst, flagObs(name+".Ema12Slope", tf.Ema12Slope, p && tf.Ema12SlopeOK))
	dst = append(dst, flagObs(name+".VwemaAccel", tf.VwemaAccel, p && tf.VwemaAccelOK))
	dst = append(dst, flagObs(name+".RsiClose", tf.RsiClose, p && tf.RsiCloseOK))
	dst = append(dst, flagObs(name+".RsiCloseSlope", tf.RsiCloseSlope, p && tf.RsiCloseSlopeOK))
	dst = append(dst, flagObs(name+".RsiCloseAccel", tf.RsiCloseAccel, p && tf.RsiCloseAccelOK))
	dst = append(dst, flagObs(name+".CloseMid", tf.CloseMid, p && tf.CloseMidOK))
	dst = append(dst, flagObs(name+".CloseUp", tf.CloseUp, p && tf.CloseUpOK))
	dst = append(dst, flagObs(name+".CloseDn", tf.CloseDn, p && tf.CloseDnOK))
	dst = append(dst, flagObs(name+".CloseWidth", tf.CloseWidth, p && tf.CloseWidthOK))
	dst = append(dst, flagObs(name+".CloseWidthChange", tf.CloseWidthChange, p && tf.CloseWidthChangeOK))
	dst = append(dst, flagObs(name+".Ema7", tf.Ema7, p && tf.Ema7OK))
	dst = append(dst, flagObs(name+".Ema7Slope", tf.Ema7Slope, p && tf.Ema7SlopeOK))
	dst = append(dst, flagObs(name+".Ema7Accel", tf.Ema7Accel, p && tf.Ema7AccelOK))
	dst = append(dst, flagObs(name+".Macd", tf.Macd, p && tf.MacdOK))
	dst = append(dst, flagObs(name+".MacdSlope", tf.MacdSlope, p && tf.MacdSlopeOK))
	dst = append(dst, flagObs(name+".MacdAccel", tf.MacdAccel, p && tf.MacdAccelOK))
	return dst
}

func appendRSX(dst []CoordObservation, name string, tf StarTF, rsx StarRSX) []CoordObservation {
	p := tf.Present
	dst = append(dst, flagObs(name+".Value", rsx.Value, p && rsx.ValueOK))
	dst = append(dst, flagObs(name+".Slope", rsx.Slope, p && rsx.SlopeOK))
	dst = append(dst, flagObs(name+".Accel", rsx.Accel, p && rsx.AccelOK))
	dst = append(dst, flagObs(name+".Signal", rsx.Signal, p && rsx.SignalOK))
	dst = append(dst, flagObs(name+".SignalSlope", rsx.SignalSlope, p && rsx.SignalSlopeOK))
	return dst
}

func finiteObs(name string, v float64, present bool) CoordObservation {
	if !present || !starFinite(v) {
		return CoordObservation{Name: name}
	}
	return CoordObservation{Name: name, Value: v, OK: true}
}

func flagObs(name string, v float64, ok bool) CoordObservation {
	if !ok {
		return CoordObservation{Name: name}
	}
	return CoordObservation{Name: name, Value: v, OK: true}
}

// RelationNames is the frozen 49 Matrix IDs in audit order.
func RelationNames() [49]string {
	return [49]string{
		"M15Ema7MinusMacd", "M15Ema7MinusCloseMid", "M15RsiCloseMinusCloseMid", "M15VwemaSlopeMinusMidSlope",
		"H1Ema7MinusMacd", "H1Ema7MinusCloseMid", "H1RsiCloseMinusCloseMid", "H1VwemaSlopeMinusMidSlope",
		"H4Ema7MinusMacd", "H4Ema7MinusCloseMid", "H4RsiCloseMinusCloseMid", "H4VwemaSlopeMinusMidSlope",
		"D1Ema7MinusMacd", "D1Ema7MinusCloseMid", "D1RsiCloseMinusCloseMid", "D1VwemaSlopeMinusMidSlope",
		"H1RsxMinusSignal", "H4RsxMinusSignal", "D1RsxMinusSignal",
		"M15H1OrangeMid", "M15H1Vwema", "M15H1Ema5", "M15H1Ema12", "M15H1RsiClose", "M15H1Ema7", "M15H1Macd", "M15H1Rsx",
		"H1H4OrangeMid", "H1H4Vwema", "H1H4Ema5", "H1H4Ema12", "H1H4RsiClose", "H1H4Ema7", "H1H4Macd", "H1H4Rsx",
		"H4D1OrangeMid", "H4D1Vwema", "H4D1Ema5", "H4D1Ema12", "H4D1RsiClose", "H4D1Ema7", "H4D1Macd", "H4D1Rsx",
		"M15H1Width", "M15H1CloseWidth", "H1H4Width", "H1H4CloseWidth", "H4D1Width", "H4D1CloseWidth",
	}
}

// CoordPicture is one coordinate under the fixed slice law.
// Missing is the OK-false group. Slices are discovery-period only.
type CoordPicture struct {
	Name    string
	EdgesOK bool
	Edges   [4]float64
	Missing PathSummary
	Slices  [5]PathSummary
}

func DescribeCoordinate(name string, facts []PathFact, value []float64, ok, discovery []bool) CoordPicture {
	pic := CoordPicture{Name: name}
	var sample []float64
	for i := range facts {
		if discovery[i] && ok[i] {
			sample = append(sample, value[i])
		}
	}
	edges, edgesOK := SliceEdges(sample)
	pic.EdgesOK = edgesOK
	pic.Edges = edges
	groups := make([][]PathFact, DiscoverySlices+1)
	for i := range facts {
		if !discovery[i] {
			continue
		}
		if !ok[i] {
			groups[DiscoverySlices] = append(groups[DiscoverySlices], facts[i])
			continue
		}
		if !edgesOK {
			continue
		}
		idx, in := SliceIndex(edges, value[i], true)
		if !in {
			groups[DiscoverySlices] = append(groups[DiscoverySlices], facts[i])
			continue
		}
		groups[idx] = append(groups[idx], facts[i])
	}
	for i := 0; i < DiscoverySlices; i++ {
		pic.Slices[i] = SummarizePath(groups[i])
	}
	pic.Missing = SummarizePath(groups[DiscoverySlices])
	return pic
}
