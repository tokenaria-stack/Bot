package market

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trading_bot/core/nodes"
	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/forecast"
	"trading_bot/indicators"
)

func TestResearchFeaturePlan_AnalysisV2DiffersFromV1(t *testing.T) {
	t.Parallel()
	v1, err := ResearchFeaturePlan("analysis:v1")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := ResearchFeaturePlan(analysisLogicV2)
	if err != nil {
		t.Fatal(err)
	}
	id1, err := v1.Identity()
	if err != nil {
		t.Fatal(err)
	}
	id2, err := v2.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if id1.Digest == id2.Digest {
		t.Fatal("FeaturePlan digest must change when AnalysisLogicVersion changes")
	}
	if v1.Features.Digest != v2.Features.Digest {
		t.Fatal("FeatureRecipe identity must be unchanged")
	}
	if v1.Schema[0] != forecast.FeatureRSXValue || v1.Schema[3] != forecast.FeatureTVBullAge {
		t.Fatalf("schema %+v", v1.Schema)
	}
}

func TestDumpFeatureTape_AnalysisV2ResearchArchive(t *testing.T) {
	if os.Getenv("FEATURE_TAPE_RSX_REGEN") != "1" {
		t.Skip("set FEATURE_TAPE_RSX_REGEN=1 to dump the analysis:v2 research FeatureTape")
	}
	key := ResearchMarketKey()
	settings := ResearchRSXSettings()
	plan, err := ResearchFeaturePlanMust(analysisLogicV2)
	if err != nil {
		t.Fatal(err)
	}
	bars := loadResearchClosedBars(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "research", "tapes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	planID, err := plan.Identity()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, researchTapeFileName(key, planID.Digest))
	pass2 := path + ".pass2"
	_ = os.Remove(pass2)
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("refuse overwrite of existing tape %s — move it aside first", path)
	}

	t.Logf("dumping %d bars → %s", len(bars), path)
	if err := DumpFeatureTape(path, key, plan, settings, bars); err != nil {
		t.Fatal(err)
	}
	if err := DumpFeatureTape(pass2, key, plan, settings, bars); err != nil {
		t.Fatal(err)
	}
	hdr, rows, ft, err := forecast.ReadTape(path, &key, &planID.Digest)
	if err != nil {
		t.Fatal(err)
	}
	hdr2, _, ft2, err := forecast.ReadTape(pass2, &key, &planID.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.PlanDigest != hdr2.PlanDigest || ft.ContentDigest != ft2.ContentDigest || ft.SourceRangeDigest != ft2.SourceRangeDigest || ft.RowCount != ft2.RowCount {
		t.Fatalf("second generation mismatch content=%s/%s source=%s/%s rows=%d/%d",
			ft.ContentDigest, ft2.ContentDigest, ft.SourceRangeDigest, ft2.SourceRangeDigest, ft.RowCount, ft2.RowCount)
	}
	if err := os.Remove(pass2); err != nil {
		t.Fatal(err)
	}

	readyN, notReadyN := 0, 0
	var lastAt int64
	for i, row := range rows {
		if i > 0 && row.At <= lastAt {
			t.Fatalf("At not strictly increasing at %d", i)
		}
		lastAt = row.At
		if row.Ready {
			readyN++
			if len(row.Values) != 4 {
				t.Fatalf("vector len %d at %d", len(row.Values), row.At)
			}
			for j, v := range row.Values {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("nonfinite values[%d] at %d", j, row.At)
				}
			}
		} else {
			notReadyN++
			if row.Values != nil {
				t.Fatalf("NotReady values at %d", row.At)
			}
		}
	}
	if ft.RowCount != len(rows) || ft.RowCount != len(bars) {
		t.Fatalf("row count footer=%d rows=%d bars=%d", ft.RowCount, len(rows), len(bars))
	}

	const (
		bearAnchor  int64 = 1788630300000
		bearConfirm int64 = 1788631200000
	)
	verifyKnownBear(t, bars, settings, bearAnchor, bearConfirm)
	verifyTapeNoBullLeakBeforeConfirm(t, rows)
	verifyFillSample(t, key, plan, settings, bars, rows)

	writeRegenReport(t, path, key, plan, settings, bars, hdr, rows, ft, readyN, notReadyN)
}

func researchTapeFileName(key forecast.MarketKey, plan forecast.Digest) string {
	return key.Venue + "_" + key.Instrument + "_" + key.Contract + "_" + key.Timeframe + "_plan-" + plan.Short() + ".featuretape"
}

func loadResearchClosedBars(t *testing.T) []exchange.Kline {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	data.SetDBPath(filepath.Join(root, "history.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	candles, err := exchange.LoadContinuousContractFromDB("BTCUSDT", "15m", exchange.BinanceSpotGenesisMs, now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) < 2000 {
		t.Fatalf("archive too short: %d", len(candles))
	}
	out := make([]exchange.Kline, 0, len(candles))
	for _, c := range candles {
		out = append(out, exchange.Kline{
			OpenTime: c.OpenTime, CloseTime: c.CloseTime,
			Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume,
		})
	}
	last := out[len(out)-1]
	if data.IsFormingCloseTime(last.CloseTime, now) {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		t.Fatal("no closed bars")
	}
	return out
}

func verifyKnownBear(t *testing.T, bars []exchange.Kline, settings RSXSettings, anchor, confirm int64) {
	t.Helper()
	idx := -1
	for i, k := range bars {
		if k.OpenTime == confirm {
			idx = i
			break
		}
	}
	if idx < 2 {
		t.Fatalf("confirm bar %d not in archive", confirm)
	}
	start := idx - 1023
	if start < 0 {
		start = 0
	}
	frame := NewFrame(nil, "15m", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	frame.SetRSXDemand(nodes.NeedRSXCore | nodes.NeedRSTV)
	for _, k := range bars[start : idx+1] {
		frame.UpdateKlineTick(k, true)
	}
	found := false
	for _, ev := range frame.RSTVFactsSnapshot() {
		if ev.Source == indicators.FactSourceRSXTVDiv && ev.Direction == indicators.FactDirBearish &&
			ev.AnchorAt == anchor && ev.ConfirmedAt == confirm {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("canonical rsx_tv_div bear missing on 1024-cap replay AnchorAt=%d ConfirmedAt=%d", anchor, confirm)
	}
}

func verifyTapeNoBullLeakBeforeConfirm(t *testing.T, rows []forecast.TapeRow) {
	t.Helper()
	var firstBullAt int64
	for _, row := range rows {
		if !row.Ready || len(row.Values) < 4 {
			continue
		}
		if row.Values[2] == 1 && row.Values[3] == 0 {
			firstBullAt = row.At
			break
		}
	}
	if firstBullAt == 0 {
		t.Log("no Ready tv_bull_present=1 age=0 row in this archive window (ok if no bull in FeatureHistoryBars of a Ready tip)")
		return
	}
	for _, row := range rows {
		if row.At >= firstBullAt {
			break
		}
		if !row.Ready {
			continue
		}
		if row.Values[2] != 0 {
			t.Fatalf("tv_bull_present leaked before first age=0 at %d (row %d)", firstBullAt, row.At)
		}
	}
}

func verifyFillSample(t *testing.T, key forecast.MarketKey, plan forecast.FeaturePlan, settings RSXSettings, bars []exchange.Kline, rows []forecast.TapeRow) {
	t.Helper()
	const sample = 32
	if len(bars) < sample || len(rows) != len(bars) {
		return
	}
	frame := NewFrame(nil, key.Timeframe, ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	ev, err := BindFeatureEvaluator(frame, plan)
	if err != nil {
		t.Fatal(err)
	}
	defer ev.Unbind()
	start := len(bars) - sample
	for i := 0; i < start; i++ {
		frame.UpdateKlineTick(bars[i], true)
	}
	for i := start; i < len(bars); i++ {
		frame.UpdateKlineTick(bars[i], true)
		ready, dst, err := ev.FillOwned()
		if err != nil {
			t.Fatal(err)
		}
		row := rows[i]
		if row.Ready != ready {
			t.Fatalf("Ready mismatch at %d tape=%v fill=%v", row.At, row.Ready, ready)
		}
		if !ready {
			continue
		}
		if math.Float64bits(row.Values[0]) != math.Float64bits(dst[0]) ||
			math.Float64bits(row.Values[1]) != math.Float64bits(dst[1]) {
			t.Fatalf("rsx_value/signal mismatch at %d tape=%v fill=%v", row.At, row.Values[:2], dst[:2])
		}
	}
}

func writeRegenReport(t *testing.T, path string, key forecast.MarketKey, plan forecast.FeaturePlan, settings RSXSettings, bars []exchange.Kline, hdr forecast.TapeHeader, rows []forecast.TapeRow, ft forecast.TapeFooter, readyN, notReadyN int) {
	t.Helper()
	a, err := AnalysisRecipeFromRSXSettings(settings, true, false, analysisLogicV2)
	if err != nil {
		t.Fatal(err)
	}
	aid, _ := a.Identity()
	fid := plan.Features
	pid, _ := plan.Identity()
	v1, _ := ResearchFeaturePlan("analysis:v1")
	v1id, _ := v1.Identity()
	t.Logf("FEATURE-TAPE-RSX-REGEN-1")
	t.Logf("MarketKey=%+v", key)
	t.Logf("source FirstAt=%d LastAt=%d bars=%d", ft.FirstAt, ft.LastAt, len(bars))
	t.Logf("AnalysisLogicVersion=%s AnalysisDigest=%s", a.Logic, aid.Digest)
	t.Logf("FeatureRecipeDigest=%s FeatureLogic=%v Schema=%v", fid.Digest, fid.Logic, plan.Schema)
	t.Logf("PlanDigest=%s v1PlanDigest=%s differ=%v", pid.Digest, v1id.Digest, pid.Digest != v1id.Digest)
	t.Logf("Ready=%d NotReady=%d SourceRangeDigest=%s ContentDigest=%s", readyN, notReadyN, ft.SourceRangeDigest, ft.ContentDigest)
	t.Logf("path=%s", path)
	_ = rows
}
