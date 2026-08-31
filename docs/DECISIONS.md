# Architectural Decisions (ADR-lite)

**SSOT for:** why key choices exist.  
Keep this lean. Add a record only when a decision will be re-argued later.

Format per entry: Context → Decision → Rejected (with Reason) → Consequences.

---

## WOZDUH-ACTIVE-1B — per-Frame live Wozduh demand (Aug 2026)

**Context:** 1A masked history replay. Persistent Frames still ran `WozduhMaskAll` on every tick, including unused seconds TFs.

**Decision:** Per-Frame required mask = WS `plotIDs` union (nil/empty = all) OR proven internal mask. Sleep/wake under Frame lock after releasing `clientsMu`. Wake via temp node + explicit field install. No DAG-DEMAND framework.

**Rejected:**
- Global Wozduh mask — **Reason:** demand is per Frame/TF.
- Make `/api/state` or VolCross demand — **Reason:** not proven production consumers of persistent live Wozduh.
- Rewrite `chart_cache` `finiteOrZero` (NaN→0) as part of 1B — **Reason:** that helper is on `ReplayDAGKlines` / `ReplayClosedBars` (compute-all), not the sleeping live Frame. Live `/api/state` Plots already omit NaN. Live FE hydrates Wozduh from columnar + WS. Legacy JSON oscillators are observational / backtest-adjacent, not current chart truth.
- Async wake / forming catch-up — **Reason:** correctness over latency; existing Restore+current-tick law is enough.
- Generic atom/reflection installer — **Reason:** Wozduh fields are concrete.

**Consequences:** Frozen at `1b724ef`. ChartOnly unused Frames can be mask 0 (0 stream Updates). Live unused Frames keep 4 streams/update (VolBase×2 + Wt11 + Wt22). Always-on IIR older than `dagHistoryCap` is compared with `dagShadowEpsilon`, not bit-exact. Stop Wozduh chapters here. Next when asked: ScoreNodes.

## WOZDUH-ACTIVE-1A — masked history replay, compute-all default (Aug 2026)

**Context:** WIRE-1 already filtered packing. `/api/history` still replayed every Wozduh stream over the chunk.

**Decision:** Static plot→compute bitmask. `ReplayClosedBars` stays all bits. History opts into `ReplayClosedBarsMasked`. Empty `slots` still compute-all. Inactive output slots are NaN. Live Frame unchanged.

**Rejected:**
- Change default replay callers — **Reason:** Navigator, golden, shadow, ZZ tests stay compute-all.
- Live demand/union in this chapter — **Reason:** WOZDUH-ACTIVE-1B.
- Atom interfaces — **Reason:** bit-guard existing Update().

**Consequences:** Frozen at `2cd4ca4`. Zero mask means no Wozduh work, not all. Empty history `slots` stay compute-all. Do not reopen replay masking.

---

## WOZDUH-WIRE-1 — pack only subscribed Wozduh scalars (Aug 2026)

**Context:** Paint already skipped hidden Wozduh series. Projector still packed every Wozduh scalar on history and live ticks.

**Decision:** Derive subscription from existing visibility + mandatory `woz_slow`. Filter at the wire boundary per WS client `slots`. Enable hydrates the current store window before reveal. Do not change Wozduh math.

**Rejected:**
- Second Subscribe UI — **Reason:** visibility is the chart subscription.
- Global Frame/DAG subscription — **Reason:** one client must not change another client's wire or numerical truth.
- Compute sleeping in this chapter — **Reason:** WOZDUH-ACTIVE-1.
- Filter `/api/state` Plots in the same cut — **Reason:** live path is WS `slots`; HTTP snapshot stays unfiltered (legacy).

**Consequences:** Frozen at `0c2ecce`. VISIBLE → SUBSCRIBED → REQUIRED. REQUIRED still means all current Wozduh atoms on the live Frame. Do not reopen wire/pack.

---

## RSX-VISIBILITY-1 — facts exist independently of paint (Aug 2026)

**Context:** `div_method` (tv|fractal) and `show_pivots` encoded one-detector-at-a-time. TV fact production was gated on `div_method == tv`. Five factual families already existed.

**Decision:** Delete the selector and global pivot switch. Five frontend visibility booleans filter paint by `source`. TV facts always publish. Compact `visibilityMask` in the annotation paint key. No preference migration. No backend hide flags.

**Rejected:**
- Keep ignored `div_method` / compatibility alias — **Reason:** leftover selector ownership.
- Infer family from color/label/pattern — **Reason:** `source` is identity.
- Compute sleeping / wire skip / ScoreNodes in this chapter — **Reason:** presentation only.

**Consequences:** Frozen at `749912f`. TRUTH → FACTS → FE visibility filter → paint. Next when asked: WOZDUH-ACTIVE-1 (compute). Do not reopen this chapter.

---

## RSX-SIGNAL-3 — fractal classic classes, not hidden (Aug 2026)

**Context:** Fractal RSX used chart letters L/LL/S/SS/P. LL/SS mixed Class A and Class C. Product kept the fractal detector.

**Decision:** Publish `rsx_fractal_div` with `Pattern` `class_a`/`class_b`/`class_c` and `rsx_fractal_pivot` high/low. Carry `CheckClassicDivergence` class on the hit. Facts ignore `div_method`. Paint Class B as arrow-only; A/C keep class captions.

**Rejected:**
- Map LL/SS to `hidden` — **Reason:** this family is classic A/B/C, not ZigZag hidden.
- Reconstruct class from letters later — **Reason:** letters already collapsed A vs C.
- Gate facts on `div_method == fractal` — **Reason:** same law as `rsx_zz_div`.
- Rewrite as ZigZag/TV/generic divergence — **Reason:** product kept this detector.

**Consequences:** Frozen at `c856fef` (bounded confirm-bar `FractalFactsAt`). Menu / `div_method` deletion is RSX-VISIBILITY-1.

---

## FALCON-SCORE-CLEAN-1 — delete write-only divEngine scoring (Aug 2026)

**Context:** Live ran `AnalyzeWithRSX` into `Frame.divSignal` with the annotation discarded. ChartOnly never ran it. No decision/UI/fact consumer.

**Decision:** Delete `SmartDivergenceEngine` and `divSignal`. Keep `FalconEngine` numbers. Keep Frame ZigZag for fib/geometry. Delete Frame `orangeRsi` (scoring-only). Surgical delete in `divergence_rsx.go`.

**Rejected:**
- Delete `falcon.go` — **Reason:** HTF/backtest/Live numbers.
- Delete Frame ZigZag — **Reason:** still feeds geometry/fib.
- Park saucer/V-spike helpers — **Reason:** no consumer.

**Consequences:** Next kill-check is remaining Live strategy furniture (fib/geometry/DataBus), not ScoreNodes.

---

## SLOT-CLEAN-1 — compact four dead DAG slots (Aug 2026)

**Context:** After LEGACY-SCORE-CLEAN-1, `SlotDivScore` / `SlotDivState` / `SlotMicroDivScore` / `SlotTotalScore` were unused iota holes. HistoryBus still allocated four empty stripes.

**Decision:** Delete the four constants. Let later slots renumber. `Slot` is not on the wire (`json:"-"`). No aliases.

**Rejected:**
- Reserved `_` holes — **Reason:** fake architecture.
- Poison-slot LongScore test after the constant is gone — **Reason:** deleted concept.

**Consequences:** `SlotCount` drops by 4. Falcon scoring was deleted in FALCON-SCORE-CLEAN-1. Fractal facts later.

---

## LEGACY-SCORE-CLEAN-1 — delete DAG Micro/ScoreNode, do not park math (Aug 2026)

**Context:** After 2B, the only remaining DAG score chain was MicroPatternNode (Jurik saucer/V-spike → 15/20/35) → ScoreNode ×0.5 → unused `SlotTotalScore`. No operational consumer. A separate Live Falcon/`divEngine` micro path still exists.

**Decision:** Delete the DAG nodes and tests that locked scores. Leave slot iota holes. Do not publish `rsx_micro` facts. Do not park unused detector helpers on the DAG.

**Rejected:**
- Keep detectSaucer/V-spike as unregistered DAG helpers — **Reason:** no consumer.
- Deleting Falcon `AnalyzeMicro*` in the same patch — **Reason:** different oscillators, mixed with AnalyzeMacro.

**Consequences:** Next dedicated chapter was slot compaction (SLOT-CLEAN-1). Falcon/divEngine is a later audit.

---

## RSX-SIGNAL-2B — delete DivergenceNode, do not compact slots (Aug 2026)

**Context:** After 2A/2A.1, ZigZag meaning lived twice: truthful `rsx_zz_div` facts and a dead `DivergenceNode` → `SlotDivState`/`SlotDivScore` → unused `SlotTotalScore` path. Old L/S/LL/SS was unpublished. Production already forced `LongScore = 0`.

**Decision:** Delete the node and DivState presentation helpers. Drop `SlotDivScore` from ScoreNode weights. Keep ScoreNode for Micro. Leave middle slot iota holes. Retarget `ann_rsx_div.Slot` to `SlotJurikRSX` for pane ownership only. Projector/history skip non-scalar components, so Jurik values cannot be read as DivState.

**Rejected:**
- Adapter/no-op DivergenceNode wrapping facts — **Reason:** parallel meaning.
- Mapping L/S/LL/SS onto facts — **Reason:** 2A already owns truthful Pattern.
- Compacting slot iota in this chapter — **Reason:** renumbers later slots.
- Wiring `SlotTotalScore` into `LongScore` to keep an old test green — **Reason:** production does not do that.

**Consequences:** Frozen (`3f20255`). Micro → ScoreNode → unused TotalScore was deleted in LEGACY-SCORE-CLEAN-1.

---

## RSX-SIGNAL-2A.1 — collector + revision gate, not fingerprint (Aug 2026)

**Context:** 2A semantics were green. History ran a second DAG for ZZ facts. Live `_flushDelta` sliced annotations and called `setMarkers` every tick. Session `BarIndex` + `ValueAtBar` failed after `dagHistoryCap` wrap.

**Decision:** Tiny closed-bar `ZZDivFactCollector` (not DivergenceNode). Fuse ZZ collection into `ReplayClosedBars`. Annotation paint key is store revision + `show_pivots` + series identity. Do not fingerprint the marker list.

**Rejected:**
- Hashing times+labels every tick — **Reason:** O(n) work on idle LIVE.
- Sampling RSX inside ZigZagNode or retrofitting DivergenceNode — **Reason:** mixes 2A.1 with 2B.

**Consequences:** Frozen (`39d6f78`) with 2A. 2B deleted the unused DivState/score coupling.

---

## RSX-SIGNAL-2A — ZigZag facts, not SlotDivState (Aug 2026)

**Context:** TV facts are frozen. ZigZag `DivergenceNode` compared last two same-type swings vs RSX at the swing bar, but published a level (`SlotDivState`) plus score weights. `LL` was hidden bullish; `SS` was unused.

**Decision:** New `IndicatorFactEvent` family `rsx_zz_div` + `Pattern`. Emit once per new confirmed swing. Hidden bearish is a new truthful geometry, not `SS`. Do not put OpenTime on generic `SwingEvent`; resolve kline OpenTime at emit.

**Rejected:**
- Mapping LL/SS to Grade=strong — **Reason:** hidden ≠ strong; SS was dead.
- Reading `SlotDivScore` to classify — **Reason:** meaning, unproven weights.
- Changing ZigZag RSX sensitivity in this chapter — **Reason:** geometry ownership.

**Consequences:** Frozen (`39d6f78`). 2B deleted unused DivState/score coupling. Menu later.

---

## RSX-SIGNAL-1.1 — TV presentation + Pine TV pivots (Aug 2026)

**Context:** SIGNAL-1 published TV divergence with Bull/Bear captions and no Pine pivots. Users wanted arrows-only plus the real TV pivot plots, not fractal P.

**Decision:** Keep `rsx_tv_div` facts. Projector uses empty labels and saturated green/red. Add `rsx_tv_pivot` with Direction `high`/`low`, ConfirmedAt at the knowable bar, AnchorAt = OpenTime two bars back. Show Pivots filters paint only.

**Rejected:**
- Class A/C as “strong” TV divergence — **Reason:** not in Everget TV law.
- Fractal `P` as TV pivots — **Reason:** different family.
- Putting color or “Pivot” text on the event — **Reason:** presentation.

**Consequences:** Frozen (`b4ac2ae`). TV family closed. Do not start ZigZag (`rsx_zz_div`) from this chapter.

---

## RSX-SIGNAL-1 — TV divergence is a fact, not a trade (Aug 2026)

**Context:** Everget Pine alerts on `divbull` / `divbear` with one-bar confirm and `offset=-1`. Chart markers must not wait on ScoreFactor. Decision must not act at visual anchor time.

**Decision:** One event shape `IndicatorFactEvent`. v1 source `rsx_tv_div` only. Times are closed-bar OpenTime ms. Projector owns color/shape. Reuse existing TV rolling math; do not publish ZigZag `DivState` or fractal hits on this path.

**Rejected:**
- BUY/SELL / L/LL/S/SS as the published fact — **Reason:** meaning belongs to a later ScoreNode.
- Merging TV / fractal / ZigZag under one detector — **Reason:** different fact families.
- Using RAM indexes as event identity — **Reason:** not stable across trim/replay.

**Consequences:** Frozen this chapter. RSX-SIGNAL-2 may reuse the struct for `rsx_zz_div` without pretending it is TV.

---

## RSX-TRUTH-CLEAN-1 — RSX truth vs presentation vs meaning (Aug 2026)

**Context:** Slope-vs-50 hex/`rsxColor` lived in Go and on replay/tick/HTF payloads. A future ScoreNode could treat that visual heuristic as a trading factor. Presentation already belongs on the FE (`rsxStrokeColor`, 30/50/70 scale).

**Decision:** Backend RSX exposes numerical values and factual states only. Frontend owns paint. Future `decision` / ScoreNodes derive meaning explicitly from truth. Backtest later consumes the same truth + meaning — never a second RSX implementation.

Trusted ScoreNode inputs: RSX value, RSX signal value, HTF RSX numeric values, divergence facts, thresholds.

**Rejected:**
- `RSXMomentumState` / Pine OB/OS color in Go / semantic aliases — **Reason:** replaces one presentation SSOT with another.
- Keeping hollow chart-factory wrappers (`BuildRSXChart`, empty marker sockets) — **Reason:** archaeological code is still a fake SSOT.
- Purging generic `Marker` / annotation `Color` used by real divergence events — **Reason:** those are facts, not the old L/LL/S/SS factory.

**Consequences:** Frozen (`5f8a290`). For every candidate ScoreNode factor: is this a real fact from indicator truth, or old presentation/state debt? Do not reopen RSX truth to start ScoreNodes.

---

## SPARSE-ADR010-TIP-1 — sparse HTTP tip is append-only (Aug 2026)

**Context:** Native ADR-010 uses Cap + OVERWRITE. Sparse HTTP rebuild always Replays closed rows first, then overlaid Frame Cur via same-open OVERWRITE when `OpenTime` matched the last Replay bar. That rewrote finalized RSX/Jurik with live DAG Cur. Calendar `isFormingKline` is wrong for quiet 5s–45s.

**Decision:** Same truth model as ADR-010, not the native implementation. `projectSparseSecondFormingTip` is APPEND or NONE. Compare OpenTime ms (`replayClosedOpenMs`, `LastCommittedOpenTime`, forming last kline). Empty Replay + forming child still packs. WS still updates the forming row after seed.

**Rejected:**
- Copy Cap / `now <= CloseTime` onto 5s–45s — **Reason:** silence is legal.
- Native-style OVERWRITE on the sparse projector — **Reason:** after Replay every existing row is closed truth.
- FE RSX clamp / shared native-sparse helper / origin metadata — **Reason:** Rule 1 / Rule 6; isolation.

**Consequences:** Frozen (`a452cb5`). Do not touch `projectViewportFormingTip`, reducer close law, or RSX math from this chapter.

---

## SPARSE-LIVE-INGEST-1 — island identity owns 5s–45s WS ingest (Aug 2026)

**Context:** LIVE 5s–45s hydrate arrived with `windowMode=live`, TimeCamera LIVE, and `historyHasNewer=true` (1s source after last closed child while the current child is forming). `pushLiveTickDelta` treated `hasNewer` as a detach veto. WS ticks arrived; the store tip did not move until history interaction.

**Decision:** Consumer-only. 1s keeps the `historyHasNewer` ingest veto. 5s–45s (`isSparseSecondChart`) ingest iff `windowMode === 'live'`. `historyHasNewer` remains paging/source. TimeCamera remains paint-only. Producers and ISLAND-SLIDE `droppedNewest` → `windowMode=history` unchanged.

**Rejected:**
- Wall-clock / `2 * interval` ingest guard — **Reason:** sparse silence is legal; a quiet LIVE tail must accept the next trade.
- New `detachedHistoryIsland` flag — **Reason:** `windowMode` already is island identity.
- Auto right-page / re-arm / fake tick / camera nudge — **Reason:** crutches around the wrong consumer.
- Using TimeCamera HISTORY to veto ingest — **Reason:** MICRO-2C: pan-left on a LIVE island must still ingest off-screen.

**Consequences:** Frozen (`1b67400`). Do not touch `_maybePromoteLiveWindow` unless a later rejoin-after-right-fill regression is proven.

---

## HISTORY-IDLE-PUMP-1 — viewport history demand is human-owned (Aug 2026)

**Context:** Parked near an edge, `onAfterFlush` → `scheduleHistoryLoad()` re-armed from the current runway while scroll-arm stayed sticky. Infinite 3000-bar pages, WS/HTTP spam, rAF violations, last-price blink from island slide. Not a last-price or cursor bug.

**Decision:** Human movement owns one latest coalesced viewport intent (`cause: 'userNav'`). Paint/restore/range echo consume only. Consume scroll-arm when a viewport request starts so LWC echo cannot re-page. Drag re-arms on primary-button `pointermove` (hover does not). Consume pending with one `pickHistoryPrefetchEdge`. `sourceContinue` is not gated.

**Rejected:**
- UI clamps / skip paints to hide idle heat — **Reason:** Rule 1; owning layer is demand.
- One chunk per flick (drop Wave 2 pending) — **Reason:** starves fast travel.
- Dual LEFT then RIGHT pending fall-through — **Reason:** stale opposite aftershock.
- Treating `[Violation]` / last-price blink as the next chapter — **Reason:** idle pump was the cause; smoke closed it.

**Consequences:** Chapter frozen (`636ff55`). Do not “fix” cursor overlap or the last-price line unless a regression remains after idle traffic is gone.

---

## Chart freeze — stop camera/cap/RAF experiments (`CHART_FROZEN`)

**Context:** HISTORY/LEFT/RIGHT and Fix C–G are stable. Store 9k / visible 5k is an acceptable live experiment. Remaining tick `[Violation]` logs are LWC paint cost.

**Decision:** Freeze chart/camera/hydration. Keep 9k/5k/3k/25%. Live ingest full-rate. Patch 2 stays rolled back.

**Rejected:**
- Further cap / chunk / prefetch knobs — **Reason:** 15k/12k already A/B’d; 9k feels good; more knobs are not a freeze.
- Live-delta 250 ms throttle (Patch 2) — **Reason:** desyncs indicator paint from the tape; does not silence Chrome violations.
- Eager off-screen chunk LRU — **Reason:** fights prefetch, holes, tip-drop.
- Hiding `[Violation]` or WAL pragmas as “fixes” — **Reason:** hides evidence.

**Consequences:** Next work is prove-dead cleanup, then SQLite/WAL reader audit, then TF-switch same-X. Indicator LOD and ScoreNodes later. S6 not reopened here.

---

## HIST-0/1/2 — historical window acquire + typed empty (Aug 2026)

**Context:** Microscope `/api/history` 1m at 2023-01-03 returned HTTP 503 because SQLite 1m starts Nov 2023. Empty archive is not an outage. REST must not live in `LoadKlines`. Spot must not pad a post-genesis count.

**Decision:** `GetWindow` stays read-only. `/api/history` orchestrates read → `EnsureHistoryWindow` (futures era-local `FetchClosedRangePagesExact` + `PersistClosedBarsNow`) → reread. Empty → HTTP 200 `status=no_data`. Exchange fail → 502. SQLite fail → 500. Live `/api/state` and catch-up unchanged. Pre-futures REST is DATA-1 (no spot kline client yet).

**Rejected:**
- REST inside `data.LoadKlines` — **Reason:** storage must not become a network client.
- Mapping all empty results to 503 — **Reason:** looks like warming/outage.
- Full 1m `history_sync` before microscope works — **Reason:** years of bars for a ~3300-bar window.
- Changing HISTORY 150-bar zoom in this chapter — **Reason:** data first.

**Consequences:** HIST-3 smoke matrix and DATA-1 (15m hole, 1m archive min) are separate.

---

## SQLITE-2 — no autostart MCP on `history.db`

**Context:** SQLITE-1: `wal_checkpoint(TRUNCATE)` `busy` is concurrent GetWindow **or** a second process on the same file. Project MCP `sqlite-history` opened `history.db` for the whole Cursor session.

**Decision:** Default `.cursor/mcp.json` has no servers. Enable MCP only when the user asks or when SQLITE diagnosis requires it. Recipe in `.cursor/mcp.json.example`.

**Rejected:**
- Keep sqlite MCP always on for agent convenience — **Reason:** pins WAL; hides whether the bot itself leaks readers.
- WAL pragma / timeout “fixes” while MCP still autostarts — **Reason:** treats a second-process pin as an engine bug.

**Consequences:** Remaining WAL busy logs after MCP-off are in-process history GETs. Do not hide the log.

---

## SQLITE-2b — one SQLite connection (idle pool pins TRUNCATE)

**Context:** After SQLITE-2 (MCP off), WAL busy still logged every 5 minutes with `frames=checkpointed`. Go `sql.DB` default idle=2 plus `MaxOpenConns=CPU` left extra sqlite handles open after parallel boot.

**Decision:** `SetMaxOpenConns(1)` and `SetMaxIdleConns(1)` on `history.db`.

**Rejected:**
- WAL pragma / timeout changes — **Reason:** did not address the extra connections.
- Dropping the busy log — **Reason:** still valid for in-flight GetWindow or a second process.

**Consequences:** Frame boot SELECTs serialize. Occasional busy during `/api/history` is expected; constant 5-minute busy is not.

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

---

## ADR-017 — Timeline Publishability Contract

**Context:** After offline Binance reconnect, charts showed a one-bar hole (e.g. `14:03` then `14:05`). Probes proved: Cap+settle-grace REST ends at `14:03`; pending holds only `14:05`; `14:04` never existed in pending; ungated `applyTick` flush jumped the tip; `timeline_publishable` fired without post-flush continuity. FE painted the server `times[]` faithfully.

**Decision:** A timeline may become publishable only after **all** heal mutations produce one contiguous Frame series:

1. Cap REST (existing) → Cap-contiguous closed history.
2. **Heal closed-gap fill (Exact):** if pending tip open skips ≥1 closed open after Frame tip, `FetchClosedRangePagesExact` for `[NextBarOpen(tip), PreviousBarOpen(pendingTip)]` — **without** `CapKlineEndToLastClosed(now)`. Proof of settlement: WS already has the later tip. Merge via `LoadHistoricalKlines` (no synthesis).
3. Refuse flush while a pending tip jump remains.
4. Flush pending with `applyTick` (not `ingestTipGap` — avoids reconcile recursion).
5. Verify `framesSeriesContiguous` + pending empty → then `timeline_publishable`.

**Rejected:** Replace flush `applyTick` with `ingestTipGap` — **Reason:** re-enters Reconcile with same Cap, loops/hangs. FE gap glue / interpolated candles — **Reason:** wrong ownership. Cap-only fill for the missing bar — **Reason:** settle grace still excludes bars the live tip has already proven closed.

**Consequences:** Reconnect restores missing closed bars before live ticks attach. Debt **#88**. Regression: `market/timeline_heal_b3_test.go`. Buffering UX (double `timeline_healing`, 75s safety) remains a separate debt.

---

## ADR-018 — Timeline Recovery State (Frontend)

**Context:** After ADR-017 made reconnect data contiguous, UX still felt stuck: every `timeline_healing` re-entered `beginAwaitTimelineHeal`, reset a 75s timer, and forced a full-screen Buffering overlay. Transport, timeline publishability, and UI buffering were mixed in `boot.js`.

**Decision:** Frontend owns recovery presentation via a tiny state machine:

- States: `LIVE` | `HEALING` only.
- API: `enter(reason)`, `publishable()`, `isHealing()`.
- Hooks: `onEnter` (e.g. start tick buffer), `onRecovered` (e.g. `loadDashboard`) — recovery does not know “dashboard.”
- `enter` is idempotent: duplicates ignored; watchdog never reset.
- Watchdog starts once on first enter (default 25s); diagnostic only (stalled badge + Retry).
- UI: non-blocking `#timeline-sync-badge`; chart stays painted. Not `#orderflow-buffer`.
- Server stays dumb (`timeline_healing` / `timeline_publishable` only). No server recovery FSM.

**Ownership boundaries:**

| Owner | Owns |
|-------|------|
| WS | transport |
| TimelineRecovery | recovery lifecycle + sync badge + watchdog |
| Dashboard (`boot.js`) | viewport reload via `onRecovered` |
| Toolbar / hydrate | `#orderflow-buffer` for `__isDashboardLoading` only |

**Rejected:** Multi-state reconnect ladders; server UX FSM; restarting timers on duplicate healing; heal using full-screen Buffering overlay.

**Consequences:** Duplicate healing no longer extends wait; publishable exits immediately. Debt **#89**. Module: `web/timeline-recovery.js`. Regression: `web/timeline_recovery_test.js`.

MICRO-2B: TimelineRecovery is **dense/native** recovery only. Sparse charts ignore Master heal/publishable (no enter, no `loadDashboard`). Browser reconnect uses Shot 10B + preserved VIEW. Do not teach TimelineRecovery about micro TFs.

---

## ADR-019 — PaneLayout (Footer Pane Membership)

**Context:** TradingView-style Ind footers need a single owner for which oscillator panes exist, their order, pixel heights, and fullscreen — before CSS Grid / drag / render-pause. Hardcoded Ind checkboxes and flex+static splitters would brick on HostID rename and explode with N footers.

**Decision (Phase 1 foundation):**

- FE `PaneLayout` is SSOT: `visible`, `order`, `footerHeights` (px only), `fullscreenPaneId`.
- Price is always present and is **not** a HostID; never store a price height (elastic `1fr` reserved for Phase 2 Grid).
- Init = Manifest HostIDs ∩ versioned `localStorage` (`dashboard_pane_layout`). Drop unknown hosts; append new; clear invalid fullscreen; version mismatch → defaults.
- Ind menu is dumb UI generated from catalog (no hardcoded RSX/Wozduh). Optional `renderOptions.paneTitle`; else short-id UPPER / Title case.
- Persist after every mutation. `subscribe` for Ind checkbox sync.

**Deferred (same ADR, later phases):** `ChartAdapter.setHostActive` + Store-snapshot resume. HostID→wrap map may move into Manifest when N footers grow (STACKS is fine today).

**Phase 2 (Grid apply):** `LayoutController` builds `grid-template-rows` (`minmax(120px,1fr)` + `4px` gutters + footer px). Dynamic splitters only between visible panes. Ind / legend eye → `PaneLayout.setVisible` / `toggle` → layout apply + LWC resize. Unknown HostIDs without a wrap are skipped (no brick).

**Phase 3 (height drag):** Splitter above footer `hostId` adjusts that footer's px only (drag down → shorter, drag up → taller). Price stays `1fr`. Mid-drag updates tracks only (no gutter rebuild). Chart resize coalesced to one `requestAnimationFrame`. `PaneLayout.setFooterHeight` persists. Stack budget keeps price ≥ 120px.

**Phase 4 (reorder):** Legend header drag → `PaneLayout.moveHostBefore` / `setOrder` only. Hidden hosts keep relative slots. Drop commits once (no DOM ordering SSOT). `fullscreenPaneId` unchanged by reorder.

**Phase 5 (fullscreen):** Dblclick empty LWC plot chrome → `toggleFullscreen(paneId)`. Ignore legends / scales / splitters / controls. Escape / second dblclick → `setFullscreen(null)`. LayoutController only toggles `.fullscreen-pane` from state; order/heights/visible untouched. One rAF resize after apply.

**Rejected:** Weighted `fr` footers (squashes price); trusting localStorage without ∩ manifest; Ind HTML hardcodes; deep render pause in Phase 1; server layout FSM; static HTML splitters between fixed neighbors; DOM-owned fullscreen class without PaneLayout.

**Consequences:** Debt **#90**. Modules: `web/ui/pane-layout.js`, `web/ui/layout-controller.js`. Regressions: `web/pane_layout_test.js`, `web/layout_controller_test.js`. Instance: `window.paneLayout` after DDR mount.

---

## ADR-020 — HostID ScaleController + Chart Chrome Polish

**Context:** Auto/Log was a single global SSOT bound only to price. Footers inherited create-time `autoScale` then never re-armed. Time axis labels sat on the top price pane; Ruler was stubbed. Need HostID-generic scale ownership before more oscillators (ATR, MACD).

**Decision (Phase 1 — ScaleController foundation):**

- `ScaleController.register({ context, hostId, chart, host, allowLog, scaleGroup? })` — no hardcoded pane switch.
- Prefs per `hostId` in versioned `chart_scale_prefs_v3`; migrate v2 global → `price`. New hosts default Auto ON.
- `scaleGroup` dormant (default `hostId`) — no group apply yet.
- Price: `allowLog: true`. Footers: `allowLog: false` (Auto only).
- Manual Y-gesture updates **that** hostId only. PaneLayout visibility must not reset prefs.
- UI: `.scale-controls` with `data-scale-pane` / `data-allow-log`.
- **Persistence invariant:** prefs must be self-sufficient. Valid = Auto ON, or Auto OFF + `manualRange {min,max}`. Invalid Auto OFF (no range yet) is repaired to Auto ON via pure `repairScalePrefs` (preserve Log; dirty → one write). `manualRange` is a future socket — not written this phase.

**Deferred:** persist Manual Y range. HH:mm crosshair datetime polish → **closed (Debt #91 chrome)**. Bottom axis → **ADR-023**. Ruler → **ADR-025**.

**Rejected:** Global Auto/Log for all charts; Log on osc panes; group scale apply in P1; reviving legacy adapters.

**Consequences:** Debt **#91**. Module: `web/ui/scale-controller.js`. Regression: `web/scale_controller_test.js`.

---

## ADR-021 — Chart Interaction Ownership (TimeCamera)

**Context:** Footer pan/zoom broken; Active Driver Lite (`attachSlaveWheelProxy` + price-only sync) fragmented timeline ownership. Crosshair foreign-Y deferred to later phase.

**Decision (Phases 0–1):**

- **`TimeCamera`** sole originator of canonical `visibleLogicalRange` + `barSpacing` (+ optional `rightOffset`). Atomic `commit({ visibleRange, barSpacing, rightOffset, sourceHostId })` only — no piecemeal setters. Echo lock `isSyncing`.
- **ChartAdapter** subscribes all panes, proposes via `TimeCamera.proposeFromPane`, applies only via `applyCommittedCamera`. Public `setVisibleLogicalRange` / `commitTimeCamera` originate through `commit`.
- Footers: native LWC `handleScroll` / time wheel. **`attachSlaveWheelProxy` deleted.**
- No chart is semantic master; any HostID may propose.
- **ViewportManager** / **ScaleController** untouched in P0/P1 (capture-restore and Y scale). **Superseded for navigation ownership by ADR-028** (ViewportManager becomes translator only; TimeCamera owns VIEW).

**Deferred:** CrosshairController (P2) ✅, InteractionController (P3) → **ADR-024**. Navigation semantics → **ADR-028** / TF transition → **ADR-029**.

**Rejected:** Keeping wheel proxy; price-as-master sync; InteractionController in P0/P1.

**Consequences:** Debt **#49** closed for live path. Modules: `web/ui/time-camera.js`, `web/chart-core.js`. Regression: `web/time_camera_test.js`.

---

## ADR-022 — Oscillator Scale Contribution (Debt #68)

**Context:** RSX/Wozduh Auto fitted Y to visible samples (bounce). Forcing `setVisibleRange` / special-casing ScaleController was rejected (broke load; inverted Auto≡autoScale).

**Decision:**

- **ScaleController** stays Auto / Manual / Log only (`autoScale` true/false). No oscillator domains.
- Each **DDR component** may declare `renderOptions.scaleContribution`: `dynamic` | `bounded` | `ignore`.
- Tiny **`ScaleContribution.createAutoscaleProvider`** maps contribution → LWC `autoscaleInfoProvider`.
- **SeriesFactory** strips `scaleContribution` and attaches the provider — no hostId / primary heuristics.
- Ship only three types. No `symmetric` / `boundedFloor` until a real consumer.

**Wire:**

- `line_rsx` → `bounded(-5,105)`; `line_rsx_signal` → `ignore`.
- `woz_slow` → `bounded(-5,105)`; other Wozduh lines → `ignore`.

**Rejected:** ScaleController range freeze; `hostId === "rsx"`; `isPrimaryLine`; parallel `INDICATORS_CONFIG`.

**Consequences:** Debt **#68** closed. Modules: `web/ui/scale-contribution.js`, `web/series-factory.js`, `ui_config/rsx_layout.go`, `ui_config/wozduh_layout.go`. Tests: `web/scale_contribution_test.js`, `ui_config/scale_contribution_test.go`.

---

## ADR-023 — Single Bottom Timeline Axis + Footer Layout Cleanup

**Context:** After ADR-021 (TimeCamera) and ADR-022 (scale contribution), time labels still lived only on the price pane while every footer reserved blank LWC time-scale height. That wasted vertical space and looked like “gaps,” not a CSS bug.

**Decision:**

- **One timeline, one visible bottom axis.** PaneLayout declares the owner via `resolveBottomTimeAxisHostId(state)` (fullscreen → else last visible footer in `order` → else `price`).
- **LayoutController** allocates stack geometry and marks `data-bottom-time-axis`; calls `ChartAdapter.setBottomTimeAxis(owner)`.
- **ChartAdapter** only mirrors: `timeScale.visible` / labels on for the owner, **off** for all other panes (reclaim strip height). No `hostId === "wozduh"` branching.
- **TimeCamera / Crosshair / ScaleController / ScaleContribution** unchanged for axis *ownership*. Crosshair **time label rendering** on that owner is clarified in **ADR-027 amendment** (sync owns semantics; axis owner renders).

**Keep:** 4px pane splitters (resize chrome). They are not phantom axis gaps.

**Rejected:** Negative margins / CSS overlap; blank-but-visible slave axes; hardcoding Wozduh as axis owner; deleting gutters.

**Consequences:** Closes ADR-020 deferred `getBottomPane` + `setBottomAxis` for the live stack. Modules: `pane-layout.js`, `layout-controller.js`, `chart-core.js`. Tests: `pane_layout_test.js`, `bottom_time_axis_test.js`.

---

## ADR-021 Phase 2 — CrosshairController

**Context:** Syncing crosshair with `setCrosshairPosition` onto price while hovering footers painted a horizontal line in the price domain (foreign UX).

**Decision:**

- **`CrosshairController`** owns only `hoveredHostId` + V/H policy. **Never** mutates timeline / TimeCamera state.
- **Hover ownership (refinement):** `hoveredHostId` is set **only** from browser pointer events on PaneLayout wrappers (`data-pane-host`). Never from `subscribeCrosshairMove` / synthetic LWC. LWC events are observational; browser pointer is authoritative.
- Semantic API: `setHovered(hostId)`, `syncTime({ sourceHostId, time })` — no chart/series/param.
- Hovered pane: vert + horz. Peers: vert time-sync with **target-local** Y; horz hidden (re-asserted after peer sync).
- ChartAdapter is the only LWC talker.

**Deferred:** (none for interaction stack — ADR-024 completes P3). Ruler / datetime chrome are separate ADRs.

**Consequences:** Module `web/ui/crosshair-controller.js`. Regression: `web/crosshair_controller_test.js`.

---

## ADR-024 — InteractionController (Complete ADR-021 P3)

**Context:** TimeCamera and CrosshairController worked, but ChartAdapter still routed LWC events into them directly — adapter + interaction router in one module.

**Decision:**

- **`InteractionController`** owns interaction routing only: `onPointerEnter` / `onPointerLeave` / `onRangeChanged` / `onCrosshairMove` / `dispose`.
- **ChartAdapter** keeps LWC subscriptions, DOM leave resolution, and LWC→semantic extraction; forwards to InteractionController. System/compositor `TimeCamera.commit` paths stay on ChartAdapter (not user interaction).
- **TimeCamera / CrosshairController / ScaleController** unchanged. ScaleController Y-hit remains on LayoutController until a second consumer needs IC to filter it (Rule 6 — no unused socket).
- No event bus, no PointerDispatcher, no generic framework.

**Invariant:** InteractionController accepts **only semantic interaction events**. Translation from DOM / LWC / browser-specific objects is the **exclusive** responsibility of ChartAdapter. Never `onCrosshairMove(hostId, lwcParam)`.

**Pipeline:**

```
Browser / LWC events
        │
        ▼
ChartAdapter          ← translates (DOM/LWC → semantic)
        │
        ▼
InteractionController ← routes only
        │
 ┌──────┼──────────────┬─────────────┐
 ▼      ▼              ▼             ▼
TimeCamera  CrosshairController  RulerController (ADR-025)
```

ChartAdapter translates. InteractionController routes. Controllers own behavior.

**Rejected:** Moving LWC apply hooks into IC; inventing ScaleController routing without a consumer; behavioral changes; EventBus / PointerDispatcher / InteractionContext / MouseService; merging TimeCamera or Crosshair into IC.

**Consequences:** Completes ADR-021. Module: `web/ui/interaction-controller.js`. Tests: `web/interaction_controller_test.js`. Next UI feature (Ruler) plugs into IC.

---

## ADR-027 — Display Timeline / Decoration Plane (repaired)

**Context:** ADR-026 synced peer crosshair lines in empty space via `logical`, but the LWC time scale still ended at the last real candle — no bottom ticks and no native crosshair time label in the future strip (`rightOffset` / logical past last bar).

**Fundamental invariant:**

> The `candleSeries` always contains only real market candles.  
> Decoration never enters the market-data plane.

**Decision (final — Decoration Plane):**

Two planes, one composer:

| Plane | Owns | Must not |
|-------|------|----------|
| **Market Plane** | `ColumnarStore`, DDR, `candleSeries`, forming-tip `update()` | Hold future whitespace; invent display times |
| **Decoration Plane** | `DisplayTimeline` (pure future time math), `TimelineDecoration` (sealed LWC whitespace series) | Touch store/DDR/OHLC; expose series getters; call `update()` |

- **`DisplayTimeline`** — pure future bar-open times (fixed step + UTC week/month, aligned with `data.NextBarOpen`). No LWC, no store.
- **`TimelineDecoration`** — sealed LWC owner: `attach` / `refresh` / `dispose` / `applyCrosshairTime`. Private series (`__timeline_decoration__`, `autoscaleInfoProvider: () => null`). No `getSeries`, no market `update()`.
- **`ChartAdapter`** — composer only: `setData(real)` / `update(tip)` on candles; builds decoration payload from tip + TimeCamera; fans `TimelineDecoration` across live panes (TimeCamera logical alignment + ADR-023 axis owner). Never invents business math.
- CrosshairController / TimeCamera / ScaleController / PaneLayout ownership unchanged.

**ADR-027 amendment — bottom-axis time label:**

The bottom-axis time label is **not** owned by the hovered pane.  
`CrosshairController` owns synchronized semantic position `{ logical, time? }`.  
Rendering of the label is always delegated to the configured bottom-axis owner (PaneLayout → ChartAdapter `setBottomTimeAxis` → LWC `timeScale.visible` on that pane only).  
ChartAdapter private crosshair apply re-asserts the native label on that renderer after peer sync / horz policy — translation, not a new owner.

**Why candle+whitespace mixing was rejected (tip invariant):**

An early attempt appended DisplayTimeline whitespace onto `candleSeries` so LWC’s time-scale union would extend. That made `last item !== forming tip`: LWC `update(tip)` then failed (`Cannot update oldest data`) because the series tip was decoration, not the live candle. Mixing decoration into the market series also poisoned the tip SSOT and invited store/DDR confusion.

**Why TimelineDecoration fixes the conflict:**

Decoration is a separate invisible series on each pane. LWC unions times for axis/crosshair chrome; `candleSeries` remains real-only so `update(tip)` stays O(1) and honest. Market plane and decoration plane never share a series.

**Rejected:**

- `setData(real + whitespace)` / merge onto `candleSeries` — **Reason:** breaks tip `update()` invariant; decoration enters market plane.
- Fabricating time inside CrosshairController — **Reason:** ADR-026; sync must not invent timestamps.
- Custom DOM bottom axis / second label painter — **Reason:** LWC axis owner is the renderer; duplicate chrome.
- Storing future bars in ColumnarStore / DDR / engine — **Reason:** display-only; not market truth.
- New controllers for decoration or bottom label — **Reason:** ChartAdapter composition + existing owners suffice.

**Group B — encapsulated translation (not architectural debt):**

| Detail | Owner | Note |
|--------|-------|------|
| HTML `.peer-crosshair-guide` | ChartAdapter | When `time` is null; ADR-026 logical guide |
| Mid-visible-price Y for peer `setCrosshairPosition` | ChartAdapter | Local Y only; whitespace has no series value |
| Private decoration series + legend title filter | TimelineDecoration / legend chrome | LWC needs a series handle; not market data |
| `applyCrosshairTime` seam | TimelineDecoration | Positions native label without exposing series |

Future LWC Primitive API may replace HTML guide / mid-Y; do not chase now.

**Future ADR candidates (real debt only):**

- Global ChartAdapter destroy lifecycle (`_disposers` not guaranteed on every teardown path).
- Primitive migration for peer crosshair guide (replace HTML translation).
- Any remaining true duplicate ownership found in Phase 4 audit — not speculative modules.

**Consequences:** Modules: `web/ui/display-timeline.js`, `web/ui/timeline-decoration.js`, `web/chart-core.js`. Tests: `web/display_timeline_test.js`, `web/timeline_decoration_test.js`, crosshair/bottom-axis contracts. Phase 2 approved; Phase 4 ownership burn audits against this freeze.

---

## ADR-026 — Crosshair Empty-Space Sync

**Context:** Peer vertical crosshairs synced only via `param.time`. In empty/future space LWC sets `time` undefined while still providing `logical`, so peers were cleared while the hovered pane kept a native line.

**Decision:**

- Semantic payload: `{ logical, time? }` — **logical is primary**; time is optional metadata.
- **Never fabricate/extrapolate timestamps.**
- CrosshairController clears peers only when `logical` is missing — not when `time` is null.
- Hovered pane stays native LWC.
- ChartAdapter single entry `renderPeerCrosshair` / internal `applyPeerCrosshair`: `time` → `setCrosshairPosition` (local Y); else → logical guide (`logicalToCoordinate`, DOM `.peer-crosshair-guide` behind the adapter socket).
- No `CrosshairState` store; no foreign Y; TimeCamera / Scale / PaneLayout / Ruler unchanged.

**Rejected:** Fake time for `setCrosshairPosition`; HTML as a public API; controller-owned Y; second sync socket beside native apply.

**Consequences:** Modules: `crosshair-controller.js`, `interaction-controller.js`, `chart-core.js`. Tests: `crosshair_controller_test.js`, `interaction_controller_test.js`.

---

## ADR-025 — TradingView-style Ruler (complete)

**Context:** Phase 1 foundation used drag-release + time-required points + infinite H/V guides (looked like a blue cross). Empty/future space failed when `coordinateToTime` returned null.

**Decision (final):**

- **Anchors:** `{ logical, price, time? }` only — never screen `x/y`. ChartAdapter re-projects via `logicalToCoordinate` / `priceToCoordinate` on every render (pan/zoom/resize safe).
- **Two-click FSM:** `idle → armed → placing → finished`. 1st click = A, move = preview, 2nd click = B. `pointerUp` does **not** finish. `onCancel` (Esc / right-click) → `armed` (tool stays on).
- **Bars:** `abs(logicalB − logicalA)` only — never Δtime / TF (weekend/gap safe).
- **`RulerMetrics`:** pure numbers (`deltaPrice`, `%`, `bars`, `duration`, `ticks`). ChartAdapter owns tooltip HTML lifecycle.
- **Render:** finite rectangle (shade border) only — no infinite guides. Crosshair remains the infinite primitive.
- **Empty space:** `coordinateToLogical` + price-scale `y` required; `time` optional.

**Pipeline:** `ChartAdapter → InteractionController → RulerController → RulerMetrics → ChartAdapter(render)`.

**Rejected:** DrawingManager / EventBus; pixels in controller; bars from elapsed time; requiring `time`; DOM in RulerController; Volume line in Phase 1 tooltip.

**Consequences:** Debt **#91** ruler product slice closed for Measure v1. Modules: `ruler-controller.js`, `ruler-metrics.js`, IC, `chart-core.js`. Tests: `web/ruler_controller_test.js`.

---

## ADR-028 — TimeCamera Navigation Ownership (VIEW)

**Context:** Time Domain Audit showed the TF-switch “jump left / full empty” symptom is not a rendering bug. Navigation state is fragmented across ViewportManager pin/intent/restore, direct LWC `applyOptions` / `scrollToPosition`, range-only TimeCamera commits, and a wrong live-edge predicate that treats large future overhang as “pinned.” ADR-021 made TimeCamera the canonical range writer on paper; it did not yet make **navigation** (what the human is looking at) a first-class concept with one owner.

**Amends:** ADR-021 (ViewportManager may own capture-restore policy → demoted to translator). Does not change Crosshair (026), Decoration (027), or PaneLayout (023).

**Decision:**

### Laws (permanent)

1. **TimeframeController never decides where the camera goes. TimeCamera always decides.**
2. **Navigation never depends on timeframe density.** No `if (tf === '1D')` (or any TF) branches in navigation. Preserve meaning (time), not bar density.
3. **One camera write path only:** Application → TimeCamera → ChartAdapter **CameraCommit** → LWC. No second writers.

### What TimeCamera owns

Navigation semantics only — split SSOT:

| Piece | Fields | Meaning |
|-------|--------|---------|
| **ViewIntent** | `LIVE` \| `HISTORY` (future socket: `REPLAY`) | Why the user is looking |
| **ViewGeometry** | `centerTime`, `visibleBars`, `barSpacing`, `rightPadding` | What they see |

Derived LWC fields `{ visibleRange, barSpacing, rightOffset }` are outputs of CameraCommit — not competing SSOTs.

### What TimeCamera does **not** own

- Market data / bar indexing / `nearestLogicalForTime`
- Timeframe duration math for lookup
- Rendering / LWC calls
- Crosshair, decoration, layout, scale prefs

### Lifecycle

```
capture   → from latest committed camera state only (never mid-apply / inertia half-state)
propose   → navigation semantics (intent + geometry); may request a time→logical resolve
commit    → atomic CameraCommit via ChartAdapter
```

**Propose ≠ blind restore.** When exact `centerTime` has no bar, data resolves nearest logical; TimeCamera proposes valid geometry — it does not insist on an impossible restore.

### Data lookup boundary (mandatory)

```
TimeCamera requests centerTime
        │
        ▼
Data layer / ChartCompositor  →  nearestLogicalForTime(times, centerTime)
        │
        ▼
TimeCamera receives resolved logical
        │
        ▼
CameraCommit
```

TimeCamera never searches candles and never asks “what logical index is this?” of the store itself.

### CameraCommit (one transaction)

ChartAdapter applies navigation as **one** CameraCommit `{ visibleRange, barSpacing, rightOffset }` (rightOffset derived from tip + `rightPadding` when LIVE). The rest of the app must never treat separate `setBarSpacing` / `setVisibleLogicalRange` / `scrollToPosition` as public navigation operations. Internally LWC may need multiple calls; the application sees exactly one commit.

### ViewportManager

Translator / capture helper only. Must not own LIVE/HISTORY policy, must not clamp/sticky-decide TF navigation, must not write LWC camera directly (`applyOptions` spacing/offset, `scrollToPosition`). Phase D migrates; this ADR freezes the target.

**Rejected:**

- `NavigationManager` / new controllers / event buses
- TF-owned camera / density branches
- Preserving bare logical index across TF
- Unbounded `rightPadding` as market truth
- TimeCamera performing store/bar search
- Second camera write paths (including ViewportManager direct LWC)

**Consequences:** Constitution updated. Implementation = Phase D1 (shadow) → D2 (cutover) after ADR-029. Runtime unchanged until then. Offenders to retire: `isPinnedRight` false sticky, unbounded sticky offset, VM direct LWC writers, non-atomic range-only commits.

---

## ADR-029 — Timeframe Transition Protocol

**Context:** ADR-028 freezes navigation ownership. TF switching still needs an explicit protocol so LIVE sticky-edge and HISTORY center-anchor behave like TradingView without TF-specific code. Depends on ADR-028; does not invent a second owner.

**Decision:**

### Pipeline

```
TF click
  → TimeCamera.capture()                 # from latest committed state only
  → TimeframeController sets currentTf   # TF id only — no camera
  → prepareLiveTfHandoff + loadDashboard
  → paint new market data
  → DataResolve(centerTime → nearest logical) or tip logical if LIVE
  → TimeCamera.propose(ViewIntent + ViewGeometry + resolved logical)
  → ChartAdapter CameraCommit (atomic)
```

TimeframeController never chooses LIVE vs HISTORY, never sets range/offset/spacing.

### ViewIntent classification (hysteresis)

Let `tip` = last real logical index, `to` = committed visible `to`, `overhang = to - tip`, `SLACK = 1.5`.

- **LIVE** if `overhang >= -SLACK` (tip on screen or within slack left; future/right padding allowed).
- **HISTORY** if `to < tip - SLACK` (user clearly pulled tip off the right).

Do not treat large positive overhang alone as a reason to invent density-specific behavior — clamp handles void.

### LIVE propose

- Stick to **new** series tip (`to = newTip + pad`).
- Keep captured `visibleBars`, `barSpacing`, and `rightPadding` in **bars** (same pixel X if spacing unchanged).
- Window width: same `clampVisibleLogicalWidth` as wheel zoom (`MAX_VISIBLE_BARS` / 5000). Not 50–400. Not pad-50.
- Poison spacing only (&lt; min) → healthy 6. `from < 0` is not poison.
- Missing/invalid capture → Fresh LIVE. Valid LIVE seed → no layout-defer FreshLive.

### HISTORY propose

- Preserve `centerTime` (screen-center meaning).
- DataResolve → nearest logical on the **new** series.
- Center geometry on that logical; keep `visibleBars` / `barSpacing` (poison-sanitized).
- **Never** jump to live tip.
- User TF change + valid prior VIEW: same preserve as LIVE (`visibleBars` + `barSpacing`). Initial HISTORY fetch is sized to that VIEW; **the same limit** is used for `endTime` and the fetch (island stays centered). Invalid / no prior HISTORY geometry → 150 bars / spacing 6. Scroll chunk stays 3000. Caps: `MAX_VISIBLE` 5000 (camera), `MAX_STORE` 9000 (storage).

### Error / poison

| Case | Action |
|------|--------|
| Null / failed capture | Fresh LIVE: healthy spacing, `rightPadding: 0` |
| No bar for `centerTime` | Nearest logical (data layer) |
| Empty series | Fresh LIVE / no-op until data exists |
| Accordion / crushed spacing | LIVE and HISTORY: spacing &lt; min → 6; width cap `MAX_VISIBLE_BARS`. Invalid/no prior HISTORY geometry → 150/6 fallback (not a TF-switch reset). |

### Explicit non-goals

- No TF-specific branches
- No Crosshair / Decoration / PaneLayout redesign
- No TimeCamera bar indexing
- No `fitContent` / `scrollToRealTime` on the live TF path

**Rejected:** Left-edge anchor; preserving logical index across TF; Sticky Live Edge without overhang clamp; ViewportManager owning transition policy; blind `restore()` that cannot propose nearest.

**Consequences:** Protocol ready for Phase D1/D2. Modules affected at implement time: `time-camera.js`, `viewport-manager.js` (demote), `timeframe-controller.js` / `boot.js` / `chart-compositor.js` (wire propose after paint), `chart-core.js` (CameraCommit). Tests: intent classification, clamp, nearest-bar helper (outside TimeCamera), TF contract (single write path).

---

## ADR-030 — Timestamp units without magnitude inference (#83)

**Context:** `ts < 1e12` guessed seconds vs milliseconds. Pre-2001 Unix **ms** (`730944000000`) is still `< 1e12`, so the heuristic multiplied it into year ~25132. Dual domains (Go ms vs LWC sec vs camera ms) are real and must stay explicit.

**Decision:** One unit per layer; convert only with named primitives (`ChartTimeSec`, `secToMs`, `msToChartSec`). Never inspect magnitude. Keep camera geometry in milliseconds.

**Rejected:**
- A new threshold (e.g. `1e11`) — **Reason:** same class of landmine.
- Collapsing camera keys to seconds — **Reason:** geometry/search already keyed in ms; F5b/F5c are explicit crossings.
- Sweeping inactive `app.legacy.js` / comments in the same pass — **Reason:** not on the live data path.

**Consequences:** Debt #83 closed (`TS_CONTRACT_CLEAN`). Do not reopen for leftover comments. Chart/camera layout and SQLite remain frozen.

---

## ADR-031 — Native Binance timeframe catalog + persistence class

**Context:** Live Frames and WS kline streams were a copied 10-TF list. The UI advertised `2h` and `2m` as if they were live Binance intervals. `2m` is not sold by USD-M klines. Derived / seconds / ticks were menu sockets with no builder. Persisting every TF equally would duplicate 1m into SQLite.

**Decision:** One static catalog in `exchange/timeframe_catalog.go`. Project-supported native USD-M klines:

`1m 3m 5m 15m 30m 1h 2h 4h 6h 8h 12h 1d 1w 1M`

Binance also sells `3d`. This project does **not** support it: epoch-floor 3d is not Binance’s 3d grid (phase offset + historical realignments). Re-add only with a Binance-observed 3d boundary chapter.

Live chart also includes derived views `2m←1m`, `10m←5m`, `45m←15m`, `3h←1h` (TF-B) and live `1s` from aggTrade (MICRO-1): RAM Frame, no SQLite, no REST. Persistence:

```text
NATIVE_BINANCE  → durable historical_klines (Binance authority)
DERIVED timebars → reconstructable view; no SQLite source of truth
1s (aggTrade)   → micro_klines (24h, sparse, same PersistenceQueue; not historical_klines / archive_gaps)
                 + bounded RAM (9000) hydrated from latest micro rows on boot
SECONDS 5s–45s / TICKS → catalog placeholders until derived from 1s / tick bars
```

WS kline subscriptions stay native-only. Combined WS also carries `@aggTrade` for the 1s builder only. Live-chart allow-list = native ∪ `{2m,10m,45m,3h,1s}`. Forming derived updates replace the same parent bucket; child close requires distinct closed parents.

Tick contract: **1 tick = 1 `aggTrade` event** (tick bars not implemented).

**Rejected:**
- Persist derived 2m / 10m / 45m / 3h — **Reason:** no new market information; reconstruct from parent.
- Persist seconds/ticks in `historical_klines` — **Reason:** 60× 1m (and worse); RAM rings later.
- Timeframe registry / service / lazy Frame factory — **Reason:** deletes duplicated lists; does not own runtime.
- Enabling tick bars and 5s–45s in this slice — **Reason:** 1s is the primitive; others derive after it is proven.
- Injecting synthetic child `WsTick` into `routeTick` — **Reason:** would trip native tip-gap REST on a non-exchange interval.
- Keeping native `3d` on the generic fixed-duration grid — **Reason:** Binance 3d is not Unix-epoch floor (live phase + 2019/2023 seams); unused TF poisoned Master timeline health.

**Consequences:** Native `2h 6h 8h 12h` boot and subscribe like other natives. `3d` is out of the catalog (no Frame, no WS, no menu). Live-chart allow-list = native ∪ `{2m,10m,45m,3h,1s}`. `EnsureHistoryWindow` / `historical_klines` / heal / archive stay native-only. Closed 1s persists to `micro_klines` via the same PersistenceQueue (MICRO-2A); it never writes `archive_gaps` or participates in Master continuity. Derived `/api/history` folds the parent window. Camera unchanged.

MICRO-1.1: FE live gap heal (`appendTick` 1.5× interval → `fe_gapDetected`) runs only when `requiresDenseTimeContinuity(tf)` is true (native + derived). Seconds/ticks never treat a skipped timestamp as a data hole. Do not special-case `"1s"`.

---

