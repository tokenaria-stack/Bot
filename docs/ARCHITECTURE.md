# Architecture (Current System)

**SSOT for:** how the system works **today**.  
**Not SSOT for:** engineering laws (→ `jeweler-protocol.mdc`), history (→ `HISTORY.md`), why-decisions (→ `DECISIONS.md`), backlog (→ `OPEN_DEBTS.md`).

**Version:** Core 5.0 Data Plane (Phases A–G ✅) + Core 6.0/6.1 Docs + **Debt #69A FE Memory Budget**.

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
| `Tip Ownership` | History REST = closed only; forming tip = WS only |
| `windowMode` | FE display window: `live` \| `history` (Debt #69A) |
| `STORE_BUDGET_*` | ColumnarStore TARGET 12000 / HARD_CAP 16000 bars |

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
5. **RAM ≠ SQLite.** Frame/Runtime = realtime; SQLite = archive ledger. Healthy RAM ≠ healthy DB tip.
6. **Frontend ≠ history DB.** `ColumnarStore` is a bounded display window (Debt #69A). Server owns durable history. Viewport never mutates OHLC/plots.

---

## Frontend display window (Debt #69A)

**Ownership:** Server owns history; browser owns only an active viewport window.

| Piece | Behavior |
|-------|----------|
| Budget | `STORE_BUDGET_TARGET=12000`, `STORE_BUDGET_HARD_CAP=16000` ([`web/config.js`](../web/config.js)) |
| Atomic prune | `_pruneToCount` slices times + candles.* + all plots + annotations together |
| `appendTick` | `_enforceBudget(FROM_OLDEST)` |
| `prependMonolith` | `_enforceBudget(FROM_NEWEST)` → may set `windowMode='history'` |
| `windowMode` | `live` — WS may append; `history` — WS/gap must not feed store or auto-`loadDashboard` |
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

**Tip Ownership:** REST history = closed bars only (`dropFormingTip`); forming tip = WS only.  
**Discard axis:** `window.projectionEpoch`.  
**Wozduh:** DAG bus only; Falcon Evaluate gated; legend = chrome only (no per-tick HTML metrics).  
**Floating menus:** `position:fixed` viewport (`floating-menu.js`).

**UI paint laws:**

1. Only `ChartAdapter` talks to Lightweight Charts.
2. Paint reads Store through a window (`extractWindow`), not raw full store.
3. `RenderScheduler` is the only paint initiator.
4. Cold boot camera uses width-independent APIs only (`applyOptions` barSpacing/rightOffset) — no `setVisibleLogicalRange`/`fitContent` on 0×0 containers.

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

**NEXT:** see `docs/OPEN_DEBTS.md` — primary: **#76 ScoreNodes**, **#67 Live Confirm**.
