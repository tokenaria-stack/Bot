package market

import (
	"database/sql"
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

// RepairManifestRow is one planned VOLUME-TRUTH-RECOVERY-1 assignment.
type RepairManifestRow struct {
	Symbol      string  `json:"symbol"`
	Timeframe   string  `json:"timeframe"`
	OpenTime    int64   `json:"open_time"`
	OldVolume   float64 `json:"old_volume"`
	NewVolume   float64 `json:"new_volume"`
	RepairClass string  `json:"repair_class"`
	Authority   string  `json:"authority"`
	IndexShift  bool    `json:"likely_index_shift"`
	RESTv       float64 `json:"rest_v"`
	RESTV       float64 `json:"rest_V"`
	VisionV     float64 `json:"vision_v,omitempty"`
	HasVision   bool    `json:"has_vision"`
}

// RecoveryRow is a classified dirty bar.
type RecoveryRow struct {
	RepairManifestRow
	Class exchange.RecoveryClass `json:"class"`
}

// VolumeTruthRecoveryReport is VOLUME-TRUTH-RECOVERY-1 output.
type VolumeTruthRecoveryReport struct {
	Verdict        string                      `json:"verdict"`
	Producer       string                      `json:"producer"`
	Live           map[string][]map[string]any `json:"live_recert"`
	ClassCounts    map[string]map[string]int   `json:"class_counts"`
	ManifestCount  int                         `json:"manifest_count"`
	Applied        int                         `json:"applied"`
	Unresolved     int                         `json:"unresolved"`
	WozduhEligible string                      `json:"wozduh_source_eligibility"`
	Notes          []string                    `json:"notes"`
	LiveOK         bool                        `json:"live_ok"`
}

func RunVolumeTruthRecovery(dbPath, outDir string, apply, live bool) (VolumeTruthRecoveryReport, error) {
	z := VolumeTruthRecoveryReport{
		Live:        map[string][]map[string]any{},
		ClassCounts: map[string]map[string]int{},
		Producer:    "ParseFuturesWsKlineJSON case-sensitive lowercase v",
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return z, err
	}
	ro, err := data.OpenReadOnlyHistory(dbPath)
	if err != nil {
		return z, err
	}
	defer ro.Close()
	fut, err := exchange.NewBinanceExchange("", "", false)
	if err != nil {
		return z, err
	}

	if live {
		ok, notes, rec, err := recertClosedNative(ro, fut)
		if err != nil {
			return z, err
		}
		z.LiveOK = ok
		z.Live = rec
		z.Notes = append(z.Notes, notes...)
		if !ok {
			z.Verdict = "VOLUME_TRUTH_BLOCKED_SOURCE_CONFLICT"
			z.WozduhEligible = "WOZDUH_VOLUME_NOT_READY"
			z.Notes = append(z.Notes, "NEEDS_FIXED_PARSER_RESTART or LIVE_PRODUCER_NOT_CERTIFIED — no historical mutation")
		}
	} else {
		z.Notes = append(z.Notes, "Phase A skipped (live=false)")
		z.LiveOK = false
		z.Verdict = "VOLUME_TRUTH_BLOCKED_SOURCE_CONFLICT"
		z.WozduhEligible = "WOZDUH_VOLUME_NOT_READY"
	}

	rows, err := collectRecoveryRows(ro, fut)
	if err != nil {
		return z, err
	}
	for _, r := range rows {
		if z.ClassCounts[r.Timeframe] == nil {
			z.ClassCounts[r.Timeframe] = map[string]int{}
		}
		z.ClassCounts[r.Timeframe][string(r.Class)]++
	}
	var manifest []RepairManifestRow
	var unresolved []RecoveryRow
	for _, r := range rows {
		if exchange.RepairEligible(r.Class) {
			manifest = append(manifest, r.RepairManifestRow)
		} else {
			unresolved = append(unresolved, r)
			z.Unresolved++
		}
	}
	z.ManifestCount = len(manifest)
	if err := writeJSON(filepath.Join(outDir, "dirty-row-reclassification.json"), rows); err != nil {
		return z, err
	}
	if err := writeManifestJSONL(filepath.Join(outDir, "volume-repair-manifest.jsonl"), manifest); err != nil {
		return z, err
	}

	if z.LiveOK && apply {
		data.SetDBPath(dbPath)
		if err := data.InitDB(); err != nil {
			return z, err
		}
		for _, m := range manifest {
			if !exchange.RepairEligible(exchange.RecoveryClass(m.RepairClass)) {
				return z, fmt.Errorf("manifest class not eligible: %s", m.RepairClass)
			}
			if err := data.AssignCanonicalVolume(m.Symbol, m.Timeframe, m.OpenTime, m.NewVolume); err != nil {
				return z, err
			}
			z.Applied++
		}
		_ = data.CheckpointWAL()
		z.Notes = append(z.Notes, fmt.Sprintf("applied %d assignments", z.Applied))
	} else if apply && !z.LiveOK {
		z.Notes = append(z.Notes, "APPLY ignored: producer not certified")
	} else {
		z.Notes = append(z.Notes, "dry-run manifest only")
	}

	z.WozduhEligible = wozduhEligibility(unresolved, z.Applied, len(manifest), z.LiveOK)
	z.Verdict = recoveryVerdict(z, apply)
	_ = os.WriteFile(filepath.Join(outDir, "VOLUME-TRUTH-RECOVERY-1.txt"), []byte(FormatVolumeTruthRecovery(z)), 0o644)
	raw, _ := json.MarshalIndent(z, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "volume-truth-recovery-1.json"), raw, 0o644)
	return z, nil
}

func recertClosedNative(ro *sql.DB, fut *exchange.BinanceExchange) (bool, []string, map[string][]map[string]any, error) {
	out := map[string][]map[string]any{}
	var notes []string
	okAll := true
	for _, tf := range []string{"1m", "5m", "15m", "1h"} {
		bars, err := data.ScanHistoricalOHLCV(ro, "BTCUSDT", tf)
		if err != nil {
			return false, nil, out, err
		}
		if len(bars) < 3 {
			okAll = false
			notes = append(notes, tf+": not enough stored bars")
			continue
		}
		end := len(bars) - 1
		start := end - 3
		if start < 0 {
			start = 0
		}
		sample := bars[start:end]
		auth, err := fut.FetchVolumeAuthorityPage("BTCUSDT", tf, sample[0].OpenTime, 1000)
		if err != nil {
			return false, nil, out, err
		}
		am := map[int64]exchange.VolumeAuthority{}
		for _, a := range auth {
			am[a.OpenTime] = a
		}
		var recs []map[string]any
		for _, b := range sample {
			mar, ok := am[b.OpenTime]
			if !ok {
				okAll = false
				continue
			}
			cl := exchange.ClassifyStoredVolume(b.Volume, mar)
			row := map[string]any{
				"open_time": b.OpenTime, "timeframe": tf,
				"stored": b.Volume, "rest_v": mar.Base, "rest_V": mar.TakerBuyBase,
				"class": string(cl),
			}
			recs = append(recs, row)
			if cl != exchange.VolumeClassTotalBase {
				okAll = false
			}
			if exchange.VolumeFloatEqual(b.Volume, mar.TakerBuyBase) && !exchange.VolumeFloatEqual(mar.Base, mar.TakerBuyBase) {
				okAll = false
			}
		}
		out[tf] = recs
		time.Sleep(volumeIngestPageDelay)
	}
	if !okAll {
		notes = append(notes, "new closed bars are not TOTAL_BASE vs REST v")
	}
	return okAll, notes, out, nil
}

func collectRecoveryRows(ro *sql.DB, fut *exchange.BinanceExchange) ([]RecoveryRow, error) {
	keys, err := data.ListHistoricalVolumeKeys(ro)
	if err != nil {
		return nil, err
	}
	var out []RecoveryRow
	visionCache := map[string]map[int64]exchange.Candle{}
	for _, key := range keys {
		if !exchange.IsNativeBinance(key.Interval) {
			continue
		}
		bars, err := data.ScanHistoricalOHLCV(ro, key.Symbol, key.Interval)
		if err != nil {
			return nil, err
		}
		if len(bars) == 0 {
			continue
		}
		isSpot := strings.HasSuffix(key.Symbol, "_SPOT")
		pair := strings.TrimSuffix(key.Symbol, "_SPOT")
		minOT, maxOT := bars[0].OpenTime, bars[len(bars)-1].OpenTime
		// Dirty TFs from VOLUME-INGEST-1: skip known-clean series (1d/1w/1M/12h).
		if isCleanNativeTF(key.Interval) && !isSpot {
			continue
		}
		if isSpot && key.Interval != "15m" {
			continue
		}
		var auth []exchange.VolumeAuthority
		if isSpot {
			auth, err = fetchSpotAuthority(pair, key.Interval, minOT, maxOT)
		} else {
			start := recoveryRESTStart(key.Interval, minOT)
			auth, err = fetchFuturesAuthority(fut, pair, key.Interval, start, maxOT)
		}
		if err != nil {
			return nil, fmt.Errorf("%s %s REST: %w", key.Symbol, key.Interval, err)
		}
		am := map[int64]exchange.VolumeAuthority{}
		for _, a := range auth {
			am[a.OpenTime] = a
		}
		for i, b := range bars {
			a, ok := am[b.OpenTime]
			if !ok {
				continue
			}
			ing := exchange.ClassifyStoredVolume(b.Volume, a)
			if ing == exchange.VolumeClassTotalBase {
				continue
			}
			var prev, next float64
			var hasPrev, hasNext bool
			if i > 0 {
				if p, ok := am[bars[i-1].OpenTime]; ok {
					prev, hasPrev = p.Base, true
				}
			}
			if i+1 < len(bars) {
				if n, ok := am[bars[i+1].OpenTime]; ok {
					next, hasNext = n.Base, true
				}
			}
			liveV := ing == exchange.VolumeClassTakerBuyBase
			var visPtr *exchange.VolumeAuthority
			if !liveV {
				ck := visionCacheKey(isSpot, pair, key.Interval, b.OpenTime)
				vm, cached := visionCache[ck]
				if !cached {
					y, m := yearMonth(b.OpenTime)
					got, err := exchange.FetchVisionMonthKlines(pair, key.Interval, y, m, isSpot)
					if err != nil {
						return nil, err
					}
					vm = got
					visionCache[ck] = vm
					time.Sleep(volumeIngestPageDelay)
				}
				if vm != nil {
					if c, ok := vm[b.OpenTime]; ok {
						va := exchange.VolumeAuthorityFromVisionCandle(c)
						visPtr = &va
					}
				}
			}
			class, shift := exchange.ClassifyRecoveryRow(b.OpenTime, b.Open, b.High, b.Low, b.Close, b.Volume, a, visPtr, prev, next, hasPrev, hasNext)
			nv, _ := exchange.RepairNewVolume(class, a)
			if !exchange.RepairEligible(class) {
				nv = 0
			}
			authName := "REST_v"
			if visPtr != nil {
				authName = "REST_v+Vision_v"
			}
			if class == exchange.RecoveryLiveVCollision {
				authName = "REST_v"
			}
			row := RecoveryRow{
				Class: class,
				RepairManifestRow: RepairManifestRow{
					Symbol: key.Symbol, Timeframe: key.Interval, OpenTime: b.OpenTime,
					OldVolume: b.Volume, NewVolume: nv, RepairClass: string(class),
					Authority: authName, IndexShift: shift, RESTv: a.Base, RESTV: a.TakerBuyBase,
				},
			}
			if visPtr != nil {
				row.HasVision = true
				row.VisionV = visPtr.Base
			}
			out = append(out, row)
		}
	}
	return out, nil
}

func isCleanNativeTF(tf string) bool {
	switch tf {
	case "12h", "1d", "1w", "1M":
		return true
	default:
		return false
	}
}

func recoveryRESTStart(interval string, seriesMin int64) int64 {
	// VOLUME-INGEST-1: TAKER_BUY is 2026-only except 15m ZERO/MATCH_NONE from 2023.
	if interval == "15m" {
		t := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
		if t > seriesMin {
			return t
		}
		return seriesMin
	}
	t := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if t > seriesMin {
		return t
	}
	return seriesMin
}

func visionCacheKey(spot bool, pair, interval string, ot int64) string {
	y, m := yearMonth(ot)
	mk := "f"
	if spot {
		mk = "s"
	}
	return fmt.Sprintf("%s:%s:%s:%04d-%02d", mk, pair, interval, y, m)
}

func yearMonth(ot int64) (int, int) {
	tm := time.UnixMilli(ot).UTC()
	return tm.Year(), int(tm.Month())
}

func wozduhEligibility(unresolved []RecoveryRow, applied, manifest int, liveOK bool) string {
	if !liveOK {
		return "WOZDUH_VOLUME_NOT_READY"
	}
	woz := map[string]bool{"15m": true, "1h": true, "4h": true}
	for _, r := range unresolved {
		if woz[r.Timeframe] && r.Symbol == "BTCUSDT" {
			return "WOZDUH_VOLUME_NOT_READY"
		}
	}
	if applied < manifest {
		return "WOZDUH_VOLUME_NOT_READY"
	}
	return "WOZDUH_SOURCE_ELIGIBLE"
}

func recoveryVerdict(z VolumeTruthRecoveryReport, apply bool) string {
	if !z.LiveOK {
		return "VOLUME_TRUTH_BLOCKED_SOURCE_CONFLICT"
	}
	if z.ManifestCount > 0 && z.Applied == 0 {
		return "VOLUME_TRUTH_REPAIR_READY_NEEDS_USER_STOP"
	}
	if z.Unresolved > 0 {
		if z.WozduhEligible == "WOZDUH_SOURCE_ELIGIBLE" {
			return "VOLUME_TRUTH_GREEN_WITH_QUARANTINED_OUT_OF_SCOPE_GAPS"
		}
		return "VOLUME_TRUTH_BLOCKED_SOURCE_CONFLICT"
	}
	if apply && z.Applied == z.ManifestCount {
		return "VOLUME_TRUTH_GREEN_RESTORED"
	}
	return "VOLUME_TRUTH_REPAIR_READY_NEEDS_USER_STOP"
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func writeManifestJSONL(path string, rows []RepairManifestRow) error {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func FormatVolumeTruthRecovery(z VolumeTruthRecoveryReport) string {
	var b strings.Builder
	b.WriteString("VOLUME-TRUTH-RECOVERY-1\n========================\n\n")
	b.WriteString("A. PRODUCER\n  ")
	b.WriteString(z.Producer)
	fmt.Fprintf(&b, "\n  live_ok=%v\n\nB. CLASS COUNTS\n", z.LiveOK)
	tfs := make([]string, 0, len(z.ClassCounts))
	for k := range z.ClassCounts {
		tfs = append(tfs, k)
	}
	sort.Strings(tfs)
	for _, tf := range tfs {
		fmt.Fprintf(&b, "  %s %v\n", tf, z.ClassCounts[tf])
	}
	fmt.Fprintf(&b, "\nD. MANIFEST count=%d applied=%d unresolved=%d\n", z.ManifestCount, z.Applied, z.Unresolved)
	fmt.Fprintf(&b, "\nE. WRITE LAW: Ingress equal-authority Volume ASSIGN incoming; SaveKlines volume=excluded.volume; AssignCanonicalVolume SET volume. Forming 1s still sums Qty (MAX not used on closed native).\n")
	fmt.Fprintf(&b, "\nH. WOZDUH %s\nI. VERDICT %s\n", z.WozduhEligible, z.Verdict)
	if len(z.Notes) > 0 {
		b.WriteString("\nNOTES\n")
		for _, n := range z.Notes {
			b.WriteString("  - ")
			b.WriteString(n)
			b.WriteString("\n")
		}
	}
	return b.String()
}
