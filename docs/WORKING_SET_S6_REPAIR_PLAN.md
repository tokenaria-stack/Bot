# S6 Repair Plan — Remove Artificial History Boundary

**STATUS:** DONE (implemented; S6 PASS)  
**Kind:** Plan approved → code in `web/columnar-store.js` `applyProjection`.  
**Frozen:** Working Set Contract/Acceptance, Lifetime & Capacity Constitution, S1–S5.

Evidence: `docs/WORKING_SET_S6.md` (PASS).

---

## CURRENT PATH (pre-repair — closed)

```text
loadDashboard / TF·FreshLive hydrate
  → replaceMonolith(..., { commitPaired: true })
  → applyProjection: clear RN; _enforceBudget → _pruneToCount(TARGET)
  → artificial ~12k history boundary
```

## POST-REPAIR PATH

```text
commitPaired accept
  → applyProjection: replace series; clear RN; skip _enforceBudget
  → retained = full payload
  → hasMore from payload unchanged
preserve-paired / soft / append / prepend
  → budget / VIEW∪Mutation∪RN unchanged
```

---

## ROOT CAUSE

Legacy cache pressure on commit-paired world accept with empty protected set → TARGET as left edge of new world. Conflicts with Lifetime & Capacity Rules **1, 2, 8** and **P-01**.

---

## MINIMAL REPAIR (shipped)

| Item | Detail |
|------|--------|
| File | `web/columnar-store.js` — `applyProjection` |
| Change | `commitPaired`: clear RN then **return** (skip `_enforceBudget`) |
| Not done | Cap retune; managers; Camera/paint/EOF/hydration changes |

---

## ACCEPTANCE

| Test | Result |
|------|--------|
| commitPaired N > HARD_CAP → barCount === N | **PASS** |
| first/last = payload ends; hasMore unchanged; RN cleared | **PASS** |
| preserve-paired pressure unchanged | **PASS** |
| budget / S4 / Wave 1–3 / time_camera | **PASS** |

---

## OUT OF SCOPE (still deferred)

Numeric Cap tuning · LOD · ReplayDAG · RESET_LIVE · browser FPS benchmark · Emergency policy

---

## STOP

Repair complete. No further S6 work.
