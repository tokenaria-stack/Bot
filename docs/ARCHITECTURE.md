# Architecture (Current System)

**SSOT for:** how the system works **today**.  
**Not SSOT for:** engineering laws (→ `jeweler-protocol.mdc`), Working Set guarantees (→ `WORKING_SET_CONTRACT.md`), Cache Lifetime guarantees (→ `CACHE_LIFETIME_CONTRACT.md`), history (→ `HISTORY.md`), why-decisions (→ `DECISIONS.md`), backlog (→ `OPEN_DEBTS.md`).

**Version:** Core 5.0 Data Plane (Phases A–G ✅) + Core 6.0/6.1 Docs + **Debt #69A FE Memory Budget** + **Debt #81 Timeline Publish Gate**.

**Default mode:** `ENGINE_MODE=ChartOnly`. Trading stack re-enters only with new `decision/` strategies + `ENGINE_MODE=live`.

**Chart freeze (tag `CHART_FROZEN`):** camera / hydration / working-store **closed**. Runtime caps (experiment frozen, not a new collector):

```text
MAX_STORE_BARS             = 9000
MAX_VISIBLE_BARS           = 5000
HISTORY_CHUNK_LIMIT        = 3000
HISTORY_EDGE_PREFETCH_FRAC = 0.25
```

Fix C–G retained. Patch 2 (live-delta throttle) is **not** active. Remaining live `[Violation]` RAF cost is accepted; later speed work is fewer painted indicator series, not more camera logic. Do not reopen TimeCamera, LEFT/RIGHT hydration, RenderScheduler, cap/chunk/prefetch, or tick throttling unless a real regression appears.

**HISTORY-IDLE-PUMP-1 (frozen):** viewport history demand is human-owned. Wheel / drag / navigation may note `userNav`. Paint and LWC range echo must not schedule a new page. Post-flush consumes pending only. Sparse `sourceContinue` is separate.

**SPARSE-LIVE-INGEST-1 (frozen):** 5s–45s WS ingest follows `windowMode` (island identity). `historyHasNewer` is paging/source only. 1s keeps the `historyHasNewer` detach veto.

**SPARSE-ADR010-TIP-1 (frozen):** 5s–45s HTTP tip is append-only after a matching Frame committed frontier. Closed Replay prefix is immutable. Native / 1s stay on `projectViewportFormingTip`.

---

## SSOT map

| Document | Owns |
|----------|------|
| `.cursor/rules/jeweler-protocol.mdc` | Engineering laws (always-on) |
| `.cursor/rules/senior-quant-architect.mdc` | Role / thinking style (always-on) |
| `docs/ARCHITECTURE.md` | Current architecture + **Core Ownership Model** |
| `docs/WORKING_SET_CONTRACT.md` | Working Set law (VIEW ↔ store ↔ paint) |
| `docs/CACHE_LIFETIME_CONTRACT.md` | Cache Lifetime law (CL-01…CL-07) |
| `docs/OPEN_DEBTS.md` | Open backlog / NEXT |
| `docs/HISTORY.md` | Completed phases (on request) |
| `docs/DECISIONS.md` | Why key choices were made (on request) |
| `docs/PINE_INDICATOR_SOURCES.md` | Pine/TV indicator source notes |
| `MEMORY.md` | Index only |
| `README.md` | Landing: what / build / links |
| `docs/archive/` | Closed Track B / S5–S6 / acceptance **process** files (not default reading) |

Memory-update routing (user: «сохрани в памяти» / «update MEMORY»): see Role
`.cursor/rules/senior-quant-architect.mdc` → section **When user says "save / update memory"**.

---

## Core Ownership Model (Jeweler Constitution)

**Status:** Frozen. Frontend chart architecture is stable after ADR-023, ADR-026, ADR-027, ADR-028 (navigation), ADR-029 (TF transition protocol), and the Ownership Audits.

This is not an ADR. It is the permanent constitution of the frontend.  
Future ADRs must conform to it. Future features ask one question first: **which existing owner does this belong to?** If none — write an ADR. If one exists — put the work there.

No new Registries, Services, Event Buses, or `*Manager` facades without a justifying ADR.

### 1. Core Laws

| Law | Meaning |
|-----|---------|
| **One Truth** | A fact has one authoritative source. Never copy domain truth into a second module. |
| **One Owner** | A concern has exactly one module responsible for it. |
| **Translation ≠ Ownership** | Adapting DOM/LWC/pixels to semantics is not owning the policy or the data. |
| **Decoration ≠ Market Data** | Display chrome never enters the market-data plane (`ColumnarStore` / DDR / `candleSeries`). |
| **Rendering ≠ Policy** | Controllers decide; ChartAdapter paints. Paint never invents policy. |
| **Composition ≠ Ownership** | Wiring owners together does not transfer their responsibilities to the composer. |
| **TF ≠ Navigation** | TimeframeController never decides where the camera goes. TimeCamera always decides. |
| **Navigation ≠ Density** | Navigation never depends on timeframe density. No TF branches in camera logic. Preserve time meaning. |
| **One Camera Write Path** | Application → TimeCamera → ChartAdapter CameraCommit → LWC. No second writers. |

### 2. Ownership Map

| Concern | Sole Owner |
|---------|------------|
| Market display data | ColumnarStore / DDR |
| Live candle series updates | `candleSeries` via ChartAdapter composition |
| Future timeline math | DisplayTimeline |
| Timeline decoration rendering | TimelineDecoration |
| Crosshair policy (hover + sync) | CrosshairController |
| Rendering translation (LWC / DOM) | ChartAdapter |
| **User navigation / VIEW** | **TimeCamera** (ViewIntent + ViewGeometry) |
| **time → logical lookup** | **Data layer / ChartCompositor** (`nearestLogicalForTime`) |
| Camera apply to LWC | ChartAdapter **CameraCommit only** |
| Scale preferences (Auto / Manual / Log) | ScaleController |
| Layout / visible bottom time axis | PaneLayout |
| Interaction routing | InteractionController |
| Ruler policy (measure FSM) | RulerController |
| Active timeframe id | TimeframeController |

Do not invent owners outside this map. Extend only via ADR.

### 3. Responsibilities

| Owner | Responsibility |
|-------|----------------|
| **ColumnarStore / DDR** | Bounded market display window and indicator series data. Not chrome, not camera, not crosshair policy. |
| **DisplayTimeline** | Pure future bar-open timestamps for display. No LWC, no store writes. |
| **TimelineDecoration** | Sealed LWC rendering of display-only timeline chrome. Never market OHLC; never `update(tip)`. |
| **ChartAdapter** | Sole talker to Lightweight Charts. Translates and composes. Applies navigation only via **CameraCommit**. Never owns business rules, hover policy, future-time math, or navigation intent. |
| **CrosshairController** | Hover ownership and synchronized `{ logical, time? }` + V/H policy. Never renders; never invents timestamps. |
| **TimeCamera** | Owns the user's **VIEW**: ViewIntent (`LIVE` \| `HISTORY`) + ViewGeometry (`centerTime`, `visibleBars`, `barSpacing`, `rightPadding`). Capture / propose / commit. Never formats labels, never paints, never indexes market bars. |
| **Data / ChartCompositor** | Resolves `centerTime → nearest logical` for TimeCamera propose. Owns lookup, not navigation semantics. |
| **ScaleController** | Per-pane Y-scale preferences. Does not own oscillator domain math (contribution stays with series config). |
| **PaneLayout** | Pane membership and which HostID shows the bottom time axis. Declares; does not paint LWC. |
| **InteractionController** | Routes semantic interaction events to controllers. Accepts no raw DOM/LWC objects. |
| **RulerController** | Measure-tool FSM and semantic anchors. ChartAdapter projects geometry. |
| **TimeframeController** | Active TF id + toolbar. Never moves the camera. |
| **ViewportManager** | Capture/translate helper only (ADR-028 D2). Not a camera owner; no direct LWC camera writes. |

**LayoutController** allocates DOM/CSS from PaneLayout — composition/apply, not a second layout owner.

### 4. Ownership Rules

- Future timestamps originate only from **DisplayTimeline**.
- Decoration never enters the market-data plane; `candleSeries` holds real candles only.
- Controllers never render (no LWC, no measurement DOM).
- **ChartAdapter** never owns business rules, hover policy, timeline math, or navigation intent.
- **PaneLayout** declares bottom-axis ownership; it does not own crosshair sync state. The axis owner **renders** the time label; the hovered pane does not.
- **TimeCamera** never formats time, never draws chrome, never searches candles.
- **CrosshairController** owns synchronization policy, not paint.
- **TimelineDecoration** owns decoration series lifecycle; ChartAdapter only composes refresh payloads.
- **Timeframe switching never owns navigation** — it asks TimeCamera to capture/propose after data reload.
- **Navigation never depends on TF density** — no timeframe branches in camera logic.
- Camera writes go through **one CameraCommit** only.
- Translation details (HTML peer guide, mid-Y, private series handles, Debt #80 layout fallback) stay behind the correct owner — they are not new owners and not reasons to add architecture.

### 5. ADR Relationship

These decisions **establish** the constitution; they do not replace it:

| ADR | Established |
|-----|-------------|
| **ADR-023** | Layout / single bottom time-axis ownership |
| **ADR-026** | Crosshair sync semantics (`logical` primary; no fabricated time) |
| **ADR-027** | Market plane vs Decoration plane |
| **ADR-028** | Navigation / VIEW ownership (TimeCamera ViewIntent + ViewGeometry) |
| **ADR-029** | Timeframe transition protocol (LIVE sticky + HISTORY center-anchor) |
| **ADR-030** | Timestamp units: no magnitude inference; dual domain (ms engine / sec wire / ms camera) |
| **Ownership Audits** | Verified implementation against the map (chart chrome; time domain) |

Implementation may evolve (e.g. Primitive instead of an HTML guide; Phase D navigation cutover). Ownership does not — unless a new ADR amends this constitution.

---
## Package layers

```
exchange/    transport + Ingress (Bar Source Seam, Authority, merge/validate)
data/        SQLite archive + PersistenceQueue (single runtime writer)
market/      Frame, Runtime, streaming/snapshot, Boot, MTF, falcon bus, chart replay
decision/    ScoreDecision / ScoreFactor contracts (sockets; no live ScoreEngine)
execution/   Position sizing sockets
core/        DAG runner + nodes (RSX, Wozduh, divergence slots)
server/      HTTP/WS projection (HistoryProvider, Projector, columnar wire)
web/         DDR charts (boot.js composition root)
indicators/  Streaming math (no go-talib)
vector_db/   Qdrant socket (no live consumer yet)
strategy/    doc.go beacon only (Phase F purged legacy code)
```

**Import DAG:** `exchange → market → decision → execution` (one-way).

### Timestamp units (#83)

No magnitude (`1e12`) inference on the active path.

| Domain | Unit | Crossing |
|--------|------|----------|
| Ingress / SQLite / Frame / indicators | Unix **milliseconds** | canonical |
| Wire / LWC / chart `times[]` | Unix **seconds** | Go `ChartTimeSec(ms) = ms/1000` |
| Camera / geometry (`centerTimeMs`, store keys) | Unix **milliseconds** | FE `secToMs` / `msToChartSec` only |

Navigator DTO times are ms until F3 `navigatorMsToChartSec`. Do not collapse camera geometry to seconds. Do not reopen #83 for inactive `app.legacy.js`. Tag: `TS_CONTRACT_CLEAN`.

| Package | Answers | Must not |
|---------|---------|----------|
| `market` | What is happening? | Decide trades; import `server` |
| `decision` | What to do? | Import `market`; mutate frames |
| `execution` | How much / how to place? | Analyze market |
| `server` / `web` | How to project & paint? | Recompute indicator math |

---

## Glossary (canonical)

| Term | Meaning |
|------|---------|
| `Frame` | Per-symbol/TF streaming state (`market/frame.go`) |
| `Runtime` | Data runtime; tick routing (`market/runtime.go`) |
| `streaming` / `snapshot` | O(1) intra-bar Snapshot/Restore (`streaming.go`, `snapshot.go`) |
| `Authority` | Ingress trust level: Estimated / Settled / Final (WS `x=true`) |
| `Ingress` | Single closed-bar pipeline: validate + merge by Authority |
| `Bar Source Seam` | Only closed `exchange.Kline` enters Ingress; aggregation is producer-private |
| `Boot FSM` | Connecting → Loading → Reconciling → Live (WS first) |
| `PersistenceQueue` | Sole runtime SQLite UPSERT path |
| `GetWindow` | Read-only history window (SQLite ∪ RAM). Interactive miss is filled by `EnsureHistoryWindow` then reread |
| `EnsureHistoryWindow` | HIST-1: era-local REST acquire + persist for `/api/history` only |
| `HistoryProvider` | Chart history window owner: SQLite ∪ RAM |
| `Projector` | Slot → wire packer for live plots + columnar history |
| `ScoreDecision` / `ScoreFactor` | Decision contracts in `decision/` |
| `ProjectionEpoch` | FE discard axis for TF / load / hydrate / WS |
| `Tip Ownership` | Native/1s: Cap-closed History + ADR-010 overlay (WS OVERWRITE same forming open). 5s–45s HTTP: SPARSE-ADR010-TIP-1 append-only forming row (no Replay overwrite). Frame replay = closed→forming (ADR-016) |
| `Bar boundary` | ADR-011: fixed TF = duration floor; calendar TF (`1w`/`1M`) = Monday / 1st-of-month UTC (`CurrentBarOpen` / `Prev` / `Next`) |
| `Live chart TF` | Native USD-M set (`1m`…`1d`, `1w`, `1M`) plus derived `2m/10m/45m/3h` plus live `1s` from aggTrade (`exchange/timeframe_catalog.go`, ADR-031). Durable 1s lives in `micro_klines` (24h, sparse), not `historical_klines`. `3d` unsupported. Other seconds/ticks remain placeholders |
| `windowMode` | FE display window: `live` \| `history` (Debt #69A) |
| `STORE_BUDGET_*` | ColumnarStore TARGET 12000 / HARD_CAP 16000 bars |
| `pruneDirectionFromFocal` | Debt #69C: drop side farthest from viewport center time |
| `DisplayTimeline` | ADR-027: pure future bar-open times for display whitespace (never store/DDR) |
| `TimelineDecoration` | ADR-027: sealed LWC decoration series; time-scale chrome only |
| `Decoration Plane` | Display-only timeline chrome; never mixed into `candleSeries` |
| `ViewIntent` | ADR-028: `LIVE` \| `HISTORY` — why the user is looking |
| `ViewGeometry` | ADR-028: `centerTime`, `visibleBars`, `barSpacing`, `rightPadding` |
| `CameraCommit` | ADR-028: single ChartAdapter navigation transaction to LWC |

### Banned names (Go identifiers)

Do not revive: `Marker` (as type), `MasterGeneral`, `Layer2`, `Analyst`, `ChiefAnalyst`.  
Allowed wire field: `Marker string` + `json:"marker"` for chart labels only.

| Old | New |
|-----|-----|
| `Marker` (type) | `Frame` |
| `MasterGeneral` | `Runtime` |
| `layer2.go` | `streaming.go` + `snapshot.go` |
| `score_types` in `strategy/` | `decision/score_types.go` |
| active `strategy/` code | `strategy/doc.go` beacon |

---

## Data plane invariants (Core 5.0)

1. **Source trust beats field heuristics.** Merge by Authority. WS Final never loses to REST. Field MAX/MIN only when Authority is equal.
2. **Bar Source Seam.** Closed canonical bars only in Ingress. Forming ticks (`x=false`) bypass Ingress (Frame telemetry / Core 4.8 path). Time bars = exchange klines (TradingView canon) — no trade-synthesized time bars in ledger.
3. **Boot: WS first.** REST recovery must not overwrite missed WS bars. One tick path: `Runtime.routeTick` (live + boot replay).
4. **SQLite firewall ≠ cure.** Monotonic UPSERT (`high=MAX`, `low=MIN`, `volume=MAX`) is last line of defense; root fix is REST Grace (`KlineSettleGraceMs=5000`).
5. **Time Model Rule (ADR-011).** Fixed intervals (`1m`…`1d`) use duration arithmetic. Calendar intervals (`1w`, `1M`) use bar boundaries (Monday / month-start UTC) via `CurrentBarOpen` / `PreviousBarOpen` / `NextBarOpen`. Never use `IntervalDurationMs` for Cap, REST align, next tip, or month gap checks.
6. **Indicator Configuration Rule (ADR-012).** Indicator parameters are engine state. Browser menus POST to `/api/settings/indicators`; never own live math config. Autosave on disk. Future: Registry → Config → DAG membership → Runtime → Projection (implement when 2+ indicators need enable/disable).
7. **Indicator Change Impact (ADR-013).** Classify settings via `ChangeImpact` before mutating runtime. Never `Set*` outside the IndicatorReplay transaction. AnnotationOnly must not touch Falcon/Jurik.
8. **Viewport Contract (ADR-014).** Indicator settings are projection events — never move camera/zoom/scroll/TF. Soft `applyProjection` + camera restore only.
9. **Projection Continuity (ADR-015).** Server `projectViewportFormingTip` is the sole projector (APPEND or OVERWRITE). FE applies snapshots atomically; never synthesizes Cur. First WS after soft apply is idempotent when market unchanged.
10. **Replay Lifecycle (ADR-016).** Frame runtime replay reproduces live candle lifecycle: closed (`isClosed=true` + commit) then optional forming (`isClosed=false`, never commit). Same Cap forming predicate as History tip strip. History Cap Replay stays closed-only. TipSSOT/ProjCont investigation probes are dormant (`DEBUG_TIP_SSOT` / `DEBUG_PROJ_CONT`); TransportDiag / Self-Healing stay on.
11. **Timeline Publishability (ADR-017).** Mid-session heal: Cap REST → Exact closed-gap fill (if pending tip jumps) → flush → Frame contiguity check → only then `timeline_publishable`. Never fabricate bars; never flush a tip jump.
12. **Timeline Recovery UX (ADR-018).** FE `TimelineRecovery` owns LIVE↔HEALING; duplicate healing ignored; sync badge (not full-screen Buffering); watchdog once; `publishable` exits immediately via `onRecovered`.
13. **PaneLayout (ADR-019).** FE owns footer pane membership (`visible` / `order` / `footerHeights` px / `fullscreenPaneId`). Ind menu from Manifest HostIDs; persist ∩ manifest. Price always on (not a HostID). **LayoutController** applies CSS Grid, height drag, legend reorder, and fullscreen.
14. **ScaleController (ADR-020 P1).** HostID-based Y-scale prefs (`allowLog`, dormant `scaleGroup`). Price Auto+Log; footers Auto-only. Visibility must not reset prefs. Persisted state must be self-sufficient: Auto OFF without `manualRange` is repaired to Auto ON on restore.
15. **Scale contribution (ADR-022 / #68).** What Auto *measures* is per DDR component (`renderOptions.scaleContribution`: `dynamic` | `bounded` | `ignore`) → LWC `autoscaleInfoProvider`. ScaleController never owns oscillator domains.
16. **TimeCamera (ADR-021 + ADR-028).** Sole owner of user **VIEW** navigation: ViewIntent (`LIVE` \| `HISTORY`) + ViewGeometry (`centerTime`, `visibleBars`, `barSpacing`, `rightPadding`). Capture / propose / atomic CameraCommit via ChartAdapter. Never indexes market bars (data/compositor resolves time→logical). TF transition protocol → **ADR-029**.
17. **CrosshairController (ADR-021 P2 + ADR-026).** Owns `hoveredHostId` + V/H policy only; never timeline. Hover from wrapper pointer events only; LWC move is `{ logical, time? }` (`syncPosition`). Peers: native vert when time exists, else adapter logical guide; local Y only; no foreign horz.
18. **InteractionController (ADR-024 / P3).** Routes pointer / range / crosshair-time only. ChartAdapter adapts LWC; specialized controllers own policy.
19. **RulerController (ADR-025).** Anchors `logical+price` (+ optional time); two-click FSM; `RulerMetrics` for Δ/%/bars/duration; ChartAdapter projects + tooltip. Finite rectangle only.
20. **Decoration Plane (ADR-027).** `DisplayTimeline` = pure future time math; `TimelineDecoration` = sealed LWC whitespace. `candleSeries` stays real-only (`update(tip)` invariant). ChartAdapter composes; never merges decoration into market data.
21. **Bottom time axis (ADR-023 + ADR-027 amendment).** PaneLayout declares the single visible axis owner; LayoutController allocates; ChartAdapter mirrors `timeScale.visible`. The bottom-axis **time label** is rendered on that owner from synchronized crosshair state — never owned by the hovered pane. Intermediate panes reserve zero axis height.
22. **RAM ≠ SQLite.** Frame/Runtime = realtime; SQLite = archive ledger. Healthy RAM ≠ healthy DB tip. **SQLite catch-up ≠ Frame heal** — chart/DAG truth requires `LoadHistoricalKlines` + replay, not archive enqueue alone.
23. **Frontend ≠ history DB.** `ColumnarStore` is a bounded display window (Debt #69A). Server owns durable history. Viewport never mutates OHLC/plots.
24. **Timeline publish gate.** `WS Connected ≠ History Reconciled ≠ Timeline Publishable`. Mid-session heal follows ADR-017; FE recovery presentation follows ADR-018.
25. **Dense vs sparse live series (MICRO-1.1 / MICRO-2B / MICRO-2C / MICRO-2A).** Native and derived time bars expect every bucket. Seconds and ticks do not. FE `appendTick` gap heal is dense-only (`requiresDenseTimeContinuity`). Sparse holes append; they never enter `fe_gapDetected` healing. TimelineRecovery is dense-only; sparse Master heal is a no-op; browser reconnect is a quiet RAM snapshot. Sparse HISTORY skips live delta paint; HISTORY→LIVE is one full store paint. Durable 1s is `micro_klines` only — never `SaveKlines` / `archive_gaps` / Ensure / REST.
26. **LIVE-EDGE-1.** LIVE + new bar: keep at least 1 logical bar of right slack (`TimeCamera.proposeLiveEdgeGuard`). Floor, not a forced offset. HISTORY and same-bar ticks do not move the camera.

**Interaction pipeline (canonical):**

```
Browser / LWC events
        │
        ▼
ChartAdapter          ← translates
        │
        ▼
InteractionController ← routes
        │
 ┌──────┼──────────────┬─────────────┐
 ▼      ▼              ▼             ▼
TimeCamera  CrosshairController  RulerController
```

ChartAdapter translates. InteractionController routes. Controllers own behavior.  
Invariant: IC accepts only semantic events — never raw DOM/LWC objects.

---

## Chart planes (ADR-027)

Two planes, one composer — ownership is fixed in **Core Ownership Model** above.

```
Market Plane                         Decoration Plane
─────────────────                    ────────────────────
ColumnarStore / DDR                  DisplayTimeline
candleSeries + update(tip)           TimelineDecoration
```

LWC may union times for chrome. Tip `update()` sees only the market series.  
Why / rejected alternatives → `docs/DECISIONS.md` (ADR-027).

---

## Frontend display window (Debt #69A)

**Ownership:** Server owns history; browser owns only an active viewport window.

| Piece | Behavior |
|-------|----------|
| Budget | `STORE_BUDGET_TARGET=12000`, `STORE_BUDGET_HARD_CAP=16000` ([`web/config.js`](../web/config.js)) |
| Atomic prune | `_pruneToCount` slices times + candles.* + all plots + annotations together |
| `appendTick` | `_enforceBudget(FROM_OLDEST)` (live tip path only) |
| `prependMonolith` | `_enforceBudget(pruneDirectionFromFocal(...))` — drop side farthest from viewport center; default NEWEST if no focal |
| `windowMode` | `live` — WS may append; `history` — set when NEWEST pruned; WS/gap must not feed store or auto-`loadDashboard` |
| Return to live | Pin right edge while `history` → `loadDashboard()` (server tip) |
| Paint | Track C: full retained snapshot (`selectPaintSnapshot`); no soft 15k tip-window |
| Reload Dashboard | HTF clear + `store.clear()` + `loadDashboard()` (emergency, not memory manager) |

---

## Tick path (Frame)

```
Binance WS kline
  → Runtime.routeTick
  → Frame.UpdateKlineTick(k, isClosed)
  → evaluateTickLocked
       1. restoreStreamingState()      // O(1) rollback open bar
       2. FalconEngine.Evaluate        // gated unless EngineAllowsStrategies()
       3. Volatility / oscillators / ZigZag / divergence / geometry
       4. saveStreamingState()         // only if isClosed
```

**Double-commit guard (Core 4.8):** `lastCommittedOpenTime` ensures one DAG commit per closed bar (root cause candidate for RSX tip spike #67).

**Keep:** `market/falcon.go` until ScoreNodes (#76).

---

## Boot FSM

| Phase | Behavior |
|-------|----------|
| 0 Connecting | WS first; buffer ticks (cap 4096); Frame untouched |
| 1 Loading | SQLite + REST through Ingress |
| 2 Reconciling | Replay buffer in order via `Runtime.routeTick` |
| 3 Live | `StartDataFeed` + gap-fill / catch-up loops |

### Mid-session Timeline Reconcile (Debt #81)

Not a second Boot FSM. Thin publish gate on `Runtime`:

```
Binance disconnect → unpublishable + timeline_healing
Binance reconnect  → ReconcileTimeline (forced FetchClosedRange all chart TFs)
                   → framesTimelineHealthy (ΔOpen > 1×interval = hole)
                   → flush pending → publishable + timeline_publishable
FE: gap/healing → buffering; timeline_publishable → atomic loadDashboard
```

Key files: `exchange/ws.go` (OnDisconnect/OnReconnect), `market/runtime.go` + `kline_gap.go`,
`server/webserver.go` broadcast, `web/ws.js` + `boot.js`.

---

## Decision layer (sockets)

After Phase F, live ScoreEngine / matrix / thresholds / trade FSM are **gone**.

**RSX ownership (RSX-TRUTH-CLEAN-1, frozen `5f8a290` + RSX-SIGNAL-1):**

```
BACKEND RSX     → values + factual states only
FRONTEND RSX    → presentation only (rsxStrokeColor, scale chrome)
FUTURE DECISION → derives meaning explicitly from backend truth
```

**RSX TV facts (RSX-TV-ONE-BRAIN-1 ✅ frozen `4688160`):** One closed-bar `indicators.RSTVState.UpdateClosed(openTime, close, authoritative RSX)` owns both `rsx_tv_div` and `rsx_tv_pivot`. Jurik remains numerical truth only. Forming bars do not call `UpdateClosed` (no Snapshot). History/live/wake zip closed bars + SlotJurikRSX through the same machine. There is no display-bar / prefix reconstruct. Divergence: `ConfirmedAt` = confirm bar, `AnchorAt` = prior bar. Pivot: `ConfirmedAt` = confirm bar, `AnchorAt` = two bars back. Projector paints arrow-only markers. Semantics are not gated by a UI selector; persistent materialization may sleep with zero demand (DAG-DEMAND-1).

**RSX fact visibility (RSX-VISIBILITY-1 ✅ frozen, `749912f` + DAG-DEMAND-1 `0837c77`):** Facts are independent of presentation. UI does not choose detector semantics. Factual families may coexist. Visibility can create consumer demand. Zero demand may suspend materialization. Wake deterministically reconstructs facts from closed truth. The five families remain independent facts; checkboxes filter paint by `source`. Visibility is not in the RSX fingerprint and does not POST/replay Jurik. Annotation repaint key is `annotationRevision` + `visibilityMask` + `line_rsx` series identity. Do **not** write that every detector always runs forever.

**RSX fractal facts (RSX-SIGNAL-3 ✅ frozen, `c856fef`):** `scanRSXFractalHits` + `CheckClassicDivergence` emit `rsx_fractal_div` (`Pattern` `class_a` / `class_b` / `class_c`) and `rsx_fractal_pivot` (`high` / `low`). Semantics coexist with other families; live materialization follows demand. `AnchorAt` = fractal pivot OpenTime; `ConfirmedAt` = first closed bar that completes the fractal window (`pivot+PivotRadius` for divergence, `pivot+MacroPivotRadius` for P). Live/history share `FractalFactsAt` on that confirm bar only (O(lookback), not a prefix rescan). Class B paints arrow only; A/C add `A Bull` / `A Bear` / `C Bull` / `C Bear`. Fractal pivots are blue arrows. Old L/LL/S/SS/P are not public. Do **not** reopen detector math or lookback search.

**RSX ZigZag facts (RSX-SIGNAL-2A / 2A.1 ✅ frozen, `39d6f78` + RSX-SIGNAL-2B + SLOT-CLEAN-1):** `rsx_zz_div` with `Direction` bullish/bearish and `Pattern` regular/hidden. Swings come from the existing **RSX-adaptive** ZigZag (current-bar RSX only scales ATR sensitivity — not the divergence sample). On each newly confirmed swing, a closed-bar collector samples RSX once via hist lookback from the confirm bar, then keeps `{AnchorAt, IsHigh, Price, RSX}`. Identity is `(AnchorAt, IsHigh)`. No `DivergenceNode` / DAG Micro / ScoreNode / legacy score slots. `ann_rsx_div` owns the RSX pane via `SlotJurikRSX` (layout only). History uses **one** DAG walk (`ReplayClosedBars`) for scalars + ZZ facts. Regular → arrow only; hidden → `H Bull` / `H Bear`. Live annotation paint is revision-gated (no idle `setMarkers`).

ScoreNodes may trust: RSX value, RSX signal, HTF RSX numbers, divergence facts, thresholds.  
Not: backend colors, old L/LL/S/SS sockets, presentation strings, legacy chart helpers.

Remaining contracts:

| Component | Path | Role |
|-----------|------|------|
| `ScoreDecision` / `ScoreFactor` | `decision/score_types.go` | Decision sockets |
| `Frame` accessors | `market/` | State for future scoring |
| Falcon bus | `market/falcon.go` | Numerical calculator (Live/HTF/backtest). Scoring island removed. |
| Sizing | `execution/` | Quantity math socket |
| Qdrant | `vector_db/` | Pattern memory socket (#8) |

Future strategies live under `decision/`. They consume market state without importing `market` into contract packages (pass snapshots / interfaces at the composition root).

---

## Forecast Engine (FORECAST-SPEC-1 — contracts only, frozen)

**Frozen at:** `5afabfc` + `0ed000d`. Do not reopen SPEC contracts.

**FEATURE-TAPE-1A ✅ frozen `b88bcd2`.** Do not re-audit. Host: `market.FeatureEvaluator` (Frame stays FeatureID-free). `forecast` still does not import `market`.

**FEATURE-TAPE-1B ✅ frozen `6715718`.** JSONL FeatureTape dump. **FEATURE-TAPE-RSX-REGEN-1 ✅** regenerated the four-column tape under `analysis:v2`. `FUTURES_PERP` research source starts at USD-M listing genesis, not chart spot-stitch. Do not re-audit 1B.

**ATR-TRUTH-1 ✅ frozen `84124a0`.** Canonical `indicators.ATR` (`atr:wilder-rma-first-tr-v1`). Do not reopen ATR unless a consumer regression.

**LABEL-SET-1A ✅ frozen** `690d0be` + `1433626`. **LABEL-SET-1B ✅ frozen** `8e88844`.

**FEATURE-SPEC-2 ✅ frozen `0c54848`.** CatBoost V1 sensory contract only (no tape Fill, no labels, no model). Target C is a **new** TargetSpec (`research-15m-1m-c`, H=72, U=L=2.0, ATR-14 Wilder, finer 1m). Frozen v1 remains H=24 / 1.5 / 1.0. `q = H/4 = 18` (refuse if H%4≠0). Analysis: `analysis:v2`, RSX 14 / signal **14** / hlc3 / TV lookback 90 (not live `rsx_settings.json` signal 9). Width **64**: 42 primary 15m + 11 native 1h + 11 native 4h. Identity is `FeatureRecipe` + `FeaturePlan` (`features:v2` / `plan:v2`) plus `FeatureSpec2.Identity()` — no FeatureSchemaHash. HTF is native MarketKeys, latest closed row by `CloseTime(HTF) <= CloseTime(15m)`; latest NotReady ⇒ primary NotReady (no stale fallback, no 15m aggregation). HistoryDemand is per-TF native window bars; IIR (Jurik/ATR/TV) is from certified source start. Volume quarantined. TARGET-RESOLUTION-2 deferred.

**FEATURE-TAPE-2 ✅ frozen `61d5ca0`.** Brain V2 sensory runtime: `market.FeatureRuntime2` (thin Jurik/RSTV/ATR wrap, no Frame DAG) + `feature-tape-v2`. Dump API is `DumpFeatureTape2(spec, src15, src1h, src4h)`. V1 `FeatureEvaluator` / `DumpFeatureTape` remain historical only.

**LABEL-SET-C ✅ frozen `ce3e542`.** Brain V2 native tape door: `forecast.GenerateLabelSetFromTape2` / `market.DumpLabelSetFromTape2` reads `feature-tape-v2` directly. Shared owner is the existing first-passage / 1m finer core (`buildLabelsFromCandidates`). V1 `GenerateLabelSet` remains a legacy tape door into that core. Not a v2→v1 adapter. Format stays `label-set-v2` (fields are candidate-source generic). Target C only.

**DATASET-C + VALIDATION-PLAN-C ✅ frozen `151e530`.** Native `BuildResearchDatasetFromTape2` joins Tape2 + LabelSet-C in memory (`ResearchRow2` / `FeatureVector2`). Shared exclusive partition with V1. `ResearchValidationPlanC()` binds Target C (H=72) + pinned policy; `CompileValidationPlan` is unchanged (market-time `HorizonEnd`, not row-index gap). No dataset/plan disk files.

**OOF-MATRIX-C ✅ frozen `307b5e8`.** Model-neutral certified X/y/fold socket (`GenerateOOFMatrixFromTape2`). Format stays `oof-matrix-v1` (width = `len(FeatureIDs)`). Not CatBoost OOF logits. Holdout and causal seam are physically absent. V1 `GenerateOOFMatrix` remains a legacy door into the same assembler/writer.

**CATBOOST-BRAIN-1 ✅ frozen `cecc5a3`.** First certified model consumer of OOF-MATRIX-C. `CatBoostSpec1` is the hypothesis; Python only calls `CatBoost.fit`; `portable-catboost-v1` + Go evaluator own official raw logits. Quantization is fit-local. Inner-tail end is `NextBarOpen(last outerTrain At)`, not outer ValStart. HARD STOP before calibration/rank/recipe/holdout/final development model.

**Package:** `forecast/`. **Status:** SPEC + tape + TargetSpec pins `indicators.ATRSpec` + LabelSet JSONL. `forecast` may import `indicators` and `data` (`NextBarOpen` / `CurrentBarOpen` only). Still not `exchange`/`market`/`decision`/`execution`.

Not a scoring engine. Evidence → probability engine:

```text
MARKET TRUTH → ANALYTICAL TRUTH + FACTS → FeaturePlan → feature vector
    → EvidenceModel (later) → calibrated probability + RankPercentile
    → ForecastFrame → Decision (later)
```

**Governing law:** maximum capability in identities/contracts, minimum machinery until a real consumer exists. `minimal implementation != limited architecture`.

### Object ownership

| Object | Owns | Does NOT own |
|---|---|---|
| `MarketKey` | venue + instrument + contract + timeframe | — |
| `AnalysisRecipe` | indicator periods/sources/lookbacks, init/warmup law (via `LogicVersion`) | paint, weights, feature selection, TargetSpec, Decision thresholds |
| `FeatureRecipe` | which deterministic measurements are extracted (reusable across `AnalysisRecipe`s) | weights, arbitrary scoring |
| `FeaturePlan` | compiled bind of one `AnalysisRecipe` + one `FeatureRecipe` + logic versions | node graph / registry / plugin traversal |
| `TargetSpec` | frozen first-passage event + canonical `indicators.ATRSpec` + optional pinned `FinerTimeframe` | trade exits (`ExecutionSpec`); FeatureTape columns |
| `ForecastArtifactPinned` | which Market/Analysis/Features/Plan/Target identities a future artifact must pin | weights/scaler/calibration (added in FORECAST-RUNTIME-1) |
| `ForecastFrame` | minimal output shape + fail-closed validators | model arithmetic |
| `PaintPreset` | presentation only (concept, not yet a type) | anything numerical |

### MarketFrame vs AnalysisRuntime (capability law, not yet implemented)

```text
MarketFrame     = one market/bar truth
AnalysisRuntime = one AnalysisRecipe identity's analytical truth
```

Same `AnalysisRecipe` identity → same numerical process, should share one `AnalysisRuntime`. Different `AnalysisRecipe` identities (e.g. RSX14 vs RSX21) may run simultaneously over the same `MarketFrame` — that is deliberate isolation, **not** forbidden dual truth. Forbidden dual truth is two calculators claiming the **same** `AnalysisRecipe` identity while producing different numbers. `AnalysisRuntimeKey{Market, Analysis}` is defined as an identity type only — **no runtime map/registry/refcounting exists**; today's one-DAG-per-Frame implementation remains the v1 adapter, and "one `AnalysisRecipe` per `MarketFrame`" is **not** a permanent contract.

### Identity

Every published identity has: `HumanKey` (readable, never identity) + full SHA-256 `Digest` (compared in full; `Short()` is 16-hex display-only) + explicit `LogicVersion`(s). Digest hashes the **resolved** (post-default) config payload, never a friendly `Name`, never raw source. A correctness-changing implementation fix requires an explicit `LogicVersion` bump — hashing config alone cannot detect that.

### Ready vs absent

`NotReady` (insufficient warmup, incomplete ingress reconcile, bind failure, invalid state) → no `ForecastFrame`, no model evaluation, `dst` undefined. This is distinct from a feature being legitimately **absent** while the system **is** Ready (e.g. no TV divergence right now: `present=0`, age ignored, never a magic `-1`).

### Nonfinite numeric law

Existing NaN-as-"output unavailable" sentinels (sleeping RSX/Wozduh slots) remain valid and are **not** reopened here. NaN/±Inf must never enter persistent analytical state, a `FeaturePlan` feature vector, scaler/model input, or `ForecastFrame` probability/rank fields — see `ValidateFeatureVector` / `ValidateForecastFrame`.

### Candidate / TargetSpec / dual-hit

v1 candidate universe: every eligible closed bar (no strategy pre-filter). One `TargetFamilyATRFirstPassage` in v1: barriers frozen at candidate close `t` from information known at `t`. Dual-hit (`DualHitPolicy`): `exclude_ambiguous` stays `AMBIGUOUS`/`DUAL_HIT`; `resolve_finer_history` requires exactly one `FinerTimeframe` and one same-family finer stream (`SameFamily` — never spot resolving futures or vice versa). Never guessed, never a fourth model class.

### HTF closed availability / At vs actionable

An HTF value may be consumed only if that HTF bar was closed and authoritative by the candidate's close seam (never `HTF.OpenTime <= candidate.OpenTime`). `ForecastFrame.At` is closed-bar identity (Unix ms, UTC) — it does **not** mean the forecast was actionable at that bar's open; fill timing is a later Decision/Execution law.

### FeatureHistoryBars (feature-side only, not the complete closure)

`FeatureRecipe.FeatureHistoryBars()` bounds age-bearing features (`age = min(actualAge, MaxAgeBars)`, default 256) and is carried onto `FeaturePlan.FeatureHistoryBars`. This is the **feature-side history contribution only** — it does not yet fold in `AnalysisRecipe`/Jurik/detector reconstruction requirements (no such field exists on `AnalysisRecipe` in this chapter). The complete closure is reserved under the name `RequiredHistoryBars` (`= max(AnalysisRuntime reconstruction requirement, FeatureHistoryBars, ...)`), implemented in FEATURE-TAPE-1 once a real AnalysisRuntime binding exists. A `FeaturePlan` must not remember more history than live can reconstruct while claiming the same identity; mismatch → refuse/NotReady.

### FEATURE-TAPE-1A (trusted Fill, no files)

Host: `market.FeatureEvaluator` binds `forecast.FeaturePlan` to Frame Jurik slots + `rsxTVFacts`. Frame has no FeatureID methods. `forecast` does not import `market`.

Four columns: `rsx_value`, `rsx_signal`, `tv_bull_present`, `tv_bull_age`. `AnalysisRecipe` includes `DivLookback` so TV identity matches `EffectiveRSXSettings()`. Bind refuses digest mismatch.

Demand: `applied = client | internal | forecast` (`rsxClientDemand`, `rsxForecastDemand`, `rsxInternalMask()`). Forecast is a persistent third contribution.

Fill: compiled `[]FeatureID` + switch, one RLock, reverse TV-fact walk bounded by `FeatureHistoryBars` (facts are `ConfirmedAt`-sorted). `ConfirmedAt` gated, not `AnchorAt`. Steady-state Fill: 0 allocs/op.

Parity: hydrate `NewFrame(prefix)` vs live-style `UpdateKlineTick(..., true)` — same At/Ready/first Ready At/`Float64bits` on every Ready bar.

**HARD STOP.** Frozen `b88bcd2`. Do not re-audit 1A.

### FEATURE-TAPE-1B (immutable JSONL dump)

`forecast` owns format/writer/reader. `market.DumpFeatureTape` owns O(N) Frame + frozen Fill. Caller passes a materialized `[]Kline`. FormatVersion `feature-tape-v1`. Empty source / existing final path refused.

Identities: `PlanDigest` = feature semantics (not the whole future join). `SourceRangeDigest` = MarketKey + OpenTime + OHLCV `Float64bits` (not CloseTime, not snapshot isolation). `ContentDigest` = canonical semantic file hash excluding itself. `At` = source OpenTime.

**HARD STOP.** Frozen `6715718`. Do not re-audit 1B.

### ATR-TRUTH-1 (canonical ATR)

Owner: `indicators.ATR` / `ATRSpec`. Law: `atr:wilder-rma-first-tr-v1` (first-TR seed, Wilder RMA). Same spec ⇒ same transition; bit-identical values also need the same prior state or the same ordered closed init history. Period is not a reconstruction-history guarantee.

Checked `UpdateClosed` refuses nonfinite / High<Low with no IIR mutation. ATR=0 is Ready legal truth. `ATRSeries` loops that streamer only. Legacy `ATRValues` (nil when `len<=period`) unchanged. `navigatorATR` is a different SMA-of-TR statistic, not forecast ATR. Dead `CalculateATR` deleted.

`forecast.TargetSpec.ATR` is `indicators.ATRSpec` (no duplicate spec type). Changing Target ATR does not invalidate FeatureTape.

**HARD STOP.** Frozen `84124a0`. Do not re-audit ATR.

### LABEL-SET-1A (immutable first-passage labels)

`forecast` owns LabelSet types, first-passage, writer/reader. `market.DumpLabelSet` converts materialized `[]Kline` → `CanonicalClosedBar` and calls `GenerateLabelSet`. FormatVersion `label-set-v1`. Label logic `label:first-passage-primary-v1` (how the question is scored). `TargetDigest` remains the target question.

One LabelSet row per FeatureTape row, including `Ready=false`. Feature vectors are not copied. Header pins FeatureTape `PlanDigest` + `SourceRangeDigest` + `ContentDigest`.

### RESEARCH-DATASET-1 (strict consumption) ✅ frozen `f311203`

`forecast.BuildResearchDataset(tapePath, labelPath, expect)` opens both artifacts. Provenance first: exact `MarketKey`, `FirstAt >=` caller floor (research: `BinanceFuturesGenesisMs`), current FeaturePlan digest, LabelSet pins all three tape identities, exact `TargetDigest`. Then lockstep `len` + `At[i]`. Then exclusive eligibility: `!Ready` → FeatureNotReady; else non `{UP,DOWN,TIMEOUT}` → `ExcludedByReason`; else copied `ResearchRow`. Partition must close. In-memory only — no dataset file. No `SameFamily` at this boundary.

**HARD STOP.** Frozen `f311203`. Reopen only if a downstream regression falsifies this contract.

### VALIDATION-PLAN-1 (outcome-blind walk-forward) ✅ frozen `0737c59`

`forecast.CompileValidationPlan(at, tf, plan)` is the only geometry owner. Inputs: strictly increasing `At[]`, timeframe, resolved `ValidationPlan` (logic `validation:walk-forward-v1`). Market-time packing from the holdout wall via `CurrentBarOpen` / `PreviousBarOpen` / `HorizonEnd`. Causal gap = `TargetH + ExtraGapBars` (no independent `PurgeBars`). Expanding train; disjoint val windows; fixed `HoldoutStartAt`; seam rows are not a third class. Identity hashes **rules only** (includes timeframe + TargetH resolved from TargetSpec). Ranges, not per-row tags. BTCUSDT 15m binding lives in `market.ResearchValidationPlan`.

**HARD STOP.** Frozen `0737c59`. Docs freeze `45155eb`. Do not reopen packing.

### RESEARCH-DATASET-C + VALIDATION-PLAN-C (Brain V2 population + H=72 geometry)

`forecast.BuildResearchDatasetFromTape2` reads `feature-tape-v2` + LabelSet-C natively. Shared `partitionResearchPopulation` with V1. Exclusive order: Tape2 `Ready==false` → FeatureNotReady (even with a legal label); else non-{UP,DOWN,TIMEOUT} → Excluded(reason); else `ResearchRow2` with copied `FeatureVector2`. In-memory only.

`market.ResearchValidationPlanC()` binds the pinned forecast policy (`HoldoutStartAt=1767225600000`, `FoldCount=4`, `ExtraGapBars=0`, `MinTrainRows=35040` observation floor, `ValidationSpanBars=17568` 183-day 15m clock, not DecisionResearch 8640). TargetH is copied from Target C (72). Compile with `CompileValidationPlan(trainable At[])` only. Causal gap is `HorizonEnd(..., 72)`, not `index-72`. Realized ValN may be < 17568. Do not copy V1 fold indexes. No OOF file, no CatBoost.

**HARD STOP.** Frozen `151e530`.

### OOF-MATRIX-C (Brain V2 model-neutral X/y/fold socket)

`forecast.GenerateOOFMatrixFromTape2` is the native V2 door: `ReadTape2` + `BuildResearchDatasetFromTape2` + `ResearchValidationPlanC` / caller plan + `CompileValidationPlan`. It does not convert Tape2→Tape1, call `ReadTape`, or use `FeatureEvaluator`. Shared `assembleOOFMatrix` + JSONL writer with the V1 door.

The artifact is certified supervised input: ordered FeatureSpec2 `X[64]`, Target-C outcomes, development `At[]`, compiled Plan-C ranges in the header. It is **not** model output (no logits, no CatBoost). Future model families consume this file; CATBOOST-BRAIN-1 needs only matrix path + expected ContentDigest + CatBoostSpec1.

Population is Dataset-C trainable rows `[0:DevelopmentEndIndex)` — full development, not the validation-slice union. Causal seam and holdout are physically absent. No per-row FoldID. Class order contract (not a header hash): `UP_FIRST`, `DOWN_FIRST`, `TIMEOUT`. Format remains `oof-matrix-v1`. New `research/oof/` slot via existing filename helper. MATCH exact / REFUSE different or corrupt.

**HARD STOP.** Frozen `307b5e8`.

### CATBOOST-BRAIN-1 (first certified OOF-MATRIX-C learner)

Go owns CatBoostSpec1, class map `UP_FIRST=0 / DOWN_FIRST=1 / TIMEOUT=2`, `SplitCausalTail`, resolved FitPlan, vendor JSON → `portable-catboost-v1`, prefix logloss, tree-count selection, official OOF logits. Python `research.catboost1` is `CatBoost.fit` only (numeric y, fit-local Pool, no eval_set / early stopping / scaler / class weights).

Inner tail: `tailExclusiveEnd = NextBarOpen(last outerTrain At, 15m)`. Do not reuse `CompileValidationPlan` as a fake-holdout. Outer fold ranges stay frozen from the matrix header.

Official logits are Go(`portable-catboost-v1`) on the union of the four outer-val slices (`catboost-oof-logits-v1`). V1 `oof-logits-v1` is logistic-specific and is not reused. Vendor `.cbm`/JSON are witnesses. Quantization/borders are learned per fit on that fit's train rows only.

Predeclared, not implemented: later final development model uses the same inner-tail selection on all development rows, then a fresh fit with `N_final`. Never average OOF N.

**NEXT:** CatBoost OOF calibration/rank/forecast-recipe. Do not start here.

**HARD STOP.** Frozen `cecc5a3`.

### OOF-MATRIX-1 (development-only statistical interchange)

`forecast.GenerateOOFMatrix(tapePath, labelPath, outPath, expect, validationPlan)` opens FeatureTape + LabelSet, TOCTOU-compares full source identities, calls frozen `BuildResearchDataset`, extracts `At[]`, calls frozen `CompileValidationPlan`, and writes `ResearchRows[0:DevelopmentEndIndex)` as JSONL `oof-matrix-v1` (`AtUnit=unix_ms`). Causal seam and holdout rows are physically absent. Rows are raw `At` + ordered features + `UP_FIRST|DOWN_FIRST|TIMEOUT`. Fold indices are matrix-local ranges (not per-row tags). `ValidationPlanDigest` hashes experiment rules only. Footer `ContentDigest` hashes this realized development matrix (including compiled ranges). One slot under `research/oof/`; existing exact artifact MATCH; any difference REFUSE. No scaler, model, holdout file, or SQLite reread.

**HARD STOP.** Frozen `d749042`. Do not reopen assembly.

### MODEL-FIT-1 (train-only multinomial logistic OOF logits)

Python-only fitting brain (`python -m research.modelfit`). Input is one explicit `oof-matrix-v1` path plus CLI `--expect-matrix` ContentDigest. Per fold: `fit_fold_model(X_train, y_train, spec)` then `predict_fold_logits(fitted, X_val)`. sklearn `StandardScaler` + `LogisticRegression` (lbfgs, C=1.0) are the numerical owner. Runtime pin is Python + NumPy + sklearn + SciPy (L-BFGS execution provenance; not `ModelSpecDigest`). Output `oof-logits-v1`: `At`, `Outcome`, `Logits[3]` only; fold lineage is source/output ranges in the header. Existing slot MATCH rehashes the file and does not refit. No metrics, calibration, holdout path, or Go logits stack.

**HARD STOP.** Frozen `0d60270` + SciPy pin `29cb928`. Do not reopen fitting.

### CALIBRATION-1 (one global inverse temperature)

Sibling of RANK-1. `python -m research.calibration` reads frozen `oof-logits-v1` via existing `read_oof_logits` (no MODEL-FIT sklearn gate). Fits one `β ≥ 0` on all OOF rows: `softmax(β z)`, mean NLL, analytic gradient, SciPy L-BFGS-B with explicit `gtol` and `ftol` in CalibrationSpec. Product is one JSON `temperature-calibration-v1` (β + provenance), not a probability dataset. Execution pin: Python/NumPy/SciPy only. No metrics, rank, holdout, or final development-model fit.

**HARD STOP.** Frozen `4c8c08e` + ftol pin `b550613`. Do not reopen calibration.

### RANK-1 (exact empirical directional rank)

Sibling of CALIBRATION-1 (no import, no β). `python -m research.rank` reads the same frozen `oof-logits-v1` via `read_oof_logits`. Directional evidence is `D = logit[UP_FIRST] - logit[DOWN_FIRST]` after an exact source `class_order` match. The product is one JSON `empirical-rank-v1`: exact sorted float64 sample plus `RankSpecDigest`. Query law is a scalar step-CDF midrank (`searchsorted` left/right index convention; NumPy is implementation, not a rules-digest symbol). Uniform row weights; finite D and finite query required. Reader validates sortedness and does not repair. Runtime gate is Python/NumPy only.

Statistical assumption (documented, not repaired): this reference pools expanding-fold OOF logits; a later full-development model is assumed sufficiently compatible in directional scale/order. No per-fold CDFs or rescaling.

**HARD STOP.** Frozen `cc49528`. Do not reopen rank.

### RECIPE-FREEZE-1 (forecast composition, no learning)

`python -m research.recipe` binds frozen `oof-logits-v1` + `temperature-calibration-v1` + `empirical-rank-v1` into one tiny `forecast-recipe-v1` (child ContentDigests + composition law tokens). Construction reads all three children (logits prove `ModelSpecDigest` via `pinned_model_spec()`). Runtime `load_forecast_recipe(recipe, calibration, rank)` does **not** reopen OOF logits. Scalar `project_forecast` exposes `ForecastEvidence{probabilities[3], directional_rank}`: independent branches (`β=0` uniform before max-shift softmax; raw `query_rank` from RANK-1). No β/sorted_D copy, no runtime versions in the durable recipe, no score, no thresholds, no final fit.

Transfer assumptions (documented, not repaired): OOF-learned β and pooled directional rank remain meaningful for a later full-development model.

**HARD STOP.** Frozen `c2d278a`. Do not reopen recipe.

### FINAL-MODEL-FIT-1 (all-development raw-logit model)

`python -m research.finalfit` uses frozen `forecast-recipe-v1` as precommit root (`read_recipe` only — not `load_forecast_recipe`). Provenance: recipe → `oof-logits-v1` → `oof-matrix-v1`. One fresh `fit_fold_model(X_all, y_all, pinned_model_spec())` on every matrix row in stored order. Product is `final-base-model-v1` (scaler/coef/intercept only). Recipe digest is not stored on the model. MATCH does not fit and does not require current sklearn/SciPy. No metrics, portable inference, bundle, or holdout.

**HARD STOP.** Frozen `9cb03f5`. Do not reopen final-model-fit.

### FORECAST-BUNDLE-1 (executable forecast contract)

`python -m research.bundle` proves one research world (final model + recipe + calibration + rank, with OOF logits/matrix as construction witnesses only). Durable `forecast-bundle-v1` binds law-only contracts from oof-matrix-v1 field names: ordered `feature_ids` + `feature_plan_digest`; `class_order` + `target_digest` + `label_logic_version`. No tape ContentDigest in the live feature contract. Runtime load does not use OOF artifacts. Type-state: `bind_feature_contract` → `BoundForecaster` → scalar `forecast`. Portable affine application of stored mean/scale/coef/intercept; composition is existing `project_forecast`. Python/NumPy only at execute. No metrics, decisions, or holdout.

Roadmap fork (not chosen here): holdout for forecast quality vs preserve holdout until a frozen decision/trading law.

**HARD STOP.** Frozen `a9e1228`. Do not start decision research or HOLDOUT-EVAL-1 here.

### OOF-FORECAST-EVIDENCE-1 (decision-development evidence snapshot)

`python -m research.evidence` materializes frozen `forecast-recipe-v1` over honest `oof-logits-v1` rows in source order. Product is `oof-forecast-evidence-v1`: `At`, copied `Outcome`, `probabilities[3]`, `directional_rank`. Matrix is a target-law witness only (`market`, `at_unit`, `target_digest`, `label_logic_version`). No final-model or bundle execution. No metrics, decisions, or holdout.

Statistical hierarchy (honesty): OOF raw logits are honest relative to base-model fitting. This evidence table applies the frozen global β and global rank reference (self-inclusion on historical D). It is suitable for decision development, not an unbiased end-to-end evaluation of the complete forecaster. Future decision temporal folds are robustness/selection discipline only. Sealed holdout remains the first honest full-system evaluation. Do not create fold-local β/rank variants.

Roadmap after evidence: DECISION-VALIDATION-PLAN-1 (frozen `a0da055`) → DECISION-CONTRACT-1 (frozen `b7a76b4`) → DECISION-RESEARCH-1 (selector; not holdout).

**HARD STOP.** Frozen `124f273`. Do not start decision research or HOLDOUT-EVAL here.

### DECISION-VALIDATION-PLAN-1 (temporal decision-research geometry)

`python -m research.decisionplan` reads frozen `oof-forecast-evidence-v1`, extracts `At[]` only, and calls existing `forecast.CompileValidationPlan` via a thin Go stdin bridge (`cmd/research_decision_plan`). Matrix is a target/holdout-wall witness only. Fresh logic `decision-validation:walk-forward-v1`. Current binding: 4×8640-bar (~90d) val eras, `MinTrainRows=35040`, `ExtraGapBars=0`, `TargetH` from `ResearchTargetSpec`, same `ResearchHoldoutStartAt` as the model plan. Evidence-local ranges; no model FoldID reuse. Decision logic may compile an `At[]` that ends before the wall (no holdout rows in evidence); model logic still requires a holdout membership in `At[]`.

Honesty: development selection/robustness only. Not end-to-end evaluation. `evidence.At` is the closed candidate bar, not an execution time.

**HARD STOP.** Frozen `a0da055`. Do not start decision research, execution timing, or HOLDOUT-EVAL here.

### DECISION-CONTRACT-1 (runtime decision language)

`decision.ApplyDecision(ForecastEvidence, DecisionSpec) (DirectionalIntent, error)` is the only v1 decision brain (`decision:target-utility-rank-gate-v1`). Intent is `UP_INTENT` / `DOWN_INTENT` / `ABSTAIN` — not an order. Input is frozen `ForecastEvidence` only (no `At`, Outcome, folds). `DecisionSpec` copies TargetSpec barrier multiples as target-space utilities (not PnL) plus two future search knobs (`min_EU`, `min_abs_rank`) with `0 < min_EU < min(U,L)` and `0 < min_rank < 1`. One quantity `EU = U*P_UP - L*P_DOWN`; `EU_DOWN ≡ -EU`. Conjunction of utility and rank gates. Invalid evidence is an error, never ABSTAIN. Does not reuse `ScoreDecision` BUY/SELL/WAIT. Selector/grid/folds/metrics/execution/holdout are later chapters.

**HARD STOP.** Frozen `b7a76b4`. Do not reopen the runtime contract. Selector/grid live in DECISION-RESEARCH-1.

### DECISION-RESEARCH-1 (frozen selector over development ForecastEvidence)

`python -m research.decisionresearch` binds a frozen `decision-validation-plan-v1` to `oof-forecast-evidence-v1` and runs `decision-research:fixed-grid-v1` in Go (`decisionresearch`, `cmd/research_decision_research`). Grid is exactly utility_decile × rank_decile `1..9` (81 candidates). Thresholds resolve as `base=min(U,L); min_EU=base*(float64(du)/10.0); min_rank=float64(dr)/10.0` from the existing TargetSpec owner (U/L are not hardcoded). Train selects one candidate or research-only `ABSTAIN_BASELINE` (TotalUtility 0; replace only if candidate TotalUtility `>` 0). Positive ties: higher utility_decile, then higher rank_decile. Validation evaluates only that selection via `decision.ApplyDecision` (never Outcome, never At). Evaluator owns integer intent×outcome counts and `TotalUtility = U*(up_up - down_up) + L*(down_down - up_down)`. Pooled validation sums counts then applies the same law. Acceptance is two gates: pooled validation TotalUtility `>` 0, and `positive_folds * 4 >= fold_count * 3`. `eligible_for_finalization` is that development robustness result only — not PnL, not deployable, not holdout. A negative result is a valid frozen experiment; do not densify the grid or invent indicator knobs. MATCH verifies the artifact and does not reenact the selector. Format `decision-research-v1`. No scoreboard of losing candidates. Holdout remains sealed. Do not start FINAL-DECISION-SPEC-1 or EXECUTION-TIMING-1 here.

**HARD STOP.** Frozen `789ddcd`. Research result `eligible_for_finalization=false` is a completed experiment, not an implementation failure. Do not start FINAL-DECISION-SPEC-1 or EXECUTION-TIMING-1 here.

Two source ranges in one run: **ATR source** = `[init | candidates]` (contiguous via `data.NextBarOpen`, else REFUSE generation — not a row reason); **label source** = that prefix plus the needed H tail. `ATRSeries` runs only on ATR source. `LabelSourceRangeDigest` still hashes the full label source. Restart after an archive hole is the caller's input-slice choice.

Barriers freeze at candidate close: `close[t] ± multiple * atr[t]`. Future ATR cannot move them. `atr[t] <= 0` → `AMBIGUOUS` / `ATR_ZERO`. Nonfinite ATR or barriers refuse generation.

Scan starts at the next primary closed bar (`t+1`), never candidate High/Low. Touches are inclusive High/Low. Same-bar dual-hit → `AMBIGUOUS` / `DUAL_HIT` under `exclude_ambiguous`. Complete H with no touch → `TIMEOUT`. Incomplete H without an earlier result → `AMBIGUOUS` / `TRUNCATED_HORIZON` (never fake TIMEOUT). An unexplained missing primary interval **after** the candidate and before the outcome is known → `AMBIGUOUS` / `PRIMARY_GAP`. A later gap after a definitive hit is ignored. A hole in ATR source is not `PRIMARY_GAP`.

`LabelSourceRangeDigest` hashes the exact consumed primary bars (LS1S + MarketKey + OpenTime + OHLCV `Float64bits`). Extra caller bars after the used end are not hashed. `ContentDigest` covers header identities, every row `At/Outcome/HitAt/Reason`, and footer range metadata.

**HARD STOP.** LABEL-SET-1A frozen `690d0be` + `1433626`. Do not reopen primary first-passage physics.

### LABEL-SET-1B (pinned finer dual-hit resolution)

Not a second labeler. `firstPassage` stays 1A. Only a primary `DUAL_HIT` plus `DualHitPolicy=resolve_finer_history` may call `resolveFinerDualHit`. `exclude_ambiguous` rows stay 1A (`DUAL_HIT`).

`TargetSpec.FinerTimeframe` is identity. Required and non-empty iff resolve; must be empty iff exclude. Empty field is `json omitempty` so frozen exclude `TargetDigest` bytes stay 1A-identical. `1m` vs `1s` are different digests. No fallback list.

Finer source is one materialized same-family `FinerMarketKey` (`SameFamily` = venue+instrument+contract). Header timeframe must equal `TargetSpec.FinerTimeframe`. Caller materializes bars; `forecast` does not import `market`/`exchange` and does not query archives. `market.DumpLabelSetWithFiner` only converts both `[]Kline` slices.

“Finer” is calendar tiling via `data.NextBarOpen`, not duration ranking: hops from parent open `P` must land exactly on `NextBarOpen(P, primaryTF)` and never jump past it. Current historical research TargetSpec pins **15m → 1m** (MarketKey 15m + FinerTimeframe 1m). Engine has no 1m/1s default and no runtime TF fallback.

Parent window `[P, Q)`. Finer path must begin at `P`. Missing initial segment → `FINER_MISSING`. Gap before a result → `FINER_GAP`. A later gap after a definitive finer touch is ignored. A finer bar that itself dual-hits → `FINER_DUAL_HIT`. Complete contiguous finer tiling with no touch despite primary OHLC → `FINER_INCONSISTENT` (row-level; do not kill the LabelSet). Successful resolve → `UP_FIRST`/`DOWN_FIRST` and `Reason=NONE`. `HitAt` remains the **primary** dual-hit `OpenTime`.

Format `label-set-v2`, logic `label:first-passage-finer-v1`. Header adds `FinerMarketKey`. Footer adds `FinerSourceDigest` (domain `LS1F` + FinerMarketKey + per dual-hit attempt: CandidateAt, PrimaryDualHitAt, consulted bars until stop) and `FinerWindowCount` (attempts, not successes). Zero primary dual-hits hashes FinerMarketKey with zero windows and does not consume unused finer archive. An attempted window with zero bars is a different digest. `ContentDigest` covers FinerMarketKey, rows, and those footer fields.

**HARD STOP.** Frozen `8e88844`. Kill-check GREEN. Do not reopen 1B. 1s historical microscope is **TARGET-RESOLUTION-2** in `OPEN_DEBTS.md` — a separate TargetSpec, not an upgrade of 15m→1m.

### LABEL-SET-C (Brain V2 native labels)

Product path: `feature-tape-v2` → `GenerateLabelSetFromTape2` → canonical label core. Shared core knows ordered `At[]`, primary/finer bars, and `TargetSpec` only. It does not read `Values[64]`. Ready and NotReady tape rows are both candidates (one tape row → one LabelSet row). `FeatureTapeSourceRangeDigest` binds Tape2 **primary** `PrimarySource`. `FeatureTapeContentDigest` witnesses the exact Tape2 realization (including HTF columns that are not label math). Format remains `label-set-v2`. V1 dump/entrypoints unchanged.

**HARD STOP.** Frozen `ce3e542`. Next when asked: DATASET-C / VALIDATION-PLAN-C preparation. Do not start here.

### Fail closed

`PublishForecastFrame` is the single gate: `NotReady` or an invalid/nonfinite probability set (`ValidateForecastFrame`: finite, in `[0,1]`, sums to ~1) → **no** `ForecastFrame`. Never zero-fill, never reuse a stale vector/frame, never fall back to a default recipe. Same law applies to `MarketKey` mismatch, schema/logic mismatch, and artifact digest failure once those paths exist.

### OOF vs final refit (research law, not yet implemented)

Walk-forward fold models exist only to produce honest out-of-fold predictions for calibration/RankPercentile. Production weights come from one **final** development-window refit; calibration/rank tables stay OOF-derived. Never deploy "the best-looking fold model."

### Configuration lifecycle (vocabulary only)

`Draft → Save As → Publish → Activate → Promote`. No global "current settings"; no `LiveSettings`/`BacktestSettings` copies. Long-running surfaces use bindings; finite jobs pin an immutable `RunManifest`. No UI/config DB/persistence in this chapter.

### Invalidation table

| Changed | FeatureTape | Labels | Model | Calibration/Rank | DecisionRun |
|---|---|---|---|---|---|
| PaintPreset | NO | NO | NO | NO | NO |
| AnalysisRecipe | YES | YES | YES | YES | YES |
| FeatureRecipe | YES | YES | YES | YES | YES |
| TargetSpec | NO | YES | YES | YES | YES |
| ValidationPlan | NO | NO | YES | YES | YES |
| ModelSpec | NO | NO | YES | YES | YES |
| DecisionSpec | NO | NO | NO | NO | YES |
| ExecutionSpec | NO | NO | NO | NO | YES |

### v1 scope / deferred machinery

v1 is closed-bar forecast only (no forming-bar probability). Explicitly deferred, **not** built here: Python/training, model inference, multi-runtime map/registry/refcounting, `FeaturePlan` union across models, config database, Save As UI, activation infrastructure (`atomic.Pointer` swap / `EffectiveFrom` seam ownership), `MarketDataSnapshot`/`SourceManifest` machinery, Decision, backtest, Qdrant/Reliability, TARGET-RESOLUTION-2 (separate 15m→1s TargetSpec). REST/WS stitching and reconcile remain owned by the existing Boot/Ingress FSM — Forecast only checks Frame continuity/publishable, never a second reconcile path.

**Do not touch in this or future chapters without an explicit new debt:** RSX/Wozduh formulas, TV/Fractal/ZZ fact semantics, `AnchorAt`/`ConfirmedAt`, DAG-DEMAND-1, Wozduh demand, market reducers, sparse-seconds architecture, Boot/REST/WS reconcile, timestamp architecture, history/camera, Falcon, old scores, strategies, execution, backtest, Decision.

---

## MTF (two modes — do not mix)

**Scoring MTF** (when strategies return): walk-forward boundary tracker, zero look-ahead (`GetCandlesStrictlyBefore`), updates only on HTF close boundaries.

**UI MTF** (navigators / overlays): `HTFProvider` + DAG navigator series for chart chrome. Must not silently drive live entry math unless wired through the scoring path.

---

## Projection & charts

Pipeline: **State → Projection → Transport → Paint**.

| Concern | Owner |
|---------|-------|
| History window | `server/HistoryProvider` (SQLite ∪ RAM) |
| Slot → wire | `server/wire` Projector |
| Live tick broadcast | `BroadcastChartTick` / `RouteChartTick` (strict per-TF; `1m` ≠ `1M`) |
| FE composition | `web/boot.js` |
| Store | `web/columnar-store.js` (gap-detect; no silent hole glue) |
| Paint | `chart-compositor.js` + `RenderScheduler` (F1/F2/F3) |
| Camera | Sticky Live Edge / Microscope (`viewport-manager.js`) — TF mechanics CLOSED |
| Scale | `ScaleController` = Auto/Manual/Log; DDR `scaleContribution` = what Auto measures (ADR-022) |

**Tip Ownership:** Native/1s History = Cap-closed only (`dropFormingTip` + Replay). Viewport may seed Frame forming tip (ADR-010); WS OVERWRITE same open. 5s–45s HTTP: closed Replay immutable; at most one Frame forming row appended (`projectSparseSecondFormingTip`, SPARSE-ADR010-TIP-1). Frame runtime replay = closed→forming (ADR-016); never commit forming during replay.  
**Discard axis:** `window.projectionEpoch`.  
**Time axis labels:** UTC unix data unchanged. Crosshair uses detailed local-TZ `localization.timeFormatter`; axis ticks use minimal `tickMarkFormatter` by LWC `TickMarkType` ([`web/chart-core.js`](../web/chart-core.js)). Bottom-axis owner via ADR-023 `timeScale.visible`; future strip via ADR-027 Decoration Plane; crosshair time label always rendered on that owner (not the hovered pane).  
**Wozduh:** DAG bus only; Falcon Evaluate gated; legend = chrome only (no per-tick HTML metrics). **WOZDUH-WIRE-1 frozen** (`0c2ecce`): live/history pack only subscribed Wozduh scalar plot IDs. **WOZDUH-ACTIVE-1A frozen** (`2cd4ca4`): `/api/history` replay runs only the requested Wozduh compute closure; `ReplayClosedBars` default is still compute-all. **WOZDUH-ACTIVE-1B frozen** (`1b724ef`): persistent Frame Wozduh mask is per-TF WS union OR Live fast/slow closure; ChartOnly unused Frames compute none. Enable hydrates the current store window before reveal. `woz_slow` stays on the wire while hidden (pane/crosshair owner).

**DAG-DEMAND-1 ✅ frozen** (`0837c77`). PRESENTATION does not own computation. Layers are distinct: bar truth (always) / analytical truth (consumers) / fact materialization (consumers) / transport (subscribe) / paint (visibility). `RSXCore` does not imply TV, Fractal, or ZZ. ScoreNodes later OR into the same mask — no redesign. Do **not** reopen.

**Future-indicator merge checklist (no framework):** (1) outputs (2) consumers (3) dependencies (4) stateful/stateless (5) closed-history reconstruction (6) zero-consumer behavior (7) history/live parity (8) unavailable-output (NaN, omit) (9) forming-bar law (10) measured cost.  
**Floating menus:** `position:fixed` viewport (`floating-menu.js`).

**UI paint laws:**

1. Only `ChartAdapter` talks to Lightweight Charts.
2. Paint reads Store through a window (`extractWindow`), not raw full store.
3. `RenderScheduler` is the only paint initiator.
4. Cold boot camera uses width-independent APIs only (`applyOptions` barSpacing/rightOffset) — no `setVisibleLogicalRange`/`fitContent` on 0×0 containers. **Debt #80:** compositor defers TimeCamera.propose until host has layout (no raw LWC restore).

---

## Persistence

| Piece | Role |
|-------|------|
| `data/history_db.go` | SQLite kline cache; **one** `sql.DB` conn (`SetMaxOpenConns(1)`) so WAL TRUNCATE can run |
| `data/persistence_queue.go` | Async sole runtime UPSERT + periodic WAL checkpoint |
| `market` catch-up / gap-fill | Enqueue closed bars; never sync-write from Frame hot path |
| `cmd/repair_volumes` | Ops healer for stuck volumes (run with bot stopped) |

---

## Key file map

| Area | Paths |
|------|-------|
| Ingress | `exchange/ingress.go`, `exchange/klines.go` |
| Frame / streaming | `market/frame.go`, `streaming.go`, `snapshot.go` |
| Runtime / Boot | `market/runtime.go`, `boot_controller.go` |
| Timeline publish gate | `market/kline_gap.go`, `exchange/ws.go` hooks, `web/boot.js` + `ws.js` |
| Decision | `decision/score_types.go` |
| DAG | `core/runner.go`, `core/nodes/`, `market/dag_shadow.go` |
| Falcon | `market/falcon.go` |
| History delivery | `server/history_provider.go`, `server/columnar_history.go`, `server/wire/` |
| Frontend | `web/boot.js`, `columnar-store.js`, `chart-compositor.js`, `ui/viewport-manager.js` |
| Strategy beacon | `strategy/doc.go` |

---

## Env / run

```bash
cp .env.example .env
go build ./...
go run .          # dashboard :8080, ChartOnly by default
```

Important env: `ENGINE_MODE` (`ChartOnly` | `live`), `TRADING_SYMBOL`, `TRADING_TIMEFRAME`, Binance keys, `READ_ONLY`, `SANDBOX_MODE`.

**NEXT:** see `docs/OPEN_DEBTS.md`. Next chapter is CatBoost OOF calibration/rank/forecast-recipe. TARGET-RESOLUTION-2 deferred.
