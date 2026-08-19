# Track B — Cache Lifetime · Step 3 Report

**Status:** Implemented. Stop here (no validation in this stage).  
**Date:** Jul 2026  
**Frozen plan:** [`TRACK_B_STEP3_PLAN.md`](TRACK_B_STEP3_PLAN.md)  
**Frozen laws:** [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md), [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md)

---

## Runtime law

> **Exploration events must never be interpreted as contraction events.**  
> Retention lifetime is governed only by pressure or explicit world replacement — never by ordinary exploration.

Exploration events (examples): VIEW narrowing · fetch completion · successful prepend · successful append · projection merge.

Steps 1–2 preserved: Mutation Set same-op immunity · Retained Neighborhood absorb / cross-op protect / world-replace reset.

---

## Implementation summary

| Change | Purpose |
|--------|---------|
| Soft `applyProjection` captures RN bars before replace; restores any omitted RN bars after | Projection merge must not amputate RN |
| `_absorbIntoRetainedNeighborhood` documented expand-only | Exploration never shrinks RN bounds |
| `_clearRetainedNeighborhood` only on commit-paired / `clear` / empty wipe | World replacement remains the Lifetime reset |
| `_enforceBudget` commented as existing pressure trigger | Exploration may expand; pressure may drop only outside VIEW ∪ Mutation ∪ RN |

No new services, APIs, managers, timers, or layers. TimeCamera / Paint unchanged.

---

## Affected paths

| Path | Behaviour |
|------|-----------|
| `prependMonolith` | Absorb Mutation → RN; pressure may drop outside RN; RN bounds never shrink |
| `appendTick` (new bar) | Same |
| Soft `applyProjection` | Restore missing RN bars; absorb; pressure outside protected sets |
| `commitPaired` replace | Clear RN; world replace may prune |
| VIEW narrow / fetch-without-merge | No store contraction (no RN clear/shrink path) |

---

## Tests

[`web/columnar-store_budget_test.js`](../web/columnar-store_budget_test.js) — Track B Step 3 block:

- VIEW narrowing does not contract RN  
- Fetch completion alone does not contract RN  
- Successful prepend does not contract prior RN (bounds expand-only; members retained)  
- Successful append does not contract prior RN  
- Soft projection merge restores omitted RN bars; RN bounds never shrink  
- Explicit world replacement clears RN  

Prior Step 1–2 and Working Set budget tests remain green.

---

## Lifetime score after Step 3 (honest)

| ID | Status |
|----|--------|
| L1, L3–L6 | **Pass** (maintain) |
| **L2** | **Pass** (exploration ≠ contraction) |
| **L7** | **Partial** (not claimed Pass) |
| C1 | **Pass** (narrow does not collapse RN) |
| C2–C3 | Maintain Pass |

**Lifetime headline: ≈ 6 / 7.** Do not claim L7.

Working Set / Mutation Set / Retained Neighborhood: preserved.

---

## Remaining gaps

- L7 / long-roam wall scenarios (not claimed here)  
- Product residuals (RESET_LIVE, capture-null) outside this step  
- Hull vs sparse RN shape (optional polish — not a new Lifetime responsibility)

Lifetime category model (Mutation → RN → guaranteed RN lifetime) is complete for Track B. No further Lifetime abstraction.

---

## Stop

Step 3 implementation only. No validation. No Step 4. No repair proposals.
