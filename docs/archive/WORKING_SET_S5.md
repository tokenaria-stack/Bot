# Gate S5 — Pressure ≠ Navigation

**STATUS:** PASS  
**Kind:** Evidence gate only. No HARD_CAP change. S1–S4 not reopened.

---

## Invariant

Memory pressure may reduce retention outside the Working Set, but must never become a navigation authority (no CameraCommit / VIEW change / return-to-live from pressure).

---

## Findings (S5 only)

1. `_enforceBudget` / prune mutate store columns + optional `windowMode` fact only — no TimeCamera / CameraCommit / `loadDashboard` / `setVisibleLogicalRange`.
2. `windowMode = 'history'` after FROM_NEWEST tip drop is a **Data fact**. Boot `maybeReturnToLiveFromHistory` is a no-op (Wave 1). No `HARD_CAP → FreshLive` coupling on live Boot.
3. Preserve-paired prepend paint publishes `proposePreserveViewport` (facts only). Wave1 tests: preserve failure does **not** FreshLive.
4. Delta paint after `appendTick` pressure does not propose navigation.
5. Commit-paired `loadDashboard` (`commitPaired: true` + `viewport: fresh|restore`) is intentional world replace — not an S5 violation.
6. Correction: residual capture-null can still amputate VIEW *data* (WS-01 conditional). That is retention coverage residual, not a pressure→CameraCommit path. Does not fail S5 navigation-authority proof.

---

## Classification

| Path | Class |
|------|--------|
| Boot prepend / append / soft applyProjection + budget | Preserve-paired |
| `loadDashboard` replaceMonolith commitPaired | Commit-paired |
| `app.legacy.js` budget callers | Legacy |
| Tip overwrite / updatePlots | N/A (no budget) |

---

## Evidence

- `web/columnar-store.js` — `_enforceBudget`, `_pruneOutsideProtected`, `windowMode` on droppedNewest  
- `web/boot.js` — `maybeReturnToLiveFromHistory` no-op; `pushLiveTickDelta` gates on `windowMode` (ingest only)  
- `web/chart-compositor.js` — `_publishPrependViewportFacts` → `proposePreserveViewport`  
- `web/wave1_invariant_test.js` — no loadDashboard/windowMode nav from return-to-live  
- `docs/WORKING_SET_STEP1_COVERAGE.md` — preserve vs commit classification  

---

## Tests

- `node web/wave1_invariant_test.js` — PASS  
- `node web/time_camera_test.js` — PASS (preserve ≠ FreshLive)  
- `node web/columnar-store_budget_test.js` — PASS  

---

## Scorecard

**S5: PASS**

---

## Residual (qualifies, does not reopen S5 fail)

- Capture-null VIEW on preserve-paired pressure (WS-01 conditional).  
- `windowMode=history` stops live tick ingest — product feel, not CameraCommit.  
- Artificial 16k wall / Lifetime product — next gate, not S5.

---

## NEXT

Lifetime / product acceptance gate (P-01 / artificial wall) — do not raise/remove HARD_CAP inside S5.
