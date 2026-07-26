# Track B — Cache Lifetime · Step 2 Plan

**Status:** Plan only (not implemented). Ready for implementation when approved.  
**Kind:** Smallest next Lifetime step after validated Step 1.  
**Not:** code, Step 3, constitution edits, Capacity/Emergency freeze, redesign.

**Frozen laws:** [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md), [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md).  
**Frozen gates:** [`WORKING_SET_ACCEPTANCE.md`](WORKING_SET_ACCEPTANCE.md), [`CACHE_LIFETIME_ACCEPTANCE.md`](CACHE_LIFETIME_ACCEPTANCE.md).  
**Prior step:** [`TRACK_B_STEP1.md`](TRACK_B_STEP1.md) · validated [`TRACK_B_STEP1_VALIDATION.md`](TRACK_B_STEP1_VALIDATION.md) · Lifetime **3 / 7**.

**Vocabulary:** Prefer **valid architectural boundary**. Prefer **Retained Neighborhood** / **Protected Neighborhood** — never “Neighborhood Cache” (that blurs into Capacity).

---

## 1. Step 2 objective

Remove **multi-operation** `fetch → prune → refetch` thrash of the active exploration span, without Capacity levers and without weakening Working Set or Step 1 Mutation Set guarantees.

```text
Step 1 closed:   growth → same-op discard of Mutation Set → refetch
Step 2 closes:   growth A → growth B → discard A → refetch A   (while still exploring)
```

Not the objective: bigger HARD_CAP / TARGET / chunks, timers, or hysteresis.

---

## 2. Lifetime law introduced by Step 2

### Step 1 law (unchanged)

> A successful cache growth operation establishes a temporary Mutation Set. Any prune caused by that same growth operation must not remove members of that Mutation Set.

### Step 2 law (new)

> **Successful exploration growth expands the Retained Neighborhood by absorbing that operation’s Mutation Set. Under pressure, discard must not remove members of the Retained Neighborhood. The Retained Neighborhood is cleared only by intentional world replacement (commit-paired replace / explicit world-changing CameraCommit), never by a subsequent growth operation alone.**

This is **Neighborhood Lifetime** (CL-03 category + CL-05 continuity), not a Capacity budget.

### Naming

| Prefer | Avoid |
|--------|--------|
| Retained Neighborhood | Neighborhood Cache |
| Protected Neighborhood | Bigger cache / high-water buffer |
| Neighborhood Lifetime | Memory budget / sliding window product |

### Sets after Step 2

```text
Committed VIEW
      │
      ▼
Working Set (WS)              ← Track A — sacrosanct
      │
      ▼
Mutation Set                  ← Step 1 — temporary, same growth op only
      │
      ▼
Retained Neighborhood         ← Step 2 — survives across growth ops
      │
      ▼
Remaining Cache               ← still eligible under existing pressure triggers
```

| Set | Rule |
|-----|------|
| Working Set | Must not regress WS-01…WS-05 |
| Mutation Set | Same-op protection **preserved** (Step 1) |
| Retained Neighborhood | Cross-op protection; absorbs Mutation Sets from successful exploration growth |
| Remaining Cache | Eligible under **existing** pressure triggers (no new Capacity philosophy) |

Pressure prune candidates = outside **VIEW ∪ Mutation Set ∪ Retained Neighborhood**.

---

## 3. Scope

### In scope

1. Introduce Retained Neighborhood as a Lifetime protected category (store-local fact — **not** a new manager, bus, or layer).  
2. On successful growth that establishes a Mutation Set: **absorb** that Mutation Set into the Retained Neighborhood (eager expand).  
3. On pressure prune: protect Retained Neighborhood in addition to VIEW and (same-op) Mutation Set.  
4. On commit-paired world replace (`commitPaired: true` / FreshLive / TF hydrate): **clear** Retained Neighborhood (intentional world replacement — CL contract §4).  
5. Preserve Step 1 Mutation Set behaviour on all classified growth paths.  
6. Preserve WS-01…WS-05; no Camera / Paint / EOF ownership changes.  
7. Tests: multi-op prepend does not discard the prior Mutation Set / prior neighborhood span; WS + Step 1 non-regression; commit-paired clears neighborhood and may prune.

### Growth-path classification (Step 2)

| Growth operation | Mutation Set (Step 1) | Expands Retained Neighborhood? | Clears RN? |
|------------------|----------------------|--------------------------------|------------|
| prepend (added > 0) | Yes | **Yes** (absorb) | No |
| appendTick new bar | Yes | **Yes** (absorb; keeps law uniform) | No |
| applyProjection preserve/soft | Yes | **Yes** (absorb) | No |
| replaceMonolith / apply `commitPaired: true` | Excluded | No | **Yes** (world replace) |

Append absorb is deliberately uniform and cheap: tip bars are usually already in VIEW; the primary thrash win is **prepend**. Soft apply absorb may temporarily make RN ≈ full series until next commit-paired replace — accepted Lifetime eagerness, not a Capacity number.

### Explicitly one step

Do **not** in Step 2:

- Raise HARD_CAP / TARGET / chunk size as the fix  
- Timers, idle expiry, hysteresis, high-water recipes  
- New Capacity or Emergency constitution  
- Full CL-02 product polish beyond what RN naturally provides  
- Camera, Paint, Adapter, ReplayDAG, RESET_LIVE, LOD  
- New managers / services / buses  
- Constitution file edits (laws already cover exploration neighborhood as a category)

---

## 4. Non-goals

| Non-goal | Why |
|----------|-----|
| Lifetime 7 / 7 | Step 2 is incremental |
| Capacity policy | Numbers ≠ Neighborhood Lifetime |
| “Bigger cache” | Different abstraction; blurs Capacity |
| Emergency / OOM / FPS | Unspecified; later |
| Working Set S6/S7 full Pass | May advance; not the sole goal |
| Speculative FSM / Revision fields | Jeweler: sockets only when needed — RN is a fact, not a plant |

---

## 5. Runtime paths affected

| Path / file | Likely plan-level touch |
|-------------|-------------------------|
| `ColumnarStore` pressure prune | Protect VIEW ∪ Mutation ∪ **Retained Neighborhood** |
| Successful growth (`prependMonolith`, `appendTick` new bar, soft `applyProjection`) | After Mutation Set established: absorb into RN |
| `commitPaired` apply / `replaceMonolith` | Clear RN, then existing commit-paired prune |
| Boot `loadDashboard` | Already commit-paired — clears RN by law |
| Tests (`columnar-store_budget_test.js`) | Multi-op prepend anti-thrash; Step 1 + WS green; commit clears |
| `index.html` | Cache bump if scripts change |

Unlikely: compositor, TimeCamera, hydration EOF semantics, server, CameraCommit publishers.

**Architecture constraint:** RN lives as store retention state (time bounds or equivalent), updated by growth/commit paths already in the store. No new orchestrator.

---

## 6. Acceptance criteria

| Check | Pass |
|-------|------|
| Step 2 law | Prior Mutation Set / absorbed neighborhood survives a **later** growth’s pressure prune |
| Step 1 law | Same-op Mutation Set still never discarded by that same growth |
| WS | No regression WS-01…WS-05 on main path |
| Clear on world replace | commit-paired hydrate clears RN; prune may drop former neighborhood |
| Camera / Paint | No behaviour change |
| Capacity | No ceiling philosophy change; no timers/hysteresis |
| Architecture | No new layer / manager |
| Continuity | C2 advances from Partial → **Pass** (or strong Pass) for continuous leftward multi-chunk roam |

Objective scenario (must pass in tests):

```text
store near HARD_CAP with VIEW on right/middle
prepend chunk A  → A in Mutation then absorbed into RN
prepend chunk B  → B Mutation protected same-op; A still in RN
pressure prune   → A retained (not discarded then immediately needed again)
```

---

## 7. Honest Lifetime score after Step 2

Baseline validated: **3 / 7 Pass** (L1, L4, L6).

| ID | After Step 2 (expected) |
|----|-------------------------|
| **L1** | **Pass** (maintain) |
| **L2** | Fail or **Partial** — RN does not shrink on VIEW narrow alone; full lazy-contract polish may remain |
| **L3** | **Pass** or strong Partial — discard outside VIEW + Retained Neighborhood (category exists and is enforced) |
| **L4** | **Pass** (maintain) |
| **L5** | **Pass** (or strong Partial→Pass) — multi-op thrash of active neighborhood closed on covered paths |
| **L6** | **Pass** (maintain) |
| **L7** | **Partial** — continuity better; Capacity walls / true EOF roam still open |

| Continuity | After Step 2 |
|------------|--------------|
| **C2** | **Primary claim → Pass** |
| C3 | Maintain |
| C1, C4, C5 | Not fully claimed |

Honest Lifetime headline after Step 2: **~4–5 / 7**, not 7 / 7.

---

## 8. Remaining Lifetime gaps (after Step 2)

Still out of Step 2 (and still Lifetime / later Capacity — **not** solved by raising caps):

| Gap | Layer |
|-----|--------|
| Full CL-02 product (narrow never shrinks retained) if any non-pressure shrink path remains | Lifetime |
| Exploration neighborhood **shape** beyond absorb-Mutation (constitution leaves size unspecified) | Lifetime (further steps) / Capacity only when numbers enter |
| Long roam artificial wall when VIEW ∪ RN leaves nothing eligible under pressure | Capacity + L7 soft |
| C4 true-EOF-only stop; C5 full pressure taxonomy | Lifetime + Capacity/Emergency |
| Capture-null VIEW residual; RESET_LIVE; LOD; ReplayDAG | Product / other tracks |

**Rejected for Step 2:** HARD_CAP↑ · TARGET↑ · chunk↑ · timers · hysteresis · “Neighborhood Cache” framing.

---

## Risks

| Risk | Mitigation |
|------|------------|
| RN grows without bound under continuous roam | Accept as Lifetime eagerness; Capacity later defines when pressure may shrink **outside** RN — never by deleting RN to “make room” while exploration continues |
| Soft apply absorbs full series into RN | Allowed; cleared on next commit-paired world replace |
| Smuggling Capacity via huge fixed neighborhood constant | Forbidden — RN expands by absorbing Mutation Sets, not by a magic bar count |
| Weakening Mutation Set | Absorb **after** same-op protection; Step 1 law untouched |
| Treating RN as paint/camera input | Forbidden — retention fact only |

---

## Dependencies

Validated Step 1 Mutation Set · Track A VIEW protection · Frozen CL-01 / CL-03 / CL-05.  
Capacity constitution **not** required.

---

## Why this Step 2

Smallest Lifetime extension that makes the exploration neighborhood **cross-operational** (CL-03 / CL-05 / C2) without Capacity numbers and without a new architectural layer.

**Rejected alternatives:** bigger cache · raise caps · timers · hysteresis · Step 3 scope creep.

---

## Stop

Plan only. No code. No Step 3.

**Next:** Implementation approval against this law, then implement Step 2 only and stop for validation.
