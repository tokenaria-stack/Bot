# Track B — Cache Lifetime · Step 2 Report

**Status:** Implemented. Stop here for validation before Step 3.  
**Date:** Jul 2026  
**Frozen laws:** [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md), [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md)  
**Plan:** [`TRACK_B_STEP2_PLAN.md`](TRACK_B_STEP2_PLAN.md)

---

## Runtime law (implemented)

> **Successful exploration growth expands the Retained Neighborhood by absorbing the operation's Mutation Set. Under pressure, discard must not remove members of the Retained Neighborhood. The Retained Neighborhood persists across exploration growth and is reset only by an explicit world replacement (CameraCommit / commit-paired transition).**

Step 1 Mutation Set law **unchanged** (same-op temporary protection).

### Naming

| Use | Avoid |
|-----|--------|
| Retained Neighborhood / Protected Neighborhood | Neighborhood Cache |
| Neighborhood Lifetime | Bigger cache / high-water |

---

## Sets

```text
VIEW → Working Set → Mutation Set (same-op) → Retained Neighborhood (cross-op) → Remaining eligible
```

Pressure prune protects **VIEW ∪ Mutation Set ∪ Retained Neighborhood**.

---

## What changed

| File | Change |
|------|--------|
| `web/columnar-store.js` | RN as logical `[fromSec, toSec]`; absorb on growth; protect in prune; clear on `commitPaired` / `clear()` |
| `web/columnar-store_budget_test.js` | Multi-op prepend anti-thrash; world-replace clears RN; Step 1 non-regression |
| `web/index.html` | Script cache bump |

**Not changed:** Camera, Paint, HARD_CAP/TARGET philosophy, timers, hysteresis, ReplayDAG, EOF ownership.

---

## Implementation notes / corrections

1. **Law wording** uses CameraCommit / commit-paired transition (implementation-agnostic), not only the flag name. Runtime today clears RN on `commitPaired: true` and `clear()` (Boot FreshLive / TF hydrate path).

2. **RN is logical, not “everything ever loaded.”** It is the hull union of absorbed Mutation Set time intervals from exploration growth since the last world replacement. Bars never in a Mutation Set and outside that hull remain eligible under pressure.

3. **Eviction policy unspecified.** Step 2 does not define when RN may shrink under Capacity/Emergency. Capacity retains the right to apply future limits **outside** Lifetime’s protected categories — Step 2 must not be read as a hidden Capacity implementation.

4. **Unbounded growth risk (documented, not “solved”).** Continuous roam can expand RN until VIEW ∪ RN leaves little eligible room; store may remain above HARD_CAP. Accepted Lifetime eagerness. Capacity later — not HARD_CAP↑ as the Step 2 fix.

5. **Append absorb** follows the approved plan (uniform growth law). Live tip appends expand RN rightward; combined with left prepends the hull can span tip↔history. Called out so Step 3 / Capacity do not mistake this for a finished eviction design.

6. **Soft `applyProjection`** absorbs the full applied series into RN until the next world replace — same eagerness; intentional.

---

## Acceptance (honest)

| ID | After Step 2 |
|----|----------------|
| **L1** | **Pass** |
| **L2** | Fail / soft Partial — no dedicated shrink-on-narrow path closed beyond RN persistence |
| **L3** | **Pass** — discard outside VIEW + Retained Neighborhood (category enforced) |
| **L4** | **Pass** |
| **L5** | **Pass** on covered multi-op exploration paths |
| **L6** | **Pass** |
| **L7** | **Partial** — continuity better; Capacity walls remain |

| Continuity | After Step 2 |
|------------|--------------|
| **C2** | **Pass** (primary claim) |
| C3 | Maintain |
| C1, C4, C5 | Not fully claimed |

**Lifetime headline: ~5 / 7 Pass** (L1, L3, L4, L5, L6). L2 Fail; L7 Partial. Aligns with planned ~4–5 / 7.

Working Set: no intentional regression (RN additive to VIEW + Mutation).

---

## Remaining gaps

| Gap | Layer |
|-----|--------|
| Full CL-02 lazy-contract product polish | Lifetime |
| RN eviction / contraction under pressure | Capacity (unspecified) — **not** Step 2 |
| Long-roam artificial wall when eligible empty | Capacity + L7 |
| C1 / C4 / C5 | Lifetime + Capacity/Emergency |
| Capture-null / RESET_LIVE / LOD / ReplayDAG | Product / other |

---

## Stop

Step 2 only. Validate next; do **not** begin Step 3 in this stage.
