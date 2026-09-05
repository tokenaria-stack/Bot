package market

import (
	"math"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
)

func TestDumpFeatureTape_MatchesDirectFill(t *testing.T) {
	settings := tape1ASettings()
	plan := tape1APlan(t, settings)
	klines := tape1ATVKlines(80)
	key := forecast.MarketKey{
		Venue:      "BINANCE",
		Instrument: "BTCUSDT",
		Contract:   "FUTURES_PERP",
		Timeframe:  "1m",
	}

	direct := NewFrame(nil, key.Timeframe, ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	direct.ApplyBacktestRSXConfig(settings)
	ev, err := BindFeatureEvaluator(direct, plan)
	if err != nil {
		t.Fatal(err)
	}
	defer ev.Unbind()
	want := make([]featureObs, 0, len(klines))
	for _, k := range klines {
		direct.UpdateKlineTick(k, true)
		want = append(want, observeFill(t, ev))
	}

	path := filepath.Join(t.TempDir(), "t.featuretape")
	if err := DumpFeatureTape(path, key, plan, settings, klines); err != nil {
		t.Fatal(err)
	}
	planID, err := plan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	hdr, rows, ft, err := forecast.ReadTape(path, &key, &planID.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.VectorLen != 4 || ft.RowCount != len(klines) {
		t.Fatalf("header/footer %+v %+v", hdr, ft)
	}
	if len(rows) != len(want) {
		t.Fatalf("rows %d want %d", len(rows), len(want))
	}
	canon := make([]forecast.CanonicalClosedBar, len(klines))
	for i, k := range klines {
		canon[i] = forecast.CanonicalClosedBar{
			OpenTime: k.OpenTime, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, Volume: k.Volume,
		}
	}
	if ft.SourceRangeDigest != forecast.SourceRangeDigest(key, canon) {
		t.Fatal("SourceRangeDigest mismatch vs hashed fixture bars")
	}
	sawReady := false
	sawNotReady := false
	sawPresentZero := false
	for i := range rows {
		if rows[i].At != want[i].At || rows[i].At != klines[i].OpenTime {
			t.Fatalf("At mismatch i=%d row=%d want=%d src=%d", i, rows[i].At, want[i].At, klines[i].OpenTime)
		}
		if rows[i].Ready != want[i].Ready {
			t.Fatalf("Ready mismatch i=%d", i)
		}
		if !rows[i].Ready {
			sawNotReady = true
			if rows[i].Values != nil {
				t.Fatalf("NotReady values present i=%d", i)
			}
			continue
		}
		sawReady = true
		if len(rows[i].Values) != 4 {
			t.Fatalf("values len i=%d", i)
		}
		for j := 0; j < 4; j++ {
			if math.Float64bits(rows[i].Values[j]) != math.Float64bits(want[i].Vals[j]) {
				t.Fatalf("bits i=%d j=%d", i, j)
			}
		}
		if rows[i].Values[2] == 0 {
			sawPresentZero = true
		}
	}
	if !sawReady {
		t.Fatal("expected at least one Ready row")
	}
	if !sawNotReady && !sawPresentZero {
		// Fixture may be Ready from bar 0 (Jurik finite). present=0 is still required as distinct encoding.
		t.Log("no NotReady in this fixture; Ready+present=0 coverage is in forecast package tests")
	}
}
