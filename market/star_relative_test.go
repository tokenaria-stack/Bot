package market

import (
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestStarRelationsCountAndSource(t *testing.T) {
	body, err := os.ReadFile("star_relative.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, forbidden := range []string{
		"replayStarSeries",
		"ReplayClosedBars",
		"WozduhNode",
		"RSXNode",
		"FeatureRuntime2",
		"NewJurikRSX",
		"NewRSI(",
		"NewEMA(",
		"NewMACD(",
		"NewWozduhNode",
		"NewRSXNode",
		"loadClosedRange",
		"historical_klines",
		"exchange.Kline",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("matrix contains %s", forbidden)
		}
	}
	n := 0
	rt := reflect.TypeOf(StarRelations{})
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Type == reflect.TypeOf(Rel{}) {
			n++
			name := rt.Field(i).Name
			if strings.Contains(name, "M15H4") || strings.Contains(name, "M15D1") {
				t.Fatalf("non-adjacent relation %s", name)
			}
		}
	}
	if n != 49 {
		t.Fatalf("relations %d", n)
	}
	var blank StarRelations
	if got := len(blank.Relations()); got != 49 {
		t.Fatalf("ordered relations %d", got)
	}
}

func TestStarRelationArithmetic(t *testing.T) {
	m15 := tfReady()
	m15.Vwema, m15.ChanMid = 10, 4
	m15.Width, m15.CloseWidth = 7, 3
	m15.CloseMid, m15.Macd, m15.RsiClose = 11, 8, 5
	m15.Ema5, m15.Ema7, m15.Ema12 = 2, 9, 6
	m15.Slope, m15.MidSlope = 1, 12
	h1 := tfReady()
	h1.Vwema, h1.ChanMid = 8, 1
	h1.Width, h1.CloseWidth = 5, 9
	h1.Ema5, h1.Ema7, h1.Ema12, h1.Macd, h1.RsiClose = 3, 7, 10, 2, 6
	row := StarSnapshotV3{StarSnapshot: StarSnapshot{
		Side: "up", AnchorAt: 100, ConfirmedAt: 100,
		M15: m15, H1: h1,
		RSX: 12, RSXOK: true,
		H1RSX: StarRSX{Value: 1, ValueOK: true, Signal: 4, SignalOK: true},
	}}
	got := projectRelations(row)

	if g := got.M15Ema7MinusMacd; !g.OK || g.Value != 9-8 {
		t.Fatalf("ema7-macd %+v", g)
	}
	if g := got.M15Ema7MinusCloseMid; !g.OK || g.Value != 9-11 {
		t.Fatalf("ema7-mid %+v", g)
	}
	if g := got.M15RsiCloseMinusCloseMid; !g.OK || g.Value != 5-11 {
		t.Fatalf("rsi-mid %+v", g)
	}
	if g := got.M15VwemaSlopeMinusMidSlope; !g.OK || g.Value != 1-12 {
		t.Fatalf("slope gap %+v", g)
	}
	if g := got.M15H1Vwema; !g.OK || g.Value != 10-8 {
		t.Fatalf("vwema gap %+v", g)
	}
	if g := got.M15H1OrangeMid; !g.OK || g.Value != 4-1 {
		t.Fatalf("mid gap %+v", g)
	}
	if g := got.M15H1Rsx; !g.OK || g.Value != 12-1 {
		t.Fatalf("rsx reads the root, got %+v", g)
	}
	if g := got.H1RsxMinusSignal; !g.OK || g.Value != 1-4 {
		t.Fatalf("1h rsx-signal %+v", g)
	}
	if g := got.M15H1Width; !g.OK || g.Value != 7-5 {
		t.Fatalf("width %+v", g)
	}
	if g := got.M15H1CloseWidth; !g.OK || g.Value != 3-9 {
		t.Fatalf("close width %+v", g)
	}
	if got.M15H1Ema5.Value != 2-3 || got.M15H1Ema12.Value != 6-10 || got.M15H1RsiClose.Value != 5-6 || got.M15H1Ema7.Value != 9-7 || got.M15H1Macd.Value != 8-2 {
		t.Fatalf("level set %+v", got)
	}

	// Equal operands are a real zero, not a missing relation.
	same := tfReady()
	same.Vwema, same.ChanMid = 5, 5
	same.Width, same.CloseWidth, same.CloseMid = 5, 5, 5
	same.Macd, same.RsiClose, same.Ema5, same.Ema7, same.Ema12 = 5, 5, 5, 5, 5
	same.Slope, same.MidSlope = 5, 5
	zero := projectRelations(StarSnapshotV3{StarSnapshot: StarSnapshot{M15: same, H1: same, RSX: 5, RSXOK: true, H1RSX: StarRSX{Value: 5, ValueOK: true, Signal: 5, SignalOK: true}}})
	if !zero.M15H1Vwema.OK || zero.M15H1Vwema.Value != 0 {
		t.Fatalf("zero gap %+v", zero.M15H1Vwema)
	}

	// A finite VWEMA stays an observation when ValuesOK is false.
	// Orange mid is not finite, so that relation is missing.
	lo := tfReady()
	lo.Vwema = 0
	lo.ValuesOK = false
	lo.ChanMid = math.NaN()
	hi := tfReady()
	hi.Vwema, hi.ChanMid = 4, 2
	partial := projectRelations(StarSnapshotV3{StarSnapshot: StarSnapshot{M15: lo, H1: hi, RSX: 1, RSXOK: true, H1RSX: StarRSX{Value: 1, ValueOK: true}}})
	if !partial.M15H1Vwema.OK || partial.M15H1Vwema.Value != 0-4 {
		t.Fatalf("real zero vwema %+v", partial.M15H1Vwema)
	}
	if partial.M15H1OrangeMid.OK || partial.M15H1OrangeMid.Value != 0 {
		t.Fatalf("missing mid %+v", partial.M15H1OrangeMid)
	}

	// One missing endpoint clears the relation and leaves the other side's numbers unused.
	broken := h1
	broken.Ema7OK = false
	broken.Ema7 = 0
	miss := projectRelations(StarSnapshotV3{StarSnapshot: StarSnapshot{M15: m15, H1: broken}})
	if miss.H1Ema7MinusMacd.OK || miss.H1Ema7MinusMacd.Value != 0 {
		t.Fatalf("missing ema7 %+v", miss.H1Ema7MinusMacd)
	}
	if !miss.M15Ema7MinusMacd.OK {
		t.Fatal("15m relation followed the 1h hole")
	}

	// Absent daily stores 0. Every daily relation stays missing.
	h4 := tfReady()
	h4.Vwema, h4.Ema7, h4.Macd = 9, 7, 5
	daily := projectRelations(StarSnapshotV3{StarSnapshot: StarSnapshot{
		Side: "down", AnchorAt: 1567965600000, ConfirmedAt: 1567965600000,
		M15: m15, H1: h1, H4: h4,
		RSX: 12, RSXOK: true,
		H1RSX: StarRSX{Value: 1, ValueOK: true, Signal: 4, SignalOK: true},
		H4RSX: StarRSX{Value: 3, ValueOK: true, Signal: 1, SignalOK: true},
	}})
	rels := daily.Relations()
	for i, g := range rels[12:16] {
		if g.OK || g.Value != 0 {
			t.Fatalf("daily same-bar %d %+v", i, g)
		}
	}
	if daily.D1RsxMinusSignal.OK || daily.H4D1Vwema.OK || daily.H4D1Width.OK || daily.H4D1CloseWidth.OK || daily.H4D1Rsx.OK {
		t.Fatalf("daily neighbor leaked %+v", daily)
	}
	if n := len(dailyRelations(daily)); n != 15 {
		t.Fatalf("daily relations %d", n)
	}
	if !daily.M15H1Vwema.OK || !daily.H1H4Vwema.OK || !daily.H4Ema7MinusMacd.OK {
		t.Fatal("lower relations fell with the missing daily bar")
	}
}

func tfReady() StarTF {
	return StarTF{
		Present: true, ValuesOK: true,
		WidthOK: true, CloseWidthOK: true, CloseMidOK: true,
		MacdOK: true, RsiCloseOK: true, Ema5OK: true, Ema7OK: true, Ema12OK: true,
		SlopeOK: true, MidSlopeOK: true,
	}
}
