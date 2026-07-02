package strategy

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// btTradeEntryAudit captures why a backtest position was opened.
type btTradeEntryAudit struct {
	signalBarIndex  int
	signalKind      string
	entryReason     string
	strategySource  string
	activeFactors   []string
	factorsSnapshot []string
	score           float64
}

func winningActionForSide(side string) ActionType {
	if side == "SELL" {
		return SellAction
	}
	return BuyAction
}

func buildTradeEntryAudit(decision ScoreDecision, side string) (entryReason string, factorsSnapshot []string, score float64) {
	want := winningActionForSide(side)
	if side == "SELL" {
		score = float64(decision.ShortScore)
	} else {
		score = float64(decision.LongScore)
	}

	type scored struct {
		label string
		score int
	}
	items := make([]scored, 0, len(decision.Factors))
	for _, factor := range decision.Factors {
		if factor.Direction != want || factor.Score <= 0 {
			continue
		}
		name := strings.TrimSpace(factor.Name)
		if name == "" {
			continue
		}
		items = append(items, scored{
			label: fmt.Sprintf("%s (+%d)", name, factor.Score),
			score: factor.Score,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].label < items[j].label
	})

	factorsSnapshot = make([]string, len(items))
	names := make([]string, len(items))
	for i, item := range items {
		factorsSnapshot[i] = item.label
		names[i] = strings.Split(item.label, " (+")[0]
	}
	entryReason = strings.Join(names, " + ")
	if entryReason == "" {
		entryReason = "No scored factors"
	}
	return entryReason, factorsSnapshot, score
}

func auditFromDecision(decision ScoreDecision, side string, barIndex int, signalKind string) btTradeEntryAudit {
	entryReason, factorsSnapshot, score := buildTradeEntryAudit(decision, side)
	return btTradeEntryAudit{
		signalBarIndex:  barIndex,
		signalKind:      signalKind,
		entryReason:     entryReason,
		strategySource:  StrategySourceForSide(decision, side),
		activeFactors:   ActiveFactorsForSide(decision, side),
		factorsSnapshot: factorsSnapshot,
		score:           score,
	}
}

func snapshotHasRSXFactor(factors []string) bool {
	for _, f := range factors {
		upper := strings.ToUpper(f)
		if strings.Contains(upper, "RSX") {
			return true
		}
	}
	return false
}

func rsxBatchMarkerImpliesBuy(label string) bool {
	return label == "L" || label == "LL"
}

func rsxBatchMarkerImpliesSell(label string) bool {
	return label == "S" || label == "SS"
}

func (e *BacktestEngine) logRSXSignalIgnored(barIndex int, markerLabel string, decision ScoreDecision) {
	if !IsRSXTradingMarker(markerLabel) {
		return
	}

	longTh := e.cfg.LongThreshold
	shortTh := e.cfg.ShortThreshold
	if longTh <= 0 {
		longTh = DefaultScoreThreshold
	}
	if shortTh <= 0 {
		shortTh = DefaultScoreThreshold
	}

	switch {
	case rsxBatchMarkerImpliesBuy(markerLabel):
		if decision.FinalAction == BuyAction {
			return
		}
		log.Printf("[Info] RSX Signal Ignored: bar %d annotation %s | Long score %d < threshold %d (FinalAction=%s vetoed=%v)",
			barIndex, markerLabel, decision.LongScore, longTh, decision.FinalAction, decision.IsVetoed)
	case rsxBatchMarkerImpliesSell(markerLabel):
		if decision.FinalAction == SellAction {
			return
		}
		log.Printf("[Info] RSX Signal Ignored: bar %d annotation %s | Short score %d < threshold %d (FinalAction=%s vetoed=%v)",
			barIndex, markerLabel, decision.ShortScore, shortTh, decision.FinalAction, decision.IsVetoed)
	}
}

func logTradeEntryExecution(entryTime int64, side string, audit btTradeEntryAudit) {
	timeStr := fmt.Sprintf("%d", entryTime)
	if entryTime > 0 {
		timeStr = fmt.Sprintf("%s (%d)", time.Unix(entryTime, 0).UTC().Format("2006-01-02 15:04"), entryTime)
	}
	log.Printf("[TRADE SOURCE] Bar %d: Side=%s, Score=%.2f, Factors=%v",
		audit.signalBarIndex, side, audit.score, audit.activeFactors)
	log.Printf("🔥 [TRADE ENTRY] Time: %s | Side: %s | Score: %.2f | Kind: %s | Source: %s | Reason: %s | Factors: %v",
		timeStr, side, audit.score, audit.signalKind, audit.strategySource, audit.entryReason, audit.factorsSnapshot)
	if !snapshotHasRSXFactor(audit.factorsSnapshot) {
		log.Printf("[Info] Non-RSX entry at signal bar %d — chart RSX marker may differ from execution factors",
			audit.signalBarIndex)
	}
}
