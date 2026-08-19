# Gate S6 — Lifetime / Product Boundary

**STATUS:** PASS (repair implemented)  
**Kind:** Audit FAIL → approved repair → acceptance green. S1–S5 not reopened.

---

## HEADLINE

P-01 artificial history wall from **commit-paired accept → TARGET prune** is closed. `applyProjection` with `commitPaired: true` now accepts the full monolith (skip `_enforceBudget` for that accept only). HARD_CAP/TARGET numeric values unchanged; preserve-paired pressure / RN / Lazy Contract unchanged; EOF still only from authoritative `hasMore === false`. Browser LWC/FPS at large VIEW remains **UNKNOWN** (deferred Cap benchmark — out of S6 scope).

---

## Implementation (exact change)

**File:** `web/columnar-store.js` — `applyProjection`

When `options.commitPaired === true`:

1. Replace series from payload (unchanged).
2. Clear Retained Neighborhood (unchanged).
3. **Return without calling `_enforceBudget`** — no TARGET/HARD_CAP truncation on world accept.
4. Soft / preserve-paired paths still restore RN bars, absorb Mutation, and call `_enforceBudget` as before.

`config.js`: `STORE_BUDGET_TARGET=12000`, `STORE_BUDGET_HARD_CAP=16000` — **not modified**.

---

## Acceptance evidence

| Criterion | Result |
|-----------|--------|
| commit-paired payload `N > HARD_CAP` retained in full | **PASS** (`columnar-store_budget_test` S6 block: N=200 > 120) |
| oldest/newest = payload oldest/newest | **PASS** |
| `hasMore` not forced to EOF by accept | **PASS** (`snapshot().meta.hasMore`) |
| RN cleared on commit-paired | **PASS** |
| preserve-paired over HARD_CAP still Working-Set-safe | **PASS** (barCount=200 with VIEW∪RN∪Mutation) |
| HARD_CAP/TARGET values unchanged | **PASS** |
| No CameraCommit from pressure | **PASS** (S5 / time_camera regressions green) |
| Wave 1–3 / S4 paint / budget preserve paths | **PASS** (suite below) |

### Tests run

```text
node web/columnar-store_budget_test.js          OK  (S6 + Track A/B)
node web/chart_compositor_extract_window_test.js OK  (S4)
node web/wave1_invariant_test.js                ALL PASS
node web/wave2_pending_intent_test.js           ALL PASS
node web/wave3_completion_outcome_test.js       ALL PASS
node web/time_camera_test.js                    ALL PASS
```

Obsolete expectations that asserted commitPaired → TARGET prune were updated to retain-full (constitution Rule 8).

---

## Remaining limitations (not S6 FAIL)

1. **Browser capacity** at 50k–200k VIEW (LWC/FPS/heap) still UNKNOWN — Cap numbers deferred.
2. **Preserve-paired** growth can still exceed HARD_CAP via RN (by design); Emergency/Capacity eviction policy unspecified.
3. **Capture-null VIEW** residual closed SAFE — see `WORKING_SET_CAPTURE_NULL.md`.
4. Boot sets `historyHasMore` from **wire** `columnar.hasMore`, not a store getter — pre-existing; S6 does not alter EOF wiring. Store `_meta.hasMore` is set from payload on accept.

---

## Prior audit (historical)

The original FAIL audit content (limit catalog, Node probes showing commit-paired → 12k, preserve growth → ~34k) remains valid as pre-repair evidence. Post-repair: commit-paired over HARD_CAP retains full payload; the TARGET amputate path on that accept is gone.

See also: `docs/WORKING_SET_S6_REPAIR_PLAN.md` (approved plan; now DONE).

---

## STOP

S6 PASS. Do not proceed to Cap tuning, browser bench, RESET_LIVE, ReplayDAG, or LOD from this gate.
