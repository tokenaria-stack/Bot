# Open Debts

**SSOT for:** unfinished work and NEXT priorities.  
Completed items live in `docs/HISTORY.md` — do not re-list them here.

Update this file when a debt opens, closes, or changes priority.

---

## Chart freeze

**Status:** Frozen (tag `CHART_FROZEN`). Caps: store **9000** / visible **5000** / chunk **3000** / prefetch **25%**. Fix C–G kept. Patch 2 not active.

Do **not** change TimeCamera, hydration, RenderScheduler, store/render-window, chunk/prefetch/caps, or tick throttling unless a real regression appears.

**HISTORY-IDLE-PUMP-1 ✅ frozen** (`636ff55`). Do not reopen viewport-history demand, cursor-overlap, or last-price-line work from idle-pump / rAF symptoms.

**SPARSE-LIVE-INGEST-1 ✅ frozen** (`1b67400`). Do not reopen 5s–45s WS ingest, `_maybePromoteLiveWindow`, ISLAND-SLIDE, or `historyHasNewer` producers unless a real regression appears.

**SPARSE-ADR010-TIP-1 ✅ frozen** (`a452cb5`). Do not reopen `projectSparseSecondFormingTip`, sparse OVERWRITE, or calendar `isFormingKline` on 5s–45s. Native `projectViewportFormingTip` stays isolated.

**After freeze (cleanup rule):** prove dead → delete → tests → smoke → checkpoint. No speculative deletion of TimeCamera / hydration / prune.

**NEXT order (do not start inside this freeze):**

1. Dead-code / legacy cleanup ✅ CLEAN-1–4 + DOC-1  
2. SQLite/WAL — **SQLITE-1 ✅** + **SQLITE-2 ✅** (MCP off) + **SQLITE-2b ✅** (single-conn pool; idle handles were pinning TRUNCATE)  
3. TF-switch UX — **TF-1 ✅** + **TF-2A ✅**. **HIST frozen** (0/1/2 + 1.1 + 3). **DATA-1A ✅** (spot `history_sync` key + BTCUSDT 15m Vision Jan 2018–Sep 2019). **DATA-1B** next: choose ledger cleanup vs listing-day seam ownership from smoke (do not assume 16:00 becomes READY).  
4. FE paint skip + Wozduh demand: HIDDEN-RENDER-SKIP-1 + WOZDUH-OWNER-1 + **WOZDUH-WIRE-1 frozen** (`0c2ecce`) + **WOZDUH-ACTIVE-1A frozen** (`2cd4ca4`) + **WOZDUH-ACTIVE-1B frozen** (`1b724ef`). **Do not reopen Wozduh.**  
5. **DAG-DEMAND-1 ✅ frozen** (`0837c77`). **FORECAST-SPEC-1 ✅** `5afabfc`+`0ed000d`. **FEATURE-TAPE-1A ✅ frozen** (`b88bcd2`). **FEATURE-TAPE-1B ✅ frozen** (`6715718`). **ATR-TRUTH-1 ✅ frozen** (`84124a0`). **LABEL-SET-1A ✅ frozen** (`690d0be` + `1433626`). **LABEL-SET-1B ✅ frozen** (`8e88844`). **RSX-TV-ONE-BRAIN-1 ✅ frozen** (`4688160`). **FEATURE-TAPE-RSX-REGEN-1 ✅**. **RESEARCH-DATASET-1 ✅ frozen** (`f311203`). Next when asked: **VALIDATION-PLAN-1**. **TARGET-RESOLUTION-2** deferred.

**RSX-TRUTH-CLEAN-1 ✅ frozen** (`5f8a290`). Backend RSX is numerical/factual only. Live paint stays FE. Do not reopen slope-vs-50 color, `rsxColor` wire, or empty L/LL/S/SS sockets.

**RSX-SIGNAL-1 ✅ frozen** (`1c353e0`). Pine TV divergence facts (`rsx_tv_div`).

**RSX-SIGNAL-1.1 ✅ frozen** (`b4ac2ae`). TV family closed.

**RSX-SIGNAL-2A ✅ frozen** + **RSX-SIGNAL-2A.1 ✅ frozen** (`39d6f78`). ZigZag facts + one-walk/wrap collector + annotation revision gate. Do **not** reopen 2A plumbing.

**RSX-SIGNAL-2B ✅** — obsolete ZigZag DivState/DivScore path deleted.

**LEGACY-SCORE-CLEAN-1 ✅** — DAG MicroPatternNode / ScoreNode deleted.

**SLOT-CLEAN-1 ✅** — compacted dead slots.

**FALCON-SCORE-CLEAN-1 ✅** — write-only SmartDivergenceEngine / `divSignal` deleted. FalconEngine numerical calculator kept. Frame ZigZag kept (fib/geometry).

**RSX-SIGNAL-3 ✅ frozen** (`c856fef`). Fractal facts (`rsx_fractal_div` class_a/b/c, `rsx_fractal_pivot`) + bounded `FractalFactsAt`. Do **not** reopen detector math or lookback search.

**RSX-VISIBILITY-1 ✅ frozen** (`749912f`). Five FE visibility flags; `div_method` / `show_pivots` deleted. Facts independent of **presentation** (not of compute demand). Visibility not in RSX fingerprint. Do **not** reopen.

**WOZDUH-ACTIVE-1B ✅ frozen** (`1b724ef`). Persistent Frame Wozduh mask = per-TF WS union OR proven internal (`Live`: VolBase|Wt11|Wt22). ChartOnly unused Frames are mask 0. Do **not** reopen Wozduh compute.

**DAG-DEMAND-1 ✅ frozen** (`0837c77`). Per-TF RSX analytical demand. ChartOnly unused: Core/TV/Fractal/DAG-ZZ/ZZ collector = 0. Live internal: Core only. Facts `*[]string` tri-state. One coherent RSX series per wake. HTTP history independent. Frame `a.zigzag` untouched.

**FORECAST-SPEC-1 ✅ frozen** (`5afabfc` + `0ed000d`). **FEATURE-TAPE-1A ✅ frozen** (`b88bcd2`). **FEATURE-TAPE-1B ✅ frozen** (`6715718`). **ATR-TRUTH-1 ✅ frozen** (`84124a0`). **LABEL-SET-1A ✅ frozen** (`690d0be` + `1433626`). **LABEL-SET-1B ✅ frozen** (`8e88844`). Do not reopen ATR, 1A physics, or 1B. **TARGET-RESOLUTION-2** deferred (separate 15m→1s TargetSpec).

**MARKET-RSX-PARITY-1 ✅ audit closed (no production code).** Stages 1–4: OHLC exact vs Binance; Jurik matches literal Pine replica; prefix cold-start ~105–130 then trailing facts match full archive; published TV facts miss a real Bear because `rsxTVHitAtDisplayBar` is a second Everget reconstruct (3×lookback ratchet restart, one winner). Full `scanRSXTVHits` has the Bear. Not TV builtin / not Jurik / not prefix. Fixture: BTCUSDT USD-M 15m AnchorAt `1788630300000` (2026-09-05 17:45 UTC = 01:45 UTC+8), ConfirmedAt `1788631200000`.

**RSX-TV-ONE-BRAIN-1 ✅ frozen `4688160`.** One `RSTVState` owns `rsx_tv_div` + `rsx_tv_pivot`. AnalysisLogicVersion `analysis:v2`. UI Bull/Bear matches TradingView. Do **not** reopen RSX/Everget unless a real regression. **FEATURE-TAPE-RSX-REGEN-1 ✅ closed.** **TV-BULL-QUARANTINE-1 ✅ closed.**

**Parked (found on the RSX audit path — not ONE-BRAIN):**

| ID | What | Why later |
|----|------|-----------|
| **VOLUME-INGEST-1** | From 2026-09-06 00:45 UTC in Stage 1 sample, stored `Kline.Volume` matched Binance **taker-buy base**, not total `v`. OHLC exact. | All volume-derived facts uncertified. Not RSX. |
| **FRACTAL-MARKER-SSOT-1** | `rsxFractalHitAtDisplayBar` / `scanRSXFractalHits` vs `FractalFacts` / `FractalFactsAt`. | Local-radius math, not Everget carry. Inventory consumers, then delete leftover marker path if unused. Do not reopen RSX-SIGNAL-3 detector math. |
| **ATR-VALUES-FRAME-1** | `market/frame.go` still hydrates via `indicators.ATRValues` (legacy batch). ATR-TRUTH-1 left `ATRSeries` as canonical. | ATR leftover, not TV facts. |
| **FEATURE-TAPE-RSX-REGEN-1 ✅ closed** | analysis:v2 four-column tape regenerated with `DumpFeatureTape`. | Consumed by RESEARCH-DATASET-1. Do not reuse analysis:v1 tapes. |
| **TV-BULL-QUARANTINE-1 ✅ closed** | Visual Bull/Bear match TV after ONE-BRAIN. | Features eligible; old `analysis:v1` tapes must not be reused. |
| **TV-HIGHESTBARS-TIE-1** | Stage 3 leftover: possible TV builtin `highestbars` vs Go newest-wins on equal RSX. | Do **not** mix into ONE-BRAIN. Reopen only with real TV Data Window evidence of a mismatch after the Bear is published. |

Do not start a generic “indicator certification framework.”

**MICRO-IDLE-1 ✅ closed (not worth implementing).** Idle ChartOnly, no micro charts: five child reducers + unused Frame ticks ≈ **6µs per 1s parent** (~6µs CPU per wall-clock second). Forming ticks dominate count (6000 forming / 505 closed per 1200 parents); that is OHLCV + empty DAG skip, not Jurik. Sleeping that path is not worth a second lifecycle. Reducers and sparse tip stay frozen.

**Parked (keep intentionally — not DAG-DEMAND):** Wozduh SaveState while asleep; wake under Frame lock; 1024 IIR epsilon; legacy `finiteOrZero`; unfiltered WS = Wozduh all.

**Product later (do not mix in):** #69 S6/69D, DATA-1B, #81 P1/P2, #82 FE calendar snap, fib/drawings, #29 backtest projection.

**WOZDUH-ACTIVE-1A ✅ frozen** (`2cd4ca4`). `/api/history` replay uses a fixed Wozduh compute mask. Do **not** reopen.

**WOZDUH-WIRE-1 ✅ frozen** (`0c2ecce`). Pack/send only subscribed Wozduh scalar plot IDs. Enable hydrates current window before reveal. Do **not** reopen.

S6 / Working Set lifetime remains a later debt — **not** reopened by this freeze.

---

## Forecast deferred laws (OPEN_DEBTS ledger)

At chapter start: read rows whose **Owner** is this chapter. Implement or reassign with reason. Do not invent a second backlog file.

**ATR-TRUTH-1 resolved:** canonical `indicators.ATR` `atr:wilder-rma-first-tr-v1`; ATRSpec Period+Method+Logic; IIR provenance (spec ≠ state); `ATRSeries` over streamer; `ATRValues` legacy; `CalculateATR` deleted; `navigatorATR` noncanonical; ATR=0 legal; nonfinite/malformed no-commit; Save/Restore tested; no runtime map.

### Owner: LABEL-SET-1A ✅

Implemented: TargetSpec ATR does not rebuild FeatureTape; label source = ATR prefix + candidates + needed H tail; ATR source = prefix + candidates and must be `NextBarOpen`-contiguous else REFUSE generation; `ATRSeries` stops at last candidate; future-path gap → `PRIMARY_GAP` (not whole-run refuse); barriers frozen at `t`; scan `t+1`; High/Low; ATR<=0 → `ATR_ZERO`; nonfinite ATR/barriers refuse; truncated no-hit → `TRUNCATED_HORIZON`; caller prefix is the actual IIR init (no invented history); 1:1 tape rows including Ready=false; primary dual-hit → `DUAL_HIT`; canonical ATR only.

### Owner: LABEL-SET-1B ✅ frozen `8e88844`

Implemented: one pinned `FinerTimeframe` on resolve TargetSpec (`omitempty` when empty so exclude digests stay 1A); SameFamily finer MarketKey; calendar tiling via `data.NextBarOpen`; resolver only after primary `DUAL_HIT`; `FINER_MISSING` / `FINER_GAP` / `FINER_DUAL_HIT` / `FINER_INCONSISTENT`; successful resolve is UP/DOWN + `Reason=NONE`; `HitAt` stays primary; `label-set-v2` + `FinerSourceDigest` of consulted evidence; `FinerWindowCount` = attempts. No 1s fallback, no provider, no second walker.

### Owner: TARGET-RESOLUTION-2

CURRENT LAW: historical 15m research TargetSpec pins `FinerTimeframe=1m` so deep-history resolution is uniform.

DEFERRED: when durable 1s history exists, publish a **separate** TargetSpec (`primary=15m`, `finer=1s`). Do not silently upgrade 15m→1m. Different `FinerTimeframe` ⇒ different TargetDigest, LabelSet, and downstream ForecastArtifact/calibration/ranking.

CURRENT 1s DATA: about 24h durable history today.

RESEARCH BEFORE ADOPTION (do not implement in 1B): on the overlap where 1m and 1s both exist, compare total 15m primary dual-hits; % resolved by 1m; % remaining `FINER_DUAL_HIT` under 1m; % additionally resolved by 1s; 1m vs 1s UP/DOWN disagreement; ambiguity reduction from 1s. Purpose: whether years of durable 1s storage are worth the cost.

### Owner: chart TargetBarrier chapter

Go-computed `{At,ATR,Upper,Lower,TargetSpecID}`. No JS ATR. Forecast barriers ≠ execution SL/TP.

### Owner: live TargetBarrier / FORECAST-RUNTIME

Same MarketKey+ATRSpec → eligible to share state; different spec → separate. Same spec ≠ equal value if init history differs. Reconstruction policy (persist / replay / bounded) undecided. No runtime map yet. Do not claim 1024-bar live replay equals 2019→2026 research ATR.

### Owner: future FeaturePlan ATR columns

Canonical ATR only. Feature ATRSpec is FeaturePlan identity → **does** invalidate FeatureTape. Distinct from TargetSpec ATR.

### Owner: execution

May use canonical ATR for stops/sizing with a **different** ATRSpec than TargetSpec. Sharing is an optimization when spec+state match.

---

## NEXT (priority)

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **76** | **ScoreNodes → Forecast engine** | 🟡 **model next** | **RESEARCH-DATASET-1 frozen** `f311203`. Next when asked: **VALIDATION-PLAN-1**. **TARGET-RESOLUTION-2** deferred. |
| **93** | **DAG-DEMAND-1** — unused TF analytical CPU (RSX/facts/ZZ) | ✅ frozen `0837c77` | ChartOnly unused 1s–45s: 0 Jurik/ZZ/TV/Fractal/ZZ-col Updates. |
| **94** | **MICRO-IDLE-1** — unused 5s–45s reducer/forming fanout | ✅ closed | Measured ~6µs/1s parent for five unused children. Not worth implementing. |
| **67** | **Closed-bar Boundary + Viewport Tip** | ✅ | ADR-009 Cap + ADR-010 viewport forming tip (TV Model 2). Engine identity proven. F5 handoff = OVERWRITE same open |
| **84** | **RSX settings SSOT (B0)** | ✅ | ADR-012: engine owns config, default hlc3, autosave `rsx_settings.json`, dumb menu POST pipe |
| **85** | **ChangeImpact + Viewport (B1)** | ✅ | ADR-013/014: classify impact before Set*; soft indicator paint; debounce/Abort/generation |
| **86** | **Projection continuity (ADR-015)** | ✅ **B2.1+B2.2** | Soft `applyProjection`; projector APPEND + **OVERWRITE** same-open tip. ADR-015 probe skips heal/new-bar |
| **87** | **Replay Lifecycle Ownership (ADR-016)** | ✅ | Frame `replayStreamingLocked`: closed→forming; never commit forming tip. History Cap stays closed-only |
| **88** | **Timeline Publishability (ADR-017)** | ✅ **B3.0** | Exact closed-gap fill before pending flush; publishable only if Frame contiguous. Buffering UX separate |
| **89** | **TimelineRecovery UX (ADR-018)** | ✅ | FE LIVE↔HEALING; idempotent enter; sync badge; 25s watchdog; boot wires only |
| **90** | **PaneLayout / Ind (ADR-019)** | 🟡 **P5** | P1–P5 layout done. Optional later: `setHostActive` |
| **91** | **Scale / time axis / Ruler (ADR-020)** | ✅ | Scale + bottom axis + Ruler (ADR-025) + **HH:mm datetime chrome**. Fib/drawings = future product, not blocking |
| **68** | Osc fixed scale bounds (RSX/Wozduh TV-like `[-5,105]`) | ✅ | ADR-022: per-component `scaleContribution` → `autoscaleInfoProvider` |
| **69** | **MemoryBudget / WindowPolicy** | 🟡 **S1–S5 PASS; S6 FAIL (constitution frozen)** | Lifetime+Capacity constitution frozen. Fix: commit-paired ≠ TARGET wall. Cap numbers deferred. |
| **69C** | Focal-time prune (drop side farthest from viewport center) | ✅ | `pruneDirectionFromFocal` + boot passes `ViewportManager.capture` into `prependMonolith` |
| **69D** | Full sliding viewport window + paint alignment | 🟡 partial | Track A + Track B Lifetime Steps 1–3 done. Lifetime category model complete. |
| **80** | `ViewportManager.restore` 0×0 width risk (`setVisibleLogicalRange`) | ✅ | D2: layout deferral via `whenHostHasLayout` → TimeCamera.propose (no raw LWC); live restore retired |
| **81** | **Timeline Publish Gate** (reconnect heal) | ✅ | Phases A–D + P0: WS hooks, Runtime gate, forced REST@1bar, FE await `timeline_publishable`. P1/P2 (status poll / GetWindow degraded) deferred |
| **82** | **Calendar bar boundary** (`1w`/`1M` time model) | ✅ **A1+A2** | ADR-011 Cap/align/CloseTime. A2: catch-up/gap/reconcile via `NextBarOpen`/`BarStepsBetween`; `intervalSkipsKlineGapFill` removed. FE snap deferred unless runtime proves need |

---

## SQLITE-1 — WAL reader lifetime (audit only, Aug 2026) ✅

Log: `[WAL] checkpoint blocked by readers (frames=N checkpointed=N) — will retry next tick`.

**Mechanism:** `PersistenceQueue` calls `PRAGMA wal_checkpoint(TRUNCATE)` every **5 minutes**. `busy≠0` means another SQLite connection still has a snapshot (in-process pool or another process). Passive `wal_autocheckpoint=1000` is starved the same way. **Do not hide the log. Do not tune WAL pragmas in SQLITE-2 until the readers are classified in a live run.**

**In-process readers (request-scoped, `defer rows.Close()` — no held `*sql.Rows` / `Begin` on the read path):**

| Path | When | Duration |
|------|------|----------|
| `GetWindow` → `LoadContinuousContractBeforeEnd` → `LoadKlinesBeforeEnd` | `/api/history` LEFT/prefetch/TF hydrate | Query + scan (~chunk size) |
| `sqliteHasBarsBefore` / `liveHistoryHasMore` → `QueryKlineCacheBounds` | After every GetWindow | `COUNT(*)` then `MIN/MAX` per futures+spot — **table scan class**, two round-trips |
| `LoadRAMHistoryFromDB` → `LoadContinuousContractFromDB` (`limit=0` window) | Parallel Frame **boot** only | Large range scan; many TFs at once (`SetMaxOpenConns` = CPU 4–16) |
| Catch-up / gap heal | Every **5 min** (same cadence as checkpoint) | Short `QueryRow` / `ListArchiveGaps`; REST does **not** hold SQLite |
| `SaveKlines` write tx | Persist worker | Same goroutine as checkpoint, but HTTP `NoteGapsFromOpenTimes` can write from GetWindow |

**Not a lifetime leak in Go:** no stored iterators; comments blaming “long-lived catch-up readers” are stale — catch-up does not keep `Rows` open across REST.

**Out-of-process (likely persistent pin):** `.cursor/mcp.json` `sqlite-history` → `mcp-server-sqlite --db-path …/history.db`. Second process on the **same file** for the whole Cursor session. Python sqlite often leaves a deferred read txn open after SELECT. That alone can make **every** 5-minute TRUNCATE log `busy`. Repair tools (`cmd/repair_*`, `history_sync`) same class if run while the bot is up.

**Verdict:** log cadence matching chart use = overlapping GetWindow (expected). Log every ~5 min while idle in Cursor = MCP (or idle pool is innocent; MCP is not). SQLITE-2 removed autostart MCP (see below). Do not hide the WAL log; do not retune pragmas unless GetWindow-only busy remains after MCP-off.

---

## SQLITE-2 — stop autostart MCP on `history.db` (Aug 2026) ✅

**Fix:** empty `.cursor/mcp.json` (`mcpServers: {}`). Recipe lives in `.cursor/mcp.json.example`. Agents: `.cursor/rules/mcp-on-request.mdc`.

Enable `sqlite-history` only when the user asks, or when a WAL/archive diagnosis cannot proceed without it — then remove it again. GitHub MCP is the same (not autostart). No `PRAGMA` / `busy_timeout` change.

At SQLITE-2 apply time, `lsof` showed only the bot `main` on `history.db`; no `mcp-server-sqlite` process.

---

## SQLITE-2b — idle pool pins WAL TRUNCATE (Aug 2026) ✅

Live proof after MCP-off: `[WAL] checkpoint blocked` still every **5 minutes** (`20:11:29` then `20:16:29`, frames=checkpointed). That is the ticker, not overlapping GetWindow.

**Cause:** `SetMaxOpenConns(CPU)` + Go default `MaxIdleConns=2`. Idle sqlite handles stay open after parallel Frame boot. `wal_checkpoint(TRUNCATE)` cannot restart WAL while any other connection exists — so every tick logged busy even with no history GET.

**Fix:** `SetMaxOpenConns(1)` + `SetMaxIdleConns(1)`. Boot SELECTs serialize. Busy log kept; if it still fires, include `open/in_use/idle` (in-flight GetWindow or a second process). No pragma change. Test: `TestCheckpointWAL_TruncateAfterParallelReads`.

---

## TF-1 — LIVE TF switch (Aug 2026) ✅ phase 1–2

**Wanted:** Same bar count, same forming-bar screen X, same spacing. Not time-span. Not Mode B.

**Phase 1:** LIVE `proposeAfterData` keeps bars/spacing/pad; width = `clampVisibleLogicalWidth` / `MAX_VISIBLE_BARS`. Capture `from < 0` not poison. No FreshLive on valid LIVE layout-defer.

**Phase 2:** Deleted dead `clampRightPadding` (no production caller after phase 1). HISTORY still uses `sanitizeVisibleBars` (50–400) + healthy 150 at `cameraIntentForTfSwitch`.

---

## Data / exchange sockets

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **44** | Order Flow / TickBarBuilder / `@aggTrade` | ⏸ | Amputated until settings UI + consumer; seam documented in Ingress |
| **8** | Qdrant wired in `main` + AI veto consumer | 🔜 | `vector_db/` exists; no live consumer |
| **64** | Navigators full `ReplayDAGKlines` each request (CPU) | 🟡 | Later: live HistoryBus tail |

---

## Trading stack (paused — ChartOnly)

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **35** | DAG → TradeManager wiring | ⏸ | Re-enable only with `ENGINE_MODE=live` + new strategies |
| **36** | TradeIntent wire contract | ⏸ | `decision/score_types.go` |
| **37** | Execution gate `isClosed` only | ⏸ | TickLiveCh frozen in ChartOnly |
| **38** | Risk/settings SSOT parity | ⏸ | Legacy matrix purged in Phase F; revisit with new strategies |

---

## Frontend / UX polish

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **79** | Self-heal `loadDashboard` resets camera to fresh | 🟢 | Wire `viewportAnchor` on gap-heal/reconnect |
| **49** | Active Driver / slave scroll (Shot 6B) | ✅ **ADR-021 P0–P3** | TimeCamera + Crosshair + **InteractionController** (ADR-024); ChartAdapter = LWC adapter only |
| **35** (charts) | Phase 8B annotations UI on prepend | 🔜 | `applyUniversalAnnotations` |
| **29** | Backtest history bypasses Projection | 🟡 | Asymmetry vs live Atomic |
| **82** | `prependMonolith` times not normalized via `chartTime` | 🟢 | Latent; server sends seconds |
| **92** | **DAG-DEMAND-1** (was “backend skip unused plots”) | ✅ | Implemented with #93. |

---

## Explicitly dead (do not revive)

| Item | Reason |
|------|--------|
| Name / type `Analyst`, `ChiefAnalyst`, `MasterGeneral`, `Marker` (type), `Layer2` | Phase G vocabulary — see Glossary |
| Legacy ScoreEngine / matrix / thresholds / risk_settings APIs | Phase F purge |
| `strategy/` active code | Beacon `doc.go` only |
| Micro-candles / synthesized time bars in ledger | Phase D purge; TickBarBuilder is the future socket |
| Second merge implementation outside Ingress | Debt #19 closed in Core 5.0 Phase A |

---

## Ops

- `cmd/repair_volumes` — volume healer; run only with bot **stopped**.
