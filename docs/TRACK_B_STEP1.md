# Track B — Cache Lifetime · Step 1 Report

**Status:** Implemented. Stop here for validation before Step 2.  
**Date:** Jul 2026  
**Frozen laws:** [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md), [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md)  
**Plan:** [`TRACK_B_STEP1_PLAN.md`](TRACK_B_STEP1_PLAN.md)

---

## Runtime law (implemented)

> **A successful cache growth operation establishes a temporary Mutation Set. Any prune caused by that same growth operation must not remove members of that Mutation Set.**

Applies to **growth operations**, not every function. Independent of prepend/append/projection/streaming mechanics.

### Sets

```text
Committed VIEW → Working Set (Track A) → Mutation Set (this step) → Remaining Cache (eligible)
```

---

## Growth-path classification (audit)

| Growth operation | Creates Mutation Set? | Same-operation prune possible? | Step 1 protected? |
|------------------|----------------------|--------------------------------|-------------------|
| `prependMonolith` | Yes (prepended bar times) | Yes | Yes |
| `appendTick` (new bar) | Yes (that bar’s time) | Yes | Yes |
| `applyProjection` (preserve / soft) | Yes (entire applied series) | Yes | Yes |
| `replaceMonolith` / apply with `commitPaired: true` | World replacement | Not applicable | **Excluded by design** |

Boot: `loadDashboard` → `replaceMonolith(..., { commitPaired: true })`.

---

## What changed

| File | Change |
|------|--------|
| `web/columnar-store.js` | `_pruneOutsideProtected(view ∪ mutation)`; growth callers pass Mutation Set; `commitPaired` skips Mutation Set |
| `web/boot.js` | Commit-paired hydrate flagged |
| `web/columnar-store_budget_test.js` | Mutation Set + WS non-regression + commit-paired contrast |
| `web/index.html` | Script cache bump |

**Not changed:** Camera, paint ownership, Capacity numbers/philosophy, hysteresis, HARD_CAP retune, EOF, ReplayDAG.

---

## Acceptance (honest)

| ID | After Step 1 |
|----|----------------|
| **L1** | Pass (Mutation Set law on covered growth paths) |
| **L2** | Unchanged / Fail (lazy contract on VIEW narrow — Step 2+) |
| **L3** | Partial (Mutation Set only; not global neighborhood) |
| **L4** | Pass (maintain EOF) |
| **L5** | Partial on covered growth paths |
| **L6** | Pass (no CameraCommit from retention) |
| **L7** | Partial |

| Continuity | Claim |
|------------|--------|
| **C2** | Primary claim (growth → same-op discard thrash closed on covered paths) |
| C1, C4 | Not claimed |
| C3 | Maintain WS |

**Lifetime headline: ~2–3 / 7** (not 7 / 7).

Working Set main-path: no intentional regression (VIEW still sacrosanct; Mutation Set is additive).

---

## Remaining gaps (explicitly not Step 1)

- Full CL-02 lazy contract when VIEW narrows  
- Global exploration-neighborhood / Capacity policy  
- High-water / hysteresis product  
- Capture-null VIEW hole  
- RESET_LIVE, LOD, ReplayDAG  

---

## Stop

Step 1 only. Validate before planning Track B Step 2.
