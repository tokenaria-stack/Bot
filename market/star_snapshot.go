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

// StarSnapshot is the V1 projection of one closed-bar Star.
// The numeric truth stays on the three HistoryBus values from that call.
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

	// TVDirection is bullish, bearish, or none.
	// TVAgeOK false means no causal rsx_tv_div. Age 0 with TVAgeOK means the
	// fact is confirmed on the Star bar. Age is native 15m bars, uncapped.
	TVDirection   string
	TVConfirmedAt int64
	TVAge         int
	TVAgeOK       bool
}

// StarTF is one timeframe read at a single causal bar.
// Slope is VWEMA(HL2)[t] - VWEMA(HL2)[t-1] on that timeframe.
// Width is orange-channel upper minus lower. Distance is VWEMA minus mid.
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
	open  []int64
	close []int64
	vwema []float64
	mid   []float64
	up    []float64
	dn    []float64
	rsx   []float64
	sig   []float64
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
	s.rsx = historySlotSeries(replay.Hist, core.SlotJurikRSX)
	s.sig = historySlotSeries(replay.Hist, core.SlotJurikSignal)
	for _, col := range [][]float64{s.vwema, s.mid, s.up, s.dn, s.rsx, s.sig} {
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
	return tf, nil
}

func fillRSX(row *StarSnapshot, s starSeries, i int) {
	rsxV, sigV := s.rsx[i], s.sig[i]
	row.RSXOK = starFinite(rsxV)
	row.SignalOK = starFinite(sigV)
	if row.RSXOK {
		row.RSX = rsxV
	}
	if row.SignalOK {
		row.Signal = sigV
	}
	if row.RSXOK && row.SignalOK {
		row.RSXMinusSignal = rsxV - sigV
		row.RSXMinusOK = true
	}
	if i > 0 && row.RSXOK && starFinite(s.rsx[i-1]) {
		row.RSXSlope = rsxV - s.rsx[i-1]
		row.RSXSlopeOK = true
	}
	if i > 0 && row.SignalOK && starFinite(s.sig[i-1]) {
		row.SignalSlope = sigV - s.sig[i-1]
		row.SignalSlopeOK = true
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
