# Open Debts

**SSOT for:** unfinished work and NEXT priorities.  
Completed items live in `docs/HISTORY.md` — do not re-list them here.

Update this file when a debt opens, closes, or changes priority.

---

## NEXT (priority)

| # | Debt | Status | Notes |
|---|------|--------|-------|
| **76** | **ScoreNodes** — move Score/Falcon decision graph into DAG nodes | 🔜 | Do **not** delete `market/falcon.go` until done |
| **67** | **Closed-bar Boundary SSOT** (was: IIR Tip SSOT) — one last-closed law for Frame / History / GetWindow / Replay | 🟡 | **ADR-009:** `GetWindow` → `CapKlineEndToLastClosed`. Data-plane tip mismatch closed. Continuous Live Confirm (forming tip vs TV) may remain |
| **68** | Osc fixed scale bounds (RSX/Wozduh TV-like `[-5,105]`) | 🟡 | After #67 |
| **69** | **MemoryBudget / WindowPolicy** | 🟡 **69A+69C done** | Bounded store + atomic prune + `windowMode` + WS/gap gates + **focal-time prune (69C)**. **69D** full sliding window + viewport-centered paint 🔜. |
| **69C** | Focal-time prune (drop side farthest from viewport center) | ✅ | `pruneDirectionFromFocal` + boot passes `ViewportManager.capture` into `prependMonolith` |
| **69D** | Full sliding viewport window + paint alignment | 🔜 | **RED FLAG:** when Store becomes viewport-centered, `ChartCompositor.extractWindow` (currently tip-tail) MUST become viewport-centered too |
| **80** | `ViewportManager.restore` 0×0 width risk (`setVisibleLogicalRange`) | ✅ | Guard + fresh `applyOptions` fallback + deferred restore; TF null-capture → fresh; ChartAdapter no-op on 0×0 |
| **81** | **Timeline Publish Gate** (reconnect heal) | ✅ | Phases A–D + P0: WS hooks, Runtime gate, forced REST@1bar, FE await `timeline_publishable`. P1/P2 (status poll / GetWindow degraded) deferred |

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
| **49** | Active Driver / slave scroll (Shot 6B) | 🔜 | Wheel proxy → master |
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
