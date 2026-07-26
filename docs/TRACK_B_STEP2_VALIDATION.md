# Track B Step 2 — Validation Audit

**Status:** Investigation complete.  
**Kind:** Validation only — no implementation, no redesign, no Step 3 plan.  
**Date:** Jul 2026  

**Assumes frozen:** Working Set Contract, Cache Lifetime Contract, both Acceptances.  
**Assumes implemented:** Track B Step 2 ([`TRACK_B_STEP2.md`](TRACK_B_STEP2.md)).

**Verdict:** Retained Neighborhood law is correctly implemented on classified growth paths. No new Working Set or Lifetime violations introduced. Monotonic RN growth until world replace is **expected Step 2 behaviour** (eviction unspecified), not a bug and not a smuggled Capacity policy.

**Recommendation:** **Proceed to Track B Step 3** (do not repair Step 2 first).

---

## Runtime law under audit

> Successful exploration growth expands the Retained Neighborhood by absorbing the operation's Mutation Set. Under pressure, discard must not remove members of the Retained Neighborhood. The Retained Neighborhood persists across exploration growth and is reset only by an explicit world replacement (CameraCommit / commit-paired transition).

---

## Q1 — Does every exploration growth absorb its Mutation Set into RN?

| Runtime path | Growth? | Absorb Mutation → RN? | Verdict |
|--------------|---------|----------------------|---------|
| `prependMonolith` (added > 0) | Yes | **Yes** — `_absorbIntoRetainedNeighborhood(prependTimes[0], prependTimes[added-1])` before budget | Compliant |
| `appendTick` (new bar) | Yes | **Yes** — absorb `(time, time)` before budget | Compliant (plan: uniform) |
| `appendTick` (overwrite tip) | No | No | Correct exclusion |
| `applyProjection` soft / preserve (`commitPaired ≠ true`) | Yes (series apply) | **Yes** — absorb full series span | Compliant |
| `applyProjection` / `replaceMonolith` with `commitPaired: true` | World replace | **No absorb** — `_clearRetainedNeighborhood()` then prune | Compliant reset |
| `clear()` | World wipe | Clears RN | Compliant |
| Boot `loadDashboard` | commit-paired | Clears via `commitPaired: true` | Compliant |
| Boot history `prependMonolith` | Yes | Absorb via store | Compliant |
| Boot `appendTick` / soft `applyProjection` | Yes | Absorb via store | Compliant |

**Answer:** Every classified **exploration growth** that establishes a Mutation Set absorbs it into RN before pressure prune. World replacement resets RN instead of absorbing.

**Nuance (not a fail):** RN is stored as a **time hull** `[min, max]` of absorbed Mutation Sets. Bars whose times fall inside that hull but were never themselves a Mutation Set are also protected while present. That is coarser than a sparse set-of-chunks, still **logical** (not “everything ever loaded” outside the hull), and was documented in the Step 2 report.

---

## Q2 — Can the same prune remove RN members?

**Mechanism:** `_enforceBudget` resolves `neighborhood = _viewIndexRange(_rnFromSec, _rnToSec)` and passes it to `_pruneOutsideProtected`, which `mark`s VIEW, Mutation, and neighborhood. Only unmarked indices enter `toDrop`.

| Situation | Can prune remove RN members? | Proof |
|-----------|------------------------------|-------|
| Pressure prune during/after exploration growth | **No** | Protected flags; multi-op test: chunk A survives prepend B (`timesSec().includes(aFirst)`, `barCount === 200`) |
| Same-op prune with Mutation ∪ RN | **No** | Mutation + RN both marked; Step 1 tests still green |
| `commitPaired` world replace | N/A — RN cleared first | Test: bounds `null`, then prune to TARGET allowed |
| Legacy `_pruneToCount` | Only when VIEW, Mutation, **and** RN all absent | After clear / before any absorb |

**Answer:** Under exploration pressure prune, **no**. World replace clears RN by law, then may discard former neighborhood members — intentional, not a Lifetime thrash bug.

---

## Q3 — Does RN weaken Working Set guarantees?

| WS | Effect |
|----|--------|
| WS-01…WS-03 | **No weaken** — VIEW still marked; RN is additive |
| WS-04 | **Untouched** — paint / extractWindow unchanged |
| WS-05 | **Untouched** — retention does not CameraCommit |

`keepGoal` still floors on VIEW span; VIEW indices never enter `toDrop`.

**Answer:** **No** WS regression from Step 2.

---

## Q4 — Does RN accidentally become Capacity?

| Check | Finding |
|-------|---------|
| HARD_CAP / TARGET changed? | **No** |
| Timers / hysteresis / bar-count neighborhood constant? | **No** |
| Persistent Capacity “keep N bars” product? | **No** |
| Eviction / contraction policy? | **Unspecified** (documented) |
| Side-effect: store may stay above HARD_CAP when VIEW ∪ RN leaves little eligible | Lifetime eagerness — **not** a Capacity constitution |

**Answer:** **No.** RN is Neighborhood Lifetime. Capacity still owns future numeric limits; Step 2 did not implement them.

---

## Q5 — Can RN become monotonic (grow forever)?

**Yes**, until an explicit world replacement (`commitPaired` / `clear()`).

| Classification | Fit? |
|----------------|------|
| Expected for Step 2 | **Yes** — eager expand; eviction unspecified by design |
| Implementation bug | **No** — matches approved law and plan |
| Capacity responsibility | **Yes (future)** — when/how to contract outside or redefine pressure |
| Unspecified by constitution | **Partially** — CL-03 says neighborhood **exists**; size/shape unspecified; CL-02 says contraction only under pressure (Capacity/Emergency define pressure) |

Hull + append absorb can expand RN from left history to live tip, protecting interstitial bars. Documented Step 2 consequence, not a repair trigger.

**Answer:** Monotonic until world replace — **expected Step 2**; Capacity/Emergency later for eviction; not a Step 2 bug.

---

## Q6 — Does Step 2 remove Stage E4 fetch → prune → refetch thrash?

Stage E4 chain: grow past trigger → discard off-VIEW bars **including the just-fetched exploration neighborhood** → continue roam → refetch same neighborhood.

| Layer | Status |
|-------|--------|
| Same-op discard of Mutation Set | Closed in Step 1; maintained |
| Multi-op discard of prior Mutation / neighborhood | **Closed** on covered paths (Step 2 test: A survives B) |
| Residual “wall” when eligible empty under HARD_CAP | Remains — **Capacity / L7**, not the E4 thrash cycle |

**Answer:** **Yes** for the identified Lifetime thrash (C2 / L5) on main exploration growth paths. Not a claim that Capacity walls are gone.

---

## Q7 — Honest score update

### L1–L7

| ID | Status | Notes |
|----|--------|-------|
| **L1** | **Pass** | RN subordinate to WS; additive |
| **L2** | **Partial** | RN does not clear on VIEW narrow; no dedicated shrink-on-narrow acceptance proof → not full Pass |
| **L3** | **Pass** | Discard outside VIEW + Retained Neighborhood enforced |
| **L4** | **Pass** | EOF unchanged |
| **L5** | **Pass** | Multi-op thrash closed on covered paths |
| **L6** | **Pass** | No CameraCommit from retention |
| **L7** | **Partial** | Continuity improved; Capacity ceilings can still feel like walls |

**Lifetime headline: 5 / 7 Pass** (L1, L3, L4, L5, L6). L2 Partial; L7 Partial. Matches planned ~4–5 / 7; counts as **5** full Pass.

### C1–C5

| ID | Status |
|----|--------|
| C1 | **Partial** — RN persists across narrow; not a dedicated C1 test suite |
| **C2** | **Pass** |
| C3 | **Pass** (maintain) |
| C4 | **Fail** / not addressed (true-EOF-only long roam) |
| C5 | **Partial** — pressure may discard outside VIEW+RN; full Capacity/Emergency taxonomy absent |

Working Set S1–S5: **no Step 2 regression found**. S6–S7 still need fuller Lifetime + Capacity for 7/7 Gate.

---

## Q8 — Remaining issues (classified)

### A. Lifetime

- Full CL-02 / C1 product proof (narrow-alone never shrinks retained)
- Neighborhood **shape** beyond time-hull of Mutation Sets (sparse vs hull)
- L7 residual discontinuity when pressure cannot shrink without touching RN
- C4 long roam to true EOF

### B. Capacity

- HARD_CAP / TARGET as only pressure triggers (constitution still unspecified)
- RN **eviction / contraction** under pressure (intentionally unspecified in Step 2)
- Artificial wall when VIEW ∪ RN leaves nothing eligible
- Whether append-driven hull expansion should be limited by Capacity numbers later

### C. Product

- RESET_LIVE / tip rehydration
- Capture-null VIEW residual on non-growth / legacy paths
- `app.legacy.js` replace without `commitPaired` (not main Boot)

### D. Performance

- Temporary / prolonged `n > HARD_CAP` while RN protects exploration (unmeasured)
- Non-contiguous `_gatherIndices` after protected prune (unmeasured)

---

## Evidence ledger delta

| Evidence | Delta after Step 2 validation |
|----------|-------------------------------|
| E3-05 / E4 multi-op thrash | **Closed** on covered growth paths |
| Same-op Mutation (Step 1) | **Maintained** |
| RN monotonic until world replace | **Confirmed expected**; eviction unspecified |
| RN = Capacity policy | **Disproven** |
| WS weakened by RN | **Not observed** |
| L2 / L7 / C4 | **Still open** (as classified) |

---

## Implementation fidelity

| Plan / report claim | Observed |
|---------------------|----------|
| Absorb Mutation on exploration growth | Yes |
| Protect RN under pressure | Yes |
| Persist across growth | Yes (multi-op test) |
| Reset only on world replace | Yes (`commitPaired`, `clear`) |
| No Camera / Paint / Capacity philosophy change | Yes |
| No new architectural layer | Yes — store-local bounds |

**Corrections to note (not repairs):** None required for law fidelity. Hull-vs-sparse and append-hull expansion remain **documented design consequences**, not Step 2 defects.

---

## Recommendation

**Proceed to Track B Step 3.**

Do **not** repair Step 2 first. Remaining gaps are Lifetime Step 3+ / Capacity / Product by classification — not failed Step 2 implementation.

**Stop.** No implementation. No Step 3 plan in this document.
