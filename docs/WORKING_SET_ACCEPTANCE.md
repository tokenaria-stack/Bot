# Working Set — Acceptance Criteria

**Status:** Frozen (pre-implementation gate).  
**Kind:** Objective completion criteria for bringing runtime into compliance with [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md).  
**Not:** an implementation plan, ADR, API design, class design, cache-lifetime policy, or performance programme.

**Track name:** **Implementation Track A — Working Set Compliance** (Debt **#69D**).  
**Not called:** “Wave 4.” Waves 1–3 repaired event-ownership invariants. Track A brings **state** into compliance with an already-frozen contract.

**Goal (only):** Bring runtime into compliance with `WORKING_SET_CONTRACT.md` and this document.  
Every change must map to a **failed** acceptance item (Sx), contract clause (WS-xx / P-xx), and/or blocking evidence (E3-xx) — not to a symptom nickname (“fix jump”, “fix zoom”).

**Baseline:** Stage E3 scorecard **0 / 7**.  
**Done:** all three gates in §1 pass.  
**Out of scope for “done”:** RESET_LIVE product work, ReplayDAG optimisation, LOD, measured OOM/FPS policy, accordion lifetime tuning (unless required to keep S5–S7).

---

## 0. Implementation Rule (before coding)

Every commit in Track A must reference **exactly** which items it advances:

| Field | Example |
|-------|---------|
| Contract | `WS-02`, `WS-04` |
| Acceptance | `S2`, `S4` |
| Evidence | `E3-01`, `E3-05` |
| UX (if applicable) | `U1`, `U4` |

Unmapped “nice improvements,” unrelated refactors, and symptom-only patches are out of track.  
Progress is reported as **scorecard `n / 7`**, never as a bug-fix count.

---

## 1. How we know we are done

Track A is complete **only if all three gates pass**:

| Gate | Name | Requirement |
|------|------|-------------|
| **1** | Architecture | Scorecard **S1…S7** → **7 / 7** (§2) |
| **2** | Evidence | Blocking E3 items cleared (§3) |
| **3** | Real UX | Scenarios **U1…U7** pass (§4) |

“Feels better,” raising a numeric HARD_CAP alone, or fixing one symptom in isolation is **not** done.

---

## 2. Scorecard gate (normative) — Gate 1

From Working Set Contract §6. Each must pass with evidence (test and/or audited runtime path).

| ID | Criterion | Pass means |
|----|-----------|------------|
| S1 | Committed VIEW always inside retained store data | No path leaves visible times absent from store |
| S2 | Visible bars never invalidated without CameraCommit | Prune/replace/merge/compaction cannot drop or identity-replace on-screen bars unless VIEW was intentionally recommitted |
| S3 | Prune never removes committed VIEW | Retention may only drop bars outside VIEW |
| S4 | Paint represents committed VIEW | No tip-tail (or other fixed slice) that omits VIEW while store holds other data |
| S5 | Memory pressure never changes VIEW | Pressure work does not CameraCommit or force clamp/snap navigation |
| S6 | Artificial history boundary impossible (P-01) | Under normal exploration, history stops only at true EOF |
| S7 | Artificial discontinuity impossible (P-02) | Load/cache/prune/page/replay do not appear as navigation events |

**Metric:** report progress as `n / 7`, not as a count of bugs fixed.

---

## 3. E3 evidence gate — Gate 2

### Must clear (blocking)

| ID | Finding | Cleared when |
|----|---------|--------------|
| E3-01 | Prune can remove bars still required by VIEW | S2 + S3 pass |
| E3-02 | Paint tip-tail vs viewport | S4 pass |
| E3-03 | HARD_CAP/TARGET act as product walls | S5 + S6 pass (no user-visible bar budget wall) |
| E3-04 | Preserve nearest-snaps after pruned anchor | S2 pass (anchor time still present or VIEW explicitly recommitted) |
| E3-05 | load → prepend → prune → paint → clamp → jump | S1–S5 pass; chain cannot amputate VIEW then clamp |
| E3-10 | Working Set contract / hierarchy missing | Contract frozen (E3.5) + runtime obeys scorecard |

### Non-blocking for this gate (explicitly deferred)

| ID | Finding | Why deferred |
|----|---------|--------------|
| E3-06 | Stale ARCHITECTURE return-to-live wording | Doc debt; Wave 1 already removed Boot path |
| E3-07 | Accordion / naive shrink risk | Lifetime policy out of contract until measured |
| E3-08 | Capacity profile unmeasured | Performance track; not scorecard |
| E3-09 | LiveKlineRAMCap vs FE roam | Ownership clarification only |

---

## 4. UX scenario gate — Gate 3

Scenarios must pass on the live chart without artificial history walls or navigation discontinuities. Use one liquid symbol/TF representative of normal work (e.g. BTC). Exact bar counts are illustrative stress points, not product limits.

| ID | Scenario | Pass |
|----|----------|------|
| U1 | **Rapid zoom out** then hold | Full committed VIEW remains painted; no tip/last-chunk amputation; no jump to live edge |
| U2 | **Rapid zoom in** after U1 | No refetch thrash required for correctness; no snap; VIEW stable (lifetime policy may retain extra bars) |
| U3 | **Fast left-scroll** with continuous prepend | Chunks load; camera preserves exploration; no right-edge stick of the first candle; empty left space eventually fills or hits true EOF |
| U4 | **Cross former 16k stress** | Behaviour past the old HARD_CAP region is continuous; no thrash cut/jump cycle at ~12k–16k |
| U5 | **Long roam** (months of history at a working TF) | Exploration continuous until authoritative EOF; user never hits a fixed “max bars” wall |
| U6 | **Zoom out while near retention pressure** | If retention runs, on-screen bars stay; any drop is off-VIEW only; no CameraCommit-free clamp |
| U7 | **Return toward live tip** by user scroll (not system yank) | Motion is user-driven; no spontaneous snap from prune/windowMode alone |

Automated tests should cover S1–S5 where deterministic; U1–U7 may be manual checklist plus targeted regressions for prune/paint/preserve.

---

## 5. Explicit non-goals (do not block “done”)

- RESET_LIVE / same-TF full reload product behaviour  
- ReplayDAG or applyFullData performance optimisation  
- LOD / downsampling  
- Freezing hysteresis, soft/hard ceilings, or OOM numbers  
- New managers, WorkingSet classes, or APIs invented for their own sake  
- Closing unrelated Stage E2 leftovers except where they violate this contract  
- Naming or structuring work as “Wave 4”

---

## 6. Evidence to publish when claiming done

1. Scorecard table with **7 / 7** and pointer to tests or audit notes.  
2. E3-01…E3-05 and E3-10 marked resolved (or superseded by scorecard evidence).  
3. UX checklist U1–U7 signed off (manual and/or automated).  
4. Confirmation: no new VIEW owner; ADR-028/029 and Working Set Contract unchanged in meaning.  
5. Commit history shows §0 mapping (WS / S / E3) on Track A commits.

---

## Document status

Frozen as the pre-implementation acceptance gate for **Implementation Track A — Working Set Compliance (#69D)**.  
Implementation may begin only against this document + [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md).  
Completion is objective: **Gate 1 + Gate 2 + Gate 3**.
