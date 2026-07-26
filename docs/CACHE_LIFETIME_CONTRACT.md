# Cache Lifetime Contract

**Status:** Frozen (Stage E4).  
**Kind:** Normative lifetime constitution (guarantees only).  
**Not:** an ADR, implementation plan, repair wave, API proposal, class design, algorithm, timer/hysteresis recipe, or capacity number sheet.

**Assumes frozen:** ADR-028 / ADR-029, Core Ownership Model, [`WORKING_SET_CONTRACT.md`](WORKING_SET_CONTRACT.md).  
**Scope:** What every retention/lifetime policy must **guarantee**. Not how it is implemented.  
**Stability:** Remains valid if the store, transport, replay engine, chart library, or storage backend is replaced.

**Relationship to Working Set:**

```text
Working Set Contract     →  what data must exist for VIEW
Cache Lifetime Contract  →  when data beyond that requirement may be retained or discarded
Capacity policy          →  engineering ceilings (intentionally unspecified here)
Emergency policy         →  OOM / FPS brakes (intentionally unspecified here)
```

Cache Lifetime is **strictly subordinate** to the Working Set Contract. Nothing herein may weaken WS-01…WS-05.

Product guarantees **P-01** and **P-02** are defined in the Working Set Contract. This document states the lifetime obligations required to uphold them.

---

## 1. Purpose

Runtime implementation details of retention must never become user-visible navigation behaviour, and must never create artificial history walls or exploration discontinuities.

“Cache Lifetime” names the **retention guarantee** for data outside the Working Set requirement. It is not a mandate to create a Lifetime class, manager, controller, service, or bus.

---

## 2. Ownership

| Role | Owns / defines |
|------|----------------|
| **TimeCamera** | VIEW. Sole owner of user-facing navigation. Issues CameraCommit. |
| **Working Set Contract** | What data must exist to satisfy VIEW. |
| **Cache Lifetime Contract** | When data beyond the Working Set may be retained or discarded. Normative law only — not a runtime owner. |
| **Store** (any implementation) | Holds market display data; must obey Working Set and Cache Lifetime. |
| **Hydration / fetch** | Loads history; must not treat discard as end-of-history. |
| **Compositor** | Paints data satisfying Working Set for the committed VIEW. |
| **ChartAdapter** | Applies paint and CameraCommit to the chart surface. |

No other component owns VIEW.  
Lifetime policy must never become a second navigation owner.

---

## 3. Core invariants

### CL-01 — Subordinate to Working Set

Cache lifetime decisions **must never** violate WS-01…WS-05.

Any discard, compaction, paging, or retention shrink that would invalidate the committed VIEW is forbidden unless preceded by an explicit CameraCommit (WS-03).

### CL-02 — Eager expand, lazy contract

The retained set may grow as soon as exploration or VIEW requires more data.

The retained set must **not** shrink merely because VIEW narrowed or a fetch completed.

Contraction is permitted only under **pressure**. What constitutes pressure is defined by capacity and emergency policy — not by this contract.

### CL-03 — Discard only outside the protected set

Under pressure, discard is allowed only for data that is:

1. outside the committed VIEW, and  
2. outside the **exploration neighborhood** that lifetime protects for continuous roam.

The exploration neighborhood **exists** as a protected category. Its size and shape are unspecified here.

Discard inside the committed VIEW is always a Working Set violation, not a lifetime choice.

### CL-04 — Discard ≠ end of history

Discarding retained data must never be published or interpreted as authoritative end-of-history (**true EOF**).

Authoritative EOF remains a completion fact of history fetch. Lifetime must not clear, invent, or fake it.

### CL-05 — No thrash discontinuity

Lifetime must not cause continuous exploration to lose and then immediately need the same exploration neighborhood again solely because lifetime discarded it while that neighborhood was still required for that exploration (P-02).

This forbids self-defeating retention cycles. It does not prescribe algorithms, timers, or constants.

### CL-06 — Pressure is not navigation

Pressure-driven retention work must not issue CameraCommit or otherwise change VIEW.

If discard would leave the committed VIEW without resolvable data or anchors, discard is forbidden (CL-01 / WS-03).

### CL-07 — Uphold P-01 and P-02

Lifetime and capacity behaviour must uphold Working Set product guarantees:

- **P-01** — no artificial history boundary from retention or capacity numbers; history ends only at true EOF.  
- **P-02** — load, discard, compact, or page must not appear as navigation events.

True EOF and explicit CameraCommits remain the only legitimate exploration “ends” and intentional “jumps.”

---

## 4. What may be discarded (category only)

**Permitted in principle** (subject to CL-01…CL-07 and pressure):

- Data far outside VIEW and outside the exploration neighborhood  
- Data rendered obsolete by an explicit CameraCommit that replaces the world (for example timeframe switch)  
- Redundant retained state after a commit-paired world replace  

**Forbidden:**

- Anything required by the committed VIEW  
- Discard presented as end-of-history  
- Discard that breaks continuous exploration of the active neighborhood (CL-05)  
- Discard that changes VIEW without CameraCommit  

---

## 5. Explicitly out of contract

Intentionally **unspecified** (capacity, emergency, or performance policy):

- Soft or hard bar counts, ceilings, high-water marks  
- Hysteresis, debounce, idle delay, or any timing recipe  
- Exact exploration-neighborhood dimensions  
- OOM / FPS thresholds and emergency UX  
- Replay cost, transport chunk sizes, LOD / downsampling  
- Classes, APIs, managers, event buses, timers  

Absence of numbers here does **not** authorize violations of CL-01…CL-07 or of WS-01…WS-05 / P-01 / P-02.

---

## 6. Acceptance

Objective checks: [`CACHE_LIFETIME_ACCEPTANCE.md`](CACHE_LIFETIME_ACCEPTANCE.md).

Working Set scorecard items **S6** and **S7** require obedience to this contract in addition to WS-01…WS-05.

---

## Document status

- **Frozen** as the Cache Lifetime constitution.  
- Complements the Working Set Contract; does not replace it.  
- Does not authorize new owners or abstractions beyond existing components satisfying this law.
