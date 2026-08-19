# Gate — History-Roam Continuity Audit

**STATUS:** PASS (investigation complete; both residuals have reproducible chains)  
**Kind:** Investigation only. No code. Track C paint not reopened.

---

## HISTORY-ARM

### reproduction
Live BTCUSDT 15m (`?hist-audit=1`), wheel-arm, set `from=5` (&lt; `LIVE_HISTORY_SCROLL_THRESHOLD=50`):

| Step | bars | from | underThresh | grew? |
|------|------|------|-------------|-------|
| before | 3001 | 2850 | no | — |
| set-left-5 | 3001 | 5 | yes | — |
| after-chunk | 6000 | 5 | yes | +2999 |
| hold 5s (no scroll) | 6000 | **3004** | **no** | **0** |
| disarm + from=2 | 8999→same | 2 | yes | **0** |
| re-arm + from=3 | 11998 | 3002 | no | +2999 |

### root cause
Two concrete Boot/Hydration gates, not a lost Wave-2 pending while busy:

1. **Post-chunk logical remapping clears continuation**  
   Successful prepend → paint → `proposePreserveViewport` remaps the same left **time** to a higher logical `from` (≈ `from + addedBars`).  
   `scheduleHistoryLoad` / `shouldLoad` require `range.from < 50`. Remapped `from` (e.g. 3004) fails the gate.  
   Pending was already cleared when the in-flight request started (`_pendingLeftIntent = null` before fetch).  
   After completion, `tryConsumePending()` is a no-op (no pending). The preserve-driven range event does **not** re-note (from ≥ 50).  
   **Result:** loading stops until the user scrolls again so `from < 50`. Holding still after a chunk does not auto-continue even with `hasMore=true`.

2. **`liveHistoryScrollArmed` is wheel/pointer-only**  
   `disarmLiveHistoryScroll` (TF / clear / prepare handoff) + programmatic range changes without wheel → `shouldLoad` / `scheduleHistoryLoad` return false even when `from < 50` (proven: disarm blocked a left-threshold range).

Busy delay itself is healthy (Wave 2). The stall is **missing re-arm of left-void need after preserve remaps indices**, plus the **arm bit**.

### evidence
- `web/boot.js`: `scheduleHistoryLoad`, `shouldLoad` (`liveHistoryScrollArmed`, `from >= THRESHOLD`), `attachLiveHistoryScrollArm` (wheel/pointer only), `disarmLiveHistoryScroll`
- `web/hydration-orchestrator.js`: clear pending on start (`_tryStartPending`); on consume, `shouldLoad(liveRange)` false → **cancel pending**; `tryConsumePending` after prepend/paint
- `web/ui/time-camera.js`: `proposePreserveViewport` remaps by left time → new logical `from`
- Live probe table above

### affected invariant
- Wave 2 “busy must never lose” **holds** for in-flight busy.  
- Gap: **after success**, exploration continuation is not a remembered need — it depends on a new detector event with `from < 50`. Preserve remapping often prevents that event from qualifying.

```text
scroll left (from<50, armed)
  → noteLeftHistoryIntent → clear pending → fetch/merge
  → markDirty prepend → setData → proposePreserveViewport
  → logical from ≈ oldFrom + added  (≥50)
  → tryConsumePending: no pending
  → rangeChange scheduleHistoryLoad: from≥50 → no-op
  → STALL until user scrolls to from<50 again (and armed)
```

---

## SPARSE-PAINT

### reproduction
Observed during Track C live runs and heal:

1. **Timeline heal → full hydrate** (strongest product flash)  
2. **Prepend paint ordering** (brief, same stack / rAF)

### root cause

**A — Heal recovery (primary “flash” of sparse/incomplete vs prior roam)**  
```text
timeline_healing / gap / WS reconnect
  → TimelineRecovery.enter → tick buffer only (store kept)
  → publishable / watchdog Retry
  → onRecovered → loadDashboard()   // NOT keepProjection
  → replaceMonolith(commitPaired) tip hydrate (~HISTORY_CHUNK_LIMIT)
  → markDirty full viewport:fresh
  → camera FreshLive
```
Roamed ~39k world is replaced by a ~3k tip monolith + fresh camera. User sees a sudden sparse tip frame (and “Synchronizing…” badge). Not missing SQLite data — **intentional world replace** on heal.

**B — Prepend / large setData intermediate**  
```text
merge prepend → markDirty(prepend)
  → rAF F1: applyFullData(full retained series) then proposePreserveViewport
```
Preserve is same turn after `setData`, but LWC may paint one frame before camera commit; large Track C full-series `setData` can be heavy. Classify as **expected short intermediate / race of paint vs camera**, not missing store bars (store already merged).

**C — Lonely-candle skip** (`barCount < 2`) skips paint — only if store was cleared (heal/TF clear path), not normal prepend.

### evidence
- `web/boot.js` `initTimelineRecovery` → `onRecovered → loadDashboard`; `loadDashboard` commitPaired tip replace + `viewport:'fresh'`
- `web/timeline-recovery.js` badge “Synchronizing live data…”
- `web/chart-compositor.js` `_flushPrepend`: setData then preserve; lonely guard
- `web/render-scheduler.js` prepend via double rAF F1/F2 (F2 no-op)

### affected invariant
- WS-04 / Track C: retained bars are present before paint; flash is **navigation/world-replace (heal)** or **transient LWC frame**, not a reintroduced 15k tip-window.
- Wave 1: heal path **does** FreshLive via `loadDashboard` — Data path allowed for recovery, but it is a user-visible discontinuity after long roam.

```text
SYMPTOM: sparse/tip flash after sync
→ trigger: publishable/reconnect/gap heal
→ transition: loadDashboard commitPaired tip world + FreshLive
→ layer: Boot + TimelineRecovery (not Compositor window)
→ why invariants don’t block: recovery explicitly replaces world; keepProjection used for TF handoff only
```

---

## LEGACY FOUND

| Item | Role |
|------|------|
| `LIVE_HISTORY_SCROLL_THRESHOLD` on **logical `from`** | Fragile after preserve index shift |
| `liveHistoryScrollArmed` wheel/pointer-only | Programmatic/CDP pan cannot load after disarm |
| Hydration `debounceTimer` 200ms | Existing debounce (not a retry loop) |
| `maybeReturnToLiveFromHistory` still subscribed | No-op (Wave 1) — dead listener |
| `app.legacy.js` duplicate history-arm | Not live Boot |
| Heal → `loadDashboard` FreshLive | Pre–keepProjection recovery; wipes roam |
| `windowMode==='history'` blocks `appendTick` | Separate tip-ingest gate after FROM_NEWEST |

Not found as cause: duplicate retry queues, paint soft 15k window (removed Track C), MemoryManager.

---

## TESTS

- Existing: Wave 2 pending-intent, Wave 3 EOF, TimeCamera preserve, Track C paint selection — not re-run as regressions for this audit (no code).  
- New: live CDP continuity probe (table above) — investigation only.

---

## RECOMMENDATION

**Done (later gates):** FE history continuation after prepend; server count-based history window retrieval (see FOLLOW-UP below).

Heal sparse flash remains a **separate** product choice (`loadDashboard` vs keepProjection soft recover).

---

## BOUNDARY AUDIT — zero overlap after continuation (Aug 2026)

**STATUS:** PASS (investigation; root cause proven; no code this gate)

**HEADLINE:** After the first left chunk reaches the archive gap edge, time-span retrieval returns only the boundary candle (`t == FE oldest`) → `prependMonolith` adds 0; count-based `BeforeEnd` returns 2999 mergeable older bars at the same cursor.

**Proven timestamps (BTCUSDT 1m, `history.db`):**
- Tip-hydrate window (~3000): FE oldest ≈ `1786096380` sec (`1786096380000` ms)
- Gap edge: `1786077480000` ms
- At gap edge, OLD `end−N×interval`: returned count **1**, oldest=newest=`1786077480000` → mergeable older **0**
- At gap edge, NEW `BeforeEnd`: returned display **3000**, oldest `1785429300000`, newest `1786077480000` → mergeable older **2999**

**FE “zero overlap” meaning:** misnomer — `prependMonolith` requires `t < store.first` (strictly older). Shared overlap candle is **not** required. Adjacent older chunks are valid.

**TransportDiag `history loaded` / `tip handoff`:** `loadDashboard` full hydrate + first WS tick — **not** prepend continuation.

**SQLITE_BUSY:** PersistenceQueue write contention; `PRAGMA busy_timeout=5000`; BeforeEnd reads succeed — **NOT CAUSAL**.

---

**STATUS:** PASS (server retrieval; FE continuation unchanged)

**Defect:** `loadRESTKlinesFromStore` used `end - N×interval` time-span load. Across a multi-day 1m archive gap, the window contained only the boundary candle while `hasMore` stayed true (older rows exist).

**Fix:** `data.LoadKlinesBeforeEnd` + `exchange.LoadContinuousContractBeforeEnd` (count-based, gap-tolerant). GetWindow / history-chunk paths call BeforeEnd. Zero-progress ≠ EOF (Wave 3 unchanged).

**Proof:** `data/history_db_before_end_test.go`, `exchange/continuous_contract_before_end_test.go`, `TestLiveArchive_1mFormerGap_BeforeEndProgress`.

---

## ARCHIVE INTEGRITY GATE (Aug 2026)

**Data:** BTCUSDT 1m hole `2026-08-01 18:34` → `2026-08-07 04:37` restored via `cmd/repair_archive_gap` (7804 bars). Binance source confirmed.

**Persistence:** `PersistenceQueue` retries SQLITE_BUSY; Enqueue no longer drop-on-full; exhausted retries spill + hard log (never silent discard).

**Continuity:** `archive_gaps` ledger + edge notes on persist + `NoteGapsFromOpenTimes` on BeforeEnd chunks; catch-up heals tip **and** known gaps. Tip freshness ≠ completeness (H3).

**Retrieval:** live GetWindow / history-chunk = `LoadContinuousContractBeforeEnd` only.

---

## STOP
