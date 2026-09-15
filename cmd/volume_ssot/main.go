package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

type step1Row struct {
	OpenTime    int64   `json:"open_time"`
	UTC         string  `json:"utc"`
	Stored      float64 `json:"stored"`
	REST15      float64 `json:"rest_15m_v"`
	Vision15    float64 `json:"vision_15m_v"`
	REST1mN     int     `json:"rest_1m_n"`
	REST1mSum   float64 `json:"rest_1m_sum_v"`
	Vision1mN   int     `json:"vision_1m_n"`
	Vision1mSum float64 `json:"vision_1m_sum_v"`
	Support     string  `json:"support"`
}

type manifestRow struct {
	Symbol                    string  `json:"symbol"`
	Timeframe                 string  `json:"timeframe"`
	OpenTime                  int64   `json:"open_time"`
	UTC                       string  `json:"utc"`
	StoredVolume              float64 `json:"stored_volume"`
	Step1Stored               float64 `json:"step1_stored"`
	REST15mV                  float64 `json:"rest_15m_v"`
	REST1mSum                 float64 `json:"rest_1m_sum"`
	Vision15mV                float64 `json:"vision_15m_v"`
	Vision1mSum               float64 `json:"vision_1m_sum"`
	RESTChildCount            int     `json:"rest_child_count"`
	RESTOHLCIdentityOK        bool    `json:"rest_ohlc_identity_ok"`
	RESTIntervalOK            bool    `json:"rest_interval_ok"`
	OpenMismatch              bool    `json:"open_mismatch"`
	Step1Classification       string  `json:"step1_classification"`
	Step2PolicyClassification string  `json:"step2_policy_classification"`
	CanonicalVolume           float64 `json:"canonical_volume"`
	CanonicalSource           string  `json:"canonical_source"`
	MutationRequired          bool    `json:"mutation_required"`
	ParentOpen                float64 `json:"parent_open"`
	ParentHigh                float64 `json:"parent_high"`
	ParentLow                 float64 `json:"parent_low"`
	ParentClose               float64 `json:"parent_close"`
	AggOpen                   float64 `json:"agg_open"`
	AggHigh                   float64 `json:"agg_high"`
	AggLow                    float64 `json:"agg_low"`
	AggClose                  float64 `json:"agg_close"`
	AfterVolume               float64 `json:"after_volume,omitempty"`
	Assigned                  bool    `json:"assigned"`
}

type seriesCert struct {
	Interval      string `json:"interval"`
	N             int    `json:"n"`
	MinOpenTime   int64  `json:"min_open_time"`
	MaxOpenTime   int64  `json:"max_open_time"`
	ContentDigest string `json:"content_digest"`
}

type liveSample struct {
	Interval string  `json:"interval"`
	OpenTime int64   `json:"open_time"`
	Stored   float64 `json:"stored"`
	RESTv    float64 `json:"rest_v"`
	Match    bool    `json:"match"`
}

type step2Artifact struct {
	TruthVersion      string       `json:"truth_version"`
	Market            string       `json:"market"`
	Symbol            string       `json:"symbol"`
	Semantic          string       `json:"semantic"`
	CanonicalPolicy   string       `json:"canonical_source_policy"`
	PolicyDigest      string       `json:"policy_digest"`
	CertifiedTFs      []string     `json:"certified_timeframe_set"`
	Series            []seriesCert `json:"series"`
	BundleDigest      string       `json:"bundle_digest"`
	ArbitrationDigest string       `json:"arbitration_set_digest"`
	CertifiedAtUTC    string       `json:"certified_at_utc"`
	Verdict           string       `json:"verdict"`
	LiveSamples       []liveSample `json:"live_samples"`
	DBMutated         int          `json:"db_mutated"`
	Writers           string       `json:"history_db_writers"`
	Note              string       `json:"note"`
}

func main() {
	outDir := "research/volume"
	if v := os.Getenv("VOLUME_SSOT_OUT"); v != "" {
		outDir = v
	}
	src := filepath.Join(outDir, "volume-source-arbitration-1.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		log.Fatal(err)
	}
	var step1 []step1Row
	if err := json.Unmarshal(raw, &step1); err != nil {
		log.Fatal(err)
	}
	sort.Slice(step1, func(i, j int) bool { return step1[i].OpenTime < step1[j].OpenTime })
	if len(step1) != 20 {
		log.Fatalf("arbitration set N=%d want 20", len(step1))
	}
	seen := map[int64]struct{}{}
	for _, r := range step1 {
		if _, ok := seen[r.OpenTime]; ok {
			log.Fatal("duplicate open_time in step1")
		}
		seen[r.OpenTime] = struct{}{}
	}

	fut, err := exchange.NewBinanceExchange("", "", false)
	if err != nil {
		log.Fatal(err)
	}

	var man []manifestRow
	unresolved := 0
	for _, s := range step1 {
		rest15p, err := fut.FetchVolumeAuthorityPage("BTCUSDT", "15m", s.OpenTime, 2)
		if err != nil {
			log.Fatal(s.OpenTime, err)
		}
		var parent exchange.VolumeAuthority
		for _, a := range rest15p {
			if a.OpenTime == s.OpenTime {
				parent = a
				break
			}
		}
		if parent.OpenTime == 0 {
			log.Fatalf("REST 15m missing %d", s.OpenTime)
		}
		rest1p, err := fut.FetchVolumeAuthorityPage("BTCUSDT", "1m", s.OpenTime, 20)
		if err != nil {
			log.Fatal(s.OpenTime, err)
		}
		time.Sleep(160 * time.Millisecond)
		var kids []exchange.VolumeAuthority
		end := s.OpenTime + 15*60_000
		for _, a := range rest1p {
			if a.OpenTime >= s.OpenTime && a.OpenTime < end {
				kids = append(kids, a)
			}
		}
		dec := exchange.ResolveFuturesRESTFamily15m(parent, kids)
		aligned, aerr := exchange.AlignExact1mChildren(s.OpenTime, kids)
		var aggO, aggH, aggL, aggC, restSum float64
		nKids := len(kids)
		ohlcOK := false
		intervalOK := false
		if aerr == nil {
			restSum, aggO, aggH, aggL, aggC = exchange.AggregateAligned1m(aligned)
			ohlcOK = exchange.REST15mOHLCIdentity(parent, aggO, aggH, aggL, aggC)
			intervalOK, _ = exchange.REST15mIntervalOK(parent, aggO, aggH, aggL, aggC, exchange.VolumeFloatEqual(parent.Base, restSum))
			nKids = 15
		} else {
			n, sum, _, _, _, _, _, _ := exchange.SumChildBase(s.OpenTime, 15*60_000, kids)
			nKids, restSum = n, sum
		}
		stored := s.Stored
		row := manifestRow{
			Symbol: "BTCUSDT", Timeframe: "15m", OpenTime: s.OpenTime, UTC: s.UTC,
			StoredVolume: stored, Step1Stored: s.Stored, REST15mV: parent.Base, REST1mSum: restSum,
			Vision15mV: s.Vision15, Vision1mSum: s.Vision1mSum,
			RESTChildCount: nKids, RESTOHLCIdentityOK: ohlcOK, RESTIntervalOK: intervalOK,
			OpenMismatch:        dec.OpenMismatch,
			Step1Classification: s.Support, Step2PolicyClassification: dec.PolicyClass,
			CanonicalVolume: dec.Volume, CanonicalSource: dec.Source,
			ParentOpen: parent.Open, ParentHigh: parent.High, ParentLow: parent.Low, ParentClose: parent.Close,
			AggOpen: aggO, AggHigh: aggH, AggLow: aggL, AggClose: aggC,
		}
		if dec.Source == exchange.CanonicalUnresolved {
			unresolved++
			row.CanonicalVolume = 0
			row.MutationRequired = false
		} else {
			row.MutationRequired = !exchange.VolumeFloatEqual(stored, dec.Volume)
		}
		man = append(man, row)
		fmt.Fprintf(os.Stderr, "STEP2 %s src=%s class=%s stored=%.6f canon=%.6f ohlc=%v kids=%d mut=%v\n",
			s.UTC, dec.Source, dec.PolicyClass, stored, row.CanonicalVolume, ohlcOK, nKids, row.MutationRequired)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	ro0, err := data.OpenReadOnlyHistory("history.db")
	if err != nil {
		log.Fatal(err)
	}
	for i := range man {
		if man[i].CanonicalSource == exchange.CanonicalUnresolved {
			continue
		}
		var vol float64
		err := ro0.QueryRow(
			`SELECT volume FROM historical_klines WHERE symbol = ? AND interval = ? AND open_time = ?`,
			"BTCUSDT", "15m", man[i].OpenTime).Scan(&vol)
		if err != nil {
			log.Fatal("stored volume", man[i].OpenTime, err)
		}
		man[i].StoredVolume = vol
		man[i].MutationRequired = !exchange.VolumeFloatEqual(vol, man[i].CanonicalVolume)
		fmt.Fprintf(os.Stderr, "STORED %s sqlite=%.6f canon=%.6f mut=%v\n",
			man[i].UTC, vol, man[i].CanonicalVolume, man[i].MutationRequired)
	}
	_ = ro0.Close()

	mjs, _ := json.MarshalIndent(man, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "volume-source-arbitration-1-step2-manifest.json"), mjs, 0o644); err != nil {
		log.Fatal(err)
	}

	writers := historyWriters()
	needMut := 0
	for _, r := range man {
		if r.MutationRequired {
			needMut++
		}
	}
	verdict := ""
	mutated := 0
	if unresolved > 0 {
		verdict = "VOLUME_SSOT_BLOCKED_REST_FAMILY_UNRESOLVED"
	}
	if needMut > 0 && writersBusy(writers) && os.Getenv("VOLUME_SSOT_APPLY") != "1" {
		if verdict == "" {
			verdict = "VOLUME_SSOT_REPAIR_READY_NEEDS_USER_STOP"
		}
	} else if needMut > 0 || unresolved == 0 {
		if err := data.InitDB(); err != nil {
			log.Fatal(err)
		}
		for i := range man {
			r := &man[i]
			if r.CanonicalSource == exchange.CanonicalUnresolved {
				continue
			}
			if !r.MutationRequired {
				got, err := data.LoadKlines("BTCUSDT", "15m", r.OpenTime, r.OpenTime+1, 0)
				if err != nil || len(got) != 1 {
					log.Fatal("reread", r.OpenTime, err)
				}
				r.AfterVolume = got[0].Volume
				continue
			}
			if err := data.AssignCanonicalVolume("BTCUSDT", "15m", r.OpenTime, r.CanonicalVolume); err != nil {
				log.Fatal(err)
			}
			got, err := data.LoadKlines("BTCUSDT", "15m", r.OpenTime, r.OpenTime+1, 0)
			if err != nil || len(got) != 1 || !exchange.VolumeFloatEqual(got[0].Volume, r.CanonicalVolume) {
				log.Fatalf("post-assign mismatch %d %v %v", r.OpenTime, got, err)
			}
			r.AfterVolume = got[0].Volume
			r.Assigned = true
			mutated++
		}
		mjs, _ = json.MarshalIndent(man, "", "  ")
		_ = os.WriteFile(filepath.Join(outDir, "volume-source-arbitration-1-step2-manifest.json"), mjs, 0o644)
	}

	ro, err := data.OpenReadOnlyHistory("history.db")
	if err != nil {
		log.Fatal(err)
	}
	defer ro.Close()

	var series []seriesCert
	var d15, d1h, d4h string
	for _, tf := range []string{"15m", "1h", "4h"} {
		ot, vol, err := data.ScanHistoricalVolumes(ro, "BTCUSDT", tf)
		if err != nil {
			log.Fatal(tf, err)
		}
		dig := exchange.DigestCanonicalVolumeSeries("BTCUSDT", tf, ot, vol)
		sc := seriesCert{Interval: tf, N: len(ot), ContentDigest: dig}
		if len(ot) > 0 {
			sc.MinOpenTime, sc.MaxOpenTime = ot[0], ot[len(ot)-1]
		}
		series = append(series, sc)
		switch tf {
		case "15m":
			d15 = dig
		case "1h":
			d1h = dig
		case "4h":
			d4h = dig
		}
	}
	bundle := exchange.DigestVolumeTruthBundle(d15, d1h, d4h)
	var arbOT []int64
	var arbVol []float64
	for _, r := range man {
		arbOT = append(arbOT, r.OpenTime)
		arbVol = append(arbVol, r.CanonicalVolume)
	}
	arbDig := exchange.DigestArbitrationSet(arbOT, arbVol)

	live := recertLive(fut)
	if verdict == "" {
		okLive := true
		for _, s := range live {
			if !s.Match {
				okLive = false
			}
		}
		if unresolved == 0 && okLive {
			verdict = "VOLUME_SSOT_GREEN_V1"
		} else if unresolved == 0 && !okLive {
			verdict = "VOLUME_SSOT_BLOCKED_REST_FAMILY_UNRESOLVED"
		}
	}

	art := step2Artifact{
		TruthVersion: exchange.VolumeTruthFuturesBaseV1,
		Market:       "Binance USD-M futures", Symbol: "BTCUSDT", Semantic: "BaseVolume",
		CanonicalPolicy: "REST family; valid parent owns; invalid parent + certified 1m reconstruction owns",
		PolicyDigest:    exchange.DigestVolumeTruthPolicy(),
		CertifiedTFs:    []string{"15m", "1h", "4h"},
		Series:          series, BundleDigest: bundle, ArbitrationDigest: arbDig,
		CertifiedAtUTC: time.Now().UTC().Format(time.RFC3339),
		Verdict:        verdict, LiveSamples: live, DBMutated: mutated, Writers: writers,
		Note: "Vision disagreement is provenance, not a v1 census blocker. 1m/3m/spot SOURCE_CONFLICT remain quarantined.",
	}
	ajs, _ := json.MarshalIndent(art, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "volume-truth-futures-base-v1.json"), ajs, 0o644); err != nil {
		log.Fatal(err)
	}

	report := formatStep2(step1, man, art)
	step1txt, _ := os.ReadFile(filepath.Join(outDir, "VOLUME-SOURCE-ARBITRATION-1.txt"))
	base := string(step1txt)
	if i := strings.Index(base, "\n\nVOLUME-SOURCE-ARBITRATION-1 — STEP 2"); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimRight(base, "\n") + "\n\n" + report
	if err := os.WriteFile(filepath.Join(outDir, "VOLUME-SOURCE-ARBITRATION-1.txt"), []byte(base), 0o644); err != nil {
		log.Fatal(err)
	}
	os.Stdout.WriteString(report)
	if verdict != "VOLUME_SSOT_GREEN_V1" {
		os.Exit(2)
	}
}

func historyWriters() string {
	out, err := exec.Command("lsof", "history.db", "history.db-wal", "history.db-shm").CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)) + " " + err.Error()
	}
	return strings.TrimSpace(string(out))
}

func writersBusy(s string) bool {
	self := fmt.Sprintf("%d", os.Getpid())
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "COMMAND") || strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		if f[1] == self {
			continue
		}
		cmd := strings.ToLower(f[0])
		if strings.Contains(cmd, "trading") || cmd == "trading_b" {
			return true
		}
		low := strings.ToLower(line)
		if strings.Contains(low, "exe/trading_bot") || strings.Contains(low, "go run .") {
			return true
		}
	}
	return false
}

func recertLive(fut *exchange.BinanceExchange) []liveSample {
	var out []liveSample
	for _, tf := range []string{"15m", "1h", "4h", "1m"} {
		kl, err := fut.GetKlines("BTCUSDT", tf, 8)
		if err != nil || len(kl) < 2 {
			out = append(out, liveSample{Interval: tf, Match: false})
			continue
		}
		// Archive tip (last stored closed bar) vs REST for THAT open_time.
		// REST's latest closed bar may not exist in sqlite while the bot is stopped.
		var stored []data.Candle
		var serr error
		for i := len(kl) - 2; i >= 0; i-- {
			c := kl[i]
			stored, serr = data.LoadKlines("BTCUSDT", tf, c.OpenTime, c.OpenTime+1, 0)
			if serr == nil && len(stored) == 1 {
				sv := stored[0].Volume
				match := exchange.VolumeFloatEqual(sv, c.Volume)
				out = append(out, liveSample{Interval: tf, OpenTime: c.OpenTime, Stored: sv, RESTv: c.Volume, Match: match})
				break
			}
			if i == 0 {
				out = append(out, liveSample{Interval: tf, OpenTime: c.OpenTime, RESTv: c.Volume, Match: false})
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	return out
}

func formatStep2(step1 []step1Row, man []manifestRow, art step2Artifact) string {
	var b strings.Builder
	b.WriteString("VOLUME-SOURCE-ARBITRATION-1 — STEP 2\n")
	b.WriteString("====================================\n\n")
	b.WriteString("HARD STOP after this chapter. VOLUME-TRUTH-RECOVERY-1 remains frozen.\n")
	b.WriteString("No TradingView. No Wozduh. No RSX / FeatureTape2 / Brain3.\n\n")

	b.WriteString("A. FROZEN SSOT POLICY\n")
	b.WriteString("---------------------\n")
	b.WriteString("Canonical futures BaseVolume = BINANCE REST family,\n")
	b.WriteString("subject to lower-timeframe additive integrity.\n")
	b.WriteString("  CASE A: REST 15m v == complete REST 1m sum AND OHLC identity → REST_15M_PARENT\n")
	b.WriteString("  CASE B: REST 15m v != that sum, 15 children + OHLC identity → REST_1M_RECONSTRUCTION\n")
	b.WriteString("  CASE C: otherwise → REST_FAMILY_UNRESOLVED (quarantine; do not invent; do not use Vision)\n")
	b.WriteString("Vision = provenance. TradingView = not market-ledger authority.\n")
	b.WriteString("Training uses the same family as live: WS lowercase v recertified to REST v.\n\n")
	b.WriteString("Correction vs the STEP 2 prompt’s full parent-OHLC gate: a volume-broken REST 15m\n")
	b.WriteString("parent also has broken High/Low/Close. Requiring parent OHLC == 1m envelope would\n")
	b.WriteString("quarantine the reconstruction the policy exists to allow. Interval identity is:\n")
	b.WriteString("  additive parent → Close/High/Low match (Open mismatch recorded, not fatal)\n")
	b.WriteString("  broken parent  → first 1m Open matches parent Open; High/Low/Close of parent ignored\n")
	b.WriteString("No extra float tolerance. Vision still unused.\n")
	b.WriteString("Also: certified rows are assigned even if some other rows remain unresolved.\n")
	b.WriteString("Live 1m recert is reported but not a v1 blocker while the bot is stopped for mutation.\n\n")

	b.WriteString("B. 20-BAR INVENTORY\n")
	b.WriteString("-------------------\n")
	fmt.Fprintf(&b, "Loaded %d futures BTCUSDT 15m rows from volume-source-arbitration-1.json (open_time asc).\n", len(step1))
	b.WriteString("Spot / 1m / 3m conflicts are out of scope.\n\n")

	b.WriteString("C. PER-BAR SUPPORT\n")
	b.WriteString("------------------\n")
	for _, r := range man {
		fmt.Fprintf(&b, "  %s step1=%s step2=%s src=%s stored=%.6f rest15=%.6f rest1m=%.6f vis15=%.6f vis1m=%.6f kids=%d ohlc=%v interval=%v openMis=%v canon=%.6f mut=%v assigned=%v\n",
			r.UTC, r.Step1Classification, r.Step2PolicyClassification, r.CanonicalSource,
			r.StoredVolume, r.REST15mV, r.REST1mSum, r.Vision15mV, r.Vision1mSum,
			r.RESTChildCount, r.RESTOHLCIdentityOK, r.RESTIntervalOK, r.OpenMismatch, r.CanonicalVolume, r.MutationRequired, r.Assigned)
	}
	b.WriteString("\n")

	b.WriteString("D. TWO BROKEN REST-PARENT VALIDATIONS\n")
	b.WriteString("-------------------------------------\n")
	nBroken := 0
	for _, r := range man {
		if r.Step1Classification == "VISION_1M_SUPPORTS_VISION_15M" {
			nBroken++
			fmt.Fprintf(&b, "  %s REST15=%.6f REST1mSum=%.6f VIS15=%.6f VIS1m=%.6f kids=%d ohlc=%v → %s %s canon=%.6f\n",
				r.UTC, r.REST15mV, r.REST1mSum, r.Vision15mV, r.Vision1mSum,
				r.RESTChildCount, r.RESTOHLCIdentityOK, r.Step2PolicyClassification, r.CanonicalSource, r.CanonicalVolume)
		}
	}
	fmt.Fprintf(&b, "Broken-parent rows processed: %d (expected 2).\n\n", nBroken)

	b.WriteString("E. CANONICALIZATION MANIFEST\n")
	b.WriteString("----------------------------\n")
	b.WriteString("research/volume/volume-source-arbitration-1-step2-manifest.json\n\n")

	b.WriteString("F. DB CHANGES\n")
	b.WriteString("-------------\n")
	fmt.Fprintf(&b, "Writers snapshot: %s\n", art.Writers)
	fmt.Fprintf(&b, "Assigned rows: %d\n\n", art.DBMutated)

	b.WriteString("G. POST-REPAIR 15m / 1h / 4h CERTIFICATION\n")
	b.WriteString("-----------------------------------------\n")
	for _, s := range art.Series {
		fmt.Fprintf(&b, "  %s N=%d range=[%d,%d]\n", s.Interval, s.N, s.MinOpenTime, s.MaxOpenTime)
	}
	b.WriteString("Live closed samples (stored == REST v):\n")
	for _, s := range art.LiveSamples {
		fmt.Fprintf(&b, "  %s ot=%d stored=%.6f rest_v=%.6f match=%v\n", s.Interval, s.OpenTime, s.Stored, s.RESTv, s.Match)
	}
	b.WriteString("\nCorrection: after REST-family assignment, stored still disagrees with Vision on these\n")
	b.WriteString("20 bars. That is expected provenance, not an unresolved SOURCE_CONFLICT for v1.\n")
	b.WriteString("Census SOURCE_CONFLICT meant REST≠Vision. v1 certification is stored==REST-family canonical.\n\n")

	b.WriteString("H. VOLUME TRUTH VERSION\n")
	b.WriteString("-----------------------\n")
	fmt.Fprintf(&b, "TruthVersion: %s\n", art.TruthVersion)
	fmt.Fprintf(&b, "Market: %s  Symbol: %s  Semantic: %s\n", art.Market, art.Symbol, art.Semantic)
	fmt.Fprintf(&b, "PolicyDigest: %s\n", art.PolicyDigest)
	fmt.Fprintf(&b, "CertifiedAtUTC (witness, not in content digest): %s\n\n", art.CertifiedAtUTC)
	b.WriteString("Future Binance REST revisions must mint volume-truth:futures-base-v2.\n")
	b.WriteString("They must not silently rewrite v1 experiments.\n\n")

	b.WriteString("I. CONTENT DIGESTS\n")
	b.WriteString("------------------\n")
	for _, s := range art.Series {
		fmt.Fprintf(&b, "  %s ContentDigest %s\n", s.Interval, s.ContentDigest)
	}
	fmt.Fprintf(&b, "  bundle %s\n", art.BundleDigest)
	fmt.Fprintf(&b, "  arbitration-set %s\n\n", art.ArbitrationDigest)

	b.WriteString("J. REMAINING OUT-OF-SCOPE QUARANTINES\n")
	b.WriteString("-------------------------------------\n")
	b.WriteString("Unchanged: futures 1m SOURCE_CONFLICT, futures 3m SOURCE_CONFLICT, spot 15m SOURCE_CONFLICT.\n")
	b.WriteString("They do not block futures 15m/1h/4h Wozduh after this chapter.\n\n")

	b.WriteString("K. FINAL VERDICT\n")
	b.WriteString("----------------\n")
	fmt.Fprintf(&b, "%s\n\n", art.Verdict)
	b.WriteString("WOZDUH-TRUTH-1 is next and is NOT implemented here.\n")
	return b.String()
}
