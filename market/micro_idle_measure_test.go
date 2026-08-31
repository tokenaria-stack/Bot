package market

import (
	"fmt"
	"testing"
	"time"

	"trading_bot/core/nodes"
	"trading_bot/exchange"
)

func micro1sParents(n int) []exchange.Kline {
	out := make([]exchange.Kline, n)
	start := int64(1_700_000_000_000)
	step := int64(1000)
	for i := 0; i < n; i++ {
		ot := start + int64(i)*step
		px := 100.0 + float64(i%17)*0.01
		out[i] = exchange.Kline{
			OpenTime:  ot,
			CloseTime: ot + step - 1,
			Open:      px,
			High:      px + 0.2,
			Low:       px - 0.2,
			Close:     px + 0.05,
			Volume:    1,
		}
	}
	return out
}

func fanoutSparseOnce(accs map[string]*exchange.SparseSecondReducer, frames map[string]*Frame, parent exchange.Kline) (forming, closed int) {
	for _, e := range exchange.SparseSecondChildren() {
		c, f, didClose, ok := accs[e.Name].OnParent(parent)
		if didClose {
			closed++
			frames[e.Name].UpdateKlineTick(c, true)
		}
		if ok {
			forming++
			frames[e.Name].UpdateKlineTick(f, false)
		}
	}
	return forming, closed
}

func newIdleSparseFrames(t *testing.T, allRSX bool) (map[string]*exchange.SparseSecondReducer, map[string]*Frame) {
	t.Helper()
	accs := make(map[string]*exchange.SparseSecondReducer)
	frames := make(map[string]*Frame)
	for _, e := range exchange.SparseSecondChildren() {
		acc, err := exchange.NewSparseSecondReducer(e.Name)
		if err != nil {
			t.Fatal(err)
		}
		accs[e.Name] = acc
		f := NewFrame(nil, e.Name, ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
		if allRSX {
			f.SetRSXDemand(nodes.RSXWorkAll)
			f.SetWozduhDemand(nodes.WozduhMaskAll)
		}
		frames[e.Name] = f
	}
	return accs, frames
}

func TestMicroIdle_ResidualCostVsOldAnalytics(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	const n = 1200 // 20 minutes of 1s parents
	parents := micro1sParents(n)

	idleAcc, idleFrames := newIdleSparseFrames(t, false)
	var idleForm, idleClose int
	t0 := time.Now()
	for _, p := range parents {
		f, c := fanoutSparseOnce(idleAcc, idleFrames, p)
		idleForm += f
		idleClose += c
	}
	idleDur := time.Since(t0)

	oldAcc, oldFrames := newIdleSparseFrames(t, true)
	t1 := time.Now()
	for _, p := range parents {
		fanoutSparseOnce(oldAcc, oldFrames, p)
	}
	oldDur := time.Since(t1)

	redAcc := make(map[string]*exchange.SparseSecondReducer)
	for _, e := range exchange.SparseSecondChildren() {
		acc, err := exchange.NewSparseSecondReducer(e.Name)
		if err != nil {
			t.Fatal(err)
		}
		redAcc[e.Name] = acc
	}
	t2 := time.Now()
	for _, p := range parents {
		for _, e := range exchange.SparseSecondChildren() {
			redAcc[e.Name].OnParent(p)
		}
	}
	redDur := time.Since(t2)

	perParentIdle := idleDur / time.Duration(n)
	oldPer := oldDur / time.Duration(n)
	redPer := redDur / time.Duration(n)
	ratio := float64(idleDur) / float64(oldDur)

	for _, e := range exchange.SparseSecondChildren() {
		mask, rsxU, zzU, tv, fr, zz, _, _ := idleFrames[e.Name].RSXLiveStats()
		woz, wozU, _ := idleFrames[e.Name].WozduhLiveStats()
		if mask != 0 || rsxU != 0 || zzU != 0 || tv != 0 || fr != 0 || zz != 0 || woz != 0 || wozU != 0 {
			t.Fatalf("%s leaked analytics mask=%#b rsx=%d zz=%d tv=%d fr=%d zzE=%d woz=%#b wozU=%d",
				e.Name, mask, rsxU, zzU, tv, fr, zz, woz, wozU)
		}
	}

	t.Logf("idle ChartOnly, no micro charts, %d 1s parents", n)
	t.Logf("reducer-only (5 children OnParent): %s  (%s/parent)", redDur, redPer)
	t.Logf("reducers + unused Frame ticks:      %s  (%s/parent)  formingTicks=%d closedTicks=%d",
		idleDur, perParentIdle, idleForm, idleClose)
	t.Logf("same fanout with RSXWorkAll+Wozduh: %s  (%s/parent)", oldDur, oldPer)
	t.Logf("residual/old-analytics ratio=%.4f  (idle is %.1fx cheaper)", ratio, 1/ratio)
	fmt.Printf("MICRO-IDLE residual: idle=%s oldAll=%s ratio=%.4f forming=%d closed=%d\n",
		idleDur, oldDur, ratio, idleForm, idleClose)

	// One 1s parent per wall-clock second in production. ~6µs of CPU per parent
	// for five unused children is noise versus process CPU. MICRO-IDLE-1 would
	// skip this OHLCV/empty-DAG path; that is not worth a second lifecycle.
	if perParentIdle > 100*time.Microsecond {
		t.Fatalf("unexpected idle cost %s/parent — re-open MICRO-IDLE-1", perParentIdle)
	}
}
