# Architectural Decisions (ADR-lite)

**SSOT for:** why key choices exist.  
Keep this lean. Add a record only when a decision will be re-argued later.

Format per entry: Context → Decision → Rejected (with Reason) → Consequences.

---

## ADR-001 — Authority instead of Revision / CandleSource FSM

**Context:** Multiple merge copies and volume drift (SQLite vs REST). Need one trust model for competing bar updates.

**Decision:** Ingress `Authority` levels (Estimated / Settled / Final). Higher Authority replaces wholly; equal Authority uses field heuristics (High/Low/Volume MAX, Close = incoming).

**Rejected:**
- Per-bar Revision counters — **Reason:** shotgun surgery; trust belongs in merge policy, not on every Kline field.
- Speculative candle-state FSMs — **Reason:** overengineering without a consumer (Rule 6).
- Per-exchange settlement registries — **Reason:** power plant; no second exchange consumer yet.

**Consequences:** One merge SSOT in `exchange/ingress.go`. Debt #19 closed. New bar producers must declare Authority, not fork merge logic.

---

## ADR-002 — Bar Source Seam

**Context:** Micro-candles and trade-synthesized bars polluted the ledger and chart path.

**Decision:** Only closed canonical `exchange.Kline` values enter the Ingress pipeline. How a producer aggregates (time / ticks / volume) is private. Forming ticks bypass Ingress.

**Rejected:**
- Re-synthesizing full rings on every request (old micro_candles) — **Reason:** expensive, non-canonical, hid holes.
- Silent hole-filling in the ledger — **Reason:** violates time honesty; a gap must stay a gap.

**Consequences:** Future `TickBarBuilder` (#44) plugs into the seam; tick TF menu entries remain sockets until then.

---

## ADR-003 — Boot: WebSocket first

**Context:** REST recovery before WS connect overwrote truth and raced with live ticks.

**Decision:** Boot FSM Connecting → Loading → Reconciling → Live. Buffer WS ticks first; one tick path `Runtime.routeTick` for live and replay.

**Rejected:**
- REST-as-truth during boot — **Reason:** REST lag loses to live WS Final Authority.
- Separate boot tick path in `main` — **Reason:** second pipeline; drift and missed invariants.

**Consequences:** Gap-fill/catch-up loops start only after Live.

---

## ADR-004 — Delete > Stub (Phase F)

**Context:** Legacy ScoreEngine / trade FSM / matrix / risk settings blocked a clean decision layer and confused AI/humans.

**Decision:** Delete dead strategy code. Keep thin sockets (`decision/score_types.go`, `execution/`, `falcon.go`, `vector_db/`). Default `ENGINE_MODE=ChartOnly`.

**Rejected:**
- Rebranding legacy modules in place — **Reason:** keeps lie-names and dead paths; Delete > Deprecate.
- Keeping stub engines "just in case" — **Reason:** false sockets; no consumer, high confusion cost.

**Consequences:** New strategies must be written against contracts in `decision/`, not revived `strategy/` implementations.

---

## ADR-005 — Vocabulary and package split (Phase G)

**Context:** Names `Marker` / `MasterGeneral` / `Layer2` / `Analyst` lied about responsibilities and invited wrong imports.

**Decision:** `Frame` + `Runtime` in `market/`; contracts in `decision/`; `strategy/` = `doc.go` beacon. Import DAG `exchange → market → decision → execution`.

**Rejected:**
- Keeping types in `strategy/` with new names only — **Reason:** package boundary still wrong; museum code invites revival.
- Allowing `decision → market` imports — **Reason:** breaks one-way DAG; contracts must stay pure.

**Consequences:** FE may keep `json:"marker"` label field; Go type `Marker` is banned.

---

## ADR-006 — Snapshot / Restore on streaming engines

**Context:** Intra-bar ticks poisoned IIR / oscillator state and caused history/live cliffs.

**Decision:** O(1) Snapshot/Restore around open-bar evaluation; save only on close. Double-commit guard via `lastCommittedOpenTime` (Core 4.8).

**Rejected:**
- Frontend tip clamping — **Reason:** duct tape; hides engine poison (Rule 1).
- Warmup-depth-only fixes after continuity tests — **Reason:** disproved; wrong root cause class.

**Consequences:** Closed-bar tip identity → ADR-009. ZigZag/geometry may remain repaint-by-design if documented.

---

## ADR-007 — Documentation split (Core 6.0 / 6.1)

**Context:** `MEMORY.md` mixed constitution, architecture, changelog, and debts → attention dilution and token waste.

**Decision:** Always-on = Protocol + Role. On-demand = Architecture / Open Debts / History / Decisions. Controlled English. `MEMORY.md` is an index; README is the landing page. Core 6.1 adds checklist, identity, when-not-refactor — inside existing SSOT files only.

**Rejected:**
- Eight overlapping docs (Principles/Glossary/Checklist as separate volumes) — **Reason:** docs power plant; duplicates Protocol/Architecture.
- Keeping full Protocol duplicated inside MEMORY — **Reason:** two sources of truth.
- Loading History on every task — **Reason:** attention dilution.

**Consequences:** Update the owning SSOT file only; do not copy facts across docs.

---

## ADR-008 — Timeline Publish Gate (not HealingManager)

**Context:** After Binance WS offline, FE Self-Healing reloaded GetWindow before missing closed bars were in `Frame`. Backend gap threshold `> 2×interval` false-negatived a 1-bar hole; `publishable` fired too early. Chart painted (and F5 kept) a hole.

**Decision:** Thin mid-session publish gate on `Runtime` — not an FSM/Manager.
`WS Connected ≠ History Reconciled ≠ Timeline Publishable`. On reconnect: forced REST tip fetch for all chart TFs → contiguous@1bar (`ΔOpen > 1×interval`) → flush pending → broadcast `timeline_publishable`. FE buffers until that signal (P1 status poll deferred).

**Rejected:**
- `HealingManager` / ReconnectFSM / RecoveryCoordinator — **Reason:** sockets, not power plants (Rule 6); BootController already covers cold start.
- FE REST merge of klines — **Reason:** frontend ≠ history SSOT (Rule 1).
- `loadDashboard` on every gap/reconnect hoping archive is full — **Reason:** symptom fix; GetWindow was still gappy.

**Consequences:** Debt #81 closed. Gap-branch of #67 addressed by heal+replay. Closed-bar Boundary SSOT → ADR-009.

---

## ADR-009 — Closed-bar Boundary SSOT

**Context:** #67 tip cliff (History vs Live RSX Δ ≈ 0.8–2.7) survived Warmup / Replay / Snapshot / Live-continuation falsification. Real data-plane probe proved: `GetWindow(Now)` tip ≠ `GetWindow(CapKlineEnd)` tip; Cap-aligned path was bit-identical on OHLCV+RSX. Root cause was two definitions of "last closed bar."

**Decision:** Canonical last-closed open time is `data.CapKlineEndToLastClosed` (`KlineSettleGraceMs`). `GetWindow` resolves every end through `resolveClosedBarBoundary` — same law as Frame boot and REST fetch. Wall-clock `Now()` is not a closed-bar boundary.

**Rejected:**
- FE tip clamp / RSX morph — **Reason:** duct tape (Rule 1); math was already correct on identical OHLC.
- Bumping `FrameBootKlineLimit` / DAGInit depth — **Reason:** WarmupTrap disproved (Δ≈0).
- Separate Cap only for columnar, leave JSON history on Now — **Reason:** second boundary; SSOT violation repeats.

**Consequences:** RSX Tip SSOT is a *consequence* of Closed-bar Boundary SSOT, not a separate engine bug. Viewport forming tip → ADR-010.

---

## ADR-010 — Viewport Forming Tip (TradingView Model 2)

**Context:** Engine math (Replay ≡ Live on same OHLC) and Cap boundary (ADR-009) are proven. F5 tip “hook” remained because History REST painted Cap-closed only while the first WS tick appended the next open (`deltaSec=60`). TradingView’s visible series tip equals `currentOpen` (forming bar) — Tip Ownership Model 2, not RSX smoothing.

**Decision:** Keep **History** Cap-closed only (`dropFormingTip` + `ReplayDAGKlines`). **Viewport projection** may attach Frame’s current forming bar + `BuildTickJSON` live Cur plots after closed Replay (`projectViewportFormingTip`). Only on the live Cap edge; deep-history windows unchanged. First WS tick **overwrites** the same open time.

**Rejected:**
- FE morph / interpolation — **Reason:** duct tape (Rule 1).
- Feeding forming bars into `ReplayDAGKlines` — **Reason:** poisons closed History SSOT.
- ViewportBuilder Manager / new subsystem — **Reason:** power plant (Rule 6); one projection function on the existing columnar path.
- REST “becomes live” — **Reason:** projection combines two canonical sources (closed window + Frame snapshot); History stays pure.

**Consequences:** Tip Ownership = History closed XOR Replay; Viewport = History projection + optional current. Debt #67 product branch closed for F5 continuity; #68 scale bounds next.

---

## ADR-011 — Time Model: Fixed Duration vs Calendar Boundary

**Context:** `CapKlineEndToLastClosed` and REST `alignOpenTimeMs` used `(now/step)*step` for every TF. That matches Binance for fixed intervals (`1m`…`1d`) but is wrong for calendar intervals: Binance `1w` opens Monday 00:00 UTC; `1M` opens on the 1st. Epoch-week floors land on Thursday; `30d` months invent false CloseTimes and gap thresholds. `intervalSkipsKlineGapFill("1w"|"1M")` masked the debt by disabling heal (Phase A2 still pending).

**Decision:** Bar-boundary helpers in `data/` — `CurrentBarOpen`, `PreviousBarOpen`, `NextBarOpen`, `BarCloseTimeMs`. Cap = `PreviousBarOpen(CurrentBarOpen(settledNow))`. REST align floors via `CurrentBarOpen`. Fixed TFs remain step-floor (bit-identical). Calendar TFs use Monday / month-start UTC. No interface polymorphism (Binance-only; three behaviors in one switch).

**Rejected:**
- Removing `intervalSkipsKlineGapFill` in the same change — **Reason:** Phase A2; would mix boundary math with healing behavior.
- `BoundaryPolicy` interface / multi-exchange registry — **Reason:** power plant (Rule 6); three behaviors suffice.
- Reimplementing calendar snap in FE — **Reason:** prefer trusting server opens (A2 FE pass).

**Consequences:** Phase A1 lands the correct time model with skip still on. Phase A2 enables calendar-aware healing: catch-up/gap/reconcile use `NextBarOpen` / `BarStepsBetween`; `intervalSkipsKlineGapFill` removed. FE calendar snap deferred until runtime proves need. Expected: weekly archive can heal to Cap; Buffering loops driven by stale 1w tip should stop once catch-up runs.

---

## ADR-012 — Indicator Configuration Rule (RSX B0)

**Context:** Live RSX settings were owned by a poisoned FE path (localStorage → URL query → history reload → `pushRsxSettingsToServer: noop`). Engine Frames kept default `close` while TradingView uses HLC3. Patches would preserve dual ownership.

**Decision:** Indicator parameters are **engine state**. Browser is a control surface only.

- SSOT: `market.GetRSXSettings` / `ApplyRSXSettings` (compare-before-mutate, generation bump).
- Autosave: `rsx_settings.json` on change; `LoadRSXSettingsFromDisk` at process start.
- Default RSX `source = hlc3` (TV parity).
- Menu: collect form → `POST /api/settings/indicators` → apply + Frame replay → viewport rebuild. GET hydrates menu; localStorage is cache only (never authoritative).
- Long-term (docs only until Wozduh+#2): Registry → Config → DAG membership → Runtime → Projection. No IndicatorManager in B0.

**Rejected:**
- Repairing noop push / local-wins merge — **Reason:** dual SSOT (Rule 1 / 5).
- Building IndicatorRegistry/graph for one indicator — **Reason:** power plant (Rule 6); extract after Wozduh.

**Consequences:** Phase B0 enables TV source parity experiments. B1 = ChangeImpact + Viewport Contract (ADR-013/014). Remaining tip vs TV, if any = forming-bar Model 2 only.

---

## ADR-013 — Indicator Change Impact

**Context:** Live RSX settings apply called `SetRSXLength` / `SetRSXSignalLength` before deciding whether to replay. Those setters clear Jurik buffers. DivMethod-only changes therefore wiped runtime and skipped replay → Cur tip mutated permanently (TV↔Fractal↔TV did not restore).

**Decision:** Configuration changes are classified by **ChangeImpact** before any runtime mutation. Engine owns classification; browser never decides replay.

```
ProjectionOnly < AnnotationOnly < IndicatorReplay < GraphReplay
```

| Impact | Meaning | Examples (RSX B1) |
|--------|---------|-------------------|
| ProjectionOnly | Visual only | colors, visibility (FE) |
| AnnotationOnly | Derived overlays; Jurik untouched | DivMethod, pivot, lookback, deltas |
| IndicatorReplay | Walk-forward math; Set* then Replay in one transaction | Length, SignalLength, Source |
| GraphReplay | Reserved enum only | enable/disable / DAG membership (future) |

**Hard invariant:** Runtime may never be mutated unless the corresponding rebuild path executes in the same transaction.

```
// forbidden
SetLength(); if replay { Replay() }
// required
if IndicatorReplay { SetLength(); Replay() }
```

B1: only RSX implements `RSXImpactOfChange`. Wozduh/ATR later implement the same idea — no registry/platform in B1.

**Rejected:** Always-replay on any settings change — **Reason:** wasteful; AnnotationOnly must not touch Falcon. Building IndicatorRegistry — **Reason:** power plant (Rule 6).

**Consequences:** DivMethod flips preserve bit-identical RSX tip. Length/Source still cold-replay. Annotation rebuild may remain a stub (Phase F); that is acceptable — do not fake Jurik replay.

---

## ADR-014 — Viewport Contract

**Context:** B0 settings sync used `replaceMonolith` + `viewport: 'fresh'`, treating indicator apply as navigation and jumping the camera to the right edge.

**Decision:** Indicator settings are **projection events**, never navigation events.

Indicator apply may rebuild engine state, annotations, and plots. It **must never** change:

- visible range, zoom, scroll, timeframe, crosshair

Only pan, zoom, and timeframe selection may move the camera.

FE apply path: debounce 200ms → POST → soft `updatePlots` + `mode: 'indicators'`. Save / outside-click flush pending or close if already synced. AbortController + generation ignore stale responses.

**Rejected:** Full remount / `viewport: 'fresh'` on settings — **Reason:** wrong layer (navigation vs projection).

**Consequences:** Camera stays put across RSX edits. Soft plot reload merges with live tip via store `lastTimeSec`.

---

## ADR-015 — Projection Continuity (investigation)

**Context:** After B1, TipSSOT shows `DATA_PLANE_MATCH` while the chart tip still jumps on the first WS tick after soft settings apply. Architecture already has a single projector (`projectViewportFormingTip`, ADR-010). Suspected consumer seam: `updatePlots` truncates server-appended forming tip to store length.

**Decision (contract — prove before code fix):**

There is exactly one owner of the projected forming bar. History API and the realtime stream must expose the same forming state. A consumer may refresh/repaint without visible discontinuity if market data has not changed.

Corollaries:

- `projectViewportFormingTip` is the sole projector.
- Frontend applies projection; it never synthesizes Cur.
- Soft consumers must preserve the projection returned by the server.
- The first WS update after a history refresh must be idempotent if market state is unchanged.

**Probe (opt-in / dormant):** `[TipSSOT]` and `[ProjCont]` runtime spam default OFF.
Enable with `DEBUG_TIP_SSOT=1` / `DEBUG_PROJ_CONT=1` (server) or `localStorage DEBUG_PROJ_CONT=1` / `?debug_proj_cont=1` (FE).
Helpers retained for future indicator tip ownership investigations.

**B2.1 fix:** Soft settings path uses `ColumnarStore.applyProjection(snapshot)` (columnar response as ProjectionSnapshot). Atomic times+OHLC+plots. Never `updatePlots` for projected tips. Camera via capture/restore (ADR-014). Regression: `web/projection_apply_test.js`.

**B2.2 fix:** `projectViewportFormingTip` modes `none|append|overwrite`. Same-open Cap tip ← Frame OHLC + `BuildTickJSON` (no append). ADR-015 FE probe skips on new bar / timeline heal / long elapsed.

**Rejected:** FE “stick Cur” / duplicate Close — **Reason:** second ownership (Rule 5 / ADR-010). Smarter `updatePlots` forever — **Reason:** partial consume remains the root anti-pattern.

**Consequences:** Soft apply preserves N+1; same-open settings apply paints Frame Cur so first WS is idempotent when market unchanged. Debt **#86** closed.

---

## ADR-016 — Replay Lifecycle Ownership

**Context:** After ADR-015 (projection continuity), TipSSOT/`ProjCont` showed faithful projection (`lastRSX == frameCurRSX`) while the chart tip still jumped on the first WS tick after RSX settings `IndicatorReplay`. Audit proved: `replayStreamingLocked` evaluated the live forming tip with `isClosed=true` and `markTailCommittedLocked` pinned that open — then WS `isClosed=false` re-applied Jurik from a Save that already included the tip (double IIR pass). Cold boot was fine because closed history and forming were separated; settings replay collapsed them into one closed loop.

**Decision:** Frame runtime replay must reproduce the live candle lifecycle:

1. Split `a.klines` by Cap forming predicate (`data.IsFormingCloseTime` — same as `dropFormingTip` / `isFormingKline`).
2. Replay closed bars only with `isClosed=true`.
3. `markTailCommittedLocked(closed)` — commit last **closed** bar only.
4. If a forming tip exists: evaluate once with `isClosed=false`; **never** commit it.

History/Cap Replay remains closed-only (`dropFormingTip` + `ReplayDAGKlines`). This ADR owns **Frame runtime** replay only (`replayStreamingLocked` / `warmupStreaming` via shared `replayLifecycleLocked`).

**Invariant:** A forming candle must never become `lastCommittedOpenTime` during replay. Same open + same OHLC + `isClosed=false` after settings replay must be idempotent.

**Rejected:** Append `evaluate(forming,false)` after an all-closed loop — **Reason:** triple lifecycle / double Jurik. Patch `markTailCommitted` with last-bar exceptions — **Reason:** hides wrong caller contract. FE/projector heal of committed tip — **Reason:** wrong layer (ADR-015 already honest).

**Consequences:** Soft settings apply publishes live-forming Cur; first WS tick no longer cliffs. Debt **#87** closed. Regression: `market/replay_lifecycle_test.go`.

