# Architecture (Current System)

**SSOT for:** how the system works **today**.  
**Not SSOT for:** engineering laws (→ `jeweler-protocol.mdc`), history (→ `HISTORY.md`), why-decisions (→ `DECISIONS.md`), backlog (→ `OPEN_DEBTS.md`).

**Version:** Core 5.0 Data Plane (Phases A–G ✅) + Core 6.0/6.1 Docs + **Debt #69A FE Memory Budget** + **Debt #81 Timeline Publish Gate**.

**Default mode:** `ENGINE_MODE=ChartOnly`. Trading stack re-enters only with new `decision/` strategies + `ENGINE_MODE=live`.

---

## SSOT map

| Document | Owns |
|----------|------|
| `.cursor/rules/jeweler-protocol.mdc` | Engineering rules (always-on) |
| `.cursor/rules/senior-quant-architect.mdc` | Role / thinking style (always-on) |
| `docs/ARCHITECTURE.md` | Current architecture + **Core Ownership Model** (frontend constitution) |
| `docs/OPEN_DEBTS.md` | Open backlog / NEXT |
| `docs/HISTORY.md` | Completed phases (on request) |
| `docs/DECISIONS.md` | Why key choices were made (ADR-lite) |
| `MEMORY.md` | Index only — no content duplication (never rebuild as encyclopedia) |
| `README.md` | Landing: what / build / links |

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
| **ViewportManager** | Capture/translate helper only (ADR-028). Not a camera owner; no direct LWC camera writes (target after Phase D). |

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
| `HistoryProvider` | Chart history window owner: SQLite ∪ RAM |
| `Projector` | Slot → wire packer for live plots + columnar history |
| `ScoreDecision` / `ScoreFactor` | Decision contracts in `decision/` |
| `ProjectionEpoch` | FE discard axis for TF / load / hydrate / WS |
| `Tip Ownership` | History = Cap-closed only; Viewport may seed Frame forming tip (ADR-010 / TV Model 2); WS overwrites that tip; Frame replay = closed→forming (ADR-016) |
| `Bar boundary` | ADR-011: fixed TF = duration floor; calendar TF (`1w`/`1M`) = Monday / 1st-of-month UTC (`CurrentBarOpen` / `Prev` / `Next`) |
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
| Paint | `extractWindow` is still tip-tail (15k). **Future 69D:** if store is mid-history, paint must follow viewport |
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

Remaining contracts:

| Component | Path | Role |
|-----------|------|------|
| `ScoreDecision` / `ScoreFactor` | `decision/score_types.go` | Decision sockets |
| `Frame` accessors | `market/` | State for future scoring |
| Falcon bus | `market/falcon.go` | Keep until #76 |
| Sizing | `execution/` | Quantity math socket |
| Qdrant | `vector_db/` | Pattern memory socket (#8) |

Future strategies live under `decision/`. They consume market state without importing `market` into contract packages (pass snapshots / interfaces at the composition root).

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

**Tip Ownership:** History = Cap-closed only (`dropFormingTip` + Replay). Viewport may seed Frame forming tip after projection (ADR-010). WS updates that tip (OVERWRITE). Frame runtime replay = closed→forming lifecycle (ADR-016); never commit forming during replay.  
**Discard axis:** `window.projectionEpoch`.  
**Time axis labels:** UTC unix data unchanged. Crosshair uses detailed local-TZ `localization.timeFormatter`; axis ticks use minimal `tickMarkFormatter` by LWC `TickMarkType` ([`web/chart-core.js`](../web/chart-core.js)). Bottom-axis owner via ADR-023 `timeScale.visible`; future strip via ADR-027 Decoration Plane; crosshair time label always rendered on that owner (not the hovered pane).  
**Wozduh:** DAG bus only; Falcon Evaluate gated; legend = chrome only (no per-tick HTML metrics).  
**Floating menus:** `position:fixed` viewport (`floating-menu.js`).

**UI paint laws:**

1. Only `ChartAdapter` talks to Lightweight Charts.
2. Paint reads Store through a window (`extractWindow`), not raw full store.
3. `RenderScheduler` is the only paint initiator.
4. Cold boot camera uses width-independent APIs only (`applyOptions` barSpacing/rightOffset) — no `setVisibleLogicalRange`/`fitContent` on 0×0 containers. **Debt #80:** `ViewportManager.restore` uses the same rule (fresh fallback + deferred restore when host has layout).

---

## Persistence

| Piece | Role |
|-------|------|
| `data/history_db.go` | SQLite kline cache |
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

**NEXT:** see `docs/OPEN_DEBTS.md` — primary: **#76 ScoreNodes**, **#69D**.
