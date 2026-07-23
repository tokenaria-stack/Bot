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
| `docs/ARCHITECTURE.md` | Current architecture (this file) |
| `docs/OPEN_DEBTS.md` | Open backlog / NEXT |
| `docs/HISTORY.md` | Completed phases (on request) |
| `docs/DECISIONS.md` | Why key choices were made (ADR-lite) |
| `MEMORY.md` | Index only — no content duplication (never rebuild as encyclopedia) |
| `README.md` | Landing: what / build / links |

Memory-update routing (user: «сохрани в памяти» / «update MEMORY»): see Role
`.cursor/rules/senior-quant-architect.mdc` → section **When user says "save / update memory"**.

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
13. **RAM ≠ SQLite.** Frame/Runtime = realtime; SQLite = archive ledger. Healthy RAM ≠ healthy DB tip. **SQLite catch-up ≠ Frame heal** — chart/DAG truth requires `LoadHistoricalKlines` + replay, not archive enqueue alone.
14. **Frontend ≠ history DB.** `ColumnarStore` is a bounded display window (Debt #69A). Server owns durable history. Viewport never mutates OHLC/plots.
15. **Timeline publish gate.** `WS Connected ≠ History Reconciled ≠ Timeline Publishable`. Mid-session heal follows ADR-017; FE recovery presentation follows ADR-018.

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
| Scale | `ScaleController` SSOT (`chart_scale_prefs_v2`, Auto ON default) |

**Tip Ownership:** History = Cap-closed only (`dropFormingTip` + Replay). Viewport may seed Frame forming tip after projection (ADR-010). WS updates that tip (OVERWRITE). Frame runtime replay = closed→forming lifecycle (ADR-016); never commit forming during replay.  
**Discard axis:** `window.projectionEpoch`.  
**Time axis labels:** UTC unix data unchanged; LWC `localization` + `tickMarkFormatter` format in browser local TZ ([`web/chart-core.js`](../web/chart-core.js)).  
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

**NEXT:** see `docs/OPEN_DEBTS.md` — primary: **#76 ScoreNodes**, **#68 Osc scale**, **#69D**.
