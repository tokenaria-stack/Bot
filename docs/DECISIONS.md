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

**Consequences:** Phase A1 lands the correct time model with skip still on. Phase A2 may un-skip `1w`/`1M` and enable catch-up/reconcile once consumers use `NextBarOpen` for gaps.

