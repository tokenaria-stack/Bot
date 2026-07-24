# Open Debts

**SSOT for:** unfinished work and NEXT priorities.  
Completed items live in `docs/HISTORY.md` — do not re-list them here.

Update this file when a debt opens, closes, or changes priority.

---

## NEXT (priority)

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **76** | **ScoreNodes** — move Score/Falcon decision graph into DAG nodes | 🔜 | Do **not** delete `market/falcon.go` until done |
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
| **69** | **MemoryBudget / WindowPolicy** | 🟡 **69A+69C done** | Bounded store + atomic prune + `windowMode` + WS/gap gates + **focal-time prune (69C)**. **69D** full sliding window + viewport-centered paint 🔜. |
| **69C** | Focal-time prune (drop side farthest from viewport center) | ✅ | `pruneDirectionFromFocal` + boot passes `ViewportManager.capture` into `prependMonolith` |
| **69D** | Full sliding viewport window + paint alignment | 🔜 | **RED FLAG:** when Store becomes viewport-centered, `ChartCompositor.extractWindow` (currently tip-tail) MUST become viewport-centered too |
| **80** | `ViewportManager.restore` 0×0 width risk (`setVisibleLogicalRange`) | ✅ | Guard + fresh `applyOptions` fallback + deferred restore; TF null-capture → fresh; ChartAdapter no-op on 0×0 |
| **81** | **Timeline Publish Gate** (reconnect heal) | ✅ | Phases A–D + P0: WS hooks, Runtime gate, forced REST@1bar, FE await `timeline_publishable`. P1/P2 (status poll / GetWindow degraded) deferred |
| **82** | **Calendar bar boundary** (`1w`/`1M` time model) | ✅ **A1+A2** | ADR-011 Cap/align/CloseTime. A2: catch-up/gap/reconcile via `NextBarOpen`/`BarStepsBetween`; `intervalSkipsKlineGapFill` removed. FE snap deferred unless runtime proves need |
| **83** | **Timestamp normalization** (`ensureUnixMillis`) | 🔜 | Split load-bounds vs persist; kill `ts < 1e12` heuristic; honest logs; clamp lookback to exchange genesis. Boot spam: `1M`×400 → 1993-02-01 mis-coerced. Does **not** block #82/#67 |

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
