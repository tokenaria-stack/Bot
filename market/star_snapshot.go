package market

import (
	"fmt"
	"math"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

// StarSnapshotSchemaV2 is the expanded static-state projection.
// Schema 1 is the frozen artifact written before these fields existed.
// Existing fields keep their schema-1 meaning.
const StarSnapshotSchemaV2 = 2

// StarSnapshot is the closed-bar projection of one Star.
// The numeric truth stays on the HistoryBus from the three timeframe walks.
// This struct is not a second indicator and not a strategy.
type StarSnapshot struct {
	Side        string
	AnchorAt    int64
	ConfirmedAt int64

	M15 StarTF
	H1  StarTF
	H4  StarTF

	RSX            float64
	RSXOK          bool
	RSXSlope       float64
	RSXSlopeOK     bool
	Signal         float64
	SignalOK       bool
	SignalSlope    float64
	SignalSlopeOK  bool
	RSXMinusSignal float64
	RSXMinusOK     bool
	// RSXAccel is the 15m RSX two-bar difference. Signal acceleration is not stored.
	RSXAccel   float64
	RSXAccelOK bool

	// TVDirection is bullish, bearish, or none.
	// TVAgeOK false means no causal rsx_tv_div. Age 0 with TVAgeOK means the
	// fact is confirmed on the Star bar. Age is native 15m bars, uncapped.
	TVDirection   string
	TVConfirmedAt int64
	TVAge         int
	TVAgeOK       bool

	// H1RSX and H4RSX are the causal higher-timeframe RSX pair.
	// They do not repeat RSXMinusSignal. That gap stays a research subtraction.
	H1RSX StarRSX
	H4RSX StarRSX
}

// StarRSX is RSX and its signal at one causal bar.
// Slope is one closed bar of that timeframe. Accel is the next difference.
// A false OK flag stores 0 and is not an observation.
type StarRSX struct {
	Value         float64
	ValueOK       bool
	Slope         float64
	SlopeOK       bool
	Accel         float64
	AccelOK       bool
	Signal        float64
	SignalOK      bool
	SignalSlope   float64
	SignalSlopeOK bool
}

// StarTF is one timeframe read at a single causal bar.
// Slope is VWEMA(HL2)[t] - VWEMA(HL2)[t-1] on that timeframe.
// Width is orange-channel upper minus lower. Distance is VWEMA minus mid.
// Later fields are the schema-2 projection of other slots on the same bar.
// Slope and acceleration use that timeframe's own closed bars.
// Present false means no causal bar. A false OK flag means that number is
// unavailable. Unavailable numbers are stored as 0 and must not be read.
type StarTF struct {
	Present       bool
	OpenTime      int64
	CloseTime     int64
	Vwema         float64
	ChanMid       float64
	ChanUp        float64
	ChanDn        float64
	ValuesOK      bool
	Slope         float64
	SlopeOK       bool
	Width         float64
	WidthOK       bool
	WidthChange   float64
	WidthChangeOK bool
	Distance      float64
	DistanceOK    bool

	MidSlope   float64
	MidSlopeOK bool
	MidAccel   float64
	MidAccelOK bool

	Ema5        float64
	Ema5OK      bool
	Ema5Slope   float64
	Ema5SlopeOK bool
	Ema5Accel   float64
	Ema5AccelOK bool

	Ema12        float64
	Ema12OK      bool
	Ema12Slope   float64
	Ema12SlopeOK bool

	VwemaAccel   float64
	VwemaAccelOK bool

	RsiClose        float64
	RsiCloseOK      bool
	RsiCloseSlope   float64
	RsiCloseSlopeOK bool
	RsiCloseAccel   float64
	RsiCloseAccelOK bool

	CloseMid           float64
	CloseMidOK         bool
	CloseUp            float64
	CloseUpOK          bool
	CloseDn            float64
	CloseDnOK          bool
	CloseWidth         float64
	CloseWidthOK       bool
	CloseWidthChange   float64
	CloseWidthChangeOK bool

	Ema7        float64
	Ema7OK      bool
	Ema7Slope   float64
	Ema7SlopeOK bool
	Ema7Accel   float64
	Ema7AccelOK bool

	Macd        float64
	MacdOK      bool
	MacdSlope   float64
	MacdSlopeOK bool
	MacdAccel   float64
	MacdAccelOK bool
}

// ExtractStarSnapshots replays 15m, 1h, and 4h once each, then reads Stars.
// Star count does not add DAG walks. RSX, TV, and HTF context do not create,
// cancel, or side a Star.
func ExtractStarSnapshots(k15, k1h, k4h []exchange.Kline, rsx RSXSettings) ([]StarSnapshot, error) {
	rsx = NormalizeRSXSettings(rsx)
	m15, err := replayStarSeries(k15, "15m", rsx)
	if err != nil {
		return nil, err
	}
	h1, err := replayStarSeries(k1h, "1h", rsx)
	if err != nil {
		return nil, err
	}
	h4, err := replayStarSeries(k4h, "4h", rsx)
	if err != nil {
		return nil, err
	}

	closes := make([]float64, len(k15))
	opens := make([]int64, len(k15))
	openAt := make(map[int64]int, len(k15))
	for i, k := range k15 {
		closes[i] = k.Close
		opens[i] = k.OpenTime
		openAt[k.OpenTime] = i
	}
	divs := indicators.TVDivergenceFacts(closes, m15.rsx, opens, rsx.DivLookback)

	var out []StarSnapshot
	for i := 1; i < len(k15); i++ {
		side := nodes.DetectCrossoverEdge(m15.vwema[i-1], m15.mid[i-1], m15.vwema[i], m15.mid[i])
		if side != nodes.WozduhXoverSideUp && side != nodes.WozduhXoverSideDown {
			continue
		}
		row, err := projectStar(m15, h1, h4, divs, openAt, i, side)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

type starSeries struct {
	open     []int64
	close    []int64
	vwema    []float64
	mid      []float64
	up       []float64
	dn       []float64
	ema5     []float64
	ema12    []float64
	rsiClose []float64
	closeMid []float64
	closeUp  []float64
	closeDn  []float64
	ema7     []float64
	macd     []float64
	rsx      []float64
	sig      []float64
}

func replayStarSeries(klines []exchange.Kline, interval string, rsx RSXSettings) (starSeries, error) {
	var s starSeries
	if len(klines) == 0 {
		return s, fmt.Errorf("market: star snapshot %s series is empty", interval)
	}
	closes, err := starBarCloses(klines, interval)
	if err != nil {
		return s, err
	}
	// Cap above len so HistoryBus.Get can see bar 0. ReplayClosedBars uses
	// ValidateHistoryCap(len), which equals len on a power of two and hides
	// that bar. This does not change Get.
	capN := core.ValidateHistoryCap(len(klines) + 1)
	replay := replayClosedBarsCap(klines, rsx, capN, nodes.WozduhMaskAll)
	if replay.Hist == nil || replay.Hist.Count() != len(klines) {
		return s, fmt.Errorf("market: star snapshot %s history len %d != %d", interval, histCount(replay.Hist), len(klines))
	}
	if replay.Hist.Cap() <= replay.Hist.Count() {
		return s, fmt.Errorf("market: star snapshot %s ring cap %d does not exceed count %d", interval, replay.Hist.Cap(), replay.Hist.Count())
	}
	s.open = make([]int64, len(klines))
	for i, k := range klines {
		s.open[i] = k.OpenTime
	}
	s.close = closes
	s.vwema = historySlotSeries(replay.Hist, core.SlotWozduhRsiHl2Vwema)
	s.mid = historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma5ChanMid)
	s.up = historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma5ChanUp)
	s.dn = historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma5ChanDn)
	s.ema5 = historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma5)
	s.ema12 = historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma12)
	s.rsiClose = historySlotSeries(replay.Hist, core.SlotWozduhRsiClose)
	s.closeMid = historySlotSeries(replay.Hist, core.SlotWozduhRsiCloseChanMid)
	s.closeUp = historySlotSeries(replay.Hist, core.SlotWozduhRsiCloseChanUp)
	s.closeDn = historySlotSeries(replay.Hist, core.SlotWozduhRsiCloseChanDn)
	s.ema7 = historySlotSeries(replay.Hist, core.SlotWozduhRsiCloseEma7)
	s.macd = historySlotSeries(replay.Hist, core.SlotWozduhMacdRsiClose)
	s.rsx = historySlotSeries(replay.Hist, core.SlotJurikRSX)
	s.sig = historySlotSeries(replay.Hist, core.SlotJurikSignal)
	for _, col := range [][]float64{
		s.vwema, s.mid, s.up, s.dn, s.ema5, s.ema12, s.rsiClose,
		s.closeMid, s.closeUp, s.closeDn, s.ema7, s.macd, s.rsx, s.sig,
	} {
		if len(col) != len(klines) {
			return s, fmt.Errorf("market: star snapshot %s column len %d != %d", interval, len(col), len(klines))
		}
	}
	return s, nil
}

func histCount(h *core.HistoryBus) int {
	if h == nil {
		return 0
	}
	return h.Count()
}

func starBarCloses(klines []exchange.Kline, interval string) ([]int64, error) {
	out := make([]int64, len(klines))
	var prev int64
	for i, k := range klines {
		if k.OpenTime <= 0 || (i > 0 && k.OpenTime <= prev) {
			return nil, fmt.Errorf("market: star snapshot %s open time not increasing at %d", interval, i)
		}
		prev = k.OpenTime
		want, err := data.BarCloseTimeMs(k.OpenTime, interval)
		if err != nil {
			return nil, err
		}
		if k.CloseTime != 0 && k.CloseTime != want {
			return nil, fmt.Errorf("market: star snapshot %s close %d != bar close %d at %d", interval, k.CloseTime, want, i)
		}
		out[i] = want
	}
	return out, nil
}

func projectStar(m15, h1, h4 starSeries, divs []indicators.IndicatorFactEvent, openAt map[int64]int, i int, side string) (StarSnapshot, error) {
	local, err := readStarTF(m15, i)
	if err != nil {
		return StarSnapshot{}, err
	}
	if !local.Present || local.OpenTime != m15.open[i] {
		return StarSnapshot{}, fmt.Errorf("market: star snapshot missing 15m bar %d", i)
	}
	h1tf, err := readCausalTF(h1, local.CloseTime)
	if err != nil {
		return StarSnapshot{}, err
	}
	h4tf, err := readCausalTF(h4, local.CloseTime)
	if err != nil {
		return StarSnapshot{}, err
	}
	row := StarSnapshot{
		Side:        side,
		AnchorAt:    local.OpenTime,
		ConfirmedAt: local.OpenTime,
		M15:         local,
		H1:          h1tf,
		H4:          h4tf,
	}
	fillRSX(&row, m15, i)
	if h1tf.Present {
		idx := latestCloseIndex(h1.close, local.CloseTime)
		if idx < 0 {
			return StarSnapshot{}, fmt.Errorf("market: star snapshot 1h rsx without a bar")
		}
		row.H1RSX = projectStarRSX(h1, idx)
	}
	if h4tf.Present {
		idx := latestCloseIndex(h4.close, local.CloseTime)
		if idx < 0 {
			return StarSnapshot{}, fmt.Errorf("market: star snapshot 4h rsx without a bar")
		}
		row.H4RSX = projectStarRSX(h4, idx)
	}
	dir, at, age, ok, err := causalTVDiv(divs, openAt, local.OpenTime, i)
	if err != nil {
		return StarSnapshot{}, err
	}
	row.TVDirection = dir
	row.TVConfirmedAt = at
	row.TVAge = age
	row.TVAgeOK = ok
	if err := checkStarRow(row, h1, h4); err != nil {
		return StarSnapshot{}, err
	}
	return row, nil
}

func readCausalTF(s starSeries, starClose int64) (StarTF, error) {
	idx := latestCloseIndex(s.close, starClose)
	if idx < 0 {
		return StarTF{}, nil
	}
	tf, err := readStarTF(s, idx)
	if err != nil {
		return StarTF{}, err
	}
	if !tf.Present || tf.CloseTime > starClose {
		return StarTF{}, fmt.Errorf("market: star snapshot htf close %d after star close %d", tf.CloseTime, starClose)
	}
	next := idx + 1
	if next < len(s.close) && s.close[next] <= starClose {
		return StarTF{}, fmt.Errorf("market: star snapshot skipped a closer htf bar at %d", s.open[next])
	}
	return tf, nil
}

func latestCloseIndex(closes []int64, starClose int64) int {
	lo, hi := 0, len(closes)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if closes[mid] <= starClose {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo - 1
}

func readStarTF(s starSeries, i int) (StarTF, error) {
	if i < 0 || i >= len(s.open) {
		return StarTF{}, fmt.Errorf("market: star snapshot bar %d out of range", i)
	}
	tf := StarTF{
		Present:   true,
		OpenTime:  s.open[i],
		CloseTime: s.close[i],
		Vwema:     s.vwema[i],
		ChanMid:   s.mid[i],
		ChanUp:    s.up[i],
		ChanDn:    s.dn[i],
	}
	tf.ValuesOK = starFinite(tf.Vwema) && starFinite(tf.ChanMid) && starFinite(tf.ChanUp) && starFinite(tf.ChanDn)
	if tf.ValuesOK {
		tf.Width = tf.ChanUp - tf.ChanDn
		tf.WidthOK = true
		tf.Distance = tf.Vwema - tf.ChanMid
		tf.DistanceOK = true
	}
	if i > 0 && starFinite(s.vwema[i]) && starFinite(s.vwema[i-1]) {
		tf.Slope = s.vwema[i] - s.vwema[i-1]
		tf.SlopeOK = true
	}
	if i > 0 && tf.WidthOK && starFinite(s.up[i-1]) && starFinite(s.dn[i-1]) {
		prev := s.up[i-1] - s.dn[i-1]
		tf.WidthChange = tf.Width - prev
		tf.WidthChangeOK = true
	}
	tf.MidSlope, tf.MidSlopeOK, tf.MidAccel, tf.MidAccelOK = slopeAccel(s.mid, i)
	_, _, tf.VwemaAccel, tf.VwemaAccelOK = slopeAccel(s.vwema, i)
	tf.Ema5, tf.Ema5OK, tf.Ema5Slope, tf.Ema5SlopeOK, tf.Ema5Accel, tf.Ema5AccelOK = projectSample(s.ema5, i)
	tf.Ema12, tf.Ema12OK, tf.Ema12Slope, tf.Ema12SlopeOK, _, _ = projectSample(s.ema12, i)
	tf.RsiClose, tf.RsiCloseOK, tf.RsiCloseSlope, tf.RsiCloseSlopeOK, tf.RsiCloseAccel, tf.RsiCloseAccelOK = projectSample(s.rsiClose, i)
	tf.CloseMid, tf.CloseMidOK = finiteAt(s.closeMid, i)
	tf.CloseUp, tf.CloseUpOK = finiteAt(s.closeUp, i)
	tf.CloseDn, tf.CloseDnOK = finiteAt(s.closeDn, i)
	if tf.CloseUpOK && tf.CloseDnOK {
		tf.CloseWidth = tf.CloseUp - tf.CloseDn
		tf.CloseWidthOK = true
	}
	if i > 0 && tf.CloseWidthOK && starFinite(s.closeUp[i-1]) && starFinite(s.closeDn[i-1]) {
		tf.CloseWidthChange = tf.CloseWidth - (s.closeUp[i-1] - s.closeDn[i-1])
		tf.CloseWidthChangeOK = true
	}
	tf.Ema7, tf.Ema7OK, tf.Ema7Slope, tf.Ema7SlopeOK, tf.Ema7Accel, tf.Ema7AccelOK = projectSample(s.ema7, i)
	tf.Macd, tf.MacdOK, tf.MacdSlope, tf.MacdSlopeOK, tf.MacdAccel, tf.MacdAccelOK = projectSample(s.macd, i)
	return tf, nil
}

func finiteAt(col []float64, i int) (float64, bool) {
	if i < 0 || i >= len(col) || !starFinite(col[i]) {
		return 0, false
	}
	return col[i], true
}

func slopeAccel(col []float64, i int) (slope float64, slopeOK bool, accel float64, accelOK bool) {
	if i < 0 || i >= len(col) {
		return 0, false, 0, false
	}
	if i > 0 && starFinite(col[i]) && starFinite(col[i-1]) {
		slope = col[i] - col[i-1]
		slopeOK = true
	}
	if i > 1 && starFinite(col[i]) && starFinite(col[i-1]) && starFinite(col[i-2]) {
		accel = col[i] - 2*col[i-1] + col[i-2]
		accelOK = true
	}
	return slope, slopeOK, accel, accelOK
}

func projectSample(col []float64, i int) (value float64, valueOK bool, slope float64, slopeOK bool, accel float64, accelOK bool) {
	value, valueOK = finiteAt(col, i)
	slope, slopeOK, accel, accelOK = slopeAccel(col, i)
	return value, valueOK, slope, slopeOK, accel, accelOK
}

func projectStarRSX(s starSeries, i int) StarRSX {
	value, valueOK, slope, slopeOK, accel, accelOK := projectSample(s.rsx, i)
	signal, signalOK, signalSlope, signalSlopeOK, _, _ := projectSample(s.sig, i)
	return StarRSX{
		Value: value, ValueOK: valueOK,
		Slope: slope, SlopeOK: slopeOK,
		Accel: accel, AccelOK: accelOK,
		Signal: signal, SignalOK: signalOK,
		SignalSlope: signalSlope, SignalSlopeOK: signalSlopeOK,
	}
}

func fillRSX(row *StarSnapshot, s starSeries, i int) {
	rsx := projectStarRSX(s, i)
	row.RSX = rsx.Value
	row.RSXOK = rsx.ValueOK
	row.RSXSlope = rsx.Slope
	row.RSXSlopeOK = rsx.SlopeOK
	row.RSXAccel = rsx.Accel
	row.RSXAccelOK = rsx.AccelOK
	row.Signal = rsx.Signal
	row.SignalOK = rsx.SignalOK
	row.SignalSlope = rsx.SignalSlope
	row.SignalSlopeOK = rsx.SignalSlopeOK
	if row.RSXOK && row.SignalOK {
		row.RSXMinusSignal = row.RSX - row.Signal
		row.RSXMinusOK = true
	}
}

// causalTVDiv picks the latest rsx_tv_div with ConfirmedAt <= starOpen.
// Two different directions at that same timestamp leave the fact unavailable.
func causalTVDiv(facts []indicators.IndicatorFactEvent, openAt map[int64]int, starOpen int64, starIndex int) (dir string, at int64, age int, ok bool, err error) {
	dir = "none"
	var best int64
	var bestDir string
	var bestN int
	for _, ev := range facts {
		if ev.Source != indicators.FactSourceRSXTVDiv {
			continue
		}
		if ev.ConfirmedAt <= 0 || ev.ConfirmedAt > starOpen {
			continue
		}
		idx, found := openAt[ev.ConfirmedAt]
		if !found || idx > starIndex {
			return "none", 0, 0, false, fmt.Errorf("market: star snapshot tv fact %d is not on the 15m series", ev.ConfirmedAt)
		}
		if ev.Direction != indicators.FactDirBullish && ev.Direction != indicators.FactDirBearish {
			continue
		}
		if bestN == 0 || ev.ConfirmedAt > best {
			best = ev.ConfirmedAt
			bestDir = ev.Direction
			bestN = 1
			continue
		}
		if ev.ConfirmedAt == best && ev.Direction != bestDir {
			bestN = 2
		}
	}
	if bestN == 0 {
		return "none", 0, 0, false, nil
	}
	if bestN != 1 {
		return "none", 0, 0, false, nil
	}
	idx := openAt[best]
	return bestDir, best, starIndex - idx, true, nil
}

func checkStarRow(row StarSnapshot, h1, h4 starSeries) error {
	if row.Side != nodes.WozduhXoverSideUp && row.Side != nodes.WozduhXoverSideDown {
		return fmt.Errorf("market: star snapshot side %q", row.Side)
	}
	if row.AnchorAt != row.ConfirmedAt || row.AnchorAt != row.M15.OpenTime {
		return fmt.Errorf("market: star snapshot clock %d %d %d", row.AnchorAt, row.ConfirmedAt, row.M15.OpenTime)
	}
	if row.H1.Present && row.H1.CloseTime > row.M15.CloseTime {
		return fmt.Errorf("market: star snapshot 1h close after star")
	}
	if row.H4.Present && row.H4.CloseTime > row.M15.CloseTime {
		return fmt.Errorf("market: star snapshot 4h close after star")
	}
	if row.TVAgeOK && (row.TVConfirmedAt > row.ConfirmedAt || row.TVAge < 0) {
		return fmt.Errorf("market: star snapshot tv after star")
	}
	if !row.TVAgeOK && (row.TVDirection != "none" || row.TVAge != 0 || row.TVConfirmedAt != 0) {
		return fmt.Errorf("market: star snapshot missing tv encoded as age 0")
	}
	if !row.H1.Present && row.H1RSX != (StarRSX{}) {
		return fmt.Errorf("market: star snapshot 1h rsx without a bar")
	}
	if !row.H4.Present && row.H4RSX != (StarRSX{}) {
		return fmt.Errorf("market: star snapshot 4h rsx without a bar")
	}
	if err := checkLatest(h1, row.M15.CloseTime, row.H1); err != nil {
		return err
	}
	return checkLatest(h4, row.M15.CloseTime, row.H4)
}

func checkLatest(s starSeries, starClose int64, got StarTF) error {
	idx := latestCloseIndex(s.close, starClose)
	if idx < 0 {
		if got.Present {
			return fmt.Errorf("market: star snapshot htf present without a bar")
		}
		return nil
	}
	if !got.Present || got.OpenTime != s.open[idx] || got.CloseTime != s.close[idx] {
		return fmt.Errorf("market: star snapshot htf open %d != %d", got.OpenTime, s.open[idx])
	}
	return nil
}

func starFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
