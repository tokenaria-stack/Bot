package market

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

// TestLabelSetPreflight1 is a read-only refuse-closed gate.
// It never calls DumpLabelSet and never joins tape/label rows.
func TestLabelSetPreflight1(t *testing.T) {
	wantKey := ResearchMarketKey()
	if wantKey != (forecast.MarketKey{Venue: "BINANCE", Instrument: "BTCUSDT", Contract: "FUTURES_PERP", Timeframe: "15m"}) {
		t.Fatalf("FAIL: ResearchMarketKey()=%+v", wantKey)
	}

	plan, err := ResearchFeaturePlanMust(analysisLogicV2)
	if err != nil {
		t.Fatalf("FAIL: ResearchFeaturePlan: %v", err)
	}
	planID, err := plan.Identity()
	if err != nil {
		t.Fatalf("FAIL: FeaturePlan identity: %v", err)
	}

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	tapePath := filepath.Join(root, "research", "tapes", researchTapeFileName(wantKey, planID.Digest))

	spec, err := resolveIntendedResearchTargetSpec()
	if err != nil {
		t.Fatalf("FAIL: TargetSpec: %v", err)
	}
	targetID, err := spec.Identity()
	if err != nil {
		t.Fatalf("FAIL: TargetSpec identity: %v", err)
	}
	labelPath := filepath.Join(root, "research", "labels", researchLabelSetFileName(wantKey, planID.Digest, targetID.Digest))

	report := func(format string, args ...any) { t.Logf(format, args...) }

	hdr, rows, ft, err := forecast.ReadTape(tapePath, nil, nil)
	if err != nil {
		t.Fatalf("FAIL: FeatureTape integrity: path=%s err=%v", tapePath, err)
	}
	integrity := "GREEN"
	genesisOK := ft.FirstAt >= exchange.BinanceFuturesGenesisMs
	marketOK := hdr.Market == wantKey
	planOK := hdr.PlanDigest == planID.Digest

	report("LABELSET-PREFLIGHT-1")
	report("FEATURE TAPE")
	report("1. path=%s", tapePath)
	report("2. MarketKey=%s", hdr.Market)
	report("3. FirstAt=%d", ft.FirstAt)
	report("4. LastAt=%d", ft.LastAt)
	report("5. row count=%d", ft.RowCount)
	report("6. integrity=%s", integrity)
	report("7. futures-genesis-floor FirstAt>=BinanceFuturesGenesisMs (%d) = %v", exchange.BinanceFuturesGenesisMs, genesisOK)
	report("8. Tape PlanDigest=%s", hdr.PlanDigest)
	report("9. computed ResearchFeaturePlan digest=%s", planID.Digest)
	report("10. plan match=%v", planOK)
	report("11. SourceRangeDigest=%s", ft.SourceRangeDigest)
	report("12. ContentDigest=%s", ft.ContentDigest)
	_ = rows

	if !marketOK {
		t.Fatalf("FAIL: Tape.MarketKey=%s want exact ResearchMarketKey()=%s", hdr.Market, wantKey)
	}
	if !genesisOK {
		t.Fatalf("FAIL: Tape.FirstAt=%d < BinanceFuturesGenesisMs=%d", ft.FirstAt, exchange.BinanceFuturesGenesisMs)
	}
	if !planOK {
		t.Fatalf("FAIL: Tape.PlanDigest != current ResearchFeaturePlan digest")
	}

	finer := hdr.Market
	finer.Timeframe = spec.FinerTimeframe
	familyOK := hdr.Market.SameFamily(finer) && finer.Timeframe == spec.FinerTimeframe && finer.Timeframe != hdr.Market.Timeframe

	report("TARGET")
	report("13. TargetDigest=%s", targetID.Digest)
	report("14. family=%s", spec.Family)
	report("15. ATRSpec=%+v", spec.ATR)
	report("16. upper=%g", spec.UpperATRMultiple)
	report("17. lower=%g", spec.LowerATRMultiple)
	report("18. H=%d", spec.HorizonBars)
	report("19. DualHit=%s", spec.DualHit)
	report("20. FinerTimeframe=%s", spec.FinerTimeframe)
	report("21. resolved finer MarketKey=%s", finer)
	report("22. finer-family=%v", familyOK)

	if !familyOK {
		t.Fatalf("FAIL: finer MarketKey crosses family or is not pinned TF: primary=%s finer=%s", hdr.Market, finer)
	}

	_, err = os.Stat(labelPath)
	exists := err == nil
	report("LABELSET")
	report("23. explicit path=%s", labelPath)
	report("24. exists=%v", exists)

	if !exists {
		report("FINAL")
		report("32. verdict=MISS")
		report("33. next: generate exactly one LabelSet using frozen LABEL-SET-1B against this exact canonical tape")
		fmt.Fprintf(os.Stderr, "LABELSET-PREFLIGHT-1 MISS\n")
		return
	}

	lh, lrows, lf, err := forecast.ReadLabelSet(labelPath, nil)
	if err != nil {
		t.Fatalf("FAIL: LabelSet integrity: %v", err)
	}
	planMatch := lh.FeatureTapePlanDigest == hdr.PlanDigest
	srcMatch := lh.FeatureTapeSourceRangeDigest == ft.SourceRangeDigest
	contentMatch := lh.FeatureTapeContentDigest == ft.ContentDigest
	mktMatch := lh.Market == hdr.Market
	tgtMatch := lh.TargetDigest == targetID.Digest

	report("25. LabelSet integrity=GREEN")
	report("26. rows=%d FirstAt=%d LastAt=%d", lf.RowCount, lf.FirstAt, lf.LastAt)
	report("27. FeatureTapePlanDigest match=%v", planMatch)
	report("28. FeatureTapeSourceRangeDigest match=%v", srcMatch)
	report("29. FeatureTapeContentDigest match=%v", contentMatch)
	report("30. exact PrimaryMarketKey match=%v", mktMatch)
	report("31. TargetDigest match=%v", tgtMatch)
	_ = lrows

	verdict := "MATCH"
	next := "RESEARCH-DATASET-1 lockstep join (not this step)"
	if !planMatch || !srcMatch || !contentMatch || !mktMatch || !tgtMatch {
		verdict = "STALE"
		next = "generate exactly one LabelSet using frozen LABEL-SET-1B against this exact canonical tape"
	}
	report("FINAL")
	report("32. verdict=%s", verdict)
	report("33. next=%s", next)
	fmt.Fprintf(os.Stderr, "LABELSET-PREFLIGHT-1 %s\n", verdict)
}

func resolveIntendedResearchTargetSpec() (forecast.TargetSpec, error) {
	return forecast.ResolveTargetSpec("research-15m-1m", forecast.TargetSpecDraft{
		HorizonBars:      24,
		UpperATRMultiple: 1.5,
		LowerATRMultiple: 1.0,
		ATRPeriod:        14,
		DualHit:          forecast.DualHitResolveFinerHistory,
		FinerTimeframe:   "1m",
	}, "labels:v1")
}

func researchLabelSetFileName(key forecast.MarketKey, plan, target forecast.Digest) string {
	return key.Venue + "_" + key.Instrument + "_" + key.Contract + "_" + key.Timeframe +
		"_plan-" + plan.Short() + "_target-" + target.Short() + ".labelset"
}
