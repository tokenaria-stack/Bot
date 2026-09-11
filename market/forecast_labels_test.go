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

func TestDumpLabelSetWithFiner_ConvertsBothSeries(t *testing.T) {
	step15, err := data.IntervalDurationMs("15m")
	if err != nil {
		t.Fatal(err)
	}
	base, err := data.CurrentBarOpen(1_700_000_000_000, "15m")
	if err != nil {
		t.Fatal(err)
	}
	primary := make([]exchange.Kline, 10)
	canon := make([]forecast.CanonicalClosedBar, 10)
	for i := range primary {
		c := 100.0 + float64(i)
		ot := base + int64(i)*step15
		primary[i] = exchange.Kline{OpenTime: ot, Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1, CloseTime: ot + step15 - 1}
		canon[i] = forecast.CanonicalClosedBar{OpenTime: ot, Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1}
	}
	key := forecast.MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "15m"}
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
		DualHit:        forecast.DualHitResolveFinerHistory,
		FinerTimeframe: "1m",
	}, "labels:v1")
	if err != nil {
		t.Fatal(err)
	}
	fm := key
	fm.Timeframe = "1m"
	out := filepath.Join(dir, "t.labelset")
	if err := DumpLabelSetWithFiner(out, tapePath, spec, primary, nil, fm, nil); err != nil {
		t.Fatal(err)
	}
	lh, rows, ft, err := forecast.ReadLabelSet(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lh.FormatVersion != forecast.LabelSetFormatV2 || lh.FinerMarket != fm || len(rows) != 1 {
		t.Fatalf("finer dump %+v %+v %+v", lh, rows, ft)
	}
}

func TestDumpLabelSetFromTape2_ConvertsKlines(t *testing.T) {
	spec2, err := ResearchFeatureSpec2()
	if err != nil {
		t.Fatal(err)
	}
	step15, err := data.IntervalDurationMs("15m")
	if err != nil {
		t.Fatal(err)
	}
	base, err := data.CurrentBarOpen(1_700_000_000_000, "15m")
	if err != nil {
		t.Fatal(err)
	}
	primary := make([]exchange.Kline, 30)
	canon := make([]forecast.CanonicalClosedBar, 30)
	for i := range primary {
		c := 100.0
		ot := base + int64(i)*step15
		primary[i] = exchange.Kline{OpenTime: ot, Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1, CloseTime: ot + step15 - 1}
		canon[i] = forecast.CanonicalClosedBar{OpenTime: ot, Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1}
	}
	dummy := []forecast.CanonicalClosedBar{{OpenTime: 1, Open: 1, High: 2, Low: 0.5, Close: 1.2, Volume: 1}}
	sid, _ := spec2.Identity()
	pid, _ := spec2.Plan.Identity()
	fid, _ := spec2.Features.Identity()
	aid, _ := spec2.Analysis.Identity()
	tid, _ := spec2.Target.Identity()
	hdr := forecast.Tape2Header{
		FormatVersion: forecast.FeatureTapeFormatV2, SpecDigest: sid.Digest, PlanDigest: pid.Digest,
		FeaturesDigest: fid.Digest, AnalysisDigest: aid.Digest, TargetDigest: tid.Digest,
		Primary: spec2.Primary, HTF1h: spec2.HTF1h, HTF4h: spec2.HTF4h, FeatureIDs: forecast.FeatureSpec2IDs(),
		VectorLen: forecast.Spec2FeatureWidth, Q: spec2.Q, Demand: spec2.Demand,
		PrimarySource: forecast.SourceRangeDigest(spec2.Primary, canon[10:11]),
		HTF1hSource:   forecast.SourceRangeDigest(spec2.HTF1h, dummy),
		HTF4hSource:   forecast.SourceRangeDigest(spec2.HTF4h, dummy),
	}
	dir := t.TempDir()
	tapePath := filepath.Join(dir, "t.featuretape2")
	w, err := forecast.CreateTape2Writer(tapePath, hdr)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(canon[10].OpenTime, forecast.NotReady, forecast.ReasonPrimaryWarmup, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	fm := spec2.Primary
	fm.Timeframe = spec2.Target.FinerTimeframe
	out := filepath.Join(dir, "t.labelset")
	if err := DumpLabelSetFromTape2(out, tapePath, spec2, primary, nil, fm, nil); err != nil {
		t.Fatal(err)
	}
	lh, rows, _, err := forecast.ReadLabelSet(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lh.FormatVersion != forecast.LabelSetFormatV2 || lh.Market != spec2.Primary || len(rows) != 1 || rows[0].At != canon[10].OpenTime {
		t.Fatalf("tape2 dump %+v %+v", lh, rows)
	}
	if lh.TargetDigest != tid.Digest {
		t.Fatal("Target C")
	}
}
