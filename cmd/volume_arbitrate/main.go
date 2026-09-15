package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"trading_bot/exchange"
)

type conflictRow struct {
	Symbol    string  `json:"symbol"`
	Timeframe string  `json:"timeframe"`
	OpenTime  int64   `json:"open_time"`
	OldVolume float64 `json:"old_volume"`
	RESTv     float64 `json:"rest_v"`
	VisionV   float64 `json:"vision_v"`
	Class     string  `json:"class"`
	HasVision bool    `json:"has_vision"`
}

type arbRow struct {
	OpenTime      int64   `json:"open_time"`
	UTC           string  `json:"utc"`
	Stored        float64 `json:"stored"`
	REST15        float64 `json:"rest_15m_v"`
	Vision15      float64 `json:"vision_15m_v"`
	REST1mN       int     `json:"rest_1m_n"`
	REST1mSum     float64 `json:"rest_1m_sum_v"`
	Vision1mN     int     `json:"vision_1m_n"`
	Vision1mSum   float64 `json:"vision_1m_sum_v"`
	RESTQuote15   float64 `json:"rest_15m_q"`
	VisionQuote15 float64 `json:"vision_15m_q"`
	REST1mQuote   float64 `json:"rest_1m_sum_q"`
	Vision1mQuote float64 `json:"vision_1m_sum_q"`
	Support       string  `json:"support"`
}

func main() {
	src := "research/volume/dirty-row-reclassification.json"
	if v := os.Getenv("VOLUME_ARB_SRC"); v != "" {
		src = v
	}
	outDir := "research/volume"
	raw, err := os.ReadFile(src)
	if err != nil {
		log.Fatal(err)
	}
	var all []conflictRow
	if err := json.Unmarshal(raw, &all); err != nil {
		log.Fatal(err)
	}
	var ots []int64
	meta := map[int64]conflictRow{}
	for _, r := range all {
		if r.Symbol != "BTCUSDT" || r.Timeframe != "15m" || r.Class != "SOURCE_CONFLICT" {
			continue
		}
		ots = append(ots, r.OpenTime)
		meta[r.OpenTime] = r
	}
	sort.Slice(ots, func(i, j int) bool { return ots[i] < ots[j] })
	if len(ots) == 0 {
		log.Fatal("no futures 15m SOURCE_CONFLICT rows")
	}

	fut, err := exchange.NewBinanceExchange("", "", false)
	if err != nil {
		log.Fatal(err)
	}

	months := map[[2]int]struct{}{}
	for _, ot := range ots {
		tm := time.UnixMilli(ot).UTC()
		months[[2]int{tm.Year(), int(tm.Month())}] = struct{}{}
	}

	vis15 := map[int64]exchange.VolumeAuthority{}
	vis1 := map[int64]exchange.VolumeAuthority{}
	for ym := range months {
		a15, err := exchange.FetchVisionMonthAuthorities("BTCUSDT", "15m", ym[0], ym[1], false)
		if err != nil {
			log.Fatal("vision 15m ", ym, err)
		}
		for k, v := range a15 {
			vis15[k] = v
		}
		time.Sleep(200 * time.Millisecond)
		a1, err := exchange.FetchVisionMonthAuthorities("BTCUSDT", "1m", ym[0], ym[1], false)
		if err != nil {
			log.Fatal("vision 1m ", ym, err)
		}
		for k, v := range a1 {
			vis1[k] = v
		}
		time.Sleep(200 * time.Millisecond)
	}

	var rows []arbRow
	counts := map[string]int{}
	for _, ot := range ots {
		rest15p, err := fut.FetchVolumeAuthorityPage("BTCUSDT", "15m", ot, 2)
		if err != nil {
			log.Fatal(err)
		}
		var rest15 exchange.VolumeAuthority
		for _, a := range rest15p {
			if a.OpenTime == ot {
				rest15 = a
				break
			}
		}
		rest1p, err := fut.FetchVolumeAuthorityPage("BTCUSDT", "1m", ot, 20)
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(160 * time.Millisecond)
		var visKids, restKids []exchange.VolumeAuthority
		end := ot + 15*60_000
		for _, a := range rest1p {
			if a.OpenTime >= ot && a.OpenTime < end {
				restKids = append(restKids, a)
			}
		}
		for t := ot; t < end; t += 60_000 {
			if a, ok := vis1[t]; ok {
				visKids = append(visKids, a)
			}
		}
		rn, rsum, rq, _, _, _, _, _ := exchange.SumChildBase(ot, 15*60_000, restKids)
		vn, vsum, vq, _, _, _, _, _ := exchange.SumChildBase(ot, 15*60_000, visKids)
		v15 := vis15[ot]
		sup := exchange.Classify15mChildSupport(rest15.Base, v15.Base, rn, vn, rsum, vsum)
		counts[string(sup)]++
		m := meta[ot]
		visV := v15.Base
		if visV == 0 && m.HasVision {
			visV = m.VisionV
		}
		rows = append(rows, arbRow{
			OpenTime: ot, UTC: time.UnixMilli(ot).UTC().Format(time.RFC3339),
			Stored: m.OldVolume, REST15: rest15.Base, Vision15: visV,
			REST1mN: rn, REST1mSum: rsum, Vision1mN: vn, Vision1mSum: vsum,
			RESTQuote15: rest15.Quote, VisionQuote15: v15.Quote,
			REST1mQuote: rq, Vision1mQuote: vq, Support: string(sup),
		})
		fmt.Fprintf(os.Stderr, "ARB %s stored=%.6f rest15=%.6f vis15=%.6f rest1m=%d/%.6f vis1m=%d/%.6f %s\n",
			time.UnixMilli(ot).UTC().Format("2006-01-02 15:04"), m.OldVolume, rest15.Base, visV, rn, rsum, vn, vsum, sup)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	js, _ := json.MarshalIndent(rows, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "volume-source-arbitration-1.json"), js, 0o644)
	_ = os.WriteFile(filepath.Join(outDir, "VOLUME-SOURCE-ARBITRATION-1.txt"), []byte(formatArb(rows, counts)), 0o644)
	os.Stdout.WriteString(formatArb(rows, counts))
}

func formatArb(rows []arbRow, counts map[string]int) string {
	var b strings.Builder
	b.WriteString("VOLUME-SOURCE-ARBITRATION-1 — STEP 1 (1m reconstruction)\n")
	b.WriteString("=======================================================\n\n")
	b.WriteString("VOLUME-TRUTH-RECOVERY-1 is FROZEN. No general volume repair.\n")
	b.WriteString("Question: for futures 15m SOURCE_CONFLICT bars, do 1m children\n")
	b.WriteString("internally support REST 15m v and/or Vision 15m v?\n\n")
	fmt.Fprintf(&b, "N=%d futures 15m SOURCE_CONFLICT (not 27: ClassCounts mixed 7 spot 15m).\n\n", len(rows))
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("Support tallies:\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s %d\n", k, counts[k])
	}
	b.WriteString("\nPer bar:\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s stored=%.6f REST15=%.6f VIS15=%.6f REST1m[%d]=%.6f VIS1m[%d]=%.6f %s\n",
			r.UTC, r.Stored, r.REST15, r.Vision15, r.REST1mN, r.REST1mSum, r.Vision1mN, r.Vision1mSum, r.Support)
	}
	b.WriteString("\nNo SSOT policy yet. No TradingView. No Wozduh. No DB mutation.\n")
	b.WriteString("If BOTH_INTERNALLY_CONSISTENT dominates: Binance published two histories;\n")
	b.WriteString("archaeology will not pick a winner. Policy is a later step.\n")
	return b.String()
}
