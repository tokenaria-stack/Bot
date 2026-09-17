package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"trading_bot/data"
	"trading_bot/exchange"
)

const volumeIngestPageDelay = 160 * time.Millisecond

// VolumeIngestReport is VOLUME-INGEST-1 census + recert output.
type VolumeIngestReport struct {
	Verdict        string                                   `json:"verdict"`
	Repair         string                                   `json:"repair"`
	Notes          []string                                 `json:"notes"`
	Futures        map[string]exchange.CensusCounts         `json:"futures"`
	FuturesByYear  map[string]map[int]exchange.CensusCounts `json:"futures_by_year,omitempty"`
	Spot           map[string]exchange.CensusCounts         `json:"spot"`
	Live           map[string]exchange.CensusCounts         `json:"live_recert"`
	Micro          data.MicroVolumeStats                    `json:"micro_klines"`
	Samples        []exchange.CensusSample                  `json:"mismatch_samples,omitempty"`
	DerivedChecked []string                                 `json:"derived_checked"`
}

func fetchFuturesAuthority(fut *exchange.BinanceExchange, symbol, interval string, minOT, maxOT int64) ([]exchange.VolumeAuthority, error) {
	var all []exchange.VolumeAuthority
	cur := minOT
	var prev int64 = -1
	for {
		page, err := fut.FetchVolumeAuthorityPage(symbol, interval, cur, 1000)
		if err != nil {
			return all, err
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		last := page[len(page)-1].OpenTime
		if last == prev {
			break
		}
		prev = last
		if last >= maxOT {
			break
		}
		cur = last + 1
		time.Sleep(volumeIngestPageDelay)
	}
	return all, nil
}

func fetchSpotAuthority(symbol, interval string, minOT, maxOT int64) ([]exchange.VolumeAuthority, error) {
	var all []exchange.VolumeAuthority
	cur := minOT
	var prev int64 = -1
	for {
		page, err := exchange.FetchSpotVolumeAuthorityPage(symbol, interval, cur, 1000)
		if err != nil {
			return all, err
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		last := page[len(page)-1].OpenTime
		if last == prev {
			break
		}
		prev = last
		if last >= maxOT {
			break
		}
		cur = last + 1
		time.Sleep(volumeIngestPageDelay)
	}
	return all, nil
}

func storedFromScan(ot []int64, vol []float64) []exchange.StoredVolume {
	out := make([]exchange.StoredVolume, len(ot))
	for i := range ot {
		out[i] = exchange.StoredVolume{OpenTime: ot[i], Volume: vol[i]}
	}
	return out
}

// RunVolumeIngest1 is the archive census + live recert. Read-only on history.db.
func RunVolumeIngest1(dbPath, outDir string) (VolumeIngestReport, error) {
	var z VolumeIngestReport
	z.Futures = map[string]exchange.CensusCounts{}
	z.FuturesByYear = map[string]map[int]exchange.CensusCounts{}
	z.Spot = map[string]exchange.CensusCounts{}
	z.Live = map[string]exchange.CensusCounts{}
	z.Repair = "NONE"
	z.DerivedChecked = []string{"2m←1m AssembleChildOHLCV (unit test)", "sparse seconds ← 1s AssembleChildOHLCV (unit test)"}

	db, err := data.OpenReadOnlyHistory(dbPath)
	if err != nil {
		return z, err
	}
	defer db.Close()

	z.Micro, err = data.ScanMicroVolumeStats(db)
	if err != nil {
		return z, err
	}
	z.Notes = append(z.Notes,
		"1s micro Volume is MicroTradeBaseVolume (sum aggTrade Qty). No historical aggTrade tape; REST 1s kline not used (no invented API). Producer fixtures lock the builder.")

	keys, err := data.ListHistoricalVolumeKeys(db)
	if err != nil {
		return z, err
	}

	fut, err := exchange.NewBinanceExchange("", "", false)
	if err != nil {
		return z, err
	}

	only := map[string]bool{}
	if v := strings.TrimSpace(os.Getenv("VOLUME_INGEST_ONLY")); v != "" {
		for _, p := range strings.Split(v, ",") {
			only[strings.TrimSpace(p)] = true
		}
		z.Notes = append(z.Notes, "VOLUME_INGEST_ONLY="+v)
	}

	for _, key := range keys {
		if len(only) > 0 && !only[key.Interval] {
			continue
		}
		ot, vol, err := data.ScanHistoricalVolumes(db, key.Symbol, key.Interval)
		if err != nil {
			return z, err
		}
		if len(ot) == 0 {
			continue
		}
		if !exchange.IsNativeBinance(key.Interval) {
			z.Notes = append(z.Notes, fmt.Sprintf("SKIP %s %s: not a native Binance kline interval (derived/legacy/3d). No REST equality.", key.Symbol, key.Interval))
			continue
		}
		stored := storedFromScan(ot, vol)
		minOT, maxOT := ot[0], ot[len(ot)-1]
		pair := strings.TrimSuffix(key.Symbol, "_SPOT")
		isSpot := strings.HasSuffix(key.Symbol, "_SPOT")

		var auth []exchange.VolumeAuthority
		if isSpot {
			auth, err = fetchSpotAuthority(pair, key.Interval, minOT, maxOT)
		} else {
			auth, err = fetchFuturesAuthority(fut, pair, key.Interval, minOT, maxOT)
		}
		if err != nil {
			z.Notes = append(z.Notes, fmt.Sprintf("SKIP %s %s REST: %v", key.Symbol, key.Interval, err))
			continue
		}
		counts, samples := exchange.JoinVolumeCensusWithSamples(stored, auth, 8)
		z.Samples = append(z.Samples, samples...)
		years := exchange.SplitCensusByYear(stored, auth)
		label := key.Symbol + " " + key.Interval
		if isSpot {
			z.Spot[key.Interval] = counts
		} else {
			z.Futures[key.Interval] = counts
			z.FuturesByYear[key.Interval] = years
		}
		fmt.Fprintf(os.Stderr, "VOLUME-INGEST-1 %s stored=%d compared=%d TOTAL_BASE=%d dirty=%v\n",
			label, counts.Stored, counts.Compared, counts.TotalBase, counts.Dirty())
		_ = years
	}

	liveTFs := []string{"1m", "5m", "15m", "1h", "4h"}
	for _, tf := range liveTFs {
		ot, vol, err := data.ScanHistoricalVolumes(db, "BTCUSDT", tf)
		if err != nil || len(ot) == 0 {
			continue
		}
		n := 13
		if len(ot) < n {
			n = len(ot)
		}
		// Drop the newest stored bar: it may still be forming.
		end := len(ot) - 1
		start := end - (n - 1)
		if start < 0 {
			start = 0
		}
		if end <= start {
			continue
		}
		tail := storedFromScan(ot[start:end], vol[start:end])
		auth, err := fut.FetchVolumeAuthorityPage("BTCUSDT", tf, tail[0].OpenTime, 1000)
		if err != nil {
			z.Notes = append(z.Notes, fmt.Sprintf("live recert %s: %v", tf, err))
			continue
		}
		liveCounts, liveSamples := exchange.JoinVolumeCensusWithSamples(tail, auth, 8)
		z.Live[tf] = liveCounts
		z.Samples = append(z.Samples, liveSamples...)
		time.Sleep(volumeIngestPageDelay)
	}

	z.Verdict = volumeIngestVerdict(&z)
	if err := writeVolumeIngestArtifacts(outDir, z); err != nil {
		return z, err
	}
	return z, nil
}

func volumeIngestVerdict(z *VolumeIngestReport) string {
	var anyCompared bool
	var dirty, caseB, quotesOrNone bool
	check := func(m map[string]exchange.CensusCounts) {
		for _, c := range m {
			if c.Compared > 0 {
				anyCompared = true
			}
			if c.Dirty() {
				dirty = true
			}
			if c.CaseBOnly() {
				caseB = true
			}
			if c.TotalQuote > 0 || c.TakerBuyQuote > 0 || c.MatchNone > 0 {
				quotesOrNone = true
			}
		}
	}
	check(z.Futures)
	check(z.Spot)
	check(z.Live)
	if !anyCompared {
		return "VOLUME_TRUTH_BLOCKED_UNKNOWN_CORRUPTION"
	}
	if !dirty {
		return "VOLUME_TRUTH_GREEN_NO_REPAIR"
	}
	if caseB && !quotesOrNone {
		return "VOLUME_TRUTH_CASE_B_REPAIR_CANDIDATE"
	}
	return "VOLUME_TRUTH_BLOCKED_UNKNOWN_CORRUPTION"
}

func writeVolumeIngestArtifacts(outDir string, z VolumeIngestReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(z, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "volume-ingest-1-census.json"), raw, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "VOLUME-INGEST-1.txt"), []byte(FormatVolumeIngest1(z)), 0o644)
}

func FormatVolumeIngest1(z VolumeIngestReport) string {
	var b strings.Builder
	b.WriteString("VOLUME-INGEST-1\n")
	b.WriteString("================\n\n")
	b.WriteString("A. SEMANTIC LAW\n")
	b.WriteString("  native Kline.Volume = BaseVolume = Binance total base v (REST idx 5 / WS v / Vision col 5)\n")
	b.WriteString("  1s/micro Volume = MicroTradeBaseVolume = sum ALL observed aggTrade Qty (not V)\n")
	b.WriteString("  WozduhBitVolBase = DAG compute mask, not Binance V\n\n")
	b.WriteString("B. PRODUCER TABLE\n")
	b.WriteString("  futures REST/WS/Vision: NATIVE_BASE_VOLUME (fixtures)\n")
	b.WriteString("  spot Vision/REST mapper: NATIVE_BASE_VOLUME (fixtures)\n")
	b.WriteString("  SecondBarBuilder: MICRO_TRADE_BASE_VOLUME (fixtures)\n")
	b.WriteString("  derived/sparse: DERIVED_SAME_SEMANTIC (fixtures)\n")
	b.WriteString("  Ingress/SQLite MAX: same-semantic BaseVolume only\n")
	b.WriteString("  repair_volumes: REST v via MAX; not used as type converter in this chapter\n\n")
	b.WriteString("C. CONSUMER TABLE (formulas unchanged)\n")
	b.WriteString("  Wozduh SlotVolume <- Kline.Volume  assumed BaseVolume (next chapter binds it)\n")
	b.WriteString("  geometry CheckBreakout              assumed BaseVolume\n")
	b.WriteString("  VolumeWeightedEMA                   assumed BaseVolume\n")
	b.WriteString("  chart / FeatureTape bar copies      assumed BaseVolume\n")
	b.WriteString("  No consumer intentionally expects V as Kline.Volume\n\n")
	b.WriteString("D. ARCHIVE CENSUS\n")
	writeCounts := func(title string, m map[string]exchange.CensusCounts) {
		b.WriteString("  " + title + "\n")
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			c := m[k]
			fmt.Fprintf(&b, "    %s stored=%d compared=%d no_auth=%d extra_auth=%d TOTAL_BASE=%d TAKER_BUY_BASE=%d TOTAL_QUOTE=%d TAKER_BUY_QUOTE=%d ZERO=%d MATCH_NONE=%d\n",
				k, c.Stored, c.Compared, c.NoAuthority, c.AuthorityExtra, c.TotalBase, c.TakerBuyBase, c.TotalQuote, c.TakerBuyQuote, c.ZeroMissing, c.MatchNone)
		}
	}
	writeCounts("futures BTCUSDT", z.Futures)
	writeCounts("spot BTCUSDT_SPOT", z.Spot)
	b.WriteString("\nE. MICRO / DERIVED\n")
	fmt.Fprintf(&b, "  micro_klines n=%d minOT=%d maxOT=%d\n", z.Micro.N, z.Micro.MinOT, z.Micro.MaxOT)
	fmt.Fprintf(&b, "  derived checked: %s\n", strings.Join(z.DerivedChecked, "; "))
	b.WriteString("\nF. REPAIR\n  ")
	b.WriteString(z.Repair)
	b.WriteString("\n\nG. LIVE RECERTIFICATION (newest stored vs REST v)\n")
	writeCounts("live", z.Live)
	if len(z.Samples) > 0 {
		b.WriteString("\nMismatch samples (first per class, truncated)\n")
		for _, s := range z.Samples {
			fmt.Fprintf(&b, "    ot=%d stored=%.8f v=%.8f V=%.8f class=%s\n", s.OpenTime, s.Stored, s.Base, s.TakerBuyBase, s.Class)
		}
	}
	b.WriteString("\nH. VERDICT\n  ")
	b.WriteString(z.Verdict)
	b.WriteString("\n")
	if len(z.Notes) > 0 {
		b.WriteString("\nNOTES\n")
		for _, n := range z.Notes {
			b.WriteString("  - ")
			b.WriteString(n)
			b.WriteString("\n")
		}
	}
	b.WriteString("\nCorrections vs prompt:\n")
	b.WriteString("  - 1s is not Binance V; it is sum of all observed aggTrade Qty.\n")
	b.WriteString("  - Census used REST pages of 1000 (same v as Vision col5), not Vision zip download.\n")
	b.WriteString("  - Non-native SQLite intervals (e.g. 3d) are skipped: Binance does not own them in the live catalog.\n")
	b.WriteString("  - 2026-09-06 is not a poison start date.\n")
	b.WriteString("  - Go encoding/json is case-insensitive: struct json:\"v\" can bind Binance \"V\". Production WS parse is a case-sensitive map so Volume is lowercase v only. Archive may still be TOTAL_BASE because Ingress MAX(V, later REST v) lifts to v when v>=V. That is coincidence, not a type converter. MAX remains same-semantic only.\n")
	b.WriteString("  - Derived TFs are not persisted (catalog Persist=false); aggregation certified by AssembleChildOHLCV fixtures, not archive rows.\n")
	b.WriteString("  - No historical aggTrade tape: 1s census is producer+persistence stats, not REST 1s parity.\n")
	return b.String()
}
