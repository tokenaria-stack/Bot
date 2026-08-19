# Stage E4 — Cache Lifetime & Pressure Evidence Ledger

**Status:** Investigation complete; Lifetime Contract **frozen**.  
**Kind:** Historical evidence ledger (why the Lifetime Contract exists).  
**Not:** implementation, number tuning, redesign, or living SSOT for guarantees.

**SSOT for guarantees:** [`CACHE_LIFETIME_CONTRACT.md`](CACHE_LIFETIME_CONTRACT.md)  
**SSOT for acceptance:** [`CACHE_LIFETIME_ACCEPTANCE.md`](CACHE_LIFETIME_ACCEPTANCE.md)

---

## Freeze declaration

**The Working Set Contract and Cache Lifetime Contract are now frozen.**

Capacity policy and Emergency policy remain intentionally unspecified.

---

## 1. What caused TARGET / HARD_CAP thrash (evidence at E4)

```text
retained bars exceed a capacity trigger
         ↓
retention shrinks toward a soft keep size
         ↓
off-VIEW bars dropped (VIEW itself protected after Track A)
         ↓
often includes the neighborhood just fetched for left roam
         ↓
user continues left → same neighborhood fetched again
```

After Working Set Steps 1–2, this is **not** primarily VIEW amputation. It is **lifetime thrash** against P-02 (and a soft P-01 failure if exploration feels walled).

Numeric trigger/keep values named in runtime at investigation time are **capacity policy**, not Lifetime Contract content.

---

## 2. Working Set vs Cache Lifetime boundary

| Concern | Layer |
|---------|--------|
| What must exist for VIEW | Working Set |
| Paint must represent VIEW | Working Set |
| When data beyond VIEW may die | **Cache Lifetime** |
| Lazy vs eager retention | **Cache Lifetime** |
| Discard ≠ EOF | **Cache Lifetime** (+ fetch completion facts) |
| Soft/hard numeric ceilings | Capacity (unspecified) |
| OOM / FPS brake | Emergency (unspecified) |

---

## 3. Four-way separation (frozen)

```text
Working Set Contract     (frozen)
        ↓
Cache Lifetime Contract  (frozen)
        ↓
Capacity Policy          (unspecified)
        ↓
Emergency Policy         (unspecified)
```

---

## 4. Freeze review notes (E4.1)

### Wording improvements applied

- Removed implementation language (timers, hysteresis recipes, TARGET/HARD_CAP as contract terms).  
- Removed “assumes WS-01…WS-04 implemented” from the constitution (contracts are permanent laws, not progress reports).  
- P-01/P-02 defined once in Working Set; Lifetime **upholds** them (CL-07) without re-owning the definitions.  
- Ownership table made store/transport/chart-library agnostic.  
- Acceptance baseline recorded as L **0 / 7** at freeze (honest).  
- Working Set Contract §5 updated: lifetime is no longer “unspecified” — it has its own frozen contract.

### Duplication / conflict check

| Topic | Working Set | Cache Lifetime | Conflict? |
|-------|-------------|----------------|-----------|
| VIEW ⊆ data | WS-01 | CL-01 subordinates | No — complementary |
| No VIEW invalidation | WS-02, WS-03 | CL-01, CL-06 | No |
| Paint = VIEW | WS-04 | — | Lifetime silent (correct) |
| Pressure ≠ navigation | WS-05 | CL-06 | Aligned, not conflicting |
| P-01 / P-02 | Defined | Upheld (CL-04, CL-05, CL-07) | No duplicate ownership |
| Soft ceilings | Out of WS | Out of CL | Capacity later |

### Remaining residuals (not Lifetime Contract defects)

- Capture-null legacy prune → **Working Set** residual.  
- Capacity/Emergency numbers → intentionally unspecified.

### Acceptance score at freeze

| Scorecard | At freeze |
|-----------|-----------|
| Working Set S1–S7 | ~5 / 7 (S6–S7 Partial) — unchanged by freeze |
| Lifetime L1–L7 | **0 / 7** (law frozen; runtime not yet obedient) |

---

## 5. Intentionally unspecified

- Capacity and emergency numbers  
- Hysteresis / idle / high-water algorithms  
- Exploration-neighborhood dimensions  
- Replay, LOD, transport  
- APIs / classes / managers  

---

## 6. Evidence ledger delta — Track B Step 1 validation (Jul 2026)

**Source:** [`TRACK_B_STEP1_VALIDATION.md`](TRACK_B_STEP1_VALIDATION.md) (investigation only).

| Evidence | Delta after Step 1 |
|----------|-------------------|
| Same-op growth → discard Mutation Set → refetch | **Closed** on prepend / append-new-bar / soft applyProjection |
| Multi-op continuous roam thrash (E3-05 residual) | **Still open** — Mutation Set is operation-local by law |
| Shrink-on-narrow (E3-07 / L2) | **Still open** |
| Numeric ceilings as product walls (E3-03) | **Still open** (Capacity) |
| Mutation Set persists across ops | **Disproven** — ephemeral opts only; correct |
| WS weakened by Mutation Set | **Not observed** |

| Scorecard | Post Step 1 validation |
|-----------|------------------------|
| Lifetime L1–L7 | **3 / 7 Pass** (L1, L4, L6); L3/L5/L7 Partial; L2 Fail |
| Continuity C2 | **Partial** (same-op only) |
| Working Set S1–S5 | No Step 1 regression found |

**Recommendation recorded:** Proceed to Track B Step 2 (no Step 1 repair).

---

## 7. Evidence ledger delta — Track B Step 2 (Jul 2026)

**Source:** [`TRACK_B_STEP2.md`](TRACK_B_STEP2.md).

| Evidence | Delta after Step 2 |
|----------|-------------------|
| Multi-op fetch→prune→refetch of prior Mutation Set (E3-05 / C2) | **Closed** on covered paths via Retained Neighborhood |
| Same-op Mutation Set (Step 1) | **Maintained** |
| RN = everything ever loaded | **No** — logical hull of absorbed Mutation Sets |
| Eviction under Capacity | **Unspecified** (intentional) |
| Shrink-on-narrow (L2) | **Still open** |
| Numeric ceilings as walls (E3-03 / L7) | **Still open** |

| Scorecard | Post Step 2 |
|-----------|-------------|
| Lifetime L1–L7 | **~5 / 7 Pass** (L1, L3, L4, L5, L6); L2 Fail; L7 Partial |
| Continuity C2 | **Pass** |
| Working Set | No Step 2 regression claimed |

**Next cadence:** validate Step 2 → then plan Step 3 (do not stack).

---

## 8. Evidence ledger delta — Track B Step 2 validation (Jul 2026)

**Source:** [`TRACK_B_STEP2_VALIDATION.md`](TRACK_B_STEP2_VALIDATION.md) (investigation only).

| Evidence | Delta |
|----------|-------|
| Multi-op fetch→prune→refetch (E4 / C2) | **Validated closed** on covered paths |
| RN removes Mutation Set same-op guarantee | **Not observed** |
| RN becomes Capacity | **Disproven** (ceilings unchanged; eviction unspecified) |
| RN monotonic until world replace | **Expected Step 2**; Capacity owns future eviction |
| Hull protects interstitial times inside [min,max] | Documented design consequence — not a fail |
| L2 / L7 / C4 | Still open |

| Scorecard | Post Step 2 validation |
|-----------|------------------------|
| Lifetime L1–L7 | **5 / 7 Pass** (L1, L3–L6); L2 Partial; L7 Partial |
| Continuity C2 | **Pass** |
| Continuity C1 / C5 | Partial; C4 Fail |
| Working Set | No Step 2 regression found |

**Recommendation recorded:** Proceed to Track B Step 3 (no Step 2 repair).

---

## 9. Evidence ledger delta — Track B Step 3 implementation (Jul 2026)

**Source:** [`TRACK_B_STEP3.md`](TRACK_B_STEP3.md).

| Evidence | Delta |
|----------|-------|
| Exploration ≠ contraction (CL-02 / L2) | **Implemented** — soft projection restores omitted RN bars; RN bounds expand-only |
| Soft apply amputates RN | **Closed** |
| L7 Pass | **Not claimed** |

| Scorecard | Post Step 3 implementation |
|-----------|------------------------------|
| Lifetime L1–L7 | **≈ 6 / 7** (L2 Pass; L7 Partial) |

---

## Document status

Evidence ledger closed for Stage E4; **deltas §6–§9** appended through Track B Step 3. Guarantees live only in the frozen Contract + Acceptance documents.
