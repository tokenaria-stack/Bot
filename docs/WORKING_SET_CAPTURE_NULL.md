# Gate — Capture-Null VIEW Residual

**STATUS:** PASS (SAFE)  
**Kind:** Investigation only. No code. S1–S6 not reopened.

---

## HEADLINE

Capture-null on live Boot preserve-paired paths does **not** prove a WS-01 invariant failure. Prior wording that capture-null “falls back to `_pruneToCount`” is **obsolete after Track B**: growth always installs Mutation (and usually RN), so the legacy bare `_pruneToCount` branch is not reached. Product-shaped pressure with omitted VIEW cannot remove the VIEW those paths imply.

---

## PROVEN PATH (complete Boot chain)

```text
TimeCamera / LWC committed VIEW (user scrolled or FreshLive)
  → preserve mutation (prepend | appendTick | soft applyProjection)
  → Boot captureStoreViewTimes()  [may return null]
  → ColumnarStore._enforceBudget
  → view=null, mutation and/or RN present
  → _pruneOutsideProtected (NOT _pruneToCount)
  → paint / proposePreserveViewport (camera ownership unchanged)
```

Commit-paired `loadDashboard` omit-VIEW is intentional world replace (SAFE by class) — out of residual scope.

---

## CLASSIFICATION: SAFE

| Criterion | Finding |
|-----------|---------|
| 1 Committed VIEW exists | Yes on roam / live |
| 2 Mutation can exceed HARD_CAP | Yes (preserve growth / soft large apply) |
| 3 VIEW bounds unavailable | Possible (`getVisibleLogicalRange` null / invalid) |
| 4 Fallback to `_pruneToCount` | **No on Boot growth** — Mutation and/or RN present → protected path |
| 5 VIEW bars removable | **Not on product-shaped Boot paths** (see evidence) |

**Reason:** Capture-null ≠ unprotected legacy prune. Fallback that can run is `_pruneOutsideProtected` without a VIEW mark; on Boot product shapes that still cannot amputate the path’s VIEW.

---

## EVIDENCE

| Path | Why SAFE |
|------|----------|
| History `mergeIntoStore` → `prependMonolith` | Left-scroll VIEW; if range null then `ViewportManager.capture` null → `atLiveEdge=false` → `FROM_NEWEST`; drops tip of eligible, keeps left Mutation∪RN∪old-left (probe A: 0 VIEW bars lost) |
| `appendTick` after RN growth | Absorb tip expands RN to `[left, tip]` → nothing eligible (probe C) |
| Soft `applyProjection` (RSX settings) | Mutation = full payload → nothing eligible (probe G) |
| `loadDashboard` commitPaired | Commit-paired class; Boot `limit=HISTORY_CHUNK_LIMIT` (3k) ≪ HARD_CAP (16k) → flush appends do not pressure (probe Boot 3k) |
| Bare `_pruneToCount` | Only if `!view && !mutation && !neighborhood` — not Boot growth callers |

**Store-only latent (not proven live Boot):** after `commitPaired` with `N > HARD_CAP` (S6-legal), RN cleared, capture-null `appendTick`, mid-store VIEW ⊂ eligible → `FROM_OLDEST` can drop mid bars (probe D). Boot hydrate does not fetch `N > HARD_CAP` today; after paint, a visible mid VIEW implies LWC range is usually capturable. **Not a complete live violation under current Boot transport.**

**Files:** `web/boot.js` (`captureStoreViewTimes`, `mergeIntoStore`, `pushLiveTickDelta`, `flushLiveTickBuffer`, `loadDashboard`), `web/columnar-store.js` (`_enforceBudget`, `_pruneOutsideProtected`, `_pruneToCount`), `web/hydration-orchestrator.js` (merge via Boot).

**Correction:** Docs that equated capture-null → `_pruneToCount` overstated the post–Track B seam.

---

## TESTS / PROBES

- Node store probes A–G (this gate): history-left capture-null; atLiveEdge API-only; RN append; S6 mid/tip append; bare enforce; soft apply; Boot 3k flush.
- Regressions not re-run (no code change). Relevant existing: `columnar-store_budget_test.js` (VIEW/Mutation/RN), `time_camera_test.js` (preserve ≠ FreshLive).

---

## IF BUG (not applicable)

No repair plan — residual closed as SAFE on the live Boot path.

---

## RESIDUAL (Working Set)

None for capture-null → WS-01 on live Boot.

Still elsewhere (not this gate): browser Cap/FPS UNKNOWN; Emergency eviction unspecified; `app.legacy.js` non-Boot.

---

## STOP

No code. No Cap tuning / RESET_LIVE / ReplayDAG / LOD.
