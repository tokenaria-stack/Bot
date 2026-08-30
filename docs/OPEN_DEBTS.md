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
4. FE indicator paint skip is **enough for now** (HIDDEN-RENDER-SKIP-1 `40dca59` + WOZDUH-OWNER-1 `3722baf`). User accepted live `updateTick` ~2× cheaper with most Wozduh lines unchecked; laptop load down. Do **not** start wire skip, lazy `removeSeries`, or LOD from this.  
5. **Later (parked):** proper **backend** indicator optimisation — DAG/compute + pack/wire only for subscribed plots. Not a FE workaround. Not this freeze.  
6. Then: ScoreNodes / clean strategy + indicator rebuild — **RSX-TRUTH-CLEAN-1 frozen** (`5f8a290`); do not reopen Go RSX color or old marker factory.

**RSX-TRUTH-CLEAN-1 ✅ frozen** (`5f8a290`). Backend RSX is numerical/factual only. Live paint stays FE. Do not reopen slope-vs-50 color, `rsxColor` wire, or empty L/LL/S/SS sockets.

**RSX-SIGNAL-1 ✅ frozen.** Pine TV divergence facts (`rsx_tv_div`) + RSX-pane Bull/Bear markers. Do not start **RSX-SIGNAL-2** (`rsx_zz_div`) from this chapter. Menu `div_method` / fractal / ZigZag remain distinct families.

S6 / Working Set lifetime remains a later debt — **not** reopened by this freeze.

---

## NEXT (priority)

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **76** | **ScoreNodes** — move Score/Falcon decision graph into DAG nodes | 🔜 | Start from frozen RSX truth (`5f8a290`). Each factor: fact vs presentation debt? Do **not** delete `market/falcon.go` until done |
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
| **92** | **Backend indicator opt** (DAG/wire skip unused plots) | ⏸ later | FE skip accepted. Store/wire still full. Do not start from rAF symptoms. |

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
