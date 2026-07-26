# Working Set Contract

**Status:** Frozen (Stage E3.5).  
**Kind:** Normative architecture contract (constitution).  
**Not:** an ADR, implementation plan, repair wave, API proposal, class design, or pseudocode.

**Assumes:** ADR-028 / ADR-029 and the Core Ownership Model remain frozen.  
**Scope:** What every implementation must **guarantee**. Not how it achieves those guarantees.  
**Stability:** This contract remains valid if the store implementation is replaced (ColumnarStore, paged history, mmap, or otherwise).

Evidence that today’s runtime fails these guarantees: Stage E3 Working Set & Viewport Protocol Audit.

---

## 1. Purpose

The Working Set Contract defines the relationship between:

- **TimeCamera VIEW** — what the user is looking at
- **Store** — which market bars exist in the frontend data plane (today: ColumnarStore)
- **ChartCompositor** — which data is selected for paint
- **ChartAdapter** — what is committed to Lightweight Charts

Its purpose: **runtime implementation details must never become user-visible navigation behaviour.**

“Working Set” names the **required guarantee** (data that must exist to satisfy VIEW). It is not a mandate to create a `WorkingSet` class, manager, controller, service, or bus.

---

## 2. Ownership

| Role | Owns / defines |
|------|----------------|
| **TimeCamera** | VIEW (ViewIntent + ViewGeometry). Sole owner of user-facing navigation. Issues CameraCommit. |
| **Working Set Contract** | What data must exist to satisfy the committed VIEW. Normative law only — not a runtime owner. |
| **Store** (e.g. ColumnarStore) | Implements the contract: retains (and may fetch into) data that satisfies VIEW. |
| **ChartCompositor** | Paints data that satisfies the contract for the committed VIEW. |
| **ChartAdapter** | Applies the committed paint and CameraCommit result to LWC. |

No other component owns VIEW.  
Data, hydration, prune, paint, and transport do not decide navigation.  
TimeCamera does not search market bars; data resolve remains outside TimeCamera (ADR-028).

---

## 3. Core Invariants

### WS-01 — VIEW ⊆ retained data

The store’s retained market data must always completely contain the committed VIEW.

The store may retain more than VIEW. It must never retain less.

### WS-02 — Prune must not invalidate VIEW

Pruning, compaction, paging, or any retention policy must never remove bars required by the committed VIEW.

### WS-03 — Visible bars are immutable without CameraCommit

**Nothing may invalidate a visible bar without an explicit CameraCommit.**

**Invalidation** means: a bar that was inside the committed VIEW ceases to exist in the data plane that paint and camera resolve against, or is replaced as a different market bar (disappearance or identity replacement).

**Not invalidation:**

- Updating the currently forming live tip (OHLC/plots of the open bar already in VIEW).

**Valid intentional VIEW change (explicit CameraCommit):**

- User navigation commits.
- Timeframe switch.
- Explicit FreshLive (and equivalent system commits that intentionally change VIEW).

After such a commit, the store must satisfy the **new** committed VIEW (WS-01).

### WS-04 — Paint represents VIEW

Paint must represent the committed VIEW (and any chrome that does not alter market identity).

Paint must not substitute an implementation-specific cache slice (for example a tip-only window) when that slice is not the VIEW.

### WS-05 — Memory is not navigation

Memory management is an implementation detail.

It must never become user-visible navigation behaviour (jumps, clamps, stuck edges, chopped history, artificial walls).

Pressure may trigger retention work only **outside** the committed VIEW, and must not itself change VIEW.

---

## 4. Product Guarantees

### P-01 — No artificial history boundary

The user must never encounter an artificial history boundary during normal exploration.

History ends only when authoritative history ends (**true EOF**).

Fixed product bar budgets (any numeric “max bars the user may hold”) are forbidden as UX walls. Engineering safety mechanisms, if any, must not masquerade as end-of-history.

### P-02 — Continuous exploration

The user’s exploration must remain continuous.

The terminal may load, cache, replay, prune, compact, or page data, but the user must never perceive **artificial discontinuities** caused by those implementation details (chopped chunks, disappearing bars under the camera, snap-backs, stuck edges driven by retention).

---

## 5. Explicitly Out Of Contract

Intentionally **unspecified** here (future implementation or performance policy):

- Cache lifetime, hysteresis, lazy contraction, high-water marks  
- Soft limits, hard limits, OOM strategy, FPS gates  
- ReplayDAG cost and scheduling  
- Transport chunk sizes  
- LOD / downsampling  
- Implementation classes, APIs, timers, managers  

Absence from this contract does **not** authorize violating WS-01…WS-05 or P-01/P-02.

---

## 6. Acceptance Scorecard

Architectural acceptance (normative). An implementation satisfies this contract only when all pass:

| # | Criterion |
|---|-----------|
| S1 | Committed VIEW is always inside retained store data |
| S2 | Visible bars are never invalidated without an explicit CameraCommit |
| S3 | Prune / compaction / paging never removes the committed VIEW |
| S4 | Paint represents the committed VIEW (not an arbitrary cache tip-tail) |
| S5 | Memory pressure never changes VIEW |
| S6 | Artificial history boundary is impossible under normal exploration (P-01) |
| S7 | Artificial discontinuity from cache implementation is impossible (P-02) |

Stage E3 baseline against this scorecard: **0 / 7** (investigation evidence; not a repair plan).

---

## Document status

- **Frozen** as the Working Set constitution for future store/paint work.  
- Complements the Core Ownership Model and ADR-028/029; does not replace them.  
- Does not authorize new owners or abstractions beyond existing components satisfying this law.
