# Cache Lifetime — Acceptance Criteria

**Status:** Frozen (Stage E4).  
**Kind:** Objective architectural acceptance for Cache Lifetime obedience.  
**Not:** an implementation plan, capacity number sheet, algorithm design, or performance programme.

**Depends on:** [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md), [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md).  
**Does not replace:** [`WORKING_SET_ACCEPTANCE.md`](WORKING_SET_ACCEPTANCE.md) (S1–S7).

Lifetime acceptance covers the obligations that make **S6** and **S7** Pass after WS-01…WS-05 already hold.

---

## 0. Relationship to Working Set scorecard

| Working Set | Lifetime role |
|-------------|----------------|
| S1–S5 | Must remain Pass; lifetime must not regress them (CL-01) |
| S6 (P-01) | Requires lifetime obedience (CL-04, CL-07) |
| S7 (P-02) | Requires lifetime obedience (CL-05, CL-07) |

Full Working Set Gate 1 (**S1–S7 = 7 / 7**) requires this lifetime acceptance as well.

---

## 1. Done gates

Lifetime obedience is complete only when **all** pass:

| Gate | Requirement |
|------|-------------|
| **L-A** | Lifetime scorecard **L1…L7 → 7 / 7** (§2) |
| **L-B** | Working Set **S1–S7 → 7 / 7** |
| **L-C** | Continuity scenarios **C1…C5** pass (§3) |

Changing a capacity number alone, or subjective “feels smoother,” is **not** done.

---

## 2. Lifetime scorecard (L1–L7)

Observable architectural guarantees only.

| ID | Criterion | Pass means |
|----|-----------|------------|
| L1 | Lifetime never violates WS-01…WS-05 | No retention path invalidates committed VIEW or its paint |
| L2 | Eager expand / lazy contract | Narrowing VIEW alone does not shrink retained data |
| L3 | Discard only outside protected set | Under pressure, VIEW and exploration neighborhood remain |
| L4 | Discard ≠ EOF | Authoritative end-of-history unchanged by retention discard |
| L5 | No thrash discontinuity | Continuous roam does not lose-and-need the same neighborhood solely due to discard |
| L6 | Pressure ≠ navigation | Retention alone does not change VIEW |
| L7 | P-01 / P-02 upheld | No artificial history wall; no artificial discontinuity from lifetime |

**Metric:** report lifetime progress as `n / 7`. Report Working Set `S1–S7` separately.

**Baseline at freeze:** L-scorecard **0 / 7** (contract frozen; runtime lifetime policy not yet compliant).

---

## 3. Continuity scenarios (Gate L-C)

| ID | Scenario | Pass |
|----|----------|------|
| C1 | Expand VIEW far, then narrow VIEW | Retained data does not collapse solely because VIEW narrowed; VIEW remains valid |
| C2 | Continuous leftward exploration with repeated history loads | Active exploration neighborhood is not discarded then required again in a self-defeating cycle |
| C3 | Live growth while VIEW is wide | VIEW remains; no retention-driven snap; EOF unchanged |
| C4 | Long roam toward authoritative end of history | Stops only on true EOF; no artificial bar-budget wall |
| C5 | Pressure discard (when capacity/emergency invokes it) | Only outside VIEW and exploration neighborhood; no navigation event |

---

## 4. Evidence that must clear

| Evidence | Cleared when |
|----------|--------------|
| E3-03 (numeric ceilings as product walls) | L7 + S6 Pass |
| E3-05 residual thrash | L5 + C2 Pass |
| E3-07 accordion / shrink-on-narrow threat | L2 + L5 Pass |
| Validation S6/S7 Partial | Both Pass |

---

## 5. Explicit non-goals

- Choosing capacity or emergency numbers  
- Designing hysteresis or timers  
- OOM / FPS emergency UX details  
- Replay / LOD / transport optimisation  
- RESET_LIVE product behaviour  
- New managers or APIs  

---

## 6. Evidence to publish when claiming done

1. Lifetime scorecard **7 / 7** with tests or audited paths  
2. Working Set scorecard **S1–S7 = 7 / 7**  
3. Continuity checklist C1–C5 signed off  
4. Confirmation: retention does not issue CameraCommit; discard does not alter authoritative EOF  

---

## Document status

- **Frozen** with the Cache Lifetime Contract.  
- Implementation of lifetime policy may begin only against these frozen documents.  
- Completion is objective: **Gate L-A + L-B + L-C**.
