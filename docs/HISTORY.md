# History (Completed Phases)

**SSOT for:** what was done in the past.  
**Read only on request** (regression archaeology, "how did we fix X?").  
Do **not** load this file for routine feature work — use `ARCHITECTURE.md` + `OPEN_DEBTS.md`.

Full pre-Core-6.0 Russian chronicle lived in `MEMORY.md`; git history retains it. This file is the English condensed canon.

---

## Phase ADR-025 — Ruler foundation (Jul 2026) ✅

- `RulerController` state + geometry; IC routes pointer semantics; ChartAdapter translates/renders guides+rectangle (no labels).
- Proves interaction stack accepts a new consumer without parallel mouse paths. Stats/formatters deferred.

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
