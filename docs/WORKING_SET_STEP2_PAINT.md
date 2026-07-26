# Step 2 — Working Set → Paint (WS-04)

**Status:** Complete (Track A).  
**Kind:** Paint coverage audit + minimal repair.  
**Not:** HARD_CAP product wall, cache lifetime, ReplayDAG, RESET_LIVE, LOD, performance.

**Contract:** Paint must always contain the committed VIEW (WS-04). Soft buffer may include more; must never exclude VIEW.

---

## Paint coverage table

| Paint path | Uses `extractWindow`? | Before Step 2 | After Step 2 | WS-04 |
|------------|----------------------|---------------|--------------|-------|
| Full repaint (`flush` → `_flushFull` → `applyFullData`) | Yes | Tip-tail 15k | VIEW-covering window (or full if no VIEW) | Pass |
| Prepend repaint (`flush` → `_flushPrepend` → `applyFullData`) | Yes | Tip-tail 15k | Same | Pass |
| Projection / settings full paint | Yes (via `mode: 'full'`) | Tip-tail | Same | Pass |
| Indicator soft paint (`_flushIndicators`) | Yes | Tip-tail | Same | Pass |
| Delta observation (`_flushDelta` → `_observeShadowWorld`) | Yes | Tip-tail | Same | Pass |
| Delta candle write (`applyDelta`) | No extract | Tip update only | Unchanged (no series truncate) | Pass |
| `snapshotToStoreData` helper | No extract | Pass-through | Pass-through | N/A |

**ChartAdapter / TimeCamera:** unchanged ownership. Adapter still receives compositor-prepared `storeData` only.

---

## Repair (minimal)

`ChartCompositor.extractWindow(snapshot, limit, viewOpts)`:

1. Resolve VIEW indices from `viewFromSec` / `viewToSec`.
2. Keep size = `min(n, max(softLimit, viewLen))` — expands when VIEW > soft limit.
3. Place window so **VIEW ⊆ paint slice** (slack padded around VIEW when possible).
4. If VIEW unknown → return **full snapshot** (never tip-tail an unknown VIEW).
5. Annotations filtered by painted time range (not `slice(-limit)` tip-tail).

`flush` / `_flushIndicators` / `_flushDelta` call `capturePaintViewTimes` (live logical range × snapshot times) before extract.

---

## Acceptance

| Item | Result |
|------|--------|
| S4 | **Pass** (paint represents committed VIEW) |
| E3-02 | **Resolved** |
| No paint path truncates VIEW | Proven by extract policy + tests |
| No tip-tail assumption | Tip-tail removed as default amputator |

---

## Scorecard (honest)

| ID | After Step 1 + completion | After Step 2 |
|----|---------------------------|--------------|
| S1 | Pass (retention) | Pass |
| S2 | Pass (retention) | Pass |
| S3 | Pass (retention) | Pass |
| S4 | Fail | **Pass** |
| S5 | Fail | Fail (pressure / product wall) |
| S6 | Fail | Fail (P-01 HARD_CAP UX) |
| S7 | Fail | Fail (residual discontinuity product) |

**Gate 1 progress: 4 / 7**

---

## E3 findings

| ID | Status |
|----|--------|
| E3-01 | Resolved (Step 1 + completion) |
| E3-02 | **Resolved (Step 2)** |
| E3-03 | Open (HARD_CAP product) |
| E3-04 | Resolved (retention) |
| E3-05 | Partial → improved (paint no longer tip-amputates); residual if S6 wall remains |
| E3-10 | Contract frozen; runtime partial until S5–S7 |

---

## Remaining failures (honest)

- S5–S7 / E3-03: HARD_CAP still a product-facing retention trigger (out of Step 2).
- Capture-null paint paints **full** store (WS-04-safe; may be large — not tuned here).
- UX Gate C (U1–U7) not claimed.

---

## Files changed

- `web/chart-compositor.js`
- `web/chart_compositor_extract_window_test.js` (new)
- `web/index.html` (cache bump)
- `docs/WORKING_SET_STEP2_PAINT.md` (this file)
- `docs/HISTORY.md` / `docs/ARCHITECTURE.md` (index)

**Stopped.** Do not start S5 / HARD_CAP / lifetime work in this step.
