# Lifetime & Capacity Constitution

**Status:** Frozen (S6).  
**Kind:** Normative constitution — Lifetime + Capacity principles.  
**Not:** implementation, numeric tuning, ADR, manager design, or a re-investigation of S6.

**Assumes frozen:** ADR-028 / ADR-029, [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md), [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md) (CL-01…CL-07 detail).  
**Evidence:** [`WORKING_SET_S6.md`](WORKING_SET_S6.md), Track B Steps 1–3, Gate S5 PASS.  
**Runtime:** **FAIL** against this constitution until the Current Violation is closed.

---

## Layer map

```text
A  Product              P-01 / P-02 (no artificial wall; continuous exploration)
B  Working Set          VIEW ⊆ data; prune/paint; pressure ≠ navigation
C  Cache Lifetime       when beyond-WS may be retained or discarded (eager expand / lazy contract)
D  Capacity / pressure  when pressure exists; engineering ceilings (numbers deferred)
E  Emergency            OOM / FPS survival (must not masquerade as EOF)
```

Detailed Lifetime invariants remain in [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md).  
This document **binds Lifetime and Capacity together** for product boundary S6 and freezes Capacity *principles* (not values).

---

## Constitutional Rules

1. **HARD_CAP and TARGET are never EOF.** They must not be published, implied, or felt as end-of-history.

2. **Normal exploration has no artificial frontend history boundary.** The user must be able to roam and zoom into history without a FE bar-budget wall.

3. **True history termination comes only from authoritative history EOF** (`hasMore === false` or equivalent completion fact). Discard ≠ EOF.

4. **VIEW must remain fully represented by the Working Set** (WS-01…WS-04). Lifetime and Capacity are subordinate to the Working Set Contract.

5. **Pruning / eviction may occur only outside the protected Working Set** (and Lifetime-protected neighborhood where Lifetime requires it). Never invalidate committed VIEW without CameraCommit.

6. **Narrowing VIEW does not itself create pruning pressure.** Extra retained history may remain cached (lazy contract).

7. **Capacity pressure is an implementation concern, not a navigation authority.** Pressure must not CameraCommit, clamp VIEW, FreshLive, or return-to-live (S5).

8. **Commit-paired world replacement may replace the world**, but must **not** silently reinterpret TARGET/HARD_CAP as “the oldest history available” inside that newly loaded world.

9. **Lifetime decides retention semantics; Capacity decides when pressure exists; Emergency decides survival brakes.** Lifetime never defines “enough memory.” Capacity never owns VIEW. Emergency never fakes EOF.

10. **Numeric capacity values are intentionally deferred** until a separate in-browser capacity benchmark exists. No HARD_CAP/TARGET/“200k” choice in this constitution.

---

## Current Violation

**Behavior:** `commitPaired` hydration / world replace → budget prune toward **TARGET ≈ 12k** → older bars from the loaded payload are discarded → user cannot explore that older history in the new world even though this is **not** authoritative EOF.

**Classification (do not soften because the path is commit-paired):**

| Contract | Violated? | Why |
|----------|-----------|-----|
| **P-01** (Product) | **Yes** | Artificial frontend history boundary inside the loaded world |
| **Working Set** | **Not automatically** | After an explicit CameraCommit, if the *new* VIEW ⊆ retained ~12k, WS-01…WS-05 may still hold for that VIEW |
| **Lifetime / Capacity (this constitution)** | **Yes** | Rule 1, 2, 8 — TARGET treated as oldest available history |

**Distinction:**

| Intentional world replacement | Artificial history boundary inside that world |
|------------------------------|-----------------------------------------------|
| TF switch / FreshLive / `loadDashboard` may **replace** which world is shown (CameraCommit). | After replace, FE budget truncates the new world’s retained history to TARGET and presents that cut as the practical left edge of available history. |
| Allowed as navigation. | **Forbidden** as product/Lifetime-Capacity law — not the same as choosing a new VIEW. |

Commit-paired is not a license for P-01 failure.

---

## Capacity Unknowns

Recorded only; do not invent numbers:

- Browser / LWC applyFullData time, FPS, heap above ~34k columnar bars — **UNKNOWN**
- DDR / indicator cost at large VIEW — **UNKNOWN**
- ReplayDAG cost at large VIEW — **UNKNOWN**
- Measured Node/columnar: preserve-paired growth exceeded HARD_CAP to ~**34k** with VIEW covered; commit-paired replace of 20k+ retained **~12k (TARGET)**

A **separate capacity benchmark** (in-browser) is required before selecting numeric Capacity/Emergency limits.

---

## Acceptance Criteria

**S6 = PASS** only when all hold:

1. HARD_CAP / TARGET never appear as user-visible history EOF (Rules 1–3).  
2. Commit-paired hydration does not amputate the newly loaded world’s history to TARGET in a way that creates an artificial left wall (Rule 8 / Current Violation closed).  
3. Preserve-paired exploration still obeys Working Set + Lifetime (VIEW covered; narrow ≠ pressure).  
4. Pressure still ≠ navigation (S5 maintained).  
5. Authoritative EOF remains the only true history termination.  

Raising HARD_CAP alone, or “feels smoother,” is **not** S6 PASS.

---

## Out of Scope

- Numeric HARD_CAP / TARGET / neighborhood sizing  
- Timers / hysteresis / high-water recipes  
- LOD / downsampling  
- ReplayDAG optimization  
- RESET_LIVE  
- WorkingSetManager / MemoryManager / event bus / new APIs  
- Implementation of the S6 fix (separate gate)

---

## Note

Existing [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md) remains the detailed Lifetime invariant sheet (CL-01…CL-07). Capacity was previously “intentionally unspecified” there; **Capacity principles are now frozen here** (values still deferred).
