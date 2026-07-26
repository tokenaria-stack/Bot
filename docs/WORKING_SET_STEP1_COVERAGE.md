# Step 1 Completion Audit — Working Set Retention Coverage

**Status:** Complete (Track A).  
**Kind:** Coverage classification + preserve-paired closure.  
**Not:** viewport paint (Step 2 / WS-04), ReplayDAG, RESET_LIVE, APIs, redesign.

**Contract goal of Step 1:** Prune must not invalidate the committed VIEW (WS-01 / WS-02 / WS-03).

---

## Classification rules

| Class | Meaning |
|-------|---------|
| **Preserve-paired** | Mutation must keep the user’s current VIEW. Must supply VIEW bounds into `_enforceBudget`. |
| **Commit-paired** | Explicit CameraCommit / intentional world replace (TF, FreshLive, loadDashboard). VIEW omit allowed. |
| **Legacy / unreachable** | Not on the live Boot path; documented only. |

No unclassified `_enforceBudget` caller may remain.

---

## Coverage table

| Caller | Class | VIEW supplied? | Falls back to `_pruneToCount` if no VIEW? | WS-safe? | Action |
|--------|-------|----------------|--------------------------------------------|----------|--------|
| `prependMonolith` ← Boot `mergeIntoStore` | Preserve-paired | Yes (`captureStoreViewTimes`) | Only if capture null | Yes | Done (Step 1) |
| `appendTick` ← Boot `pushLiveTickDelta` | Preserve-paired | Yes (Step 1 completion) | Only if capture null | Yes | **Closed** |
| `applyProjection` ← `reloadLiveForRsxSettings` | Preserve-paired (ADR-014 restore) | Yes (Step 1 completion) | Only if capture null | Yes | **Closed** |
| `replaceMonolith` ← `loadDashboard` | Commit-paired | No (intentional) | Yes — tip hydrate | Yes (commit-paired) | Classified; comment in Boot |
| `replaceMonolith` / `applyProjection` alias | Same as caller | Same | Same | Same | — |
| `appendTick` same-bar tip update | N/A (no budget) | — | — | Yes (WS-03 tip) | None |
| `updatePlots` | N/A (no budget) | — | — | Yes | None |
| `clear` | Commit-paired wipe | — | — | Yes iff CameraCommit follows | Classified |
| `app.legacy.js` prepend/append/replace | Legacy | No | Yes | Not live path | Documented legacy |
| Hydration merge | Via Boot prepend | Yes | — | Yes | Done |
| Delta / Adapter `applyDelta` | No store prune | — | — | N/A | None |

**Store budget call sites (only three):** `prependMonolith`, `appendTick` (new bar), `applyProjection`.

---

## Remaining contract gaps (retention only)

| Gap | Status |
|-----|--------|
| Preserve-paired Boot paths without VIEW | **Closed** |
| Capture null → legacy `_pruneToCount` | Residual risk if LWC range unavailable; not a missing classification. Prefer fail-soft over inventing VIEW. |
| Product HARD_CAP as UX wall (S6) | Out of Step 1 — acceptance Gate later |
| Tip-tail paint (S4 / E3-02) | **Step 2** — not retention |

---

## Verdict

> **Working Set retention layer complete** for all classified live Boot prune paths.

Preserve-paired mutations supply VIEW. Commit-paired mutations are explicit. Legacy is documented.

**Proceed to Step 2** (viewport-centered paint / WS-04) only after review of this audit.

---

## Mapping

| | IDs |
|--|-----|
| Contract | WS-01, WS-02, WS-03 |
| Acceptance | S1, S2, S3 (retention; paint still open) |
| Evidence | E3-01, E3-04, E3-05 (retention side) |

Scorecard Gate 1 still blocked by **S4** (and S5–S7 product/paint). Retention alone ≠ Track A done.
