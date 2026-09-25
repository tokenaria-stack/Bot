package market

import (
	"math"
	"reflect"
	"testing"

	"trading_bot/core"
	"trading_bot/core/nodes"
	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

const starFixtureOrigin int64 = 1704067200000 // 2024-01-01 UTC, aligned to 4h

func TestHistoryBusFullRingOldestNaN(t *testing.T) {
	h := core.NewHistoryBus(64)
	if h.Cap() != 64 {
		t.Fatalf("cap %d", h.Cap())
	}
	for i := 0; i < h.Cap(); i++ {
		h.Push(core.SlotJurikRSX, float64(i+1))
		h.Advance()
	}
	if h.Count() != h.Cap() {
		t.Fatalf("count %d cap %d", h.Count(), h.Cap())
	}
	if !math.IsNaN(h.Get(core.SlotJurikRSX, h.Count())) {
		t.Fatal("full ring oldest lookup must be NaN")
	}
	if got := h.Get(core.SlotJurikRSX, 1); got != float64(h.Cap()) {
		t.Fatalf("newest = %v", got)
	}
	if math.IsNaN(h.ValueAtBar(core.SlotJurikRSX, 1)) {
		t.Fatal("bar 1 must stay readable when bar 0 is hidden")
	}
}

func TestCausalTVDivAgeZeroDistinctFromMissing(t *testing.T) {
	opens := map[int64]int{1000: 0, 2000: 10, 5000: 40}
	facts := []indicators.IndicatorFactEvent{{
		Source:      indicators.FactSourceRSXTVDiv,
		Direction:   indicators.FactDirBullish,
		ConfirmedAt: 5000,
	}}
	dir, at, age, ok, err := causalTVDiv(facts, opens, 5000, 40)
	if err != nil || !ok || dir != indicators.FactDirBullish || at != 5000 || age != 0 {
		t.Fatalf("age 0 fact: dir=%s at=%d age=%d ok=%v err=%v", dir, at, age, ok, err)
	}

	dir, at, age, ok, err = causalTVDiv(nil, opens, 5000, 40)
	if err != nil || ok || dir != "none" || at != 0 || age != 0 {
		t.Fatalf("missing: dir=%s at=%d age=%d ok=%v err=%v", dir, at, age, ok, err)
	}

	early := []indicators.IndicatorFactEvent{{
		Source:      indicators.FactSourceRSXTVDiv,
		Direction:   indicators.FactDirBearish,
		ConfirmedAt: 1000,
	}}
	_, _, age, ok, err = causalTVDiv(early, opens, 5000, 40)
	if err != nil || !ok || age != 40 {
		t.Fatalf("uncapped age=%d ok=%v err=%v", age, ok, err)
	}

	future := []indicators.IndicatorFactEvent{{
		Source:      indicators.FactSourceRSXTVDiv,
		Direction:   indicators.FactDirBullish,
		ConfirmedAt: 9000,
	}}
	dir, _, _, ok, err = causalTVDiv(future, opens, 5000, 40)
	if err != nil || ok || dir != "none" {
		t.Fatalf("future fact selected: dir=%s ok=%v err=%v", dir, ok, err)
	}
}

func TestExtractStarSnapshots(t *testing.T) {
	k15 := starFixtureKlines(16*160, "15m")
	k1h := starFixtureKlines(4*160, "1h")
	k4h := starFixtureKlines(160, "4h")
	rsx := defaultRSXSettings()

	before := testDAGRunnerBorn.Load()
	rows, err := ExtractStarSnapshots(k15, k1h, k4h, rsx)
	if err != nil {
		t.Fatal(err)
	}
	if born := testDAGRunnerBorn.Load() - before; born != 3 {
		t.Fatalf("dag runners %d, stars %d", born, len(rows))
	}
	if len(rows) < 2 {
		t.Fatalf("stars %d", len(rows))
	}

	again, err := ExtractStarSnapshots(k15, k1h, k4h, rsx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows, again) {
		t.Fatal("rows differ across identical calls")
	}

	up, down := 0, 0
	var h1ok, h4ok, h1miss, h4miss, tvOK, tvNone, agePastCap int
	for _, row := range rows {
		if row.AnchorAt != row.ConfirmedAt || row.AnchorAt != row.M15.OpenTime {
			t.Fatalf("clock %d %d %d", row.AnchorAt, row.ConfirmedAt, row.M15.OpenTime)
		}
		switch row.Side {
		case nodes.WozduhXoverSideUp:
			up++
		case nodes.WozduhXoverSideDown:
			down++
		default:
			t.Fatalf("side %q", row.Side)
		}
		if !row.M15.Present || !row.M15.ValuesOK || !row.M15.DistanceOK || !row.M15.SlopeOK {
			t.Fatalf("15m context %+v", row.M15)
		}
		if row.H1.Present && row.H1.CloseTime > row.M15.CloseTime {
			t.Fatalf("1h close %d after %d", row.H1.CloseTime, row.M15.CloseTime)
		}
		if row.H4.Present && row.H4.CloseTime > row.M15.CloseTime {
			t.Fatalf("4h close %d after %d", row.H4.CloseTime, row.M15.CloseTime)
		}
		if row.TVAgeOK && row.TVConfirmedAt > row.ConfirmedAt {
			t.Fatalf("tv %d after star %d", row.TVConfirmedAt, row.ConfirmedAt)
		}
		if !row.TVAgeOK && (row.TVDirection != "none" || row.TVAge != 0) {
			t.Fatalf("missing tv %+v", row)
		}
		if row.H1.Present && row.H1.ValuesOK {
			h1ok++
		} else if !row.H1.Present {
			h1miss++
			if row.M15.CloseTime >= k1h[0].CloseTime {
				t.Fatalf("1h missing after first close star=%d first=%d", row.M15.CloseTime, k1h[0].CloseTime)
			}
		}
		if row.H4.Present && row.H4.ValuesOK {
			h4ok++
		} else if !row.H4.Present {
			h4miss++
			if row.M15.CloseTime >= k4h[0].CloseTime {
				t.Fatalf("4h missing after first close star=%d first=%d", row.M15.CloseTime, k4h[0].CloseTime)
			}
		}
		if row.TVAgeOK {
			tvOK++
			if row.TVAge > 36 {
				agePastCap++
			}
		} else {
			tvNone++
		}
	}
	if up == 0 || down == 0 {
		t.Fatalf("sides up=%d down=%d", up, down)
	}
	if h1ok == 0 || h4ok == 0 || tvOK == 0 || tvNone == 0 || agePastCap == 0 {
		t.Fatalf("finite htf 1h=%d 4h=%d tv=%d none=%d age>36=%d of %d", h1ok, h4ok, tvOK, tvNone, agePastCap, len(rows))
	}

	edges := starEdgesFromReplay(t, k15, rsx)
	if len(edges) != len(rows) {
		t.Fatalf("edge count %d != star count %d", len(edges), len(rows))
	}
	for i, row := range rows {
		if edges[i].open != row.AnchorAt || edges[i].side != row.Side {
			t.Fatalf("row %d edge %+v row %s %d", i, edges[i], row.Side, row.AnchorAt)
		}
	}

	far := int64(100000) * 4 * 60 * 60 * 1000
	shifted, err := ExtractStarSnapshots(k15, shiftStarKlines(k1h, far), shiftStarKlines(k4h, far), rsx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shifted) != len(rows) {
		t.Fatalf("context changed star count %d -> %d", len(rows), len(shifted))
	}
	for i, row := range shifted {
		if row.AnchorAt != rows[i].AnchorAt || row.Side != rows[i].Side {
			t.Fatalf("shifted star %d", i)
		}
		if row.H1.Present || row.H4.Present {
			t.Fatalf("future htf selected %+v %+v", row.H1, row.H4)
		}
	}

	t.Logf("stars=%d up=%d down=%d h1ok=%d h4ok=%d h1miss=%d h4miss=%d tv=%d tvnone=%d agePastCap=%d", len(rows), up, down, h1ok, h4ok, h1miss, h4miss, tvOK, tvNone, agePastCap)
}

type starEdge struct {
	open int64
	side string
}

func starEdgesFromReplay(t *testing.T, k15 []exchange.Kline, rsx RSXSettings) []starEdge {
	t.Helper()
	hist := ReplayClosedBars(k15, rsx).Hist
	vwema := historySlotSeries(hist, core.SlotWozduhRsiHl2Vwema)
	mid := historySlotSeries(hist, core.SlotWozduhVolRsiEma5ChanMid)
	var out []starEdge
	for i := 1; i < len(k15); i++ {
		side := nodes.DetectCrossoverEdge(vwema[i-1], mid[i-1], vwema[i], mid[i])
		if side == nodes.WozduhXoverSideUp || side == nodes.WozduhXoverSideDown {
			out = append(out, starEdge{open: k15[i].OpenTime, side: side})
		}
	}
	return out
}

func starFixtureKlines(n int, interval string) []exchange.Kline {
	step := starIntervalMs(interval)
	out := make([]exchange.Kline, n)
	for i := 0; i < n; i++ {
		x := float64(i)
		price := 100 + 18*math.Sin(x/7) + 9*math.Sin(x/2.1)
		if m := i % 48; m >= 40 {
			price += float64((i/48)+8) * float64(m-39)
		}
		if m := i % 48; m <= 6 {
			price -= float64((i/48)+6) * float64(6-m)
		}
		vol := 800 + 500*math.Sin(x/3.5) + 120*math.Sin(x/1.3)
		if vol < 20 {
			vol = 20
		}
		open := starFixtureOrigin + int64(i)*step
		ct, err := data.BarCloseTimeMs(open, interval)
		if err != nil {
			panic(err)
		}
		out[i] = exchange.Kline{
			OpenTime:  open,
			CloseTime: ct,
			Open:      price,
			High:      price + 2,
			Low:       price - 2,
			Close:     price,
			Volume:    vol,
		}
	}
	return out
}

func starIntervalMs(interval string) int64 {
	switch interval {
	case "15m":
		return 15 * 60 * 1000
	case "1h":
		return 60 * 60 * 1000
	case "4h":
		return 4 * 60 * 60 * 1000
	default:
		panic(interval)
	}
}

func shiftStarKlines(in []exchange.Kline, delta int64) []exchange.Kline {
	out := append([]exchange.Kline(nil), in...)
	for i := range out {
		out[i].OpenTime += delta
		out[i].CloseTime += delta
	}
	return out
}
