# Step 1 Completion Audit — Working Set Retention Coverage

**Status:** Re-verified (post Track A Step 2 + Track B Steps 1–3).  
**Kind:** Coverage classification only.  
**Not:** viewport paint, ReplayDAG, RESET_LIVE, APIs, redesign, Capacity.

**Step 1 contract goal (unchanged):**

> **Prune must not invalidate the committed VIEW** (WS-01 / WS-02 / WS-03).

That goal applies to **every** mutation that can prune — not only `prependMonolith`.

---

## Classification rules

| Class | Meaning |
|-------|---------|
| **Preserve-paired** | Mutation must keep the user’s current VIEW. Must supply VIEW bounds into `_enforceBudget`. |
| **Commit-paired** | Explicit CameraCommit / intentional world replace (TF, FreshLive, `loadDashboard`). VIEW omit allowed. |
| **Legacy / unreachable** | Not on the live Boot path; documented only. |

**No unclassified `_enforceBudget` caller may remain.**

---

## Store budget call sites (only three)

All `_enforceBudget` invocations live inside `ColumnarStore`:

1. `prependMonolith` (after successful add)  
2. `appendTick` (new bar only; tip overwrite does **not** budget)  
3. `applyProjection` / `replaceMonolith` (alias)

Indirect callers = Boot / hydration / legacy that invoke those three.

---

## Coverage table (live Boot)

| Caller | Preserve / Commit | VIEW supplied? | Step 1 protect? | Legacy `_pruneToCount` if unprotected? | WS-safe? | Action |
|--------|-------------------|----------------|-----------------|----------------------------------------|----------|--------|
| `prependMonolith` ← Boot `mergeIntoStore` | Preserve | Yes (`captureStoreViewTimes`) | Yes | Only if capture **and** Mutation/RN absent | Yes on Boot | Classified |
| `appendTick` ← Boot `pushLiveTickDelta` | Preserve | Yes | Yes | Same residual | Yes on Boot | Classified |
| Soft `applyProjection` ← `reloadLiveForRsxSettings` | Preserve (existing world) | Yes | Yes | Same residual | Yes on Boot | Classified |
| Soft fallback `replaceMonolith` (same path) | Preserve | Yes | Yes | Same residual | Yes on Boot | Classified |
| `replaceMonolith` ← `loadDashboard` | **Commit** | No (intentional) | N/A — world replace | Yes — tip hydrate after clear RN | Yes (commit-paired) | Classified |
| `appendTick` same-bar overwrite | N/A (no budget) | — | — | — | Yes (WS-03 tip update) | None |
| `updatePlots` | N/A (no budget) | — | — | — | Yes | None |
| `clear` | Commit-paired wipe | — | — | — | Yes iff world replace follows | Classified |
| Hydration merge | Via Boot prepend | Yes | Yes | — | Yes | Classified |
| Delta / Adapter paint | No store prune | — | — | — | N/A | None |
| `app.legacy.js` prepend/append/replace | **Legacy** | No | No | Yes | Not live Boot | Documented |

---

## Critical refinement (caller discipline)

`_enforceBudget` is contract-aware **only if** protected sets resolve:

```text
view = VIEW opts
mutation = Mutation Set opts (Track B Step 1)
neighborhood = Retained Neighborhood state (Track B Step 2)

if (!view && !mutation && !neighborhood) → legacy _pruneToCount
else → _pruneOutsideProtected(VIEW ∪ Mutation ∪ RN)
```

### What this means

| Claim | Verdict |
|-------|---------|
| Store “always” knows VIEW | **False** — VIEW is caller-supplied |
| Mutation / RN substitute for VIEW on preserve-paired paths | **False** — they protect growth/neighborhood, not necessarily the committed VIEW |
| Boot preserve-paired paths supply VIEW | **True** (when `captureStoreViewTimes` succeeds) |

**Capture-null residual:** if LWC range is unavailable, Boot passes `undefined` VIEW. Growth still establishes Mutation + RN, so prune is often *not* full legacy `_pruneToCount` — but VIEW bars **outside** Mutation ∪ RN can still be eligible. That is a conditional WS-01 hazard, not a missing classification.

Prefer fail-soft (omit VIEW) over inventing VIEW. Closing capture-null is a separate hardening item — not an unclassified path.

---

## Preserve vs replace (applyProjection)

| Intent | Class | Example |
|--------|-------|---------|
| Preserve existing world (settings sync, soft restore) | Preserve-paired | Boot soft `applyProjection` + VIEW |
| Replace world (TF / FreshLive / dashboard hydrate) | Commit-paired | `loadDashboard` → `replaceMonolith(..., { commitPaired: true })` |

Mixing these is a contract error. Current Boot keeps them distinct.

---

## Remaining contract gaps (retention only)

| Gap | Genuine WS violation? | Notes |
|-----|----------------------|-------|
| Unclassified Boot `_enforceBudget` caller | **No** — all classified | — |
| Preserve-paired Boot without intentional VIEW wiring | **No** — closed | — |
| Capture-null → VIEW omitted on preserve-paired | **Conditional residual** | Not missing classification; harden later if needed |
| `app.legacy.js` without VIEW | Legacy only | Not live Boot |
| Paint / WS-04 | Out of this audit | Already addressed in Track A Step 2 |
| Lifetime / HARD_CAP product walls | Out of this audit | Track B / later |

**No additional preserve-paired coverage implementation required on the live Boot path.**

---

## Corrections / details noted

1. Naming: this is a **Step 1 Completion Audit**, not “Step 1.5” — Step 1’s goal was VIEW-safe prune everywhere, not “protect prepend only.”  
2. Post–Track B: Mutation/RN reduce how often legacy `_pruneToCount` runs, but **do not** retire the requirement that preserve-paired callers supply VIEW.  
3. Tip overwrite and `updatePlots` correctly never call `_enforceBudget`.  
4. Empty-store `_gatherIndices` clears RN — world wipe, not an unclassified prune caller.

---

## Verdict

> **Working Set retention layer complete** for all classified live Boot prune paths.

- Every `_enforceBudget` path is Preserve-paired, Commit-paired, or Legacy.  
- Preserve-paired Boot mutations supply VIEW when capture succeeds.  
- Commit-paired world replace is explicit (`commitPaired: true`).  
- Residual: capture-null (conditional), not unclassified.

**Paint (WS-04) is a separate layer** and must assume only: the store satisfies the Working Set Contract — not TARGET, HARD_CAP, prune direction, or Lifetime internals.

No code changes in this audit. No paint. No redesign.
