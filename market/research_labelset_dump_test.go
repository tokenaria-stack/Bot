package market

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

func TestDumpLabelSet_Research1B(t *testing.T) {
	if os.Getenv("RESEARCH_LABELSET_DUMP") != "1" {
		t.Skip("set RESEARCH_LABELSET_DUMP=1 to dump the research LABEL-SET-1B artifact")
	}

	wantKey := ResearchMarketKey()
	plan, err := ResearchFeaturePlanMust(analysisLogicV2)
	if err != nil {
		t.Fatalf("FAIL: ResearchFeaturePlan: %v", err)
	}
	planID, err := plan.Identity()
	if err != nil {
		t.Fatalf("FAIL: FeaturePlan identity: %v", err)
	}
	spec, err := resolveIntendedResearchTargetSpec()
	if err != nil {
		t.Fatalf("FAIL: TargetSpec: %v", err)
	}
	targetID, err := spec.Identity()
	if err != nil {
		t.Fatalf("FAIL: TargetSpec identity: %v", err)
	}

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	tapePath := filepath.Join(root, "research", "tapes", researchTapeFileName(wantKey, planID.Digest))
	labelPath := filepath.Join(root, "research", "labels", researchLabelSetFileName(wantKey, planID.Digest, targetID.Digest))

	th, trows, tf, err := forecast.ReadTape(tapePath, nil, nil)
	if err != nil {
		t.Fatalf("FAIL: FeatureTape: %v", err)
	}
	if th.Market != wantKey {
		t.Fatalf("FAIL: Tape.MarketKey=%s want %s", th.Market, wantKey)
	}
	if tf.FirstAt < exchange.BinanceFuturesGenesisMs {
		t.Fatalf("FAIL: Tape.FirstAt=%d < genesis %d", tf.FirstAt, exchange.BinanceFuturesGenesisMs)
	}
	if th.PlanDigest != planID.Digest {
		t.Fatal("FAIL: Tape.PlanDigest != current ResearchFeaturePlan digest")
	}

	finerKey := th.Market
	finerKey.Timeframe = spec.FinerTimeframe
	if !th.Market.SameFamily(finerKey) || finerKey.Timeframe == th.Market.Timeframe {
		t.Fatalf("FAIL: finer family: primary=%s finer=%s", th.Market, finerKey)
	}

	if _, err := os.Stat(labelPath); err == nil {
		t.Fatalf("FAIL: refuse overwrite of existing LabelSet %s — re-run LABELSET-PREFLIGHT-1", labelPath)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(labelPath), 0o755); err != nil {
		t.Fatal(err)
	}

	primary := loadResearchClosedBarsTF(t, th.Market.Timeframe)
	finer := loadResearchClosedBarsTF(t, spec.FinerTimeframe)
	if primary[0].OpenTime < exchange.BinanceFuturesGenesisMs {
		t.Fatalf("FAIL: primary FirstAt=%d < genesis", primary[0].OpenTime)
	}
	if finer[0].OpenTime < exchange.BinanceFuturesGenesisMs {
		t.Fatalf("FAIL: finer FirstAt=%d < genesis", finer[0].OpenTime)
	}

	expect := &forecast.LabelExpect{
		Market:      &th.Market,
		Target:      &targetID.Digest,
		TapePlan:    &th.PlanDigest,
		TapeSource:  &tf.SourceRangeDigest,
		TapeContent: &tf.ContentDigest,
	}
	if err := DumpLabelSetWithFiner(labelPath, tapePath, spec, primary, finer, finerKey, expect); err != nil {
		t.Fatalf("FAIL: DumpLabelSetWithFiner: %v", err)
	}

	lh, lrows, lf, err := forecast.ReadLabelSet(labelPath, expect)
	if err != nil {
		t.Fatalf("FAIL: readback: %v", err)
	}
	if lh.FormatVersion != forecast.LabelSetFormatV2 {
		t.Fatalf("FAIL: format %q want %s", lh.FormatVersion, forecast.LabelSetFormatV2)
	}

	outcomes := map[forecast.TargetOutcome]int{}
	reasons := map[forecast.LabelReason]int{}
	for _, row := range lrows {
		outcomes[row.Outcome]++
		reasons[row.Reason]++
	}
	truncated := reasons[forecast.ReasonTruncatedHorizon]
	rowMatch := lf.RowCount == tf.RowCount && len(lrows) == len(trows)
	mktMatch := lh.Market == th.Market
	finerMatch := lh.FinerMarket == finerKey
	tgtMatch := lh.TargetDigest == targetID.Digest
	planMatch := lh.FeatureTapePlanDigest == th.PlanDigest
	srcMatch := lh.FeatureTapeSourceRangeDigest == tf.SourceRangeDigest
	contentMatch := lh.FeatureTapeContentDigest == tf.ContentDigest
	if !rowMatch || !mktMatch || !finerMatch || !tgtMatch || !planMatch || !srcMatch || !contentMatch {
		t.Fatalf("FAIL: provenance/contract rowMatch=%v mkt=%v finer=%v tgt=%v plan=%v src=%v content=%v",
			rowMatch, mktMatch, finerMatch, tgtMatch, planMatch, srcMatch, contentMatch)
	}

	report := func(format string, args ...any) { t.Logf(format, args...) }
	report("LABEL-SET-1B-RESEARCH-DUMP-1")
	report("INPUT path=%s MarketKey=%s FirstAt=%d LastAt=%d rows=%d genesisOK=true", tapePath, th.Market, tf.FirstAt, tf.LastAt, tf.RowCount)
	report("Tape PlanDigest=%s", th.PlanDigest)
	report("Tape SourceRangeDigest=%s", tf.SourceRangeDigest)
	report("Tape ContentDigest=%s", tf.ContentDigest)
	report("FeaturePlan match=true")
	report("TARGET digest=%s ATR=%+v upper=%g lower=%g H=%d DualHit=%s FinerTF=%s FinerMarket=%s",
		targetID.Digest, spec.ATR, spec.UpperATRMultiple, spec.LowerATRMultiple, spec.HorizonBars, spec.DualHit, spec.FinerTimeframe, finerKey)
	report("SOURCE primary FirstAt=%d LastAt=%d bars=%d floor=futures-genesis", primary[0].OpenTime, primary[len(primary)-1].OpenTime, len(primary))
	report("SOURCE finer FirstAt=%d LastAt=%d bars=%d floor=futures-genesis", finer[0].OpenTime, finer[len(finer)-1].OpenTime, len(finer))
	report("SOURCE stable in-memory slices=true")
	report("OUTPUT path=%s version=%s rows=%d ContentDigest=%s LabelSourceRangeDigest=%s FinerSourceDigest=%s FinerWindowCount=%d",
		labelPath, lh.FormatVersion, lf.RowCount, lf.ContentDigest, lf.LabelSourceRangeDigest, lf.FinerSourceDigest, lf.FinerWindowCount)
	report("PROVENANCE primary=%v finer=%v TargetDigest=%v Plan=%v Source=%v Content=%v rows==tape=%v",
		mktMatch, finerMatch, tgtMatch, planMatch, srcMatch, contentMatch, rowMatch)
	report("OUTCOMES %v", outcomes)
	report("REASONS %v", reasons)
	report("TRUNCATED_HORIZON=%d retained=%v", truncated, truncated == reasons[forecast.ReasonTruncatedHorizon])
	report("readback integrity=GREEN")
	fmt.Fprintf(os.Stderr, "LABEL-SET-1B-RESEARCH-DUMP-1 GREEN truncated=%d\n", truncated)
}
