package indicators

import (
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"
)

func TestRSTV_VerifiedTVBear(t *testing.T) {
	t.Parallel()
	opens, o, h, l, c := loadRSTVFixture(t, "testdata/rsx_tv_one_brain_btcusdt_15m.csv")
	n := len(c)
	hlc3 := make([]float64, n)
	for i := 0; i < n; i++ {
		hlc3[i] = (h[i] + l[i] + c[i]) / 3
		_ = o[i]
	}
	j := NewJurikRSX(14)
	rsx := make([]float64, n)
	for i := range hlc3 {
		rsx[i] = j.Update(hlc3[i])
	}
	facts := ReplayRSTVFacts(opens, c, rsx, DefaultRSXLookback)
	const (
		anchorAt    int64 = 1788630300000 // 2026-09-05 17:45 UTC
		confirmedAt int64 = 1788631200000 // 2026-09-05 18:00 UTC
	)
	found := false
	for _, ev := range facts {
		if ev.Source == FactSourceRSXTVDiv && ev.Direction == FactDirBearish &&
			ev.AnchorAt == anchorAt && ev.ConfirmedAt == confirmedAt {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing verified TV bear AnchorAt=%d ConfirmedAt=%d facts=%d", anchorAt, confirmedAt, len(facts))
	}
}

func TestRSTV_BatchEqualsLiveStep(t *testing.T) {
	t.Parallel()
	closes, rsx, opens := rstvForcedSeries()
	batch := ReplayRSTVFacts(opens, closes, rsx, 90)
	st := NewRSTVState(90)
	var step []IndicatorFactEvent
	for i := range rsx {
		evs, err := st.UpdateClosed(opens[i], closes[i], rsx[i])
		if err != nil {
			t.Fatal(err)
		}
		step = append(step, evs.Facts()...)
	}
	if len(batch) != len(step) {
		t.Fatalf("batch %d step %d", len(batch), len(step))
	}
	for i := range batch {
		if batch[i] != step[i] {
			t.Fatalf("i=%d batch=%+v step=%+v", i, batch[i], step[i])
		}
	}
}

func TestRSTV_WakeSamePrefix(t *testing.T) {
	t.Parallel()
	closes, rsx, opens := rstvForcedSeries()
	split := 25
	replay := func() []IndicatorFactEvent {
		st := NewRSTVState(90)
		var out []IndicatorFactEvent
		for i := 0; i < split; i++ {
			evs, err := st.UpdateClosed(opens[i], closes[i], rsx[i])
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, evs.Facts()...)
		}
		for i := split; i < len(rsx); i++ {
			evs, err := st.UpdateClosed(opens[i], closes[i], rsx[i])
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, evs.Facts()...)
		}
		return out
	}
	a := replay()
	b := replay()
	if len(a) != len(b) {
		t.Fatalf("%d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("i=%d a=%+v b=%+v", i, a[i], b[i])
		}
	}
	full := ReplayRSTVFacts(opens, closes, rsx, 90)
	if len(a) != len(full) {
		t.Fatalf("continue %d full %d", len(a), len(full))
	}
}

func rstvForcedSeries() (closes, rsx []float64, opens []int64) {
	n := 40
	closes = make([]float64, n)
	rsx = make([]float64, n)
	opens = make([]int64, n)
	const step int64 = 60_000
	base := int64(1_700_000_000_000)
	for i := 0; i < n; i++ {
		opens[i] = base + int64(i)*step
		closes[i] = 100 + float64(i)
		if i <= 12 {
			rsx[i] = 40 + float64(i)
		} else {
			rsx[i] = 52 - float64(i-12)*1.5
		}
	}
	return closes, rsx, opens
}

func rstvPivotPeakSeries() (closes, rsx []float64, opens []int64) {
	n := 24
	closes = make([]float64, n)
	rsx = make([]float64, n)
	opens = make([]int64, n)
	base := int64(1_700_000_000_000)
	for i := 0; i < n; i++ {
		opens[i] = base + int64(i)*60_000
		closes[i] = 100
		rsx[i] = 50
	}
	rsx[10] = 80
	return closes, rsx, opens
}

func TestRSTV_PivotTiming(t *testing.T) {
	t.Parallel()
	closes, rsx, opens := rstvPivotPeakSeries()
	facts := TVPivotFacts(closes, rsx, opens, 90)
	found := false
	for _, ev := range facts {
		if ev.Direction == FactDirPivotHigh && ev.AnchorAt == opens[10] && ev.ConfirmedAt == opens[12] {
			found = true
			if ev.ConfirmedAt-ev.AnchorAt != 120_000 {
				t.Fatalf("delay %d", ev.ConfirmedAt-ev.AnchorAt)
			}
		}
	}
	if !found {
		t.Fatalf("expected pivot high at i-2, facts=%+v", facts)
	}
}

func TestRSTV_MultiEventCapacity(t *testing.T) {
	t.Parallel()
	var evs RSTVEvents
	evs.push(RSTVEvent{Family: RSTVFamilyDiv, Direction: FactDirBearish, ConfirmedAt: 2, AnchorAt: 1})
	evs.push(RSTVEvent{Family: RSTVFamilyDiv, Direction: FactDirBullish, ConfirmedAt: 2, AnchorAt: 1})
	evs.push(RSTVEvent{Family: RSTVFamilyPivot, Direction: FactDirPivotHigh, ConfirmedAt: 2, AnchorAt: 0})
	evs.push(RSTVEvent{Family: RSTVFamilyPivot, Direction: FactDirPivotLow, ConfirmedAt: 2, AnchorAt: 0})
	if evs.Count != 4 {
		t.Fatalf("count %d", evs.Count)
	}
	facts := evs.Facts()
	if len(facts) != 4 {
		t.Fatalf("facts %d", len(facts))
	}
}

func TestRSTV_InvalidInputNoMutation(t *testing.T) {
	t.Parallel()
	st := NewRSTVState(8)
	if _, err := st.UpdateClosed(1000, 1, 50); err != nil {
		t.Fatal(err)
	}
	snap := st.LastOpenTime()
	bars := st.bars
	cases := []struct {
		at    int64
		close float64
		rsx   float64
	}{
		{1000, 1, 50},
		{999, 1, 50},
		{2000, math.NaN(), 50},
		{2000, 1, math.Inf(1)},
		{2000, math.Inf(-1), 50},
	}
	for _, tc := range cases {
		if _, err := st.UpdateClosed(tc.at, tc.close, tc.rsx); err == nil {
			t.Fatalf("expected refuse at=%d close=%v rsx=%v", tc.at, tc.close, tc.rsx)
		}
		if st.LastOpenTime() != snap || st.bars != bars {
			t.Fatal("state mutated after refuse")
		}
	}
	if _, err := st.UpdateClosed(2000, 1, 51); err != nil {
		t.Fatal(err)
	}
	if st.LastOpenTime() != 2000 {
		t.Fatal("valid bar must commit")
	}
}

func loadRSTVFixture(t *testing.T, path string) (opens []int64, o, h, l, c []float64) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) < 5 {
			t.Fatalf("row %d", i)
		}
		at, _ := strconv.ParseInt(row[0], 10, 64)
		ov, _ := strconv.ParseFloat(row[1], 64)
		hv, _ := strconv.ParseFloat(row[2], 64)
		lv, _ := strconv.ParseFloat(row[3], 64)
		cv, _ := strconv.ParseFloat(row[4], 64)
		opens = append(opens, at)
		o = append(o, ov)
		h = append(h, hv)
		l = append(l, lv)
		c = append(c, cv)
	}
	if len(c) < 200 {
		t.Fatalf("fixture too short %d", len(c))
	}
	return opens, o, h, l, c
}
