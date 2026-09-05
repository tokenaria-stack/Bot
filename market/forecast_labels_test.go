package market

import (
	"path/filepath"
	"testing"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/forecast"
)

func TestDumpLabelSet_ConvertsKlines(t *testing.T) {
	step, err := data.IntervalDurationMs("1m")
	if err != nil {
		t.Fatal(err)
	}
	base, err := data.CurrentBarOpen(1_700_000_000_000, "1m")
	if err != nil {
		t.Fatal(err)
	}
	klines := make([]exchange.Kline, 10)
	canon := make([]forecast.CanonicalClosedBar, 10)
	for i := range klines {
		c := 100.0 + float64(i)
		ot := base + int64(i)*step
		klines[i] = exchange.Kline{OpenTime: ot, Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1, CloseTime: ot + step - 1}
		canon[i] = forecast.CanonicalClosedBar{OpenTime: ot, Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1}
	}
	key := forecast.MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "1m"}
	hdr := forecast.TapeHeader{
		FormatVersion: forecast.FeatureTapeFormatV1,
		Market:        key,
		PlanDigest:    forecast.Digest{0xab},
		FeatureIDs:    []forecast.FeatureID{forecast.FeatureRSXValue, forecast.FeatureRSXSignal, forecast.FeatureTVBullPresent, forecast.FeatureTVBullAge},
		VectorLen:     4,
	}
	dir := t.TempDir()
	tapePath := filepath.Join(dir, "t.featuretape")
	w, err := forecast.CreateTapeWriter(tapePath, hdr)
	if err != nil {
		t.Fatal(err)
	}
	tapeBars := canon[4:5]
	if err := w.WriteRow(tapeBars[0].OpenTime, forecast.NotReady, nil); err != nil {
		t.Fatal(err)
	}
	if err := w.Finish(forecast.SourceRangeDigest(key, tapeBars)); err != nil {
		t.Fatal(err)
	}
	spec, err := forecast.ResolveTargetSpec("t", forecast.TargetSpecDraft{
		HorizonBars: 3, UpperATRMultiple: 2, LowerATRMultiple: 2, ATRPeriod: 2,
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "t.labelset")
	if err := DumpLabelSet(out, tapePath, spec, klines, nil); err != nil {
		t.Fatal(err)
	}
	lh, rows, _, err := forecast.ReadLabelSet(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lh.Market != key || len(rows) != 1 || rows[0].At != tapeBars[0].OpenTime {
		t.Fatalf("host dump %+v %+v", lh, rows)
	}
}
