# Track B Step 1 — Validation Audit

**Status:** Investigation complete.  
**Kind:** Validation only — no implementation, no fixes, no redesign.  
**Date:** Jul 2026  

**Assumes frozen:** Working Set Contract, Cache Lifetime Contract, both Acceptances.  
**Assumes implemented:** Track B Step 1 ([`TRACK_B_STEP1.md`](TRACK_B_STEP1.md)).

**Verdict:** Mutation Set law is correctly implemented on classified growth paths. No new WS or Lifetime violations introduced. Remaining failures are out of Step 1 scope by design.

**Recommendation:** **Proceed to Track B Step 2** (do not repair Step 1 first).

---

## Runtime law under audit

> A successful cache growth operation establishes a temporary Mutation Set. Any prune caused by that same growth operation must not remove members of that Mutation Set.

---

## Q1 — Does every cache growth operation establish a Mutation Set?

| Growth operation | Establishes Mutation Set? | Verdict |
|------------------|---------------------------|---------|
| `prependMonolith` (added > 0) | **Yes** — `prependTimes[0]…[added-1]` | Compliant |
| `appendTick` (new bar) | **Yes** — that bar’s time | Compliant |
| `appendTick` (overwrite forming tip) | **No** — not growth; no budget prune | Correct exclusion |
| `applyProjection` (preserve / soft, `commitPaired ≠ true`) | **Yes** — entire applied series | Compliant |
| `replaceMonolith` / apply with `commitPaired: true` | **No** — world replacement | **Excluded by design** |

Boot main path: `loadDashboard` → `replaceMonolith(..., { commitPaired: true })` — audited compliant.

**Answer:** Every **classified growth** path that may trigger same-op prune establishes a Mutation Set. Commit-paired world replace does not — correctly. Non-growth overwrite does not — correctly.

---

## Q2 — Can same-operation prune remove Mutation Set members?

**Mechanism:** `_enforceBudget` resolves `mutationFromSec/ToSec` → index range → `_pruneOutsideProtected` marks VIEW ∪ Mutation as protected; only unprotected indices enter `toDrop`.

| Path | Same-op prune can remove Mutation Set? | Proof |
|------|----------------------------------------|-------|
| prepend | **No** | Mutation times passed into `_enforceBudget`; tests: full VIEW∪Mutation → barCount stays 150; left edge = prepended[0] |
| append new bar | **No** | `mutationFromSec = mutationToSec = time`; test: tip survives with left-only VIEW |
| soft applyProjection | **No** | Mutation = full series → eligible empty when over HARD_CAP; test: barCount 150 retained |
| commit-paired replace | N/A | No Mutation Set; test: prune to TARGET 100 allowed |

**Edge:** If both VIEW and mutation opts fail to resolve (`_viewIndexRange` null), legacy `_pruneToCount` runs. Main growth callers always pass finite mutation times after successful add. Zero-add prepend returns before budget.

**Answer:** On covered growth paths, **no**.

---

## Q3 — Does Mutation Set weaken WS-01…WS-05?

| WS | Effect of Step 1 |
|----|------------------|
| WS-01…WS-03 | **No weaken** — VIEW still marked protected; Mutation is additive |
| WS-04 | **Untouched** — paint / `extractWindow` unchanged |
| WS-05 | **Untouched** — retention still does not CameraCommit |

`keepGoal = max(TARGET, viewLen)` still floors on VIEW span. Protected flags prevent VIEW indices from entering `toDrop`.

Residual (pre-existing, not introduced): capture-null VIEW → legacy prune path if mutation also absent. Growth paths now always pass mutation when they grow, so capture-null alone no longer opens same-op Mutation discard on prepend/append/soft apply.

**Answer:** **No** WS regression from Step 1.

---

## Q4 — Can Mutation Set accidentally become a Capacity policy?

| Check | Finding |
|-------|---------|
| Numeric ceilings changed? | **No** — TARGET/HARD_CAP unchanged |
| Persistent neighborhood size? | **No** — no store field; opts only |
| Permanent “keep N bars beyond VIEW”? | **No** — ends when the growth call returns |
| Soft apply Mutation = entire series | Widest same-op shield; may leave `n > HARD_CAP` until a **later** independent growth. Still operation-local Lifetime, not a Capacity constitution |

**Answer:** **No** Capacity policy introduced. Temporary overshoot above HARD_CAP after protected growth is an accepted Lifetime side-effect of Step 1, not a smuggled Capacity rule.

---

## Q5 — Can Mutation Set survive across independent growth operations?

**Proof:** No `this._mutation*` (or equivalent) on `ColumnarStore`. Mutation exists only as arguments to one `_enforceBudget` call inside the growth method.

Next growth builds a **new** temporary Mutation Set. Prior Mutation bars become Remaining Cache (eligible under pressure).

**Answer:** **No** — operation-local only. Correct.

---

## Q6 — Can repeated growth still thrash because Mutation Set disappears too early?

**Yes — expected remaining Lifetime gap.**

```text
prepend A  → Mutation(A) protected this op  → may keep n > TARGET
prepend B  → Mutation(B) only              → A eligible → may drop A
user still roaming near A                  → refetch A  → thrash resumes
```

Step 1 closes **same-operation** discard of the just-grown set (CL-05 seed / C2 partial). It does **not** implement durable exploration-neighborhood (full CL-03 / L5).

**Answer:** Multi-op thrash remains. Not a Step 1 implementation defect.

---

## Q7 — Score impact (honest)

### CL-05

| Before Step 1 | After Step 1 |
|---------------|--------------|
| Same-op growth often discarded Mutation Set | Same-op discard **closed** on classified paths |
| Multi-op neighborhood thrash | **Still open** |

**CL-05:** Fail → **Partial**.

### L1…L7

| ID | Status | Notes |
|----|--------|-------|
| **L1** | **Pass** | Additive Mutation; WS not weakened on main path |
| **L2** | **Fail** | Lazy contract on VIEW narrow — untouched |
| **L3** | **Partial** | Mutation Set only ≠ full exploration neighborhood |
| **L4** | **Pass** | EOF / discard semantics unchanged |
| **L5** | **Partial** | Same-op thrash closed; multi-op open |
| **L6** | **Pass** | No CameraCommit from retention |
| **L7** | **Partial** | Continuity improved locally; capacity walls / multi-op thrash remain |

**Lifetime headline: 3 / 7 Pass** (plus 3 Partial, 1 Fail). Aligns with planned ~2–3/7; counts as **3** full Pass.

### C1…C5

| ID | Status |
|----|--------|
| C1 | Fail / not addressed (narrow ≠ shrink) |
| **C2** | **Partial** — same-op cycle closed; continuous multi-chunk roam not |
| C3 | Maintain (WS + live growth Mutation tip) |
| C4 | Fail / not addressed (capacity wall) |
| C5 | Fail / not addressed (full protected-set under capacity pressure) |

Working Set S1–S5: **no regression claimed or found** from Step 1. S6–S7 remain Partial (need fuller Lifetime).

---

## Q8 — Remaining failures (classified)

### A. Lifetime

- Full CL-02 lazy contract (VIEW narrow must not shrink retained set)
- Full CL-03 exploration neighborhood beyond Mutation Set
- Multi-op CL-05 / L5 / C2 thrash across successive prepends
- L7 / P-02 residual discontinuity under long roam

### B. Capacity

- TARGET / HARD_CAP still the only pressure triggers (unspecified constitution)
- Post-growth `n > HARD_CAP` until next op (side-effect of Mutation protection; Capacity must later define pressure honestly)
- Artificial exploration wall when eligible room is exhausted outside VIEW∪Mutation (product feels like a wall → also L7/P-01 soft)

### C. Product

- RESET_LIVE / tip rehydration without FreshLive (pre-existing)
- Capture-null VIEW residual on non-growth or unflagged legacy paths (`app.legacy.js` replace without `commitPaired`)

### D. Performance

- Non-contiguous `_gatherIndices` after protected prune (possible; not measured; not Step 1 scope)
- Temporary store overshoot above HARD_CAP increases memory until next growth

---

## Evidence ledger delta (post Step 1)

| Evidence | Delta |
|----------|-------|
| E3-05 / same-op growth→discard→refetch | **Mitigated for same growth op** on prepend / append / soft apply |
| E3-05 / multi-chunk roam thrash | **Still open** |
| E3-03 numeric ceilings as walls | **Still open** (Capacity + L7) |
| E3-07 shrink-on-narrow | **Still open** (L2) |
| Mutation Set operation-local | **Confirmed** (no persistent field) |
| WS amputation via Mutation | **Not observed** |

---

## Implementation fidelity checklist

| Plan item | Observed |
|-----------|----------|
| Law = growth operation, not every function | Yes |
| Classification table honored | Yes |
| No Camera / Paint ownership change | Yes |
| No Capacity philosophy change | Yes |
| Tests cover Mutation + WS + commit-paired | Yes (`columnar-store_budget_test.js`) |

---

## Recommendation

**Proceed to Track B Step 2.**

Do **not** repair Step 1 first. The Mutation Set law matches the frozen Step 1 plan; proofs hold; residuals are the known next Lifetime (and Capacity) debts, not implementation bugs in Step 1.

**Stop.** No implementation. No Step 2 plan in this document.
