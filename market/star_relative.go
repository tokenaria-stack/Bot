package market

// Rel is one arithmetic relation.
// OK false stores 0. That 0 is not a reading.
// A real 0 is stored with OK true.
type Rel struct {
	Value float64
	OK    bool
}

// StarRelations is the 49-relation projection of one schema-3 Star.
// The raw schema-3 numbers stay on StarSnapshotV3.
// This struct does not copy them and does not walk a bar.
type StarRelations struct {
	Side        string
	AnchorAt    int64
	ConfirmedAt int64

	M15Ema7MinusMacd           Rel
	M15Ema7MinusCloseMid       Rel
	M15RsiCloseMinusCloseMid   Rel
	M15VwemaSlopeMinusMidSlope Rel

	H1Ema7MinusMacd           Rel
	H1Ema7MinusCloseMid       Rel
	H1RsiCloseMinusCloseMid   Rel
	H1VwemaSlopeMinusMidSlope Rel

	H4Ema7MinusMacd           Rel
	H4Ema7MinusCloseMid       Rel
	H4RsiCloseMinusCloseMid   Rel
	H4VwemaSlopeMinusMidSlope Rel

	D1Ema7MinusMacd           Rel
	D1Ema7MinusCloseMid       Rel
	D1RsiCloseMinusCloseMid   Rel
	D1VwemaSlopeMinusMidSlope Rel

	H1RsxMinusSignal Rel
	H4RsxMinusSignal Rel
	D1RsxMinusSignal Rel

	M15H1OrangeMid Rel
	M15H1Vwema     Rel
	M15H1Ema5      Rel
	M15H1Ema12     Rel
	M15H1RsiClose  Rel
	M15H1Ema7      Rel
	M15H1Macd      Rel
	M15H1Rsx       Rel

	H1H4OrangeMid Rel
	H1H4Vwema     Rel
	H1H4Ema5      Rel
	H1H4Ema12     Rel
	H1H4RsiClose  Rel
	H1H4Ema7      Rel
	H1H4Macd      Rel
	H1H4Rsx       Rel

	H4D1OrangeMid Rel
	H4D1Vwema     Rel
	H4D1Ema5      Rel
	H4D1Ema12     Rel
	H4D1RsiClose  Rel
	H4D1Ema7      Rel
	H4D1Macd      Rel
	H4D1Rsx       Rel

	M15H1Width      Rel
	M15H1CloseWidth Rel
	H1H4Width       Rel
	H1H4CloseWidth  Rel
	H4D1Width       Rel
	H4D1CloseWidth  Rel
}

// Relations returns the 49 relations in audit order.
// Same-timeframe gaps, then the three RSX-minus-signal gaps,
// then adjacent levels, then adjacent widths.
func (r StarRelations) Relations() [49]Rel {
	return [49]Rel{
		r.M15Ema7MinusMacd, r.M15Ema7MinusCloseMid, r.M15RsiCloseMinusCloseMid, r.M15VwemaSlopeMinusMidSlope,
		r.H1Ema7MinusMacd, r.H1Ema7MinusCloseMid, r.H1RsiCloseMinusCloseMid, r.H1VwemaSlopeMinusMidSlope,
		r.H4Ema7MinusMacd, r.H4Ema7MinusCloseMid, r.H4RsiCloseMinusCloseMid, r.H4VwemaSlopeMinusMidSlope,
		r.D1Ema7MinusMacd, r.D1Ema7MinusCloseMid, r.D1RsiCloseMinusCloseMid, r.D1VwemaSlopeMinusMidSlope,
		r.H1RsxMinusSignal, r.H4RsxMinusSignal, r.D1RsxMinusSignal,
		r.M15H1OrangeMid, r.M15H1Vwema, r.M15H1Ema5, r.M15H1Ema12, r.M15H1RsiClose, r.M15H1Ema7, r.M15H1Macd, r.M15H1Rsx,
		r.H1H4OrangeMid, r.H1H4Vwema, r.H1H4Ema5, r.H1H4Ema12, r.H1H4RsiClose, r.H1H4Ema7, r.H1H4Macd, r.H1H4Rsx,
		r.H4D1OrangeMid, r.H4D1Vwema, r.H4D1Ema5, r.H4D1Ema12, r.H4D1RsiClose, r.H4D1Ema7, r.H4D1Macd, r.H4D1Rsx,
		r.M15H1Width, r.M15H1CloseWidth, r.H1H4Width, r.H1H4CloseWidth, r.H4D1Width, r.H4D1CloseWidth,
	}
}

// BuildStarRelations subtracts schema-3 fields.
// It does not read a bar and it does not rebuild a line.
func BuildStarRelations(rows []StarSnapshotV3) []StarRelations {
	out := make([]StarRelations, len(rows))
	for i, row := range rows {
		out[i] = projectRelations(row)
	}
	return out
}

func projectRelations(row StarSnapshotV3) StarRelations {
	rel := StarRelations{Side: row.Side, AnchorAt: row.AnchorAt, ConfirmedAt: row.ConfirmedAt}
	rel.M15Ema7MinusMacd, rel.M15Ema7MinusCloseMid, rel.M15RsiCloseMinusCloseMid, rel.M15VwemaSlopeMinusMidSlope = sameBar(row.M15)
	rel.H1Ema7MinusMacd, rel.H1Ema7MinusCloseMid, rel.H1RsiCloseMinusCloseMid, rel.H1VwemaSlopeMinusMidSlope = sameBar(row.H1)
	rel.H4Ema7MinusMacd, rel.H4Ema7MinusCloseMid, rel.H4RsiCloseMinusCloseMid, rel.H4VwemaSlopeMinusMidSlope = sameBar(row.H4)
	rel.D1Ema7MinusMacd, rel.D1Ema7MinusCloseMid, rel.D1RsiCloseMinusCloseMid, rel.D1VwemaSlopeMinusMidSlope = sameBar(row.D1)
	rel.H1RsxMinusSignal = rsxMinusSignal(row.H1, row.H1RSX)
	rel.H4RsxMinusSignal = rsxMinusSignal(row.H4, row.H4RSX)
	rel.D1RsxMinusSignal = rsxMinusSignal(row.D1, row.D1RSX)
	rel.M15H1OrangeMid, rel.M15H1Vwema, rel.M15H1Ema5, rel.M15H1Ema12, rel.M15H1RsiClose, rel.M15H1Ema7, rel.M15H1Macd, rel.M15H1Rsx =
		levelGap(row.M15, row.H1, row.M15.Present && row.RSXOK, row.RSX, row.H1.Present && row.H1RSX.ValueOK, row.H1RSX.Value)
	rel.H1H4OrangeMid, rel.H1H4Vwema, rel.H1H4Ema5, rel.H1H4Ema12, rel.H1H4RsiClose, rel.H1H4Ema7, rel.H1H4Macd, rel.H1H4Rsx =
		levelGap(row.H1, row.H4, row.H1.Present && row.H1RSX.ValueOK, row.H1RSX.Value, row.H4.Present && row.H4RSX.ValueOK, row.H4RSX.Value)
	rel.H4D1OrangeMid, rel.H4D1Vwema, rel.H4D1Ema5, rel.H4D1Ema12, rel.H4D1RsiClose, rel.H4D1Ema7, rel.H4D1Macd, rel.H4D1Rsx =
		levelGap(row.H4, row.D1, row.H4.Present && row.H4RSX.ValueOK, row.H4RSX.Value, row.D1.Present && row.D1RSX.ValueOK, row.D1RSX.Value)
	rel.M15H1Width, rel.M15H1CloseWidth = widthGap(row.M15, row.H1)
	rel.H1H4Width, rel.H1H4CloseWidth = widthGap(row.H1, row.H4)
	rel.H4D1Width, rel.H4D1CloseWidth = widthGap(row.H4, row.D1)
	return rel
}

func sameBar(tf StarTF) (ema7Macd, ema7Mid, rsiMid, slope Rel) {
	p := tf.Present
	ema7Macd = relSub(tf.Ema7, p && tf.Ema7OK, tf.Macd, p && tf.MacdOK)
	ema7Mid = relSub(tf.Ema7, p && tf.Ema7OK, tf.CloseMid, p && tf.CloseMidOK)
	rsiMid = relSub(tf.RsiClose, p && tf.RsiCloseOK, tf.CloseMid, p && tf.CloseMidOK)
	slope = relSub(tf.Slope, p && tf.SlopeOK, tf.MidSlope, p && tf.MidSlopeOK)
	return ema7Macd, ema7Mid, rsiMid, slope
}

func levelGap(lo, hi StarTF, loRsxOK bool, loRsx float64, hiRsxOK bool, hiRsx float64) (mid, vwema, ema5, ema12, rsi, ema7, macd, rsx Rel) {
	lv, lok := lineFinite(lo, lo.Vwema)
	hv, hok := lineFinite(hi, hi.Vwema)
	vwema = relSub(lv, lok, hv, hok)
	lm, lmok := lineFinite(lo, lo.ChanMid)
	hm, hmok := lineFinite(hi, hi.ChanMid)
	mid = relSub(lm, lmok, hm, hmok)
	ema5 = relSub(lo.Ema5, lo.Present && lo.Ema5OK, hi.Ema5, hi.Present && hi.Ema5OK)
	ema12 = relSub(lo.Ema12, lo.Present && lo.Ema12OK, hi.Ema12, hi.Present && hi.Ema12OK)
	rsi = relSub(lo.RsiClose, lo.Present && lo.RsiCloseOK, hi.RsiClose, hi.Present && hi.RsiCloseOK)
	ema7 = relSub(lo.Ema7, lo.Present && lo.Ema7OK, hi.Ema7, hi.Present && hi.Ema7OK)
	macd = relSub(lo.Macd, lo.Present && lo.MacdOK, hi.Macd, hi.Present && hi.MacdOK)
	rsx = relSub(loRsx, loRsxOK, hiRsx, hiRsxOK)
	return mid, vwema, ema5, ema12, rsi, ema7, macd, rsx
}

func widthGap(lo, hi StarTF) (width, closeWidth Rel) {
	width = relSub(lo.Width, lo.Present && lo.WidthOK, hi.Width, hi.Present && hi.WidthOK)
	closeWidth = relSub(lo.CloseWidth, lo.Present && lo.CloseWidthOK, hi.CloseWidth, hi.Present && hi.CloseWidthOK)
	return width, closeWidth
}

func rsxMinusSignal(tf StarTF, rsx StarRSX) Rel {
	return relSub(rsx.Value, tf.Present && rsx.ValueOK, rsx.Signal, tf.Present && rsx.SignalOK)
}

// lineFinite is the observation test for VWEMA and orange mid.
// Those two numbers have no OK flag of their own.
// ValuesOK also requires the channel bounds, so it is not this test.
// An absent bar stores 0, and Present false keeps that 0 from becoming a reading.
func lineFinite(tf StarTF, v float64) (float64, bool) {
	if !tf.Present || !starFinite(v) {
		return 0, false
	}
	return v, true
}

func relSub(a float64, aOK bool, b float64, bOK bool) Rel {
	if !aOK || !bOK {
		return Rel{}
	}
	return Rel{Value: a - b, OK: true}
}
