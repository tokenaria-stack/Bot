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

**SCALE-GESTURE-OWNERSHIP-1 ✅ frozen.** Command: prefs → LWC. Observation: native Y-axis gesture → prefs/UI only. Do not reopen ScaleController except a real regression. No drag flags, no `manualRange` persistence.

**TIMELINE-RECOVERY-STATE-1 ✅ frozen** (live 2026-09-16 16:26 +08). Current Master bool + snapshot replace. Do not reopen browser reconnect / dense `gapDetected` recovery / native gap heal except a real regression.

**PANE-WORKSPACE-MAXIMIZE-1 ✅ frozen** (`51272a9`). Maximize = one `#charts-stack` track. Do not reopen viewport-fixed pane fullscreen, header-height offsets, or z-index wars vs `#app-chrome`. Layout suppression ≠ `visible[]`.

**CHANNEL-SPLIT-FILLS-1 ✅ frozen** (`a02aa98`). Shared `boundColor` on both Wozduh channels; RSI close split fills; Volume whole-band `fillColor`. Do not reopen pane-edge outer fills or twin Volume `upperColor`/`lowerColor`.

**WOZDUH-PANE-AUTOSCALE-OWNER-1 ✅ frozen** (`1a409e9`). Wozduh Auto domain is the Extreme Bands host (`[-5,105]`). Do not put bounded autoscale back on hideable `woz_vol_rsi_ema5`. ScaleController stays generic.

**HYDRATION-OWNERSHIP-1 ✅ frozen** (`c254253`). Plot hydrate: prefs → `requestedPlotIds` → one `/api/history` → time-keyed store. Do not positional-paste `updatePlots`. Do not refetch columns already in the store when applying visibility.

**CROSSHAIR-PANE-HOST-1 ✅ frozen** (`87fd5a8`). Oscillator native crosshair is pane chrome, not DDR plot IDs. Do not restore `CROSSHAIR_ANCHORS` / oscillator `hydratedValueAtTime`. Do not invent a history-length fake chrome series.

**RSX-PANE-AUTOSCALE-OWNER-1 ✅ frozen** (`01e2a7d`). RSX Auto domain is the RsxScaleLines host (`[-5,105]`). Do not put bounded autoscale back on hideable `line_rsx`. Do not add primitive `autoscaleInfo()` or a history-length fake chrome series. ScaleController stays generic.

**CHART-RENDERING-FAMILY-1 ✅ frozen** (after `01e2a7d`). Live indicator paint closed. Native RSX markers stay on `line_rsx`. Do not start annotation-pane-host work. Do not reopen scale / crosshair / hydration / fills / maximize unless a real regression.

**PRICE-SERIES-STYLE-1.1 ✅ frozen** (`71c929b`). Candles/bars/line are presentations of the same OHLC store. One `priceSeries`. Toolbar favorites one-click; Line color/width `applyOptions` only. Do not restore labeled Candles/Bars/Line buttons or a user-facing Reload. Do not add Renko/Kagi here.

**WOZDUH-FAMILY-EYE-1 ✅ frozen** (`b9ae83b`). Eye mutes/restores existing Wozduh visibility IDs for two visual families. Do not add `groupVisible`, a second demand system, or treat the mute snapshot as live visibility SSOT. Crossover dots stay outside both families. Do not reopen crossover overlay architecture, hydration, autoscale, or channel math.

**WOZDUH-CROSSOVER-PAINT-1 ✅ frozen.** Four closed-bar Wozduh A×B dots. Presentation microscope only. Do not restore VolCross as a slot/fact/tape/`IndicatorFactEvent`. Do not attach dots to DDR series B. Do not invent a generic marker/event/demand framework. Gear menu is paint prefs only (`wozduh_crossover_prefs_v1`). Do not reopen Wozduh numeric semantics, hydration architecture, autoscale, or crosshair for this overlay.

**Chart polish later (does not touch learning math / FeatureTape / labels / CatBoost):**

| ID | What | Why later |
|----|------|-----------|
| **RSX-ANNOTATION-PAINT-1** | Native `setMarkers` still uses `getSeries('line_rsx')`. Hiding the stroke hides arrows. Facts remain `wire.Annotation`. | Operator accepted live look. Chrome-host `setMarkers` is unsafe (historical times + `aboveBar` relative to series value). |
| **WOZDUH-AUTOSCALE-FIRSTVALUE-1** | Extreme Bands host owns `[-5,105]` on paper; LWC 4.2.1 skips null `firstValue()`. Wozduh plots already span ~0–100 so the pane *looks* correct. | Do not copy the RSX post-DDR refresh “to be safe.” Reopen only if Auto collapses when plots are hidden. |
| **UI-PALETTE-1** | Factory `ui_config` colors vs operator taste. | Explicit ask only. Not FeatureSpec. |

Do not mix these into TP-STOP / FeatureSpec / tape chapters.

**ATOMIC-VOCAB-AUDIT-1 (NEXT — strategy resume after chart freeze).** Read-only inventory of live+research information into **event atoms** vs **context readings**. Champion/challenger lineage: Wozduh context on the same ignition rows is case 1; Wozduh confirmation clock is case 2 (new version, new `ConfirmedAt` law). Do not mutate frozen setup identities in place. Do not start Setup Lab UI. Do not invent events for every line. TP-STOP-FACT-GAP-1 stays on the Brain-3 ledger; it is not this chapter.

**Cleanup (scale + timeline forest):** deleted `web/scale_blank_price_diag_test.js` (diag duplicate of paint/ownership tests). Keep `scale_paint_ownership_test.js`, `scale_controller_test.js` observation tests, timeline state/recovery tests, `[FEGap]`/`[FEGapRecovered]`, `[HealProbe]`, opt-in TipSSOT/ProjCont. Keep Brain3/opportunity canvases. Deleted volume-ingest canvas (SSOT is `research/volume/`).

**After freeze (cleanup rule):** prove dead → delete → tests → smoke → checkpoint. No speculative deletion of TimeCamera / hydration / prune.

**NEXT order (do not start inside this freeze):**

1. **PRE-STRATEGY-CLEAN-1 / SLICE-1 GREEN / FROZEN** — Falcon-era Backtest product amputated. Do not reopen old Backtest. `/api/stats` is a **FUTURE CONSUMER AUDIT**. `ApplyBacktestRSXConfig` is **SURVIVING NAME DEBT**.
2. **FALCON-REMOVE-1 frozen** — FalconEngine deleted. Canonical RSX/Wozduh are DAG + `indicators`. `ApplyBacktestRSXConfig` remains a legacy-named RSX pin/replay helper. **WOZDUH-NUMERIC-NAMES-1 frozen.** **WOZDUH-VOLCROSS-REMOVE-1 frozen.** **WOZDUH-COLOR-OVERRIDES-1 frozen** (`1c7ae3d`). **WOZDUH-STYLE-SECTION-1 frozen** (`b55b64f`). **CHANNEL-SPLIT-FILLS-1 frozen** (`a02aa98`). **WOZDUH-PANE-AUTOSCALE-OWNER-1 frozen** (`1a409e9`). **HYDRATION-OWNERSHIP-1 frozen** (`c254253`). **CROSSHAIR-PANE-HOST-1 frozen** (`87fd5a8`). **RSX-PANE-AUTOSCALE-OWNER-1 frozen** (`01e2a7d`). **CHART-RENDERING-FAMILY-1 frozen.** **WOZDUH-CROSSOVER-PAINT-1 frozen.** **WOZDUH-FAMILY-EYE-1 frozen** (`b9ae83b`). **PRICE-SERIES-STYLE-1.1 frozen** (`71c929b`). Do **not** retune `ui_config` until asked. Falcon retirement chapters frozen. Next unfinished: **DATA-1B** (ledger seam vs listing-day ownership — not chart paint).
3. Dead-code / legacy cleanup ✅ CLEAN-1–4 + DOC-1  
4. SQLite/WAL — **SQLITE-1 ✅** + **SQLITE-2 ✅** (MCP off) + **SQLITE-2b ✅** (single-conn pool; idle handles were pinning TRUNCATE)  
5. TF-switch UX — **TF-1 ✅** + **TF-2A ✅**. **HIST frozen** (0/1/2 + 1.1 + 3). **DATA-1A ✅** (spot `history_sync` key + BTCUSDT 15m Vision Jan 2018–Sep 2019). **DATA-1B** next: choose ledger cleanup vs listing-day seam ownership from smoke (do not assume 16:00 becomes READY).  
6. FE paint skip + Wozduh demand: HIDDEN-RENDER-SKIP-1 + WOZDUH-OWNER-1 + **WOZDUH-WIRE-1 frozen** (`0c2ecce`) + **WOZDUH-ACTIVE-1A frozen** (`2cd4ca4`) + **WOZDUH-ACTIVE-1B frozen** (`1b724ef`). **Do not reopen Wozduh.**  
7. **DAG-DEMAND-1 ✅ frozen** (`0837c77`). **FORECAST-SPEC-1 ✅** `5afabfc`+`0ed000d`. **FEATURE-TAPE-1A ✅ frozen** (`b88bcd2`). **FEATURE-TAPE-1B ✅ frozen** (`6715718`). **ATR-TRUTH-1 ✅ frozen** (`84124a0`). **LABEL-SET-1A ✅ frozen** (`690d0be` + `1433626`). **LABEL-SET-1B ✅ frozen** (`8e88844`). **RSX-TV-ONE-BRAIN-1 ✅ frozen** (`4688160`). **FEATURE-TAPE-RSX-REGEN-1 ✅**. **RESEARCH-DATASET-1 ✅ frozen** (`f311203`). **VALIDATION-PLAN-1 ✅ frozen** (`0737c59` / docs `45155eb`). **OOF-MATRIX-1 ✅ frozen** (`d749042`). **MODEL-FIT-1 ✅ frozen** (`0d60270` + SciPy `29cb928`). **CALIBRATION-1 ✅ frozen** (`4c8c08e` + ftol `b550613`). **RANK-1 ✅ frozen** (`cc49528`). **RECIPE-FREEZE-1 ✅ frozen** (`c2d278a`). **FINAL-MODEL-FIT-1 ✅ frozen** (`9cb03f5`). **FORECAST-BUNDLE-1 ✅ frozen** (`a9e1228`). **OOF-FORECAST-EVIDENCE-1 ✅ frozen** (`124f273`). **DECISION-VALIDATION-PLAN-1 ✅ frozen** (`a0da055`). **DECISION-CONTRACT-1 ✅ frozen** (`b7a76b4`). **DECISION-RESEARCH-1 ✅ frozen** (`789ddcd`). **FEATURE-SPEC-2 ✅ frozen** (`0c54848`). **FEATURE-TAPE-2 ✅ frozen** (`61d5ca0`). **LABEL-SET-C ✅ frozen** (`ce3e542`). **DATASET-C / VALIDATION-PLAN-C ✅ frozen** (`151e530`). **OOF-MATRIX-C ✅ frozen** (`307b5e8`). **CATBOOST-BRAIN-1 ✅ frozen** (`cecc5a3`). **DECISION-RESEARCH-C ✅ frozen** (NOT_ELIGIBLE). Brain V2 ledger (this file). Holdout sealed. **TARGET-RESOLUTION-2** deferred. Do not start FINALIZATION-C. V1 KEEP ≠ SUPPORT.

**RSX-TRUTH-CLEAN-1 ✅ frozen** (`5f8a290`). Backend RSX is numerical/factual only. Live paint stays FE. Do not reopen slope-vs-50 color, `rsxColor` wire, or empty L/LL/S/SS sockets.

**RSX-SIGNAL-1 ✅ frozen** (`1c353e0`). Pine TV divergence facts (`rsx_tv_div`).

**RSX-SIGNAL-1.1 ✅ frozen** (`b4ac2ae`). TV family closed.

**RSX-SIGNAL-2A ✅ frozen** + **RSX-SIGNAL-2A.1 ✅ frozen** (`39d6f78`). ZigZag facts + one-walk/wrap collector + annotation revision gate. Do **not** reopen 2A plumbing.

**RSX-SIGNAL-2B ✅** — obsolete ZigZag DivState/DivScore path deleted.

**LEGACY-SCORE-CLEAN-1 ✅** — DAG MicroPatternNode / ScoreNode deleted.

**SLOT-CLEAN-1 ✅** — compacted dead slots.

**FALCON-SCORE-CLEAN-1 ✅** — write-only SmartDivergenceEngine / `divSignal` deleted. FalconEngine numerical calculator later deleted in **FALCON-REMOVE-1**. Frame ZigZag kept (fib/geometry).

**RSX-SIGNAL-3 ✅ frozen** (`c856fef`). Fractal facts (`rsx_fractal_div` class_a/b/c, `rsx_fractal_pivot`) + bounded `FractalFactsAt`. Do **not** reopen detector math or lookback search.

**RSX-VISIBILITY-1 ✅ frozen** (`749912f`). Five FE visibility flags; `div_method` / `show_pivots` deleted. Facts independent of **presentation** (not of compute demand). Visibility not in RSX fingerprint. Do **not** reopen.

**WOZDUH-ACTIVE-1B ✅ frozen** (`1b724ef`). Persistent Frame Wozduh mask = per-TF WS union OR proven internal (`Live`: VolBase|VolRsiEma12|VolRsiEma5). ChartOnly unused Frames are mask 0. **WOZDUH-NUMERIC-NAMES-1 frozen.** **WOZDUH-VOLCROSS-REMOVE-1:** discrete cross event deleted. Do **not** mint a replacement Wozduh fact. **WOZDUH-COLOR-OVERRIDES-1 frozen** (`1c7ae3d`). **WOZDUH-STYLE-SECTION-1 frozen** (`b55b64f`). **CHANNEL-SPLIT-FILLS-1 frozen** (`a02aa98`). **WOZDUH-PANE-AUTOSCALE-OWNER-1 frozen** (`1a409e9`). **RSX-PANE-AUTOSCALE-OWNER-1 frozen** (`01e2a7d`). User palette next; no factory retune until asked.

**DAG-DEMAND-1 ✅ frozen** (`0837c77`). Per-TF RSX analytical demand. ChartOnly unused: Core/TV/Fractal/DAG-ZZ/ZZ collector = 0. Live internal: Core only. Facts `*[]string` tri-state. One coherent RSX series per wake. HTTP history independent. Frame `a.zigzag` untouched.

**FORECAST-SPEC-1 ✅ frozen** (`5afabfc` + `0ed000d`). **FEATURE-TAPE-1A ✅ frozen** (`b88bcd2`). **FEATURE-TAPE-1B ✅ frozen** (`6715718`). **ATR-TRUTH-1 ✅ frozen** (`84124a0`). **LABEL-SET-1A ✅ frozen** (`690d0be` + `1433626`). **LABEL-SET-1B ✅ frozen** (`8e88844`). Do not reopen ATR, 1A physics, or 1B. **TARGET-RESOLUTION-2** deferred (separate 15m→1s TargetSpec).

**MARKET-RSX-PARITY-1 ✅ audit closed (no production code).** Stages 1–4: OHLC exact vs Binance; Jurik matches literal Pine replica; prefix cold-start ~105–130 then trailing facts match full archive; published TV facts miss a real Bear because `rsxTVHitAtDisplayBar` is a second Everget reconstruct (3×lookback ratchet restart, one winner). Full `scanRSXTVHits` has the Bear. Not TV builtin / not Jurik / not prefix. Fixture: BTCUSDT USD-M 15m AnchorAt `1788630300000` (2026-09-05 17:45 UTC = 01:45 UTC+8), ConfirmedAt `1788631200000`.

**RSX-TV-ONE-BRAIN-1 ✅ frozen `4688160`.** One `RSTVState` owns `rsx_tv_div` + `rsx_tv_pivot`. AnalysisLogicVersion `analysis:v2`. UI Bull/Bear matches TradingView. Do **not** reopen RSX/Everget unless a real regression. **FEATURE-TAPE-RSX-REGEN-1 ✅ closed.** **TV-BULL-QUARANTINE-1 ✅ closed.**

**Parked (found on the RSX audit path — not ONE-BRAIN):**

| ID | What | Why later |
|----|------|-----------|
| **VOLUME-INGEST-1** | Semantic fixtures + census. Not broadly V-poisoned. | Consumed by VOLUME-TRUTH-RECOVERY-1. |
| **VOLUME-TRUTH-RECOVERY-1** | **FROZEN.** Producer green. 1265 assigned. | Do not extend. |
| **VOLUME-SOURCE-ARBITRATION-1** | **FROZEN STEP 2. Verdict VOLUME_SSOT_GREEN_V1.** REST-family SSOT + 1m additive integrity. 18 REST_15M_PARENT + 2 REST_1M_RECONSTRUCTION. Identity `volume-truth:futures-base-v1`. 1m/3m/spot SOURCE_CONFLICT still quarantined. Report `research/volume/VOLUME-SOURCE-ARBITRATION-1.txt`. | Wozduh Style frozen (`b55b64f`). Do not silently refresh v1 from future Binance history. |
| **FRACTAL-MARKER-SSOT-1** | Unused ScanRSXMarkers / display-bar fractal scanner APIs | ✅ GREEN / FROZEN. Production owner was already `FractalFactsAt`. `RSXMarkerHit` kept for `scanRSXTVHits` TV parity. Do not reopen detector math. |
| **ATR-VALUES-FRAME-1** | `market/frame.go` still hydrates via `indicators.ATRValues` (legacy batch). ATR-TRUTH-1 left `ATRSeries` as canonical. | ATR leftover, not TV facts. |
| **ANALOGUE-MEMORY-RESEARCH-1** | Does causal nearest-neighbour information add predictive/explanatory value beyond certified facts Brain3 already has? Microscope first; neighbour-derived features only if OOF lift; retrieval infra (possibly Qdrant) only if scale/latency require it. As-of law: memory at T may use only examples whose features **and** outcomes were knowable before T. | Parked until Wozduh / opportunity vocabulary / Brain3 maturity. No socket in HEAD. First experiment: in-memory kNN on a certified matrix — not Qdrant. |
| **FEATURE-TAPE-RSX-REGEN-1 ✅ closed** | analysis:v2 four-column tape regenerated with `DumpFeatureTape`. | Consumed by RESEARCH-DATASET-1. Do not reuse analysis:v1 tapes. |
| **TV-BULL-QUARANTINE-1 ✅ closed** | Visual Bull/Bear match TV after ONE-BRAIN. | Features eligible; old `analysis:v1` tapes must not be reused. |
| **TV-HIGHESTBARS-TIE-1** | Stage 3 leftover: possible TV builtin `highestbars` vs Go newest-wins on equal RSX. | Do **not** mix into ONE-BRAIN. Reopen only with real TV Data Window evidence of a mismatch after the Bear is published. |

Do not start a generic “indicator certification framework.”

**MICRO-IDLE-1 ✅ closed (not worth implementing).** Idle ChartOnly, no micro charts: five child reducers + unused Frame ticks ≈ **6µs per 1s parent** (~6µs CPU per wall-clock second). Forming ticks dominate count (6000 forming / 505 closed per 1200 parents); that is OHLCV + empty DAG skip, not Jurik. Sleeping that path is not worth a second lifecycle. Reducers and sparse tip stay frozen.

**Parked (keep intentionally — not DAG-DEMAND):** Wozduh SaveState while asleep; wake under Frame lock; 1024 IIR epsilon; legacy `finiteOrZero`; unfiltered WS = Wozduh all.

**Product later (do not mix in):** #69 S6/69D, DATA-1B, #81 P1/P2, #82 FE calendar snap, fib/drawings. **POLICY-SIMULATOR-1** after Strategy Book + ExecutionPolicy (not a renovation of the deleted Falcon backtester).

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

### Owner: future ExecutionPolicy

May use canonical ATR for stops/sizing with a **different** ATRSpec than TargetSpec. Sharing is an optimization when spec+state match.

---

## BRAIN-V2 production ledger (Sep 2026)

**Product law:** KEEP V1 ≠ SUPPORT V1. Logistic four-column experiment is frozen historical evidence. Production architecture is designed as if **Brain V2 is the first production brain**. No production arrow to `FeatureEvaluator` / `feature-tape-v1` / signal-9 / H=24 OOF.

**FEATURE-SPEC-2 ✅ frozen** `0c54848` (docs `efc3408`). Do not reopen.

**BRAIN3-FOUNDATION-1 ✅.** Isolated `brain3` + `ml` portable eval.

**BRAIN3-METALABEL-1 ✅.** Spec1 one archive OOF (4227).

**SETUP-QUALITY-CURVE-1 ✅.** Fold-local P(TP) curves. TIMEOUT_FILTER_RANKING.

**SETUP-PROBABILITY-DECOMPOSITION-1 ✅.** a strong / q FLAT vs causal resolved prior.

**TP-STOP-INFORMATION-FORK-1 ✅.** Locked binary probe. Verdict **FACT_GAP_SUPPORTED**. HARD STOP. Next **TP-STOP-FACT-GAP-1** (target-side room to +2R). Do not retune CatBoost. Do not second-ignition yet.

### Chapter sequence (do not skip causal prerequisites)

| # | Chapter | Job | Not this chapter |
|---|---------|-----|------------------|
| 1 | **FEATURE-TAPE-2** | `FeatureRuntime2` + `feature-tape-v2`. One forward pass, three native streams. | Labels, CatBoost, live host, Frame DAG, V1 dump |
| 2 | **LABEL-SET-C** ✅ `ce3e542` | Same first-passage + 1m dual-hit engine; Target C (H=72, U=L=2.0). Native Tape2 door. | New label math |
| 3 | **RESEARCH-DATASET-C** ✅ `151e530` | Tape2 + LabelSet-C lockstep on `At`. In-memory `ResearchRow2`. | Statistics / training |
| 4 | **VALIDATION-PLAN-C** ✅ `151e530` | `CompileValidationPlan`, TargetH=72. New digest. | New compiler; reuse H=24 folds |
| 5 | **OOF-MATRIX-C** ✅ | Generic matrix of X[64] + y + C folds. | Feature-specific OOF logic / CatBoost |
| 6 | **CATBOOST-BRAIN-1** ✅ | Learner sees only X, y, folds. Portable dump + Go logits. | RSX/HTF/SQL/patterns inside the model |
| 7 | **DECISION-RESEARCH-C** ✅ `018309b` | Gate A–C on frozen CatBoost OOF logits. NOT_ELIGIBLE. | Finalization; rescue via new β/grid/target |
| 7b | **SIGNAL-DIAGNOSTICS-C** ✅ MIXED | Read-only D-tail / baseline / du=1 autopsy on paid OOF. | Ticket geometry, SHAP, new CatBoost |
| 7c | **CANDIDATE-GEOMETRY-1** ✅ | Event path geometry; no TargetSpec. **NO TARGET SELECTED.** | SETUP-TARGET-1, CatBoost, combinations |
| 7d | **STRUCTURAL-STOP-1** ✅ | Fractal-pivot stop v0 + overlay. **NO TARGET SELECTED.** | CatBoost, +2R freeze, pivot alternatives |
| 7e | **MOVE-POTENTIAL-1** ✅ | R-space path on frozen STOP-2. **+2R earned as first mature-move hypothesis.** | H change, EV, CatBoost |
| 7f | **SETUP-TARGET-1 / SETUP-LABELSET-1** ✅ | LONG +2R labels; MOVE-POTENTIAL +2R MATCH. | CatBoost, FeatureSpec3, SHORT |
| 7g | **SETUP-VALIDATION-1** ✅ | Event-scale year folds; **2026 HOLDOUT**; H72 embargo. Identity includes wall digest. | Dataset, CatBoost, 2026 as OOF |
| 7h | **SETUP-DATASET-1** ✅ | DEV join 66; copy STOP-2 geometry; exact Tape2 At. N=6644. | CatBoost, FeatureSpec3, 2026 rows |
| 7i | **BRAIN3-FOUNDATION-1** ✅ | Isolated learner package; outer-fold join; width-generic portable. | Archive CatBoost, Python fitter, inner tails |
| 7j | **BRAIN3-METALABEL-1** ✅ | Spec1 causal CatBoost OOF + prior logloss. | Quality curves, 2026, final model, CatBoost grid |
| 7k | **SETUP-QUALITY-CURVE-1** ✅ | Fold-local P(TP) coverage tables. TIMEOUT_FILTER_RANKING. | Strategy, 2026, second ignition, CandidateSpec |
| 7l | **SETUP-PROBABILITY-DECOMPOSITION-1** ✅ | a vs q on frozen OOF. a strong, q FLAT. | Features, CatBoost, TV-div ignition |
| 7m | **TP-STOP-INFORMATION-FORK-1** ✅ | Locked binary probe. FACT_GAP_SUPPORTED. | Feature design, CatBoost grid, 2026 |
| 8 | **FINALIZATION-C** | Only if ELIGIBLE (this path is closed until a new eligible hypothesis). | Premature global recipe or final fit |
| 9 | Holdout / live | Sealed holdout after finalization. Live host later. | Holdout because CatBoost exists |

Do not present V2 success as an ablation vs logistic-v1 unless a separate predeclared experiment exists.

### FEATURE-TAPE-2 design freeze (implement in that chapter)

**Split (one chapter, three objects):** FeatureSpec2 = meaning; FeatureRuntime2 = causal state; feature-tape-v2 = durable provenance.

**API firewall:** `DumpFeatureTape2(spec FeatureSpec2, src15, src1h, src4h)`. Never `(plan, RSXSettings, []Kline)`. Runtime constructed from bound AnalysisRecipe (RSX14 / signal **14** / hlc3 / TV90 / analysis:v2). No `ResearchRSXSettings`, `FeatureHistoryBars`, `BindFeatureEvaluator`, `validateTape1ASchema`, `feature-tape-v1`.

**Packages:** `forecast/` identity + tape-v2 IO + `FeatureVector2 [64]float64`; `indicators/` + existing RSTV/ATR owners (wrap, do not reimplement); `market/` FeatureRuntime2 + dump host (sources, continuity, CloseTime merge). `forecast` does not import `market`. No Go `research/featuretape2` package. Python `research/` does not compute the 64 values.

**Runtime:** created once; `Update4h` / `Update1h` / `Update15m` (or equivalent); no DB inside runtime; no labels. Thin NativeRSXContext ×3. Primary adds pivots, 50-cross, ATR, naive H=72 price windows, four O(1) pattern automata. HTF exposes exactly 11. Not a mega-engine for every future FeatureSpec — reusable **primitives**, versioned **Runtime2** assembly. FeatureSpec3 may be Runtime3.

**Scheduler (invariant):** at canonical `CloseTime` T (`data.BarCloseTimeMs`, not spoken “11:00”): apply 4h@T, then 1h@T, then 15m@T, then emit. Host owns merge; runtime fail-closed `latestHTF.CloseTime <= primary.CloseTime`.

**HTF cache:** hold-last-state between native closes is legal. Replacing cache with a newer closed NotReady row ⇒ primary NotReady. Skipping a newer closed row / using previous Ready when the latest is NotReady or missing ⇒ illegal (gap abort if expected bar absent).

**HistoryDemand:** only warmup authority. Finite 90 = windows/ages/TV lookback, **not** IIR start. IIR from `ResearchSourceStartMs(MarketKey)` per TF. Native 15m / 1h / 4h; never 15m aggregation.

**Provenance:** SourceRangeDigest = actually consumed prefix: 15m start→last primary At; 1h/4h start→latest HTF row with CloseTime ≤ last primary CloseTime. Future SQLite rows after T do not enter the digest. Stable source cut (snapshot or hash-before/after). Do not change WAL/MCP.

**Rows:** one row per primary closed 15m bar. Ready ⇒ Values `[64]` finite, no NaN repair. NotReady ⇒ reason only (`PRIMARY_WARMUP` / `HTF_1H_WARMUP` / `HTF_4H_WARMUP` / `PRICE_NOT_READY`); no fake zero vector. Unexpected gap ⇒ abort `SOURCE_GAP`. Typed blocks encode to Spec-2 order (20+6+16+11+11); offset must equal 64. No FeatureID string switch on the hot path.

**Determinism:** double generate ⇒ identical ContentDigest and `Float64bits`. Single-thread per tape; parallelize across symbols later.

**Snapshot/Restore:** do **not** reopen frozen `RSTVState`. Tape-2-local state may be trivially copyable for fork tests. No snapshot framework.

**Live:** design `UpdateClosed` so a later host can share math. Do **not** build live subscriptions, forming bars, or recovery in Tape-2. Production binary must not import tape writers / trainers.

### V1 quarantine (later, not Tape-2)

**Owner: LEGACY-V1-QUARANTINE-1** — only if the import graph still creates real friction after the CatBoost live path exists.

KEEP frozen evidence (tapes, OOF, logistic, Decision-Research-1). Do not modernize V1. Do not add `FeatureEngine` V1/V2 interface or `if plan.Version` on the production path. Optional later: rename legacy cmds, docs HISTORICAL / NOT PRODUCTION, package move only if linker/startup still reach V1.

### Parked (not Brain V2 now)

VOLUME-INGEST-1 is **open CASE C** (see parked RSX-audit table above), not a Brain-V2 item. LightGBM challenger; learned pattern mining; V1 vs V2 comparison study; feature-importance theater; generic multi-brain / feature plugin / snapshot frameworks.

**Later optimization — do not build in DATASET-C / VALIDATION-PLAN-C / OOF-MATRIX-C / CATBOOST-BRAIN-1.** Two real consumers first; extract the stable abstraction second.

| ID | What | Trigger | Not now |
|----|------|---------|---------|
| **CANDIDATE-UNIVERSE-1** | LabelSet identity today binds Tape2 `PlanDigest` + `ContentDigest` + `At[]`. That is strict and correct for Brain V2. FeatureSpec3 (same 15m candidates / 15m+1m truth / Target C, different HTF or columns) would force a new LabelSet file even if first-passage math is identical. Eventual shape: candidate-universe digest + TargetSpec → LabelSet; FeatureTape identity stays on Dataset/OOF for `X`. | Second sensory tape or FeatureSpec3 that actually shares the primary candidate population | `CandidateUniverse-v1` type, public FeatureTape interface, LabelSet-v3 just to drop ContentDigest |
| **MODEL-SOCKET-1** | Durable model-facing socket is OOF-MATRIX (`At`, `X[N]`, `Outcome`, fold ranges in the header). CatBoost / LightGBM / NN each own a trainer. | Second real model family consuming the same matrix | Go `Brain` interface, trainer plugin bus, Model registry |
| **MODEL-IDENTITY-LAYERS-1** | CATBOOST-BRAIN-1 mixed hypothesis + execution + vendor `get_all_params` into one `CatBoostSpec1`. Frozen `cecc5a3` stays. Next family binds ModelSpec + ExecutionProfile + RunWitness separately. | Second model family, second trainer environment, or UI model editor | Splitting CatBoostSpec1 now; UI showing 47 CatBoost internals; double full train as everyday automation |
| **RESEARCH-AUTOMATION-1** | MATCH / GENERATE / REFUSE over frozen identities (FeatureSpec → Tape → Target → LabelSet → Dataset → ValPlan → OOF → ModelSpec → Model → decision). UI later **selects** those specs; it does not own H / U/L / formulas. | After RESEARCH-WINDOW-1 / next hypothesis | Orchestrator that reruns the whole stack; UI that duplicates Target/Feature math |
| **RESEARCH-WINDOW-1** | `ResearchWindow` = candidate `[StartAt, EndAt)`. Does not reset IIR/source. Scout ≠ qualification (never ELIGIBLE). Reuse parent X/y when FeatureSpec+Target unchanged. New FeatureSpec: replay from source start, emit in window. One scout run; one full qualification run; second full run only if ELIGIBLE; MATCH = 0. Planner never uses outcomes. Contiguous market-clock only. See **RESEARCH-SCALE-1**. | Before next hypothesis sweep (FeatureSpec3 / CatBoostSpec2 / second coin), after DECISION-RESEARCH-C | PreviewMode digest lie; IIR restart at window start; random row subsample; SplitStrategy; changing CatBoostSpec1 in place; always double-train |

### Owner: FEATURE-TAPE-2 ✅

Implemented: FeatureRuntime2 + feature-tape-v2; DumpFeatureTape2(spec, 15m, 1h, 4h); HistoryDemand/IIR from ResearchSourceStartMs; consumed-range SourceRangeDigest; equal-CloseTime HTF-before-primary; no FeatureEvaluator. Frozen `61d5ca0`.

### Owner: LABEL-SET-C ✅

Native `GenerateLabelSetFromTape2` / `DumpLabelSetFromTape2`. Canonical core `buildLabelsFromCandidates`. Format `label-set-v2`. New Target-C slot. No new label math. Frozen `ce3e542`.

### Owner: RESEARCH-DATASET-C ✅ / VALIDATION-PLAN-C ✅

Native Tape2 dataset door + shared partition core; V1 `BuildResearchDataset` stays a legacy door. In-memory join only. `ResearchRow2` keeps `FeatureVector2 [64]`. Exclusive partition: FeatureNotReady first. `ResearchValidationPlanC` + `CompileValidationPlan` on trainable `At[]`. Market-time H=72. Frozen `151e530`.

### Owner: OOF-MATRIX-C ✅

Native Tape2 door `GenerateOOFMatrixFromTape2`. Shared generic assembler/writer. Format `oof-matrix-v1`. Development-only X/y/fold socket — not OOF logits. Holdout/seam absent. Frozen `307b5e8`.

### Owner: CATBOOST-BRAIN-1 ✅

No Kline/RSX/TV/ATR/HTF/pattern/DB on the trainer path. Python fits only. Go owns portable-catboost-v1 and official OOF logits. Frozen `cecc5a3`.

### Owner: DECISION-RESEARCH-C ✅

Causal fold-local projection + frozen DecisionContract on CatBoost OOF logits. NOT_ELIGIBLE. No global recipe/evidence. Frozen `018309b`. Do not start FINALIZATION-C.

### Owner: FORECAST-RUNTIME / live brain host

Later: same FeatureRuntime2 + portable-catboost + existing β/rank + DecisionContract. Reconstruction of live IIR vs research genesis still undecided (do not claim 1024-bar live replay equals 2019→2026). Chart TargetBarrier remains a separate owner.

### Owner: future FeatureSpec3+

New certified fact / TF / named pattern ⇒ new FeatureSpec version + native demand. Same tape primitives / later same CatBoost host / same decision architecture. No indicator-specific decision knobs.

---

## NEXT (priority)

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **76** | **ScoreNodes → Forecast engine** | 🟡 **Brain 3** | **FORK-1 done (FACT_GAP_SUPPORTED).** Next TP-STOP-FACT-GAP-1. Do not reopen geometry. |
| **93** | **DAG-DEMAND-1** — unused TF analytical CPU (RSX/facts/ZZ) | ✅ frozen `0837c77` | ChartOnly unused 1s–45s: 0 Jurik/ZZ/TV/Fractal/ZZ-col Updates. |
| **94** | **MICRO-IDLE-1** — unused 5s–45s reducer/forming fanout | ✅ closed | Measured ~6µs/1s parent for five unused children. Not worth implementing. |
| **67** | **Closed-bar Boundary + Viewport Tip** | ✅ | ADR-009 Cap + ADR-010 viewport forming tip (TV Model 2). Engine identity proven. F5 handoff = OVERWRITE same open |
| **84** | **RSX settings SSOT (B0)** | ✅ | ADR-012: engine owns config, default hlc3, autosave `rsx_settings.json`, dumb menu POST pipe |
| **85** | **ChangeImpact + Viewport (B1)** | ✅ | ADR-013/014: classify impact before Set*; soft indicator paint; debounce/Abort/generation |
| **86** | **Projection continuity (ADR-015)** | ✅ **B2.1+B2.2** | Soft `applyProjection`; projector APPEND + **OVERWRITE** same-open tip. ADR-015 probe skips heal/new-bar |
| **87** | **Replay Lifecycle Ownership (ADR-016)** | ✅ | Frame `replayStreamingLocked`: closed→forming; never commit forming tip. History Cap stays closed-only |
| **88** | **Timeline Publishability (ADR-017)** | ✅ **B3.0** | Exact closed-gap fill before pending flush; publishable only if Frame contiguous. Buffering UX separate |
| **89** | **TimelineRecovery UX (ADR-018 + TIMELINE-RECOVERY-STATE-1)** | ✅ frozen | Current `timeline_state`; reconnect ≠ HEALING; recovery ends in `replaceMonolith`. Live-certified 16:26 +08. |
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
| **8** | Falcon-era Qdrant / `vector_db` | ✅ | **QDRANT-REMOVE-1:** deleted; was never wired in `main`. Analogue retrieval is **ANALOGUE-MEMORY-RESEARCH-1** (parked), not a veto in front of execution. |
| **64** | Navigators full `ReplayDAGKlines` each request (CPU) | 🟡 | Later: live HistoryBus tail |

---

## Trading stack (paused — ChartOnly)

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **35** | DAG → TradeManager wiring | ⏸ | Re-enable only with `ENGINE_MODE=live` + new strategies |
| **36** | TradeIntent wire contract | ⏸ | Type does not exist. `ScoreDecision` was deleted SCORE-WIRE-1. Do not rename `DirectionalIntent` → TradeIntent. |
| **37** | Execution gate `isClosed` only | ⏸ | TickLiveCh frozen in ChartOnly |
| **38** | Risk/settings SSOT parity | ⏸ | `execution.RiskManager` deleted SOCKET-CLEAN-1; revisit with first real ExecutionPolicy consumer |

---

## Frontend / UX polish

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **79** | Self-heal `loadDashboard` resets camera to fresh | 🟢 | Wire `viewportAnchor` on gap-heal/reconnect |
| **49** | Active Driver / slave scroll (Shot 6B) | ✅ **ADR-021 P0–P3** | TimeCamera + Crosshair + **InteractionController** (ADR-024); ChartAdapter = LWC adapter only |
| **35** (charts) | Phase 8B annotations UI on prepend | 🔜 | `applyUniversalAnnotations` |
| **29** | Backtest history bypasses Projection | ✅ closed | Falcon-era backtester amputated; SLICE-1 GREEN / FROZEN. |
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
