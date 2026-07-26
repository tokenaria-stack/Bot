# Track B — Cache Lifetime · Step 1 Plan

**Status:** Plan only (not implemented). Ready for implementation when approved.  
**Kind:** Smallest first implementation step for Lifetime obedience.  
**Not:** code (yet), Step 2, constitution edits, Capacity/Emergency freeze, redesign.

**Frozen laws:** [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md), [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md).  
**Frozen gates:** [`WORKING_SET_ACCEPTANCE.md`](WORKING_SET_ACCEPTANCE.md), [`CACHE_LIFETIME_ACCEPTANCE.md`](CACHE_LIFETIME_ACCEPTANCE.md).

**Vocabulary:** Prefer **valid architectural boundary** over “five-year architecture.” A constitution lasts as long as its assumptions last.

---

## Step 1 runtime law (frozen for this step)

> **Bars introduced by a successful growth operation must not become immediate prune candidates as a consequence of that same growth operation.**

Independent of prepend/append/chunk/LOD terminology. Still valid if transport, replay, or storage changes.

### Sets (this step only)

```text
Committed VIEW
      │
      ▼
Working Set (WS)          ← already protected (Track A)
      │
      ▼
Mutation Set              ← newly introduced by THIS successful growth
      │
      ▼
Remaining Cache           ← still eligible for prune under existing triggers
```

| Set | Step 1 rule |
|-----|-------------|
| Working Set | Protected (must not regress WS-01…WS-05) |
| Mutation Set | **Additionally protected** from the prune caused by that same growth |
| Remaining Cache | Eligible under existing capacity triggers |

This is **Lifetime** (CL-05 / local CL-03 seed), not Capacity.

---

## Step 1 objective

Eliminate the immediate **growth → discard Mutation Set → refetch** thrash (CL-05), without weakening WS-01…WS-05.

Evidence target (E4): after growth past the capacity trigger, retention often drops bars that were **just introduced by that growth**, which still sit outside the captured VIEW; continuous exploration then reloads the same neighborhood.

---

## Scope

### In scope

1. **Growth operations that may trigger prune** (any successful bar-adding path that then calls budget enforcement — today: prepend growth and new-bar append growth).  
2. **Mutation Set protection for that prune only:** the prune caused by that growth must not remove the Mutation Set.  
3. **Working Set remains sacrosanct.**  
4. **EOF unchanged** (CL-04).  
5. **No CameraCommit** from retention (CL-06 / WS-05).  
6. **No paint / TimeCamera / Adapter behavior changes.**  
7. Tests: Mutation Set survives the same-operation prune; VIEW never pruned; WS regressions green.

### Explicitly one step

Do **not** in Step 1:

- Full lazy-contract on VIEW narrow (CL-02)  
- Global exploration-neighborhood sizing (Capacity)  
- Retune capacity ceilings as the goal  
- Hysteresis timers / high-water product  
- RESET_LIVE, LOD, ReplayDAG  

---

## Non-goals

| Non-goal | Why |
|----------|-----|
| Lifetime 7/7 | Step 1 is incremental |
| Working Set S6/S7 Pass | Needs broader Lifetime |
| Capacity / Emergency constitution | Still unspecified |
| New managers / buses | Forbidden |
| Constitution edits | Frozen |
| “Raise the cap” as the fix | Numbers ≠ CL-05 |

---

## Files likely affected

| File | Likely change (plan-level) |
|------|----------------------------|
| `web/columnar-store.js` | After successful growth, budget prune must exclude Mutation Set from candidates |
| `web/columnar-store_budget_test.js` | Anti-thrash + WS non-regression |
| `web/boot.js` | Likely none if store knows what it just added |
| `web/index.html` | Cache bump if scripts change |

Unlikely: compositor, TimeCamera, hydration EOF, server.

---

## Success criteria (objective)

| Check | Pass |
|-------|------|
| Runtime law | Mutation Set not removed by the prune caused by that same successful growth |
| WS | No regression WS-01…WS-05 on main path |
| Camera | No behavior change |
| Paint | No behavior change |
| Capacity | No new policy / no ceiling philosophy change as the goal |
| Architecture | No new layer |

---

## Acceptance mapping (L1…L7)

| ID | After Step 1 (expected) |
|----|-------------------------|
| **L1** | **Pass (must)** |
| **L2** | Unchanged / Fail |
| **L3** | **Partial** (Mutation Set only) |
| **L4** | **Pass (maintain)** |
| **L5** | **Partial / strong Partial** on growth paths covered |
| **L6** | **Pass (must)** |
| **L7** | **Partial** |

| Continuity | Step 1 |
|------------|--------|
| C2 | **Primary claim** |
| C1, C4 | Not claimed |
| C3 | Maintain WS |

Honest Lifetime headline after Step 1: **~2–3 / 7**, not 7 / 7.

---

## Risks

| Risk | Mitigation |
|------|------------|
| Retained set grows under continuous growth | Accept temporary growth; Capacity later. Step 1 only blocks *same-operation* discard of Mutation Set |
| Wide VIEW + large Mutation Set → little else to drop | Allowed; never amputate WS to “make room” |
| Smuggling Capacity via oversized “neighborhood” | Mutation Set = **this operation only** |
| Fixing capture-null WS hole | Out of scope |

---

## Dependencies

Working Set VIEW protection · Paint WS-04 · Frozen CL-01/CL-03/CL-05.  
Capacity/Emergency freeze **not** required.

---

## Why this Step 1

Smallest change that freezes an objective Lifetime law, hits E4 thrash, seeds CL-03 without Capacity numbers, and cannot weaken VIEW if Mutation Set protection is *additive*.

**Rejected:** full high-water/hysteresis · remove triggers · paint/camera · raise HARD_CAP.

---

## Stop

Plan strengthened. No code until implementation approval.

**Next:** Track B Step 1 implementation against this runtime law, then stop for review.
