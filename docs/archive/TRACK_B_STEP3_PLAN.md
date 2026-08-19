# Track B — Cache Lifetime · Step 3 Plan

**Status:** Implemented — see [`TRACK_B_STEP3.md`](TRACK_B_STEP3.md).  
**Kind:** Final Lifetime *guarantee* before Capacity — not a new Lifetime abstraction or layer.  
**Not:** constitution edits, Capacity/Emergency freeze, redesign.

**Frozen laws:** [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md), [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md).  
**Frozen gates:** [`WORKING_SET_ACCEPTANCE.md`](WORKING_SET_ACCEPTANCE.md), [`CACHE_LIFETIME_ACCEPTANCE.md`](CACHE_LIFETIME_ACCEPTANCE.md).  
**Prior:** Steps 1–2 validated — Lifetime **5 / 7**; L2 Partial; L7 Partial.  
[`TRACK_B_STEP2_VALIDATION.md`](TRACK_B_STEP2_VALIDATION.md).

**Vocabulary:** Prefer **valid architectural boundary**. Prefer Retained Neighborhood — never “Neighborhood Cache,” Warm/Cold Cache, Exploration Zone, Continuity Layer, or Sticky Region as new Lifetime nouns.

---

## Critical verdict

**There is no missing Lifetime set or architectural layer between today’s Retained Neighborhood and Capacity.**

After Step 2, RN *exists* as the CL-03 exploration-neighborhood category. What is still incomplete is not another region — it is RN’s **guaranteed lifetime**: when retention may contract.

```text
VIEW / Working Set          → Track A
Mutation Set                → Track B Step 1 (same-op)
Retained Neighborhood       → Track B Step 2 (exists)
Retention lifetime of RN    → Track B Step 3 (this plan)  ← last Lifetime guarantee
Pressure / eviction         → Capacity (intentionally unspecified)
Survival under OOM/FPS      → Emergency (intentionally unspecified)
```

Inventing Exploration Zone / Continuity Layer / Sticky Region / Warm–Cold Cache after Step 3 would **not** extend Lifetime — it would add complexity without new constitutional responsibility.

**Step 3 is the last Lifetime guarantee before Capacity.** That is the natural boundary.

---

## 1. Objective

Define the **final Lifetime guarantee**:

> **Retention lifetime is governed only by pressure or explicit world replacement — never by ordinary exploration events.**

This is the transition from **“RN exists”** to **“RN has a guaranteed lifetime.”**  
It is stronger than “make CL-02 an explicit runtime law” as paperwork — it is the semantic completion of Neighborhood Lifetime.

Advances **L2** (and C1) without Capacity ownership. Does **not** claim L7 / C4 Pass.

---

## 2. Architectural law (draft only)

### Steps 1–2 (unchanged)

- Successful growth establishes a temporary Mutation Set; same-op prune must not remove it.  
- Successful exploration growth expands RN by absorbing that Mutation Set; RN persists across growth; RN resets only on explicit world replacement (CameraCommit / commit-paired transition).

### Step 3 law (new)

> **Exploration events must never be interpreted as contraction events.**  
> **The Retained Neighborhood (and retained bars it protects) may contract only under Capacity/Emergency pressure, or on explicit world replacement — never because exploration progressed.**

**Exploration events** (non-exhaustive examples — law is event-class, not API-bound):

| Exploration event (≠ contraction trigger) |
|-------------------------------------------|
| VIEW narrowed |
| VIEW expanded / panned / zoomed (without world replace) |
| Fetch completed |
| Prepend finished |
| Append finished |
| Projection merged / soft applyProjection |
| Mutation Set absorbed into RN |

**Contraction events** (only these classes — pressure undefined here):

| Contraction event |
|-------------------|
| Capacity/Emergency **pressure** prune ( Cap/Emergency own “when pressure exists”) |
| Explicit **world replacement** (CameraCommit / commit-paired transition / clear) |

This wording stays valid if ReplayDAG, chunk size, transport, LOD, or store implementation change — it binds **event semantics**, not mechanisms.

Maps to frozen **CL-02** (eager expand / lazy contract) without smuggling Capacity numbers.

---

## 3. Ownership (strengthened)

| Role | Owns | Must not own |
|------|------|----------------|
| **TimeCamera** | VIEW; CameraCommit | Retention, pressure |
| **Working Set** | What data must exist for VIEW | When beyond-WS may die; pressure |
| **Lifetime** | Retention **semantics** (Mutation, RN, exploration≠contraction) | **Never defines pressure**, ceilings, eviction recipes, “enough memory” |
| **Capacity** | **When pressure exists**; engineering limits; eviction under pressure | VIEW; pretending discard is EOF |
| **Emergency** | Survival behaviour (OOM / FPS brakes) | Navigation ownership |

**Eliminates future dispute:** Lifetime never decides “there is enough / not enough memory.” That sentence belongs only to Capacity/Emergency.

Layering (constitutional; do not reorder):

```text
VIEW
 → Working Set
 → Mutation Set
 → Retained Neighborhood
 → Cache Lifetime (semantics, including guaranteed RN lifetime)
 → Capacity          ← pressure starts here
 → Emergency
```

---

## 4. Investigation answers (frozen into this plan)

### Q1 — What is still missing?

Not a new abstraction. Missing: **guaranteed retention lifetime** for RN / retained data — contraction only under pressure or world replace, never under exploration events.

### Q2 — One region or three?

**Same thing.** RN *is* the exploration / continuity / retained neighborhood category (CL-03). Do not redesign into three Lifetime regions.

### Q3 — L2/L7 without Capacity?

| Score | Without Capacity? | Why |
|-------|-------------------|-----|
| **L2 / C1** | **Yes** | Exploration≠contraction is a Lifetime semantic |
| **L7 / C4** | **No** | Artificial wall when VIEW ∪ RN leave nothing eligible under HARD_CAP is Capacity pressure ∩ protected sets; Lifetime already forbids discarding RN to “make room” while exploring |

### Q4 — Intentionally unspecified until Capacity

Maximum retained size · neighborhood dimensions · eviction algorithms · memory pressure definition · OOM/FPS · HARD_CAP/TARGET philosophy · timers / hysteresis · adaptive limits.

### Q5 — Valid if transport/store/LOD/ReplayDAG change?

**Yes** — law is about exploration vs contraction **event classes**, not prepend/append APIs.

### Q6 — Hidden WS conflict?

**No**, if Step 3 never CameraCommits, never invalidates VIEW-required bars, and never lets retention own navigation.  
**Risk to forbid:** reading Lazy Contract as “Lifetime forbids all prune” (that steals Capacity) or as “Lifetime decides memory is tight” (also Cap/Emergency).

---

## 5. Scope (when implemented later)

### In scope

1. Encode exploration≠contraction so VIEW narrow / fetch complete / growth finish / soft merge do **not** themselves shrink RN or retained bars.  
2. Preserve Mutation Set + RN absorb/protect/reset laws.  
3. Keep pressure prune paths owned by existing Capacity triggers **as triggers only** — do not redefine pressure.  
4. Tests: C1-style expand→narrow does not collapse RN/retained; exploration events don’t clear RN; world replace still clears; WS + Steps 1–2 green.  
5. Docs: honest score; explicit “Lifetime category model complete; Cap next.”

### Explicitly out of scope

- Eviction algorithms, timers, hysteresis, HARD_CAP/TARGET retune  
- MemoryManager / new services / APIs / buses  
- Multiple neighborhood types (Warm/Cold, Sticky, Continuity Layer, …)  
- Claiming L7 / C4 / full Gate L-A  
- Constitution file edits  
- ReplayDAG, RESET_LIVE, LOD  

---

## 6. Non-goals

| Non-goal | Why |
|----------|-----|
| New Lifetime layer / set | No missing abstraction after RN |
| Lifetime 7 / 7 | L7 needs Capacity |
| Capacity constitution | Separate track |
| “RN exists” paperwork only | Step 3 is **guaranteed lifetime**, not a formality |

---

## 7. Expected Lifetime score after Step 3

Baseline validated: **5 / 7 Pass**.

| ID | After Step 3 (expected) |
|----|-------------------------|
| L1, L3–L6 | **Pass** (maintain) |
| **L2** | **Pass** (primary claim — exploration≠contraction) |
| **L7** | **Partial** (Capacity-bound walls remain) |
| **C1** | **Pass** (or strong Pass) |
| C2–C3 | Maintain Pass |
| C4 | Fail (Capacity) |
| C5 | Partial until Capacity taxonomy |

Honest headline: **~6 / 7**, not 7 / 7.

---

## 8. Remaining open work (after Step 3)

| Work | Owner |
|------|--------|
| Pressure definition, eviction, numeric ceilings, C4/C5/L7 Pass | **Capacity** (+ Emergency) |
| Hull vs sparse RN shape | Optional polish — not a new Lifetime responsibility |
| RESET_LIVE, capture-null residual | Product / other tracks |
| Any Warm/Cold/Sticky/Zone invention | **Reject** as Lifetime extension — no new responsibility |

---

## Risks

| Risk | Mitigation |
|------|------------|
| Step 3 read as “forbid all prune” | Law: contraction under **pressure** still allowed; Lifetime does not define pressure |
| Step 3 invents a new region | Forbidden — semantics on existing RN only |
| Lifetime starts deciding memory | Ownership table: Lifetime never defines pressure |
| Claiming L7 done | Explicit Partial until Capacity |

---

## Dependencies

Validated Steps 1–2 · Frozen CL-02 / CL-03 / CL-05.  
Capacity constitution **not** required to *state* the law; required to *finish* L7/C4.

---

## Why this Step 3

Completes Neighborhood Lifetime: RN does not merely exist — it has a **guaranteed lifetime** independent of ordinary exploration. Natural stop before Capacity.

**Rejected:** new Lifetime nouns · Capacity smuggling · treating Step 3 as CL-02 rubber-stamp.

---

## Stop

Implementation complete — report [`TRACK_B_STEP3.md`](TRACK_B_STEP3.md). Validate separately; do not invent further Lifetime layers.
