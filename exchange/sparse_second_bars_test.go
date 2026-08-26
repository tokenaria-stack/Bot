package exchange

import (
	"testing"
	"time"
	"trading_bot/data"
)

func secK(openMs int64, o, h, l, c, v float64) Kline {
	ct, _ := data.BarCloseTimeMs(openMs, SecondTF)
	return Kline{OpenTime: openMs, CloseTime: ct, Open: o, High: h, Low: l, Close: c, Volume: v}
}

func noonUTC() int64 {
	return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).UnixMilli()
}

func TestSparseSecondGoldenClosedEmptyForming(t *testing.T) {
	t.Parallel()
	base := noonUTC()
	res, err := FoldSparseSecondParents([]Kline{secK(base, 100, 101, 99, 100.5, 2)}, "5s", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Closed) != 0 {
		t.Fatalf("closed=%d want 0 (wall time must not close)", len(res.Closed))
	}
	if res.Forming == nil || res.Forming.OpenTime != base {
		t.Fatalf("forming=%v want 12:00:00 5s", res.Forming)
	}
	if res.Forming.Open != 100 || res.Forming.Close != 100.5 || res.Forming.Volume != 2 {
		t.Fatalf("OHLCV %+v", res.Forming)
	}
}

func TestSparseSecondSameParentReplace(t *testing.T) {
	t.Parallel()
	base := noonUTC()
	acc, err := NewSparseSecondReducer("5s")
	if err != nil {
		t.Fatal(err)
	}
	_, f, didClose, ok := acc.OnParent(secK(base, 10, 10, 10, 10, 1))
	if !ok || didClose || f.Volume != 1 {
		t.Fatalf("first %+v closed=%v", f, didClose)
	}
	_, f, didClose, ok = acc.OnParent(secK(base, 10, 12, 9, 11, 3))
	if !ok || didClose {
		t.Fatal("same 1s OpenTime must replace, not close")
	}
	if f.Open != 10 || f.High != 12 || f.Low != 9 || f.Close != 11 || f.Volume != 3 {
		t.Fatalf("replaced OHLCV %+v", f)
	}
}

func TestSparseSecondMissingSecondsStillValid(t *testing.T) {
	t.Parallel()
	base := noonUTC()
	parents := []Kline{
		secK(base, 1, 2, 1, 2, 1),
		secK(base+2000, 3, 4, 2, 3, 2),
		secK(base+4000, 5, 6, 4, 5, 3),
	}
	res, err := FoldSparseSecondParents(parents, "5s", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Closed) != 0 || res.Forming == nil {
		t.Fatalf("quiet 1s holes must still form one 5s, got closed=%d forming=%v", len(res.Closed), res.Forming)
	}
	if res.Forming.Open != 1 || res.Forming.High != 6 || res.Forming.Low != 1 || res.Forming.Close != 5 || res.Forming.Volume != 6 {
		t.Fatalf("sparse OHLCV %+v", res.Forming)
	}
}

func TestSparseSecondLaterBucketFormingClosesPriorOnce(t *testing.T) {
	t.Parallel()
	base := noonUTC()
	acc, err := NewSparseSecondReducer("5s")
	if err != nil {
		t.Fatal(err)
	}
	acc.OnParent(secK(base, 10, 10, 10, 10, 1))
	closed, forming, didClose, ok := acc.OnParent(secK(base+5000, 11, 11, 11, 11, 1))
	if !ok || !didClose {
		t.Fatal("forming 1s in later 5s bucket must close prior once")
	}
	if closed.OpenTime != base || forming.OpenTime != base+5000 {
		t.Fatalf("closed=%d forming=%d", closed.OpenTime, forming.OpenTime)
	}
	_, _, didClose, _ = acc.OnParent(secK(base+5000, 11, 12, 11, 12, 2))
	if didClose {
		t.Fatal("same new bucket must not close again")
	}
}

func TestSparseSecondSilenceDoesNotClose(t *testing.T) {
	t.Parallel()
	base := noonUTC()
	acc, _ := NewSparseSecondReducer("5s")
	acc.OnParent(secK(base, 1, 1, 1, 1, 1))
	_, f, didClose, ok := acc.OnParent(secK(base+1000, 2, 2, 2, 2, 1))
	if !ok || didClose || f.OpenTime != base {
		t.Fatalf("in-bucket 1s must keep forming, closed=%v %+v", didClose, f)
	}
}

func TestSparseSecondLiveClosedEqualsBatchFold(t *testing.T) {
	t.Parallel()
	base := noonUTC()
	parents := []Kline{
		secK(base, 1, 2, 1, 1.5, 1),
		secK(base+1000, 2, 3, 1.5, 2.5, 1),
		secK(base+5000, 4, 5, 4, 4.5, 2),
		secK(base+11000, 6, 6, 6, 6, 1),
	}
	batch, err := FoldSparseSecondParents(parents, "5s", nil)
	if err != nil {
		t.Fatal(err)
	}
	acc, _ := NewSparseSecondReducer("5s")
	var liveClosed []Kline
	var liveForming *Kline
	for _, p := range parents {
		c, f, didClose, ok := acc.OnParent(p)
		if didClose {
			liveClosed = append(liveClosed, c)
		}
		if ok {
			bar := f
			liveForming = &bar
		}
	}
	if len(liveClosed) != len(batch.Closed) {
		t.Fatalf("closed live=%d fold=%d", len(liveClosed), len(batch.Closed))
	}
	for i := range liveClosed {
		if liveClosed[i].OpenTime != batch.Closed[i].OpenTime ||
			liveClosed[i].Open != batch.Closed[i].Open ||
			liveClosed[i].Close != batch.Closed[i].Close ||
			liveClosed[i].Volume != batch.Closed[i].Volume {
			t.Fatalf("closed[%d] live=%+v fold=%+v", i, liveClosed[i], batch.Closed[i])
		}
	}
	if (liveForming == nil) != (batch.Forming == nil) {
		t.Fatal("forming presence mismatch")
	}
	if liveForming != nil && liveForming.OpenTime != batch.Forming.OpenTime {
		t.Fatalf("forming live=%d fold=%d", liveForming.OpenTime, batch.Forming.OpenTime)
	}
}

func TestSparseSecondLookBehindTruncationVsSilence(t *testing.T) {
	t.Parallel()
	base := noonUTC()
	// Fetch starts at :02; previous parent also in 12:00:00 bucket → drop.
	rest := []Kline{secK(base+2000, 2, 2, 2, 2, 1), secK(base+5000, 3, 3, 3, 3, 1)}
	lb := secK(base, 1, 1, 1, 1, 1)
	res, err := FoldSparseSecondParents(rest, "5s", &lb)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Closed) != 0 {
		t.Fatalf("truncated 12:00:00 must drop, closed=%d", len(res.Closed))
	}
	if res.Forming == nil || res.Forming.OpenTime != base+5000 {
		t.Fatalf("kept later bucket forming=%v", res.Forming)
	}

	// Previous parent in prior 5s bucket → :02 is genuine first trade.
	prior := secK(base-1000, 9, 9, 9, 9, 1)
	res, err = FoldSparseSecondParents(rest, "5s", &prior)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Closed) != 1 || res.Closed[0].OpenTime != base {
		t.Fatalf("silence prefix must keep 12:00:00 child, closed=%v", res.Closed)
	}
}

func TestSparseSecondReducerTableDrivenRatios(t *testing.T) {
	t.Parallel()
	for _, child := range []string{"5s", "10s", "15s", "30s", "45s"} {
		n, err := SparseSecondParentRows(3000, child)
		if err != nil {
			t.Fatalf("%s: %v", child, err)
		}
		ratio, _ := sparseSecondRatio(child)
		if n != 3000*ratio {
			t.Fatalf("%s rows=%d want %d", child, n, 3000*ratio)
		}
		acc, err := NewSparseSecondReducer(child)
		if err != nil {
			t.Fatalf("%s reducer: %v", child, err)
		}
		base := noonUTC()
		_, _, _, ok := acc.OnParent(secK(base, 1, 1, 1, 1, 1))
		if !ok {
			t.Fatalf("%s first parent must form", child)
		}
	}
}

func TestSparseSecondEmptyBucketEmitsNothing(t *testing.T) {
	t.Parallel()
	res, err := FoldSparseSecondParents(nil, "5s", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Closed) != 0 || res.Forming != nil {
		t.Fatalf("empty parents must emit nothing: %+v", res)
	}
}
