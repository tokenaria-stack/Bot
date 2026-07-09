package indicators

import "testing"

func TestDynamicFractal_matchesWilliamsAtTwoTwo(t *testing.T) {
	wf := NewWilliamsFractals()
	df := NewDynamicFractal(2, 2)

	highs := []float64{10, 12, 15, 13, 11, 14, 16, 12, 10, 13}
	lows := []float64{9, 10, 11, 10, 9, 11, 12, 10, 8, 11}

	for i := range highs {
		ws := wf.UpdateCandle(highs[i], lows[i])
		ds := df.UpdateCandle(highs[i], lows[i])
		if ws.UpFractal != ds.UpFractal || ws.DownFractal != ds.DownFractal {
			t.Fatalf("bar %d mismatch: williams=%+v dynamic=%+v", i, ws, ds)
		}
		if ws.UpFractal && ws.CenterHigh != ds.CenterHigh {
			t.Fatalf("bar %d center high: williams=%v dynamic=%v", i, ws.CenterHigh, ds.CenterHigh)
		}
		if ws.DownFractal && ws.CenterLow != ds.CenterLow {
			t.Fatalf("bar %d center low: williams=%v dynamic=%v", i, ws.CenterLow, ds.CenterLow)
		}
	}
}

func TestDynamicFractalSaveRestore(t *testing.T) {
	df := NewDynamicFractal(2, 2)
	for i := 0; i < 8; i++ {
		p := 100.0 + float64(i)
		df.UpdateCandle(p+1, p-1)
		df.SaveState()
	}

	beforeIdx := df.idx
	beforeCount := df.count
	df.idx = (df.idx + 1) % df.size
	df.count = 1

	df.RestoreState()
	if df.idx != beforeIdx || df.count != beforeCount {
		t.Fatalf("restore failed: idx=%d count=%d want idx=%d count=%d", df.idx, df.count, beforeIdx, beforeCount)
	}
}
