package market

import (
	"math"
	"os"
	"strings"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/exchange"
)

func TestStarExtractorDoesNotConstructIndicators(t *testing.T) {
	body, err := os.ReadFile("star_snapshot.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, forbidden := range []string{
		"FeatureRuntime2",
		"NewJurikRSX",
		"NewRSI(",
		"NewEMA(",
		"NewMACD(",
		"NewWozduhNode",
		"NewRSXNode",
		"ReplayClosedBars(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("extractor contains %s", forbidden)
		}
	}
}

func TestStarV2ProjectionLaws(t *testing.T) {
	s := blankStarSeries(4)
	for i := range s.open {
		s.open[i] = int64(i + 1)
		s.close[i] = int64(i + 1)
	}
	s.vwema = []float64{1, 2, 4, math.NaN()}
	s.mid = []float64{10, 13, 17, 20}
	s.up = []float64{12, 16, 21, 25}
	s.dn = []float64{8, 10, 13, 15}
	s.ema5 = []float64{math.NaN(), 5, 8, 12}
	s.ema12 = []float64{1, 1, 4, 4}
	s.rsiClose = []float64{30, 40, 55, 55}
	s.closeMid = []float64{20, 22, 26, 26}
	s.closeUp = []float64{28, 31, 36, 40}
	s.closeDn = []float64{12, 13, 16, 12}
	s.ema7 = []float64{7, 9, 12, 16}
	s.macd = []float64{50, 51, 53, 56}
	s.rsx = []float64{10, 14, 20, 28}
	s.sig = []float64{9, 11, 15, 21}

	tf, err := readStarTF(s, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !tf.ValuesOK || tf.Slope != 2 || !tf.SlopeOK || tf.VwemaAccel != 1 || !tf.VwemaAccelOK {
		t.Fatalf("vwema slope/accel %+v", tf)
	}
	if tf.Slope != tf.Vwema-s.vwema[1] {
		t.Fatal("slope left the vwema series")
	}
	if tf.MidSlope != 4 || !tf.MidSlopeOK || tf.MidAccel != 1 || !tf.MidAccelOK {
		t.Fatalf("mid %+v", tf)
	}
	if tf.Width != 8 || tf.WidthChange != 8-6 {
		t.Fatalf("orange width %v change %v", tf.Width, tf.WidthChange)
	}
	if tf.Ema5 != 8 || !tf.Ema5OK || tf.Ema5Slope != 3 || !tf.Ema5SlopeOK || tf.Ema5AccelOK {
		t.Fatalf("ema5 warmup %+v", tf)
	}
	if tf.Ema12 != 4 || tf.Ema12Slope != 3 || !tf.Ema12SlopeOK {
		t.Fatalf("ema12 %+v", tf)
	}
	if tf.RsiClose != 55 || tf.RsiCloseSlope != 15 || tf.RsiCloseAccel != 5 {
		t.Fatalf("rsi close %+v", tf)
	}
	if tf.CloseWidth != 20 || !tf.CloseWidthOK || tf.CloseWidthChange != 20-18 {
		t.Fatalf("close channel %v %v", tf.CloseWidth, tf.CloseWidthChange)
	}
	if tf.Ema7Slope != 3 || tf.Ema7Accel != 1 || tf.MacdSlope != 2 || tf.MacdAccel != 1 {
		t.Fatalf("ema7/macd %+v", tf)
	}

	warm, err := readStarTF(s, 1)
	if err != nil {
		t.Fatal(err)
	}
	if warm.Ema5 != 5 || !warm.Ema5OK || warm.Ema5SlopeOK || warm.Ema5Slope != 0 || warm.Ema5AccelOK {
		t.Fatalf("missing previous ema5 %+v", warm)
	}

	gap, err := readStarTF(s, 3)
	if err != nil {
		t.Fatal(err)
	}
	if gap.ValuesOK || gap.SlopeOK || gap.Slope != 0 || gap.VwemaAccelOK || gap.VwemaAccel != 0 {
		t.Fatalf("non finite vwema %+v", gap)
	}
	if gap.Ema5 != 12 || gap.Ema5Slope != 4 || !gap.Ema5AccelOK || gap.Ema5Accel != 1 {
		t.Fatalf("ema5 accel %+v", gap)
	}

	rsx := projectStarRSX(s, 2)
	if rsx.Value != 20 || rsx.Slope != 6 || rsx.Accel != 2 || rsx.Signal != 15 || rsx.SignalSlope != 4 {
		t.Fatalf("rsx %+v", rsx)
	}
	var row StarSnapshot
	fillRSX(&row, s, 2)
	if !row.RSXMinusOK || row.RSXMinusSignal != 5 || row.RSXAccel != 2 || !row.RSXAccelOK {
		t.Fatalf("root rsx %+v", row)
	}
	early := projectStarRSX(s, 0)
	if early.SlopeOK || early.AccelOK || early.SignalSlopeOK || early.Slope != 0 || early.Accel != 0 {
		t.Fatalf("first bar rsx %+v", early)
	}
}

func TestStarV2MatchesHistoryBus(t *testing.T) {
	k15 := starFixtureKlines(16*80, "15m")
	k1h := starFixtureKlines(4*80, "1h")
	k4h := starFixtureKlines(80, "4h")
	rsx := defaultRSXSettings()
	before := testDAGRunnerBorn.Load()
	rows, err := ExtractStarSnapshots(k15, k1h, k4h, rsx)
	if err != nil {
		t.Fatal(err)
	}
	if born := testDAGRunnerBorn.Load() - before; born != 3 {
		t.Fatalf("dag runners %d", born)
	}
	if len(rows) == 0 {
		t.Fatal("no stars")
	}
	m15 := loadBusSeries(t, k15, "15m", rsx)
	h1 := loadBusSeries(t, k1h, "1h", rsx)
	h4 := loadBusSeries(t, k4h, "4h", rsx)
	openAt := make(map[int64]int, len(k15))
	for i, k := range k15 {
		openAt[k.OpenTime] = i
	}
	for _, row := range rows {
		i, ok := openAt[row.AnchorAt]
		if !ok {
			t.Fatalf("missing 15m %d", row.AnchorAt)
		}
		assertTFBus(t, "15m", row.M15, m15, i)
		assertRootRSX(t, row, m15, i)
		assertHTFBus(t, "1h", row.M15.CloseTime, row.H1, row.H1RSX, h1)
		assertHTFBus(t, "4h", row.M15.CloseTime, row.H4, row.H4RSX, h4)
	}
}

func loadBusSeries(t *testing.T, klines []exchange.Kline, interval string, rsx RSXSettings) starSeries {
	t.Helper()
	capN := core.ValidateHistoryCap(len(klines) + 1)
	replay := replayClosedBarsCap(klines, rsx, capN, nodes.WozduhMaskAll)
	if replay.Hist == nil || replay.Hist.Count() != len(klines) {
		t.Fatalf("%s history %d", interval, histCount(replay.Hist))
	}
	s := starSeries{
		open:     make([]int64, len(klines)),
		vwema:    historySlotSeries(replay.Hist, core.SlotWozduhRsiHl2Vwema),
		mid:      historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma5ChanMid),
		up:       historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma5ChanUp),
		dn:       historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma5ChanDn),
		ema5:     historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma5),
		ema12:    historySlotSeries(replay.Hist, core.SlotWozduhVolRsiEma12),
		rsiClose: historySlotSeries(replay.Hist, core.SlotWozduhRsiClose),
		closeMid: historySlotSeries(replay.Hist, core.SlotWozduhRsiCloseChanMid),
		closeUp:  historySlotSeries(replay.Hist, core.SlotWozduhRsiCloseChanUp),
		closeDn:  historySlotSeries(replay.Hist, core.SlotWozduhRsiCloseChanDn),
		ema7:     historySlotSeries(replay.Hist, core.SlotWozduhRsiCloseEma7),
		macd:     historySlotSeries(replay.Hist, core.SlotWozduhMacdRsiClose),
		rsx:      historySlotSeries(replay.Hist, core.SlotJurikRSX),
		sig:      historySlotSeries(replay.Hist, core.SlotJurikSignal),
	}
	closes, err := starBarCloses(klines, interval)
	if err != nil {
		t.Fatal(err)
	}
	s.close = closes
	for i, k := range klines {
		s.open[i] = k.OpenTime
	}
	return s
}

func assertHTFBus(t *testing.T, name string, starClose int64, tf StarTF, rsx StarRSX, s starSeries) {
	t.Helper()
	idx := latestCloseIndex(s.close, starClose)
	if idx < 0 {
		if tf.Present || rsx != (StarRSX{}) {
			t.Fatalf("%s present without a bar", name)
		}
		return
	}
	if !tf.Present || tf.CloseTime > starClose || tf.OpenTime != s.open[idx] {
		t.Fatalf("%s causal %d close %d bar %d", name, starClose, tf.CloseTime, s.open[idx])
	}
	assertTFBus(t, name, tf, s, idx)
	assertRSXBus(t, name, rsx, s, idx)
}

func assertTFBus(t *testing.T, name string, tf StarTF, s starSeries, i int) {
	t.Helper()
	if tf.ValuesOK && (tf.Vwema != s.vwema[i] || tf.ChanMid != s.mid[i] || tf.ChanUp != s.up[i] || tf.ChanDn != s.dn[i]) {
		t.Fatalf("%s orange values", name)
	}
	assertDelta(t, name+" vwema", tf.Slope, tf.SlopeOK, tf.VwemaAccel, tf.VwemaAccelOK, s.vwema, i)
	assertDelta(t, name+" mid", tf.MidSlope, tf.MidSlopeOK, tf.MidAccel, tf.MidAccelOK, s.mid, i)
	assertLine(t, name+" ema5", tf.Ema5, tf.Ema5OK, tf.Ema5Slope, tf.Ema5SlopeOK, tf.Ema5Accel, tf.Ema5AccelOK, s.ema5, i)
	assertValue(t, name+" ema12", tf.Ema12, tf.Ema12OK, s.ema12, i)
	assertSlope(t, name+" ema12", tf.Ema12Slope, tf.Ema12SlopeOK, s.ema12, i)
	assertLine(t, name+" rsi", tf.RsiClose, tf.RsiCloseOK, tf.RsiCloseSlope, tf.RsiCloseSlopeOK, tf.RsiCloseAccel, tf.RsiCloseAccelOK, s.rsiClose, i)
	assertValue(t, name+" closeMid", tf.CloseMid, tf.CloseMidOK, s.closeMid, i)
	assertValue(t, name+" closeUp", tf.CloseUp, tf.CloseUpOK, s.closeUp, i)
	assertValue(t, name+" closeDn", tf.CloseDn, tf.CloseDnOK, s.closeDn, i)
	if tf.CloseWidthOK && tf.CloseWidth != tf.CloseUp-tf.CloseDn {
		t.Fatalf("%s close width", name)
	}
	assertLine(t, name+" ema7", tf.Ema7, tf.Ema7OK, tf.Ema7Slope, tf.Ema7SlopeOK, tf.Ema7Accel, tf.Ema7AccelOK, s.ema7, i)
	assertLine(t, name+" macd", tf.Macd, tf.MacdOK, tf.MacdSlope, tf.MacdSlopeOK, tf.MacdAccel, tf.MacdAccelOK, s.macd, i)
	if tf.ValuesOK && tf.Distance != tf.Vwema-tf.ChanMid {
		t.Fatalf("%s distance", name)
	}
}

func assertRootRSX(t *testing.T, row StarSnapshot, s starSeries, i int) {
	t.Helper()
	assertLine(t, "15m rsx", row.RSX, row.RSXOK, row.RSXSlope, row.RSXSlopeOK, row.RSXAccel, row.RSXAccelOK, s.rsx, i)
	assertValue(t, "15m signal", row.Signal, row.SignalOK, s.sig, i)
	assertSlope(t, "15m signal", row.SignalSlope, row.SignalSlopeOK, s.sig, i)
	if row.RSXOK && row.SignalOK && (!row.RSXMinusOK || row.RSXMinusSignal != row.RSX-row.Signal) {
		t.Fatal("rsx minus signal")
	}
}

func assertRSXBus(t *testing.T, name string, rsx StarRSX, s starSeries, i int) {
	t.Helper()
	assertLine(t, name+" rsx", rsx.Value, rsx.ValueOK, rsx.Slope, rsx.SlopeOK, rsx.Accel, rsx.AccelOK, s.rsx, i)
	assertValue(t, name+" signal", rsx.Signal, rsx.SignalOK, s.sig, i)
	assertSlope(t, name+" signal", rsx.SignalSlope, rsx.SignalSlopeOK, s.sig, i)
}

func assertSlope(t *testing.T, name string, slope float64, slopeOK bool, col []float64, i int) {
	t.Helper()
	want := i > 0 && i < len(col) && starFinite(col[i]) && starFinite(col[i-1])
	if slopeOK != want {
		t.Fatalf("%s slope ok", name)
	}
	if slopeOK && slope != col[i]-col[i-1] {
		t.Fatalf("%s slope %v", name, slope)
	}
	if !slopeOK && slope != 0 {
		t.Fatalf("%s slope stored %v", name, slope)
	}
}

func assertLine(t *testing.T, name string, value float64, valueOK bool, slope float64, slopeOK bool, accel float64, accelOK bool, col []float64, i int) {
	t.Helper()
	assertValue(t, name, value, valueOK, col, i)
	assertDelta(t, name, slope, slopeOK, accel, accelOK, col, i)
}

func assertValue(t *testing.T, name string, got float64, ok bool, col []float64, i int) {
	t.Helper()
	wantOK := i >= 0 && i < len(col) && starFinite(col[i])
	if ok != wantOK {
		t.Fatalf("%s ok %v want %v", name, ok, wantOK)
	}
	if ok && got != col[i] {
		t.Fatalf("%s %v != %v", name, got, col[i])
	}
	if !ok && got != 0 {
		t.Fatalf("%s missing stored %v", name, got)
	}
}

func assertDelta(t *testing.T, name string, slope float64, slopeOK bool, accel float64, accelOK bool, col []float64, i int) {
	t.Helper()
	wantSlopeOK := i > 0 && starFinite(col[i]) && starFinite(col[i-1])
	if slopeOK != wantSlopeOK {
		t.Fatalf("%s slope ok", name)
	}
	if slopeOK && slope != col[i]-col[i-1] {
		t.Fatalf("%s slope %v", name, slope)
	}
	if !slopeOK && slope != 0 {
		t.Fatalf("%s slope stored %v", name, slope)
	}
	wantAccelOK := i > 1 && starFinite(col[i]) && starFinite(col[i-1]) && starFinite(col[i-2])
	if accelOK != wantAccelOK {
		t.Fatalf("%s accel ok", name)
	}
	if accelOK && accel != col[i]-2*col[i-1]+col[i-2] {
		t.Fatalf("%s accel %v", name, accel)
	}
	if !accelOK && accel != 0 {
		t.Fatalf("%s accel stored %v", name, accel)
	}
}

func blankStarSeries(n int) starSeries {
	f := func() []float64 { return make([]float64, n) }
	return starSeries{
		open: fInt(n), close: fInt(n),
		vwema: f(), mid: f(), up: f(), dn: f(),
		ema5: f(), ema12: f(), rsiClose: f(),
		closeMid: f(), closeUp: f(), closeDn: f(),
		ema7: f(), macd: f(), rsx: f(), sig: f(),
	}
}

func fInt(n int) []int64 { return make([]int64, n) }
