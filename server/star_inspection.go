package server

import (
	"math"
	"sort"

	"trading_bot/market"
)

// Display grammar for the inspection shell.
// A coordinate's color position is fixed from the discovery-period
// readings of that coordinate. Holdout readings are colored with
// that range and do not enter it. The five research slices are not
// the color range. Path facts are not an input.

const (
	inspectionFamilySigned = "signed"
	inspectionFamilyLevel  = "level"
	inspectionFamilyWidth  = "width"
)

type inspectionSample struct {
	Value     float64
	OK        bool
	Discovery bool
}

type inspectionDomain struct {
	Family     string
	Saturation float64
	Rank       []float64
}

type inspectionReading struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Group    string   `json:"group"`
	Family   string   `json:"family"`
	Arrow    string   `json:"arrow,omitempty"`
	Value    float64  `json:"value"`
	OK       bool     `json:"ok"`
	Position *float64 `json:"position"`
}

func inspectionRank(sorted []float64, p float64) float64 {
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

func signedSaturation(sample []inspectionSample) float64 {
	var xs []float64
	for _, s := range sample {
		if s.Discovery && s.OK && starInspectFinite(s.Value) {
			xs = append(xs, s.Value)
		}
	}
	if len(xs) == 0 {
		return 0
	}
	sort.Float64s(xs)
	p10 := inspectionRank(xs, 0.10)
	p90 := inspectionRank(xs, 0.90)
	sat := math.Abs(p10)
	if math.Abs(p90) > sat {
		sat = math.Abs(p90)
	}
	return sat
}

func signedPosition(value, saturation float64) float64 {
	if !starInspectFinite(value) {
		return math.NaN()
	}
	if saturation == 0 {
		switch {
		case value > 0:
			return 1
		case value < 0:
			return 0
		default:
			return 0.5
		}
	}
	t := value / saturation
	if t > 1 {
		t = 1
	}
	if t < -1 {
		t = -1
	}
	return 0.5 + 0.5*t
}

func unsignedRank(sample []inspectionSample) []float64 {
	var xs []float64
	for _, s := range sample {
		if s.Discovery && s.OK && starInspectFinite(s.Value) {
			xs = append(xs, s.Value)
		}
	}
	sort.Float64s(xs)
	return xs
}

func unsignedPosition(value float64, sorted []float64) float64 {
	n := len(sorted)
	if n == 0 || !starInspectFinite(value) {
		return math.NaN()
	}
	if n == 1 {
		return 0.5
	}
	below := sort.Search(n, func(i int) bool { return sorted[i] >= value })
	above := sort.Search(n, func(i int) bool { return sorted[i] > value })
	equal := float64(above - below)
	return (float64(below) + 0.5*equal) / float64(n)
}

func buildInspectionDomain(family string, sample []inspectionSample) inspectionDomain {
	if family == inspectionFamilySigned {
		return inspectionDomain{Family: family, Saturation: signedSaturation(sample)}
	}
	return inspectionDomain{Family: family, Rank: unsignedRank(sample)}
}

func positionOnDomain(d inspectionDomain, value float64, ok bool) *float64 {
	if !ok || !starInspectFinite(value) {
		return nil
	}
	var p float64
	if d.Family == inspectionFamilySigned {
		p = signedPosition(value, d.Saturation)
	} else {
		p = unsignedPosition(value, d.Rank)
	}
	if math.IsNaN(p) {
		return nil
	}
	return &p
}

func starInspectFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func presentFinite(present bool, v float64) bool {
	return present && starInspectFinite(v)
}

type inspectionStar struct {
	Side         string
	DecisionAt   int64
	InspectionAt int64
	Holdout      bool
	Readings     []inspectionReading
	PathLabel    string
	Favorable    *float64
	Adverse      *float64
}

func inspectionPathLabel(class string) string {
	switch class {
	case "survived":
		return "Survived window"
	case "stop_first":
		return "Stop first"
	case "unordered":
		return "Stop order unresolved"
	default:
		return "No reading"
	}
}

func collectInspectionStars(snaps []market.StarSnapshotV3, rels []market.StarRelations, facts []market.PathFact) ([]inspectionStar, map[string]inspectionDomain, error) {
	if len(snaps) == 0 || len(snaps) != len(rels) || len(snaps) != len(facts) {
		return nil, nil, errInspectionJoin
	}
	raw := make([][]inspectionReading, len(snaps))
	for i := range snaps {
		if snaps[i].ConfirmedAt != facts[i].DecisionAt || snaps[i].Side != facts[i].Side || snaps[i].ConfirmedAt != rels[i].ConfirmedAt {
			return nil, nil, errInspectionJoin
		}
		raw[i] = starReadings(snaps[i], rels[i])
	}
	ids := map[string]string{}
	samples := map[string][]inspectionSample{}
	for i := range raw {
		discovery := snaps[i].ConfirmedAt < market.DiscoveryHoldoutAt
		for _, reading := range raw[i] {
			ids[reading.ID] = reading.Family
			samples[reading.ID] = append(samples[reading.ID], inspectionSample{
				Value: reading.Value, OK: reading.OK, Discovery: discovery,
			})
		}
	}
	domains := make(map[string]inspectionDomain, len(ids))
	for id, family := range ids {
		domains[id] = buildInspectionDomain(family, samples[id])
	}
	out := make([]inspectionStar, len(snaps))
	for i := range raw {
		readings := raw[i]
		for j := range readings {
			readings[j].Position = positionOnDomain(domains[readings[j].ID], readings[j].Value, readings[j].OK)
			if !readings[j].OK {
				readings[j].Value = 0
			}
		}
		fact := facts[i]
		star := inspectionStar{
			Side: snaps[i].Side, DecisionAt: snaps[i].ConfirmedAt, InspectionAt: snaps[i].M15.CloseTime,
			Holdout:  snaps[i].ConfirmedAt >= market.DiscoveryHoldoutAt,
			Readings: readings, PathLabel: inspectionPathLabel(fact.StopClass),
		}
		if fact.FavorableOK {
			v := fact.Favorable
			star.Favorable = &v
		}
		if fact.AdverseOK {
			v := fact.Adverse
			star.Adverse = &v
		}
		out[i] = star
	}
	return out, domains, nil
}

var errInspectionJoin = inspectionJoinError{}

type inspectionJoinError struct{}

func (inspectionJoinError) Error() string { return "inspection join mismatch" }

func starReadings(snap market.StarSnapshotV3, rel market.StarRelations) []inspectionReading {
	var out []inspectionReading
	addTF(&out, "m15", "15m", snap.M15)
	addRSX(&out, "m15", "15m", snap.RSX, snap.RSXOK, snap.RSXSlope, snap.RSXSlopeOK, snap.RSXAccel, snap.RSXAccelOK, snap.RSXMinusSignal, snap.RSXMinusOK)
	addTF(&out, "h1", "1h", snap.H1)
	addRSXBlock(&out, "h1", "1h", snap.H1RSX)
	addTF(&out, "h4", "4h", snap.H4)
	addRSXBlock(&out, "h4", "4h", snap.H4RSX)
	addTF(&out, "d1", "Daily", snap.D1)
	addRSXBlock(&out, "d1", "Daily", snap.D1RSX)
	addRel(&out, rel)
	return out
}

func addTF(out *[]inspectionReading, id, group string, tf market.StarTF) {
	add(out, id+".vwema", "VWEMA", group, inspectionFamilyLevel, "", tf.Vwema, presentFinite(tf.Present, tf.Vwema))
	add(out, id+".vwemaSlope", "VWEMA slope", group, inspectionFamilySigned, "slope", tf.Slope, tf.SlopeOK)
	add(out, id+".vwemaAccel", "VWEMA acceleration", group, inspectionFamilySigned, "accel", tf.VwemaAccel, tf.VwemaAccelOK)
	add(out, id+".orange", "Orange midline", group, inspectionFamilyLevel, "", tf.ChanMid, presentFinite(tf.Present, tf.ChanMid))
	add(out, id+".orangeSlope", "Orange midline slope", group, inspectionFamilySigned, "slope", tf.MidSlope, tf.MidSlopeOK)
	add(out, id+".orangeAccel", "Orange midline acceleration", group, inspectionFamilySigned, "accel", tf.MidAccel, tf.MidAccelOK)
	add(out, id+".ema5", "Volume RSI EMA5", group, inspectionFamilyLevel, "", tf.Ema5, tf.Ema5OK)
	add(out, id+".ema5Slope", "Volume RSI EMA5 slope", group, inspectionFamilySigned, "slope", tf.Ema5Slope, tf.Ema5SlopeOK)
	add(out, id+".ema5Accel", "Volume RSI EMA5 acceleration", group, inspectionFamilySigned, "accel", tf.Ema5Accel, tf.Ema5AccelOK)
	add(out, id+".ema12", "Volume RSI EMA12", group, inspectionFamilyLevel, "", tf.Ema12, tf.Ema12OK)
	add(out, id+".ema12Slope", "Volume RSI EMA12 slope", group, inspectionFamilySigned, "slope", tf.Ema12Slope, tf.Ema12SlopeOK)
	add(out, id+".rsi", "RSI close", group, inspectionFamilyLevel, "", tf.RsiClose, tf.RsiCloseOK)
	add(out, id+".rsiSlope", "RSI close slope", group, inspectionFamilySigned, "slope", tf.RsiCloseSlope, tf.RsiCloseSlopeOK)
	add(out, id+".rsiAccel", "RSI close acceleration", group, inspectionFamilySigned, "accel", tf.RsiCloseAccel, tf.RsiCloseAccelOK)
	add(out, id+".ema7", "RSI EMA7", group, inspectionFamilyLevel, "", tf.Ema7, tf.Ema7OK)
	add(out, id+".ema7Slope", "RSI EMA7 slope", group, inspectionFamilySigned, "slope", tf.Ema7Slope, tf.Ema7SlopeOK)
	add(out, id+".ema7Accel", "RSI EMA7 acceleration", group, inspectionFamilySigned, "accel", tf.Ema7Accel, tf.Ema7AccelOK)
	add(out, id+".macd", "MACD RSI close", group, inspectionFamilyLevel, "", tf.Macd, tf.MacdOK)
	add(out, id+".macdSlope", "MACD slope", group, inspectionFamilySigned, "slope", tf.MacdSlope, tf.MacdSlopeOK)
	add(out, id+".macdAccel", "MACD acceleration", group, inspectionFamilySigned, "accel", tf.MacdAccel, tf.MacdAccelOK)
	add(out, id+".width", "Channel width", group, inspectionFamilyWidth, "", tf.Width, tf.WidthOK)
	add(out, id+".closeWidth", "Close width", group, inspectionFamilyWidth, "", tf.CloseWidth, tf.CloseWidthOK)
}

func addRSX(out *[]inspectionReading, id, group string, value float64, valueOK bool, slope float64, slopeOK bool, accel float64, accelOK bool, gap float64, gapOK bool) {
	add(out, id+".rsx", "RSX", group, inspectionFamilyLevel, "", value, valueOK)
	add(out, id+".rsxSlope", "RSX slope", group, inspectionFamilySigned, "slope", slope, slopeOK)
	add(out, id+".rsxAccel", "RSX acceleration", group, inspectionFamilySigned, "accel", accel, accelOK)
	add(out, id+".rsxMinus", "RSX minus signal", group, inspectionFamilySigned, "sign", gap, gapOK)
}

func addRSXBlock(out *[]inspectionReading, id, group string, rsx market.StarRSX) {
	add(out, id+".rsx", "RSX", group, inspectionFamilyLevel, "", rsx.Value, rsx.ValueOK)
	add(out, id+".rsxSlope", "RSX slope", group, inspectionFamilySigned, "slope", rsx.Slope, rsx.SlopeOK)
	add(out, id+".rsxAccel", "RSX acceleration", group, inspectionFamilySigned, "accel", rsx.Accel, rsx.AccelOK)
}

func addRel(out *[]inspectionReading, rel market.StarRelations) {
	type item struct {
		id, label, group string
		rel              market.Rel
	}
	items := []item{
		{"rel.m15.ema7Macd", "EMA7 minus MACD", "15m relations", rel.M15Ema7MinusMacd},
		{"rel.m15.ema7Mid", "EMA7 minus channel", "15m relations", rel.M15Ema7MinusCloseMid},
		{"rel.m15.rsiMid", "RSI minus channel", "15m relations", rel.M15RsiCloseMinusCloseMid},
		{"rel.m15.slope", "VWEMA slope minus orange", "15m relations", rel.M15VwemaSlopeMinusMidSlope},
		{"rel.h1.ema7Macd", "EMA7 minus MACD", "1h relations", rel.H1Ema7MinusMacd},
		{"rel.h1.ema7Mid", "EMA7 minus channel", "1h relations", rel.H1Ema7MinusCloseMid},
		{"rel.h1.rsiMid", "RSI minus channel", "1h relations", rel.H1RsiCloseMinusCloseMid},
		{"rel.h1.slope", "VWEMA slope minus orange", "1h relations", rel.H1VwemaSlopeMinusMidSlope},
		{"rel.h1.rsx", "RSX minus signal", "1h relations", rel.H1RsxMinusSignal},
		{"rel.h4.ema7Macd", "EMA7 minus MACD", "4h relations", rel.H4Ema7MinusMacd},
		{"rel.h4.ema7Mid", "EMA7 minus channel", "4h relations", rel.H4Ema7MinusCloseMid},
		{"rel.h4.rsiMid", "RSI minus channel", "4h relations", rel.H4RsiCloseMinusCloseMid},
		{"rel.h4.slope", "VWEMA slope minus orange", "4h relations", rel.H4VwemaSlopeMinusMidSlope},
		{"rel.h4.rsx", "RSX minus signal", "4h relations", rel.H4RsxMinusSignal},
		{"rel.d1.ema7Macd", "EMA7 minus MACD", "Daily relations", rel.D1Ema7MinusMacd},
		{"rel.d1.ema7Mid", "EMA7 minus channel", "Daily relations", rel.D1Ema7MinusCloseMid},
		{"rel.d1.rsiMid", "RSI minus channel", "Daily relations", rel.D1RsiCloseMinusCloseMid},
		{"rel.d1.slope", "VWEMA slope minus orange", "Daily relations", rel.D1VwemaSlopeMinusMidSlope},
		{"rel.d1.rsx", "RSX minus signal", "Daily relations", rel.D1RsxMinusSignal},
		{"rel.m15h1.orange", "Orange 15m minus 1h", "15m to 1h", rel.M15H1OrangeMid},
		{"rel.m15h1.vwema", "VWEMA 15m minus 1h", "15m to 1h", rel.M15H1Vwema},
		{"rel.m15h1.ema5", "EMA5 15m minus 1h", "15m to 1h", rel.M15H1Ema5},
		{"rel.m15h1.ema12", "EMA12 15m minus 1h", "15m to 1h", rel.M15H1Ema12},
		{"rel.m15h1.rsi", "RSI 15m minus 1h", "15m to 1h", rel.M15H1RsiClose},
		{"rel.m15h1.ema7", "EMA7 15m minus 1h", "15m to 1h", rel.M15H1Ema7},
		{"rel.m15h1.macd", "MACD 15m minus 1h", "15m to 1h", rel.M15H1Macd},
		{"rel.m15h1.rsx", "RSX 15m minus 1h", "15m to 1h", rel.M15H1Rsx},
		{"rel.h1h4.orange", "Orange 1h minus 4h", "1h to 4h", rel.H1H4OrangeMid},
		{"rel.h1h4.vwema", "VWEMA 1h minus 4h", "1h to 4h", rel.H1H4Vwema},
		{"rel.h1h4.ema5", "EMA5 1h minus 4h", "1h to 4h", rel.H1H4Ema5},
		{"rel.h1h4.ema12", "EMA12 1h minus 4h", "1h to 4h", rel.H1H4Ema12},
		{"rel.h1h4.rsi", "RSI 1h minus 4h", "1h to 4h", rel.H1H4RsiClose},
		{"rel.h1h4.ema7", "EMA7 1h minus 4h", "1h to 4h", rel.H1H4Ema7},
		{"rel.h1h4.macd", "MACD 1h minus 4h", "1h to 4h", rel.H1H4Macd},
		{"rel.h1h4.rsx", "RSX 1h minus 4h", "1h to 4h", rel.H1H4Rsx},
		{"rel.h4d1.orange", "Orange 4h minus daily", "4h to daily", rel.H4D1OrangeMid},
		{"rel.h4d1.vwema", "VWEMA 4h minus daily", "4h to daily", rel.H4D1Vwema},
		{"rel.h4d1.ema5", "EMA5 4h minus daily", "4h to daily", rel.H4D1Ema5},
		{"rel.h4d1.ema12", "EMA12 4h minus daily", "4h to daily", rel.H4D1Ema12},
		{"rel.h4d1.rsi", "RSI 4h minus daily", "4h to daily", rel.H4D1RsiClose},
		{"rel.h4d1.ema7", "EMA7 4h minus daily", "4h to daily", rel.H4D1Ema7},
		{"rel.h4d1.macd", "MACD 4h minus daily", "4h to daily", rel.H4D1Macd},
		{"rel.h4d1.rsx", "RSX 4h minus daily", "4h to daily", rel.H4D1Rsx},
		{"rel.m15h1.width", "Width 15m minus 1h", "15m to 1h", rel.M15H1Width},
		{"rel.m15h1.closeWidth", "Close width 15m minus 1h", "15m to 1h", rel.M15H1CloseWidth},
		{"rel.h1h4.width", "Width 1h minus 4h", "1h to 4h", rel.H1H4Width},
		{"rel.h1h4.closeWidth", "Close width 1h minus 4h", "1h to 4h", rel.H1H4CloseWidth},
		{"rel.h4d1.width", "Width 4h minus daily", "4h to daily", rel.H4D1Width},
		{"rel.h4d1.closeWidth", "Close width 4h minus daily", "4h to daily", rel.H4D1CloseWidth},
	}
	for _, item := range items {
		add(out, item.id, item.label, item.group, inspectionFamilySigned, "sign", item.rel.Value, item.rel.OK)
	}
}

func add(out *[]inspectionReading, id, label, group, family, arrow string, value float64, ok bool) {
	if !ok {
		value = 0
	}
	*out = append(*out, inspectionReading{
		ID: id, Label: label, Group: group, Family: family, Arrow: arrow, Value: value, OK: ok,
	})
}
