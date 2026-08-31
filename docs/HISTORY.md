# History (Completed Phases)

**SSOT for:** what was done in the past.  
**Read only on request** (regression archaeology, "how did we fix X?").  
Do **not** load this file for routine feature work — use `ARCHITECTURE.md` + `OPEN_DEBTS.md`.

Full pre-Core-6.0 Russian chronicle lived in `MEMORY.md`; git history retains it. This file is the English condensed canon.

---

## WOZDUH-ACTIVE-1B — persistent live Wozduh demand (Aug 2026) ✅ frozen `1b724ef`

- Per-Frame mask = WS client union OR proven internal demand. ChartOnly internal = 0. Live internal = VolBase|Wt11|Wt22 (`validateDAGShadowLocked` woz_fast/slow). VolCross is not mandatory.
- Nil/empty live `plotIDs` = `WozduhMaskAll`. Sleep NaNs outputs immediately. Wake uses a temp `WozduhNode` over retained closed bars (`dagHistoryCap`), installs only newly activated fields, then SaveState.
- `clientsMu` is released before the Frame lock. Disconnect / TF change recomputes that Frame’s union.
- Measured streams/update: ALL=18, default-visible=6, Live unused=4, ChartOnly unused seconds TFs=0.
- `chart_cache` `finiteOrZero` is not live-mask packing: it maps warmup NaN→0 on `ReplayClosedBars` (compute-all) for legacy JSON history only. Live chart truth is columnar + WS; `/api/state` Plots omit non-finite. Left unchanged (not a demand source).
- Next when asked: **#76 ScoreNodes**. Not DAG-DEMAND / MICRO-DEMAND / further Wozduh polish.

- Per-Frame mask = WS client union OR proven internal demand. ChartOnly internal = 0. Live internal = VolBase|Wt11|Wt22 (`validateDAGShadowLocked` woz_fast/slow). VolCross is not mandatory.
- Nil/empty live `plotIDs` = `WozduhMaskAll`. Sleep NaNs outputs immediately. Wake uses a temp `WozduhNode` over retained closed bars (`dagHistoryCap`), installs only newly activated fields, then SaveState.
- `clientsMu` is released before the Frame lock. Disconnect / TF change recomputes that Frame’s union.
- Measured streams/update: ALL=18, default-visible=6, Live unused=4, ChartOnly unused seconds TFs=0.
- Next when asked: **#76 ScoreNodes**. Not DAG-DEMAND / MICRO-DEMAND.

## WOZDUH-ACTIVE-1A — masked stateless Wozduh history replay (Aug 2026) ✅ frozen `2cd4ca4`

- `/api/history` derives a fixed Wozduh compute mask from requested plot IDs. Same klines/warmup; unused streams do not Update.
- `ReplayClosedBars` remains compute-all.

## WOZDUH-WIRE-1 — subscribe only requested Wozduh plots (Aug 2026) ✅ frozen `0c2ecce`

- Visibility checkboxes → subscribed scalar plot IDs (channels expand to up/mid/dn). No second Subscribe UI.
- History `slots` + per-WS-client live tick filter. DAG still computes all atoms.
- Enable: subscribe → fetch current window → `updatePlots` → one `setData` → reveal. `woz_slow` stays subscribed while hidden (WOZDUH-OWNER-1).

## RSX-VISIBILITY-1 — independent RSX fact visibility (Aug 2026) ✅ frozen `749912f`

- Deleted `div_method` and `show_pivots`. Five FE flags (all default ON, no migration): `show_tv_div`, `show_tv_pivot`, `show_zz_div`, `show_fractal_div`, `show_fractal_pivot`.
- TV facts ungated from UI selection. Paint filters by annotation `source` only. Compositor key: revision + `visibilityMask` + `line_rsx` series. Visibility is not in the RSX fingerprint.
- Visibility does not define factual existence. Next when asked: WOZDUH-ACTIVE-1, not ScoreNodes.

## RSX-SIGNAL-3 — fractal facts, not L/LL/S/SS (Aug 2026) ✅ frozen `c856fef`

- Salvaged `scanRSXFractalHits` / `CheckClassicDivergence`. Published `rsx_fractal_div` (`class_a`/`class_b`/`class_c`) and `rsx_fractal_pivot`.
- Class C was documented but never assigned; classifier now returns it (new price extreme + oscillator double top/bottom). Hits carry Class at detection time (old LL/SS collapsed A vs C).
- `FractalFactsAt` is confirm-bar only (O(lookback)); not a prefix rescan. Not gated on `div_method`. Menu unchanged.

## FALCON-SCORE-CLEAN-1 — delete write-only Falcon divergence scoring (Aug 2026) ✅

- Deleted `SmartDivergenceEngine`, `AnalyzeWithRSX`, `DivSignal`, `Frame.divSignal`, Frame `orangeRsi`.
- Kept `FalconEngine.Evaluate`, HTF Falcon numbers, DAG ZigZag, Frame ZigZag (fib/geometry), TV/ZZ facts and fractal scanners.
- Surgical edits in `indicators/divergence_rsx.go`; did not delete that file.

## SLOT-CLEAN-1 — compact dead DAG slots (Aug 2026) ✅

- Removed `SlotDivScore`, `SlotDivState`, `SlotMicroDivScore`, `SlotTotalScore`. Later slots renumber via iota. No aliases.
- `SlotCount` is live layout only. HistoryBus width follows `SlotCount`. `LongScore` stays 0 with no fake slot to poison.

## LEGACY-SCORE-CLEAN-1 — remove dead DAG score chain (Aug 2026) ✅

- Deleted `MicroPatternNode` and `ScoreNode` from `newDAGRunner`. No parked helpers, no `rsx_micro` facts.
- `SlotMicroDivScore` / `SlotTotalScore` had no DAG writers. Compacted in SLOT-CLEAN-1.
- Falcon / `SmartDivergenceEngine` scoring later deleted in FALCON-SCORE-CLEAN-1.

## RSX-SIGNAL-2B — remove obsolete ZigZag divergence meaning (Aug 2026) ✅

- Deleted `DivergenceNode` and SlotDivState / SlotDivScore consumers. No adapter. Facts stay `rsx_zz_div` only.
- ScoreNode / MicroPatternNode removed later in LEGACY-SCORE-CLEAN-1.
- Slot iota not compacted (`SlotDivScore` / `SlotDivState` are holes). `ann_rsx_div.Slot` → `SlotJurikRSX` (pane ownership; not DivState).
- Not fractal, menu, or ScoreNodes.

## RSX-SIGNAL-2A.1 — ZZ plumbing / live annotation skip (Aug 2026) ✅

- **Frozen** with 2A at `39d6f78`. Idle LIVE a bit cooler. Do not reopen one-walk / collector / revision gate.
- One closed-bar DAG walk (`ReplayClosedBars`) produces Hist + `rsx_zz_div`. No second ZigZag/Jurik pass.
- `ZZDivFactCollector` samples RSX at confirm via `Hist.Get` lookback (not session `ValueAtBar`), then stores `{AnchorAt, IsHigh, Price, RSX}`.
- FE: ColumnarStore `annotationRevision`; compositor skips slice/`setMarkers` when revision + `show_pivots` + `line_rsx` series identity are unchanged.

## RSX-SIGNAL-2A — ZigZag divergence facts (Aug 2026) ✅

- **Frozen.** Commit `39d6f78` (2A + 2A.1 in one commit). `rsx_zz_div` Pattern regular|hidden. Four geometries; equal price/RSX is not a fact. Event on new confirmed swing only. Hidden paint: `H Bull` / `H Bear`. Regular: arrows only.
- ZigZag remains RSX-adaptive (ATR sensitivity). `SlotDivScore` / `SlotDivState` removed from the live path in 2B (iota holes kept).
- Not fractal, not menu, not ScoreNodes.

## RSX-SIGNAL-1.1 — TV arrows + Pine TV pivots (Aug 2026) ✅

- **Frozen.** Smoke green. Commit `b4ac2ae`. Divergence facts unchanged (`rsx_tv_div`). Projector: no Bull/Bear captions; brighter red/green arrows.
- New fact family `rsx_tv_pivot` (`high`/`low`): Pine `pivoth`/`pivotl` on rolling max_rsi/min_rsi, 2-bar confirm, `AnchorAt` two bars back. Blue arrows, no text.
- Show Pivots is presentation-only (FE filter). Detector always runs on TV closed bars.
- Not fractal `P`, not ZigZag, not Class A/C.

## RSX-SIGNAL-1 — Pine TV divergence facts on the chart (Aug 2026) ✅

- **Frozen.** `feat: publish RSX TV divergence facts`. Same TV detector as history (`rsxTVHitAtDisplayBar`); facts use closed-bar OpenTime ms (`AnchorAt` visual, `ConfirmedAt` knowledge). Projector paints Bull/Bear on `ann_rsx_div`. No ScoreFactor / BUY/SELL.
- Also fixed `UpdateKlineTick` new-bar `lastCommittedOpenTime` when the arriving bar is closed — missing pin re-Saved the previous bar and desynced DAG hist vs klines (live facts ≠ ReplayDAG).
- Next chapter was **RSX-SIGNAL-2A** (`rsx_zz_div`).

## RSX-TRUTH-CLEAN-1 — backend RSX is numerical/factual only (Aug 2026) ✅

- **Frozen.** Commit `5f8a290`. Do not reopen Go slope-vs-50 color, backend `rsxColor` wire, or old L/LL/S/SS chart-factory sockets.
- Deleted: `RSXColor` / `JurikRSXColor` / `BuildRSXChart`, replay/backtest/tick color copies, write-only `HTFState.RSXColor`, dead tail-poll / empty marker helpers, `ChartTheme.rsxMarkerStyle`.
- Kept: Jurik + signal math, Snapshot/Restore, DAG/projector scalars, Cap/closed-bar, divergence facts, HTF numeric RSX, FE `rsxStrokeColor` + 30/50/70 scale.
- Trusted inputs for future ScoreNodes: RSX value, RSX signal, HTF RSX numbers, divergence facts, thresholds. Not colors, old labels, or presentation strings.
- `TestGoldenAudit` Binance Forbidden is orthogonal; do not reopen this chapter for it.

---

## SPARSE-ADR010-TIP-1 — sparse HTTP forming tip append-only (Aug 2026) ✅

- **Frozen.** User smoke green: 5s/15s RSX closed points stable; only the forming row moves. Commit `a452cb5`.
- 5s–45s: Replay closed prefix immutable; Frame forming child APPEND-only after `replayClosedOpenMs == Frame.LastCommittedOpenTime()`. No sparse OVERWRITE. Forming identity is Frame lifecycle, not `CloseTime`.
- Native / 1s / HTF remain on `projectViewportFormingTip`. Do not reopen unless a real regression.

---

## SPARSE-LIVE-INGEST-1 — 5s–45s WS ingest uses windowMode (Aug 2026) ✅

- **Frozen.** User-confirmed: LIVE 5s–45s stay alive without mouse movement. Commit `1b67400`.
- `pushLiveTickDelta`: 1s keeps `historyHasNewer` veto; 5s–45s (`isSparseSecondChart`) ingest by `windowMode` only. TimeCamera stays paint-only.
- Do not reopen unless a real regression: `_maybePromoteLiveWindow`, ISLAND-SLIDE, HISTORY-IDLE-PUMP, source continuation, camera.

---

## HISTORY-IDLE-PUMP-1 — human-owned viewport history demand (Aug 2026) ✅

- **Frozen.** Smoke green: parked chart has no viewport-history spam; TF switch is good enough; travel/reversal still pages.
- Law: human movement creates/refreshes one coalesced viewport-history intent. Paint / restore / `setData` / range echo may consume it; they must never invent a new speculative page. `sourceContinue` / `parentResumeAfter` stay automatic (sparse source fold, not camera prefetch).
- `onAfterFlush` → `tryConsumePending()` only. Latest `cause: 'userNav'` wins (clears stale opposite slot; in-flight reverse is accepted). Paint/echo opposite notes stay ignored.
- Commit `636ff55`. Do not start cursor-overlap, last-price-line, or RAF-violation “fixes” from idle-pump symptoms. Temporary `[HISTORY-IDLE]` travel logs are not a remaining defect (1m pages advance head by a full chunk, not ~1 bar).

---

## MICRO-2A — durable sparse 1s history (Aug 2026) ✅

- Closed 1s bars enqueue on the existing PersistenceQueue into `micro_klines` (same `history.db`, same writer). Forming bars and raw aggTrade are not stored.
- `/api/history?tf=1s` reads `micro_klines` (chunk ≤3000 + warmup) with RAM overlay for unflushed closed bars. `hasMore` is true iff an older micro row exists. Empty DB is `no_data`.
- Boot hydrates the 1s Frame from the latest ≤9000 micro rows. Sparse holes are legal. No continuity/Master gate. Retention 24h (startup + hourly DELETE).
- Native `historical_klines` / `archive_gaps` / Ensure / REST / catalog `Persist` unchanged. 5s–45s, ticks, camera, MICRO-2B/2C, LIVE-EDGE-1 not in this slice.

## LIVE-EDGE-1 — live tip 1-bar floor (Aug 2026) ✅

- LIVE + new bar: if `visible.to - tip < 1`, shift range forward by the overflow. Same-bar and HISTORY never move the camera.
- Extra right pad is kept (floor, not a magnet). Width / barSpacing unchanged. One TimeCamera commit.
- LIVE `proposeAfterData` restore uses the same floor (HISTORY→LIVE has no second nudge). All TFs.

---

## MICRO-2C — sparse off-screen live paint (Aug 2026) ✅

- Sparse VIEW=HISTORY: store still ingests; no LWC `series.update` for hidden live bars.
- HISTORY→LIVE: one full paint from store (`restore`/`preserve`), then deltas resume.
- Native/derived Fix F unchanged (HISTORY new bars still delta). Camera math untouched.

---

## MICRO-2B — sparse recovery isolation (Aug 2026) ✅

- Sparse live TFs (`requiresDenseTimeContinuity` false) do not enter TimelineRecovery on Master `timeline_healing` / `timeline_publishable`.
- Browser↔bot reconnect: quiet Shot 10B RAM snapshot + tip handoff; preserve VIEW. No Synchronizing/Retry.
- Seconds buffer coalesces same OpenTime. Ticks identity unchanged. No SQLite, no paint-policy change, no builder rewrite.

---

## MICRO-1.1 — sparse micro live series (Aug 2026) ✅

- FE `appendTick` chronology-gap heal is dense-only (`requiresDenseTimeContinuity`).
- Native + derived time bars stay dense. Seconds / ticks are sparse: missing buckets append, they do not enter timeline healing.
- No synthetic no-trade candles. No 1s REST/SQLite. Camera / watchdog timing unchanged.

---

## MICRO-1 — aggTrade 1s RAM bars (Aug 2026) ✅

- Combined WS adds `BTCUSDT@aggTrade` (same client). Builder folds T/p/q into 1s OHLCV; no raw trade list.
- Own Frame, RAM cap 9000, no SQLite/REST/Ensure/heal. Quiet seconds are honest holes.
- 1s is excluded from Master/native timeline health. Does not unpublish kline Frames.
- `/api/history?tf=1s` is Frame RAM only (`hasMore=false`). Restart starts empty.
- Not in this slice: 5s–45s, tick bars, 1s persist, reconnect backfill.

---

## TF-B — derived time-bar views (Aug 2026) ✅

- Live derived TFs: `2m←1m`, `10m←5m`, `45m←15m`, `3h←1h`. Own Frame; no WS, no SQLite.
- One OHLCV law (`exchange/derived_bars.go`): history fold + live accumulator. Child close = complete distinct closed parents.
- `/api/history` fetches parent then folds. `EnsureHistoryWindow` / heal / persist remain native-only.
- Trading / HTF / backtest stay native. Camera unchanged. Seconds / ticks / 6m not in this slice.
- **`3d` withdrawn** from the project catalog: Binance sells it; our fixed-duration floor is not Binance 3d. No Frame/WS/menu. Revisit only with venue-observed 3d boundaries.

---

## HIST-1.1 — historical tail-coverage HIT (Aug 2026) ✅

- Historical `/api/history` HIT is `last GetWindow OpenTime == CurrentBarOpen(cappedEnd)`. `len>0` is not a hit.
- Stale tail: futures-era Ensure once, then same predicate; still stale → 200 `no_data` / `WINDOW_UNAVAILABLE` (do not pack). Pre-futures stale → no futures REST.
- Live-end `/api/history` unchanged.

## HIST freeze (Aug 2026) ✅

- Live seam smoke after rebuild: `15m` and `1h` @ 2019-09-08 16:00 UTC → HTTP 200 `no_data` / `WINDOW_UNAVAILABLE`.
- Do not reopen HIST unless a regression appears. Next chapter: DATA-1 archive acquisition (not Ensure-as-sync).

## DATA-1A — spot Vision 15m key + import (Aug 2026) ✅

- `history_sync -market=spot` persists `SpotStorageSymbol` (`BTCUSDT_SPOT`); Vision URL still uses `BTCUSDT`.
- Operator import: BTCUSDT spot 15m, 2018-01 through 2019-09. No stitch rewrite.
- Smoke: pre-genesis 15m READY from spot; first UM 15m (17:45) READY; 16:00 listing-day gap still `WINDOW_UNAVAILABLE`.

---

## HIST-0/1/2 — on-demand historical window (Aug 2026) ✅
- Persist via `PersistenceQueue.PersistClosedBarsNow`. Single-flight per symbol+interval.
- HTTP: 200 payload / 200 `no_data` / 502 exchange / 500 sqlite. FE keeps previous island on `NO_DATA`.
- BeforeEnd does not pad post-genesis windows with pre-genesis spot.
- Not done: HIST-3 microscope matrix, DATA-1 bulk/gap policy.

---

- `resolveLiveTfSwitchFetchLimit`: `min(MAX_STORE_BARS, max(HISTORY_CHUNK_LIMIT, ceil(visibleBars)))` on `userTfChange` + LIVE only.
- Scroll/prefetch still `HISTORY_CHUNK_LIMIT` (3000). Camera TF-1 unchanged.

---

## TF-1 phase 2 — delete dead pad clamp (Aug 2026) ✅

- Removed `clampRightPadding` (unused after LIVE phase 1). HISTORY microscope unchanged.

---

## TF-1 phase 1 — LIVE TF restore (Aug 2026) ✅

- LIVE switch keeps `visibleBars`, `barSpacing`, `rightPadding`. Width cap = `MAX_VISIBLE_BARS` via `clampVisibleLogicalWidth` (same as wheel).
- Dropped LIVE 50–400 / pad-50. Capture `from < 0` is not poison. No FreshLive on valid LIVE layout-defer.
- HISTORY microscope unchanged.

---

## SQLITE-2b — single SQLite connection (Aug 2026) ✅

- Idle `database/sql` handles (default 2 idle after boot) blocked `wal_checkpoint(TRUNCATE)` every 5 minutes after MCP-off.
- Fix: `SetMaxOpenConns(1)` / `SetMaxIdleConns(1)`. No pragma change.

---

## SQLITE-2 — MCP off by default (Aug 2026) ✅

- `.cursor/mcp.json` is empty. `sqlite-history` must not autostart on `history.db` (WAL pin).
- Opt-in copy from `.cursor/mcp.json.example` only on user request or SQLITE diagnosis; disable after.
- No WAL pragma change.

---

## SQLITE-1 — WAL reader lifetime audit (Aug 2026) ✅

- Audit only. No WAL pragma change. No SQLITE-2 fix.
- Go `data` readers close `Rows`; checkpoint `busy` is concurrent GetWindow / boot / `QueryKlineCacheBounds`, plus out-of-process `mcp-server-sqlite` on `history.db`.
- Findings: [`docs/OPEN_DEBTS.md`](OPEN_DEBTS.md) SQLITE-1 section.

---

## DOC-1 — archive closed process paperwork (Aug 2026) ✅

- Moved Track B plans/validations, Working Set scorecards/S5–S6 audits, and Cache Lifetime acceptance/evidence into [`docs/archive/`](archive/README.md).
- Law unchanged: `WORKING_SET_CONTRACT.md`, `CACHE_LIFETIME_CONTRACT.md`. Chronicle links below now point at `archive/`.

---

## Chart / camera / hydration freeze — `CHART_FROZEN` (Aug 2026) ✅

- Chapter **closed**. Local annotated tag: `CHART_FROZEN`.
- Frozen runtime: store **9000**, visible **5000**, chunk **3000**, prefetch **25%**. Pressure-only contiguous collector unchanged.
- Fix C–G retained. Patch 2 live-delta 250 ms throttle **rolled back / not active**. Live ingest full-rate.
- Known remaining: LWC live-edge `[Violation]` RAF cost — accepted. Future speed = reduce painted indicator series / LOD, not camera/hydration.
- Out of scope until a real regression: TimeCamera, LEFT/RIGHT hydration redesign, RenderScheduler, render-window, chunk/prefetch/cap experiments, tick throttle, indicator LOD, SQLite/WAL, timestamps, microscope, RAF micro-opts.
- Next chapters (not this freeze): prove-dead cleanup → SQLite/WAL reader audit → TF-switch same market time at same screen X → later LOD → ScoreNodes. S6 / Working Set lifetime **not** reopened here.

---

## #83 Timestamp contract — Go A–D+E2 + FE F2/F3/F5a–F5f (Aug 2026) ✅

- **PASS.** Tag: `TS_CONTRACT_CLEAN`.
- Canonical: ingress/SQLite/Frame = Unix **ms**; wire/LWC/chart = Unix **sec**; camera/geometry = Unix **ms** via explicit `secToMs` / `msToChartSec`.
- Go: load bounds ms-only; persist as supplied; no production `NormalizeKline` / `ensureUnixMillis`; `ChartTimeSec = ms/1000`; genesis clamp E2 on continuous-contract load (spot genesis). Calendar lookback still *computes* 1993; loaders clamp.
- FE: no active magnitude (`1e12`) inference on the data path (F2 primitives, F3 Navigator ms→sec, F5a seconds identity, F5b sec→ms, F5c search query, F5d history fetch end, F5e ruler, F5f trendline).
- Inactive leftover: `app.legacy.js` comments/helpers. Do **not** reopen #83 for that.
- Frozen: chart/camera layout, SQLite schema, genesis calendar. Next work is **not** timestamp cleanup.

---

## S6 Repair Plan — commit-paired ≠ TARGET wall (Aug 2026) ✅

- Plan only: [`WORKING_SET_S6_REPAIR_PLAN.md`](archive/WORKING_SET_S6_REPAIR_PLAN.md). STATUS: **READY**.
- Root cause: commit-paired accept → legacy `_pruneToCount(TARGET)` with no protected set.
- Minimal repair: skip TARGET/HARD_CAP truncation on commit-paired world accept only. No code yet.

## Lifetime & Capacity Constitution freeze (Aug 2026) ✅

- Frozen: [`LIFETIME_CAPACITY_CONSTITUTION.md`](LIFETIME_CAPACITY_CONSTITUTION.md).
- Runtime **FAIL**: commit-paired hydrate → TARGET ≈12k = P-01 + Lifetime/Capacity violation (not excused by commit-paired).
- Capacity **numbers** deferred; browser FPS/heap UNKNOWN. No S6 fix implemented.

## Gate S6 — Lifetime / Product Boundary Audit (Aug 2026) ✅

- Investigation only: [`WORKING_SET_S6.md`](archive/WORKING_SET_S6.md).
- STATUS: **FAIL** (P-01) — HARD_CAP/TARGET still product-visible on commit-paired truncate; preserve+RN can grow past 16k.
- Accordion-on-narrow: **not present** (Lazy Contract). LWC/FPS at 50k–200k: **UNKNOWN**.
- Next: Lifetime/Capacity constitution gate — do not retune HARD_CAP alone.

## Gate S5 — Pressure ≠ Navigation (Aug 2026) ✅

- Report: [`WORKING_SET_S5.md`](archive/WORKING_SET_S5.md).
- STATUS: **PASS** — pressure/prune is not a navigation authority on live Boot.
- Residual: capture-null (WS-01 conditional); 16k wall → next Lifetime/product gate.

## Step 1 Completion Audit — retention coverage re-verify (Aug 2026) ✅

- Re-audit: [`WORKING_SET_STEP1_COVERAGE.md`](archive/WORKING_SET_STEP1_COVERAGE.md) (post Track B).
- Every `_enforceBudget` path classified: Preserve / Commit / Legacy.
- Boot preserve-paired supplies VIEW; commit-paired explicit; capture-null = residual not unclassified.
- Verdict: **Working Set retention layer complete.** No code in this audit.

## Track B Step 3 — Lazy Contract / guaranteed RN lifetime (Jul 2026) ✅

- Report: [`TRACK_B_STEP3.md`](archive/TRACK_B_STEP3.md). Plan: [`TRACK_B_STEP3_PLAN.md`](archive/TRACK_B_STEP3_PLAN.md).
- Law: exploration events ≠ contraction; soft projection restores omitted RN bars.
- Lifetime **≈ 6 / 7** (L2 Pass; L7 not claimed). No validation / Step 4 in this stage.

## Track B Step 3 Plan — Guaranteed RN lifetime / exploration≠contraction (Jul 2026) ✅

- Plan only: [`TRACK_B_STEP3_PLAN.md`](archive/TRACK_B_STEP3_PLAN.md).
- Critical: no new Lifetime abstraction after RN; Step 3 = final Lifetime guarantee before Capacity.
- Law draft: exploration events ≠ contraction; retention lifetime only under pressure or world replace.
- Expected Lifetime ~6 / 7 (L2 Pass; L7 Partial). No code.

## Track B Step 2 Validation (Jul 2026) ✅

- Investigation only: [`TRACK_B_STEP2_VALIDATION.md`](archive/TRACK_B_STEP2_VALIDATION.md).
- Verdict: Retained Neighborhood correct; no WS/Lifetime regressions.
- Lifetime **5 / 7 Pass**; **C2 Pass**. Monotonic RN until world replace = expected (eviction unspecified).
- Recommendation: **Proceed to Track B Step 3** (no Step 2 repair).

## Track B Step 2 — Retained Neighborhood (Jul 2026) ✅

- Report: [`TRACK_B_STEP2.md`](archive/TRACK_B_STEP2.md). Plan: [`TRACK_B_STEP2_PLAN.md`](archive/TRACK_B_STEP2_PLAN.md).
- Law: absorb Mutation into Retained Neighborhood; protect under pressure; reset only on world replacement.
- Lifetime **~5 / 7**; **C2 Pass**. Eviction unspecified (Capacity later). No Step 3.

## Track B Step 2 Plan — Retained Neighborhood / Neighborhood Lifetime (Jul 2026) ✅

- Plan only: [`TRACK_B_STEP2_PLAN.md`](archive/TRACK_B_STEP2_PLAN.md).
- Objective: multi-op fetch→prune→refetch thrash via Retained Neighborhood (not Capacity).
- Preserves WS + Step 1 Mutation Set. No timers/hysteresis/HARD_CAP philosophy.
- Honest post-step Lifetime estimate ~4–5 / 7. No code in this stage.

## Track B Step 1 Validation (Jul 2026) ✅

- Investigation only: [`TRACK_B_STEP1_VALIDATION.md`](archive/TRACK_B_STEP1_VALIDATION.md).
- Verdict: Mutation Set law correct; no WS/Lifetime regressions introduced.
- Lifetime **3 / 7 Pass**; CL-05 / C2 Partial (same-op only).
- Recommendation: **Proceed to Track B Step 2** (no Step 1 repair).
- Evidence delta: [`CACHE_LIFETIME_EVIDENCE.md`](archive/CACHE_LIFETIME_EVIDENCE.md) §6.

## Track B Step 1 — Mutation Set same-op prune guard (Jul 2026) ✅

- Report: [`TRACK_B_STEP1.md`](archive/TRACK_B_STEP1.md). Plan: [`TRACK_B_STEP1_PLAN.md`](archive/TRACK_B_STEP1_PLAN.md).
- Law: successful growth establishes Mutation Set; same-op prune must not remove it.
- Paths: prepend / append new bar / soft applyProjection protected; commit-paired replace excluded.
- Lifetime headline **~2–3 / 7**. No Capacity / Camera / Paint ownership changes.

## Track B Step 1 Plan — Cache Lifetime anti-thrash (Jul 2026) ✅

- Plan only (superseded by implementation above): [`TRACK_B_STEP1_PLAN.md`](archive/TRACK_B_STEP1_PLAN.md).
- Objective: mutation-local neighborhood protection after growth prune (CL-05 / C2); WS untouched.
- Honest post-step Lifetime estimate ~2–3 / 7.

## Stage E5 — Constitution Consistency Audit (Jul 2026) ✅

- Docs-only audit: [`CONSTITUTION_CONSISTENCY_E5.md`](archive/CONSTITUTION_CONSISTENCY_E5.md).
- Cores WS-01…WS-05 / CL-01…CL-07 coherent; no hard contradictions.
- Residuals: WS-05 wording soft tension; acceptance-doc leaks; multi-VIEW / LOD identity gaps.
- Constitutions unchanged. No runtime / Lifetime implementation.

## Stage E4 Freeze — Cache Lifetime Contract (Jul 2026) ✅

- **Frozen:** [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md), [`CACHE_LIFETIME_ACCEPTANCE.md`](archive/CACHE_LIFETIME_ACCEPTANCE.md).
- Evidence ledger closed: [`CACHE_LIFETIME_EVIDENCE.md`](archive/CACHE_LIFETIME_EVIDENCE.md).
- Declaration: Working Set Contract and Cache Lifetime Contract are both frozen constitutions.
- Capacity / Emergency still unspecified. No lifetime implementation in this stage.

## Stage E4 — Cache Lifetime & Pressure Audit (Jul 2026) ✅

- Investigation only: [`CACHE_LIFETIME_EVIDENCE.md`](archive/CACHE_LIFETIME_EVIDENCE.md).
- Proposed (unfrozen): [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md), [`CACHE_LIFETIME_ACCEPTANCE.md`](archive/CACHE_LIFETIME_ACCEPTANCE.md).
- Finding: TARGET/HARD_CAP thrash is lifetime (P-01/P-02), not WS amputate; S6–S7 need CL layer.
- No TARGET/HARD_CAP changes; no implementation.

## Working Set Validation Audit (post Steps 1–2) (Jul 2026) ✅

- Investigation only: canvas `stage-e3-working-set-validation`.
- WS-01…WS-04 pass on main Boot path with VIEW capture; scorecard **~5/7** (S6–S7 Partial).
- HARD_CAP reclassified: lifetime/product on main path; conditional contract hazard if capture-null.
- No implementation in this stage.

## Track A Step 2 — Paint contains VIEW / WS-04 (Jul 2026) ✅

- [`WORKING_SET_STEP2_PAINT.md`](archive/WORKING_SET_STEP2_PAINT.md): `extractWindow` VIEW-covering; no tip-tail amputate.
- S4 pass; E3-02 resolved. Scorecard **4 / 7**. S5–S7 not in this step.

## Track A Step 1 Completion — retention coverage (Jul 2026) ✅

- Audit: [`WORKING_SET_STEP1_COVERAGE.md`](archive/WORKING_SET_STEP1_COVERAGE.md) — every `_enforceBudget` path classified.
- Preserve-paired closed: `appendTick` + soft `applyProjection` now receive VIEW bounds (Boot `captureStoreViewTimes`).
- Commit-paired: `loadDashboard` `replaceMonolith` documented (VIEW omit intentional).
- Verdict: **Working Set retention layer complete.** Step 2 paint not started.

## Track A Step 1 — VIEW-preserving prune (Jul 2026) ✅

- Contract: WS-01, WS-02, WS-03 · Acceptance: S1–S3 (partial) · Evidence: E3-01, E3-04, E3-05 (partial).
- `ColumnarStore._prunePreservingView`: prune only outside captured VIEW time bounds; never shrink below VIEW span.
- Boot `mergeIntoStore` passes `viewFromSec`/`viewToSec` from logical range before prepend.
- Paint tip-tail (S4 / E3-02) and product HARD_CAP wall (S6 / E3-03) not in this step — stopped for review.

## Stage E3.5b — Working Set acceptance gate frozen (Jul 2026) ✅

- [`WORKING_SET_ACCEPTANCE.md`](archive/WORKING_SET_ACCEPTANCE.md): done = Gate 1 (S1–S7) + Gate 2 (blocking E3) + Gate 3 (U1–U7).
- Track name: **Implementation Track A — Working Set Compliance (#69D)** — not “Wave 4.”
- Implementation Rule: each commit maps WS-xx / Sx / E3-xx (contract-driven, not symptom-driven).
- Pre-implementation only; no store/paint code in this stage.

## Stage E3.5 — Working Set Contract frozen (Jul 2026) ✅

- Normative constitution: [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md) (not ADR, not plan, not APIs).
- Freezes WS-01…WS-05, P-01/P-02, ownership, out-of-contract list, acceptance scorecard.
- Follows Stage E3 audit (runtime 0/7 vs contract). No implementation in this stage.

## Phase ADR-028 Wave 3 — Distinguishable history completion (Jul 2026) ✅

- Invariant: every history request completes with a well-defined outcome; EOF ≠ empty/overlap/error.
- `historyHasMore` / EOF only from authoritative `hasMore === false` (after success, or empty body with explicit EOF).
- Zero-overlap and bare empty payload are recoverable; errors never clear EOF flag. True EOF clears Wave 2 pending.
- TimeCamera / ChartAdapter / Boot detector unchanged. E2-03 resolved.

## Phase ADR-028 Wave 2 — Busy never loses intent (Jul 2026) ✅

- Invariant: busy may delay left-history need; silent drop impossible.
- Single pending left-history intent in HydrationOrchestrator (newest supersedes; not a queue).
- Boot detects only (`noteLeftHistoryIntent`); never retries. Consume on Hydration idle + compositor `onAfterFlush` / dashboard end — no poll/rAF/setTimeout retry loops.
- TimeCamera untouched. E2-02 resolved (busy-loss class).

## Phase ADR-028 Wave 1 — Data never changes VIEW (Jul 2026) ✅

- Invariant: Store/Hydration/Boot/Compositor publish facts; only TimeCamera decides VIEW; ChartAdapter sole CameraCommit writer.
- Boot `maybeReturnToLiveFromHistory` no longer maps `windowMode` → `loadDashboard`/FreshLive (E2-01).
- Prepend preserve via `TimeCamera.proposePreserveViewport` from left-time facts; Compositor no longer owns camera policy (E2-04).
- Heal/reconnect may still FreshLive as System via TimeCamera after hydrate (E2-06 partial). Busy-intent / EOF / RESET_LIVE / perf out of scope.

## Phase ADR-028 / ADR-029 D2 — TimeCamera cutover (Jul 2026) ✅

- Ownership migration: TimeCamera is sole live navigation policy owner; ViewportManager demoted to capture/translate (+ Debt #80 layout deferral helper).
- TF path: `capture → TF change → paint → observeCommittedWorld → propose → CameraCommit → ChartAdapter`.
- LIVE: tip sticky + healthy zoom + clamped rightPadding. HISTORY: DataResolve centerTime → nearest logical; never jumps to live.
- Retired: VM live restore policy, compositor direct camera fallbacks, boot `syncVisibleLogicalRange` shim. Same-TF `RESET_LIVE` not implemented (TODO only).

## Phase ADR-028 D1.5 — Shadow fidelity observe-only (Jul 2026) ✅

- After production setData + CameraCommit, ChartCompositor publishes `tipLogical` + `timesSec` via `TimeCamera.observeCommittedWorld`.
- Shadow fills ViewIntent + centerTime from committed world; no tip←rightOffset inference.
- ViewportManager still authoritative — zero behavior / ownership / LWC-write changes. D2 not started.

## Phase ADR-028 D1 — Shadow TimeCamera (Jul 2026) ✅

- Shadow ViewIntent + ViewGeometry inside TimeCamera; capture after commit only.
- Pure helpers (classify / center / clamp); DataResolve seam unbound.
- ViewportManager still paint authority — **zero behavior change**. D2 not started.

## Phase ADR-028 / ADR-029 — Navigation ownership + TF transition (Jul 2026) ✅ docs

- **ADR-028:** TimeCamera owns user VIEW (ViewIntent + ViewGeometry); one CameraCommit write path; ViewportManager → translator; no bar lookup in TimeCamera.
- **ADR-029:** TF transition protocol — LIVE sticky + clamped breathing room; HISTORY center-time + zoom; DataResolve for nearest logical.
- Constitution laws: TF ≠ navigation; navigation ≠ density; one camera write path.
- Runtime Phase D1/D2 **not started** — docs freeze only.

## Phase Architecture Freeze — Core Ownership Model (Jul 2026) ✅

- Frontend chart architecture declared stable (ADR-023 / 026 / 027 + Ownership Audit).
- Permanent **Core Ownership Model (Jeweler Constitution)** section in `docs/ARCHITECTURE.md` (not a separate OWNERSHIP.md).
- Strategy shift: stop inventing architecture; polish behavior within frozen owners. Navigation later extended by ADR-028/029.

## Phase ADR-027 P4 — Ownership Audit & Consolidation (Jul 2026) ✅

- Live path: one owner per concern (DisplayTimeline / TimelineDecoration / ChartAdapter / CrosshairController / PaneLayout). No Group A duplicates requiring removal.
- Consolidated: private `applyBottomAxisLabel` naming (was `ensureBottomAxisTimeLabel` — not a public concept).
- Kept: Group B translations; quarantined `adapter.legacy.js` peer sync (dormant; not loaded).
- Category C left for future ADRs: global destroy / `_disposers`, Primitive peer guide.

## Phase ADR-027 — Decoration Plane / display timeline (Jul 2026) ✅

- **Invariant:** `candleSeries` = real market candles only; decoration never enters the market-data plane.
- **Two planes:** Market (`ColumnarStore` / DDR / `candleSeries` / `update(tip)`) vs Decoration (`DisplayTimeline` math + sealed `TimelineDecoration` LWC whitespace).
- ChartAdapter composes only — rejected `setData(real+whitespace)` (broke tip `update()` invariant).
- Decoration attached on all live panes; native axis ticks + crosshair time labels in the future strip.
- **Polish:** bottom-axis time label is rendered on the ADR-023 axis owner from synchronized crosshair state — not owned by the hovered pane.
- Group B (HTML peer guide, mid-Y) left as encapsulated adapter translation. Docs freeze = Phase 3; Ownership Audit & Consolidation = Phase 4.

## Phase ADR-026 — Crosshair empty-space sync (Jul 2026) ✅

- Peer sync payload `{ logical, time? }`; clear only when logical missing.
- Native `setCrosshairPosition` when time exists; else ChartAdapter logical guide (no fake time).

## Phase Debt #91 — Datetime chrome (Jul 2026) ✅

- `localization.timeFormatter`: detailed local-TZ crosshair (`dd MMM yyyy HH:mm`).
- `tickMarkFormatter`: minimal labels by LWC `TickMarkType` (not `currentTf`).
- `vertLineChrome()` shared by create options + `applyHorzVisibility` (peer sync cannot wipe contrast).
- No new controllers/DOM; ADR-023 ownership unchanged.

## Phase ADR-025 — TV Ruler complete (Jul 2026) ✅

- Anchors `logical+price` (time optional); two-click FSM; finite rectangle; `RulerMetrics` tooltip (no Vol).
- Empty/future space via `coordinateToLogical`; bars from logical Δ; Esc/right-click cancel → stay armed.

## Phase ADR-024 / ADR-021 P3 — InteractionController (Jul 2026) ✅

- Thin router: pointer / range / crosshair-time → TimeCamera + CrosshairController.
- ChartAdapter is LWC-only for interaction; no behavior change. Ruler can plug in next.

## Phase ADR-023 — Single bottom time axis (Jul 2026) ✅

- PaneLayout owns bottom-axis HostID; LayoutController marks DOM + calls `ChartAdapter.setBottomTimeAxis`.
- Non-owner panes: `timeScale.visible: false` (no blank reserved strip). Splitter gutters kept.

## Phase ADR-022 / #68 — Oscillator scaleContribution (Jul 2026) ✅

- Per DDR component `scaleContribution` (`bounded` / `ignore` / `dynamic`) → LWC `autoscaleInfoProvider`.
- ScaleController unchanged (Auto≡autoScale). RSX/`woz_fast` bounded `[-5,105]`; peers `ignore`.

## Phase ADR-021 — Hover ownership (Jul 2026) ✅

- `hoveredHostId` from wrapper `pointerenter`/`pointerleave` only (`data-pane-host`).
- `syncTime` is time-only; cannot steal hover. Peers: local Y + horz re-asserted off.

## Phase ADR-021 / #49 P2 — CrosshairController (Jul 2026) ✅

- Hover policy: horz only on hovered HostID; peers get time-aligned vert with **local** Y only.
- Never mutates TimeCamera. Module: `web/ui/crosshair-controller.js`.

## Phase ADR-021 / #49 P0–P1 — TimeCamera (Jul 2026) ✅

- `TimeCamera` atomic commit + echo lock; ChartAdapter apply-only for timeline.
- Deleted `attachSlaveWheelProxy`; footers native pan/zoom; all panes propose. Crosshair unchanged (P2 later).

## Phase ADR-020 / #91 — Footer Y-scale Manual parity (Jul 2026) ✅

- Slave charts: Y drag/wheel/dblclick reset enabled; time-pan proxy skips price-scale hit zone.
- ScaleController already tracked Auto OFF per HostID; restore `repairScalePrefs` unchanged (incomplete Manual → Auto ON on reload).

## Phase ADR-020 / #91 — Scale prefs self-sufficiency repair (Jul 2026) ✅

- Pure `repairScalePrefs` / `hasValidManualRange`: Auto OFF without `manualRange` → Auto ON (keep Log); dirty → one persist.
- In-session Manual still works; incomplete Manual is not valid across reload until range persistence ships.

## Phase ADR-020 / #91 P1 — ScaleController HostID (Jul 2026) ✅

- Generic `register({ context, hostId, chart, allowLog, scaleGroup? })`; v3 prefs migrate from v2 price global.
- Price Auto+Log; footers Auto-only; prefs survive hide/show. Debt **#91** P1. Regression: `web/scale_controller_test.js`.
- Corrections: `toggleLog` gated by binding/`data-allow-log` (no hostId hardcode); prefs keyed by hostId across contexts; dormant `scaleGroup` only.

## Phase ADR-019 / #90 P5 — Fullscreen UX (Jul 2026) ✅

- Dblclick empty LWC plot → `PaneLayout.toggleFullscreen`; Escape / dblclick again clears.
- Ignores legends, scales, splitters, buttons. LayoutController renders `.fullscreen-pane` from state only.
- Debt **#90** Phase 5.

## Phase ADR-019 / #90 P4 — Pane Reordering (Jul 2026) ✅

- Legend drag → `PaneLayout.moveHostBefore` / `setOrder`; Grid rebuilds from `order`; heights/visible/fullscreen untouched.
- Drop-once commit (no live DOM reordering as SSOT). Debt **#90** Phase 4.

## Phase ADR-019 / #90 P3 — Adjustable Pane Heights (Jul 2026) ✅

- Splitter drag → `PaneLayout.setFooterHeight` (px only); price stays `1fr`.
- Mid-drag: update `grid-template-rows` only (keep pointer capture); resize coalesced to one rAF.
- Debt **#90** Phase 3. Regressions: `web/pane_layout_test.js`, `web/layout_controller_test.js`.

## Phase ADR-019 / #90 P2 — LayoutController CSS Grid (Jul 2026) ✅

- `LayoutController` applies Grid from `PaneLayout`: price `minmax(120px,1fr)`, footers px, dynamic gutters only between visible panes.
- Ind + legend eye collapse whole footer; price absorbs space; explicit LWC resize after apply.
- Debt **#90** Phase 2. Regression: `web/layout_controller_test.js`.

## Phase ADR-019 / #90 P1 — PaneLayout Foundation (Jul 2026) ✅

- FE SSOT: `web/ui/pane-layout.js` — `visible` / `order` / `footerHeights` (px) / `fullscreenPaneId`.
- Restore = versioned localStorage ∩ Manifest HostIDs; Ind menu generated from catalog (no hardcodes).
- No Grid / drag / `setHostActive` yet. Debt **#90** Phase 1. Regression: `web/pane_layout_test.js`.

## Phase ADR-018 — TimelineRecovery UX (Jul 2026) ✅

- FE owner: `web/timeline-recovery.js` (LIVE ↔ HEALING); idempotent `enter`; one-shot 25s watchdog.
- Non-blocking `#timeline-sync-badge`; chart stays visible. Heal no longer drives `#orderflow-buffer`.
- Debt **#89**. Regression: `web/timeline_recovery_test.js`.

## Phase ADR-017 / B3.0 — Timeline Heal Continuity (Jul 2026) ✅

- Root: Cap grace REST + pending tip jump + ungated flush → one-bar hole; publishable too early.
- Fix: Exact closed-gap fill (`FetchClosedRangePagesExact`) before flush; publishable only after Frame contiguity.
- Debt **#88**. Regression: `market/timeline_heal_b3_test.go`. Buffering 75s UX not in this phase.

## Probe dormancy — TipSSOT / ProjCont (Jul 2026)

- After ADR-016, continuous `[TipSSOT]` / `[ProjCont]` runtime logs default **OFF**.
- Opt-in: `DEBUG_TIP_SSOT=1`, `DEBUG_PROJ_CONT=1`; FE `DEBUG_PROJ_CONT` localStorage / query.
- Kept always-on: TransportDiag, Self-Healing, MemoryBudget. Regression tests untouched.

## Phase ADR-016 — Replay Lifecycle Ownership (Jul 2026) ✅

- Frame `replayStreamingLocked` / `warmupStreaming`: split by Cap forming predicate → closed `isClosed=true` + commit → optional forming `isClosed=false` (never commit).
- Root cause of post-settings tip jump: forming tip was replayed as closed then re-opened by WS (double Jurik).
- History Cap Replay unchanged (closed-only). Debt **#87**. Regression: `market/replay_lifecycle_test.go`.

## Phase B2.1 — Atomic Projection Apply (Jul 2026) ✅

- Soft RSX settings path: `applyProjection(columnar)` instead of `updatePlots` (Case 2 lost N+1 tip).
- Camera: capture/restore — never `viewport:fresh` (ADR-014).
- Regression: `web/projection_apply_test.js`. Debt #86 B2.1.

## Phase B2.2 — Projector OVERWRITE mode (Jul 2026) ✅

- `projectViewportFormingTip`: APPEND (`frameOpen > histLast`) + OVERWRITE (`frameOpen == histLast`, replace tip OHLC/plots from Frame).
- Completes ADR-010 contract (handoff OVERWRITE was documented, only APPEND implemented).
- ADR-015 probe: skip on new bar / timeline heal / elapsed > 2s.

---

- **ADR-013:** `ChangeImpact` + `RSXImpactOfChange`; SetRSX* only inside IndicatorReplay path; DivMethod = AnnotationOnly (Jurik untouched).
- **ADR-014:** settings apply = soft `updatePlots` / `mode: 'indicators'`; never `viewport: 'fresh'`.
- FE: 200ms debounce, Save/outside flush, AbortController + sync seq, fingerprint skip when synced.
- Debt **#85** closed. Remaining tip vs TV → forming-bar Model 2 only.

---

- Split knowledge SSOT: Protocol / Role (always-on) vs Architecture / Debts / History / Decisions.
- Controlled English rewrite; `MEMORY.md` → index; README = landing.
- **6.1 polish (no new files):** Project identity, When NOT to refactor, harder Anti-patterns,
  Role before-coding checklist, ADR Rejected+Reason, README read order.

## Debt #69A — Frontend Memory Budget (Jul 2026) ✅

- `ColumnarStore` TARGET/HARD_CAP + atomic prune; `windowMode` live/history.
- WS + gap-heal gated in history mode; return-to-live → `loadDashboard`.
- Cache button → Reload Dashboard (HTF + FE hydrate).
- Protocol invariants: FE bounded viewport; viewport never mutates market series.

## Debt #69C — Focal-time prune (Jul 2026) ✅

- `ColumnarStore.pruneDirectionFromFocal` / `resolveBudgetPruneDirection`.
- `prependMonolith(opts)` uses viewport `centerTimeMs` + `isAtRightEdge` from boot.
- Drop side farthest from focal; `atLiveEdge` forces FROM_OLDEST.
- Open: **69D** sliding window + viewport-centered `extractWindow`.

## Debt #80 — ViewportManager.restore 0×0 (Jul 2026) ✅

- Root: `setVisibleLogicalRange` on 0×0 host → LWC NaN barSpacing → blank chart (Core 4.10 class).
- Fix: layout guard + fresh `applyOptions` fallback + ResizeObserver deferred restore;
  ChartAdapter no-op; TF switch with null capture → fresh (no synthetic restore).

## Debt #67 — Closed-bar Boundary + Viewport Tip (Jul 2026) ✅

- Falsified: Warmup 400vs3000, Replay≡Frame math, Snapshot/commit, live continuation.
- Proven: `GetWindow(Now)` tip ≠ Cap/Frame tip during `KlineSettleGraceMs` → ΔRSX 0.8–2.7.
- Fix (ADR-009): `GetWindow` → `resolveClosedBarBoundary` = `CapKlineEndToLastClosed`.
- Verify: Cap-aligned OHLCV+RSX bit-identical; `TestClosedBoundarySSOT` locks the invariant.
- **ADR-010:** Viewport seeds Frame forming tip after closed Replay (TV Model 2); F5 OVERWRITE.
- Open: none for tip product branch; #68 scale bounds next.

## Debt #81 — Timeline Publish Gate (Jul 2026) ✅

- Invariant: WS Connected ≠ History Reconciled ≠ Timeline Publishable.
- Phases A–D: exchange reconnect hooks; Runtime pending + gate; WS `timeline_*`; FE await.
- P0: gap threshold `> 1×interval`; forced REST on `ReconcileTimeline`; publishable only if fetch OK + contiguous.
- Manual: offline→online chart contiguous (Buffering ≈ forced multi-TF REST). P1/P2 deferred.

---

## Core 5.0 — Data Plane SSOT (Phases A–G) ✅

**Goal:** one candle = one lifecycle = one canonical version. Trigger: Golden Audit volume drift + boot race + merge duplication (#19).

| Phase | Summary |
|-------|---------|
| **A Ingress SSOT** | `exchange/ingress.go`: Authority, merge, Validate, metrics, edge ledger. Deleted duplicate merges. |
| **B Boundary + WAL** | REST Grace 5s; monotonic UPSERT firewall; WAL checkpoint. |
| **C Boot FSM** | WS first → load → reconcile via `routeTick` → live. |
| **D Bar Source Seam** | Purge micro-candles; tick TF menu left as socket for TickBarBuilder. |
| **E Repair + repo** | `cmd/repair_volumes`; cleanup binaries/backups; `.gitignore`. |
| **F Legacy purge** | Delete ScoreEngine/trade FSM/matrix/risk/A-B CLI; keep sockets; ChartOnly default. |
| **G Rebrand** | `Frame`/`Runtime`; `market/` + `decision/`; `strategy/` beacon; DAG audit. |

---

## Core 4.4–4.10 — Continuity chain ✅

Trigger: chart holes + RSX tip spike vs TradingView at History/Live boundary.

| Shot | Summary |
|------|---------|
| 4.3 | Golden Audit: OHLC+RSX bit-identical vs REST; volume differed |
| 4.4–4.5 | FE gap-detect + WS reconnect → `loadDashboard` (no silent glue) |
| 4.6 | Tracer: bad `line_rsx` already wrong on WS |
| 4.7–4.8 | Root cause: double DAG commit on bar boundary; `lastCommittedOpenTime` fix |
| 4.9 | TF interval shim was hardcoded 60s → false gap storms |
| 4.10 | Cold boot camera: width-independent `applyOptions` only |

---

## Core 4.0 — Great Purge ✅

Wozduh math → DAG bus (`WozduhNode` + slots); Falcon Evaluate gated; tip via plots; TV floating UI (legend chrome only). Keep `falcon.go` until ScoreNodes (#76).

---

## Core 3.5 — Projection (11A–11E) ✅

Tip Ownership; `projectionEpoch`; Atomic TF handoff; Sticky Live Edge / Microscope; delta integrity. **TF mechanics CLOSED.**

---

## Core 3.0 — Frontend (10A–10B) ✅

`ScaleController` Auto/Log SSOT; zero-gap WS-first handoff around monolith replace.

---

## Core 2.3 — Data Foundation (Shots 9A–9J) ✅

| Shot | Summary |
|------|---------|
| 9A | `HistoryProvider.GetWindow` = SQLite ∪ RAM |
| 9B | Atomic `BroadcastChartTick` all TFs |
| 9C | `PersistenceQueue` sole runtime UPSERT |
| 9D | SQLite tip catch-up independent of RAM |
| 9E | Sterile `FetchClosedRange`; delete synthesize path |
| 9F | `EngineMode` ChartOnly \| Live |
| 9H–9J | Navigators/annotations/header from DAG; Falcon off delivery path |

**UI paint laws (still current):** ChartAdapter-only LWC; paint via window; RenderScheduler sole initiator; Store → Window → Geometry → Series → Adapter.

---

## Core 2.0 — DDR / DAG shadow ✅

DAG dual-write; columnar history hydration; frontend DDR cutover; projector annotations path.

---

## Earlier frontend SSOT (Phase 19.5–20 / 28)

Pre-fetch assembly; ViewportManager time anchors; store seal guards; backtest view-plane split; black screen root cause = HTML tab nesting (`#tab-backtest` under hidden `#tab-live`).

---

## How to use this file

- Hunting a regression → search phase/shot name, then open the files listed in that era's commits.
- Asking "why?" → prefer `DECISIONS.md`; use History for chronology.
- Closed debt numbers → mentioned in phase tables; open ones live only in `OPEN_DEBTS.md`.
