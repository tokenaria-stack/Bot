# Gate 3 — Working Set UX Acceptance (U1–U7)

**STATUS:** FAIL  
**Kind:** Real UX acceptance on live dashboard. No code. S1–S6 not reopened as architecture gates.  
**Contract:** [`WORKING_SET_ACCEPTANCE.md`](WORKING_SET_ACCEPTANCE.md) §4 only.

---

## HEADLINE

**UX ACCEPTANCE: FAIL**

Store-side roam can exceed 12k/16k without TARGET thrash, and several scenarios look healthy in metrics — but the live chart still breaks for the user: painted series stays near the **15k `RENDER_WINDOW_LIMIT`**, the camera clamps there with a large right void / few stuck candles, and return-to-live fails after pressure drops the market tip (`windowMode=history`, tip lag ≈ 3000 bars).

S1–S6 scorecard evidence remains; Gate 3 does **not** auto-pass from it.

---

## Test conditions

| Field | Value |
|-------|--------|
| Runtime | `go run .` → `http://127.0.0.1:8080/` |
| Symbol / TF | BTCUSDT **15m** |
| Start bars | ~3001 (hydrate) |
| Caps (unchanged) | `TARGET=12000`, `HARD_CAP=16000`, paint soft `RENDER_WINDOW_LIMIT=15000` |
| Method | Live page + LWC `setVisibleLogicalRange` / wheel-arm + real Hydration prepends; CDP probes; screenshots |
| Peak store | **~39k bars**, `hasMore=true`, span ≈ **340+ days** |
| Stress | Crossed 12k and 16k; continued beyond; zoom out/in; chunk while zoomed; move viewport after chunk; scroll toward tip |

---

## Scorecard U1–U7

### U1 — Rapid zoom out then hold
**STATUS:** PASS  

**SCENARIO:** Zoom out repeatedly while in history roam, then hold.  

**OBSERVED:** Span increased (~60 → ~2000 logical); `tipInView` stayed false; no jump to live; VIEW covered by store.  

**EVIDENCE:** CDP sample `u1-pre` / `u1-post`; store ~27k at sample.  

**CLASSIFICATION:** Working Set / Camera (preserve).  

**REGRESSION:** No S1–S6 invariant break observed in this step.

---

### U2 — Rapid zoom in after U1
**STATUS:** PASS  

**SCENARIO:** Zoom back in after U1.  

**OBSERVED:** First measurement flagged `leftRetained=false` because another prepend moved `firstTimeSec` **older** (extra history), not amputation. Recheck: zoom-in did not raise `firstTimeSec`; bars retained; VIEW ok.  

**EVIDENCE:** CDP `U2_recheck` (`leftAmputated: false`).  

**CLASSIFICATION:** Lifetime (may retain extra) / Working Set.  

**REGRESSION:** None.  

**Correction:** “Left changed” ≠ fail if first only moves older under Lazy Contract.

---

### U3 — Fast left-scroll with continuous prepend
**STATUS:** FAIL  

**SCENARIO:** Arm scroll, pin VIEW near left, load many history chunks.  

**OBSERVED:** Chunks **did** load (store 12k → 24k+; later ~39k); no measured rightward tip-yank during prepend steps; left void filled. **Later** the chart showed the classic failure shape: only ~2 candles on the left of the frame and a large empty region to the right, with LWC logical range clamped near **~15000** while the store held ~39k.  

**EVIDENCE:** Roam trail `grewSteps≥3`, `jumps=0`; screenshots `page-2026-08-09T02-23-06` / `02-23-33` / `02-24-04`; CDP `logicalToCoordinate` times null past ~15033; `extractWindow` soft len **15000** vs store **38987**.  

**CLASSIFICATION:** **Paint** (primary) — soft paint window vs store; Camera clamp secondary.  

**REGRESSION:** Does not refute S3 retention; **S4 UX** (“paint represents committed VIEW / continuous explore”) fails in practice once store ≫ 15k.

---

### U4 — Cross former 16k stress
**STATUS:** FAIL  

**SCENARIO:** Continue loading through and past 12k/16k; move viewport after chunk.  

**OBSERVED:** Store monotonic across 12k/16k indices; no TARGET thrash cycle on store. User-visible navigation still hits a **~15k paint ceiling** (near the old walls).  

**EVIDENCE:** `crossed12/16=true`, `mono=true`, `boundary=true`; series tip clamp ~15000; `RENDER_WINDOW_LIMIT=15000`.  

**CLASSIFICATION:** **Paint** / Product (artificial explore wall via paint soft limit).  

**REGRESSION:** S6 (store P-01) holds; Gate 3 continuity does not.

---

### U5 — Long roam
**STATUS:** FAIL  

**SCENARIO:** Keep loading well beyond 16k while `hasMore=true`.  

**OBSERVED:** Store reached ~39k, ~340 days, `hasMore=true` (no FE bar-budget EOF). Exploration **camera/paint** did not remain faithful for free roam to the live edge.  

**EVIDENCE:** Final store metrics; tip unreachable via LWC range (`canReachStoreTip: false` after interventions).  

**CLASSIFICATION:** Paint / Camera.  

**REGRESSION:** No false EOF (Wave 3 / S6 store); UX roam still broken.

---

### U6 — Zoom out near retention pressure
**STATUS:** PASS  

**SCENARIO:** Zoom out with store already over `HARD_CAP`; trigger another prepend.  

**OBSERVED:** `barCount > 16000`; previously visible VIEW times still in store (`missing=0`); no camera yank to tip in that step.  

**EVIDENCE:** CDP `U6` sample (`overCap`, `missing: 0`, `yank6: false`).  

**CLASSIFICATION:** Working Set (WS-02/03 under pressure).  

**REGRESSION:** None for retention.

---

### U7 — Return toward live tip by user scroll
**STATUS:** FAIL  

**SCENARIO:** User-driven scroll right toward live after long left roam.  

**OBSERVED:** No spontaneous mid-roam FreshLive yank detected early; **could not** place VIEW on true live tip. `windowMode=history`; store tip lag ≈ **3001** fifteen-minute bars (live tip previously dropped by `FROM_NEWEST` pressure); LWC range repeatedly forced/clamped near **14998** even after full `applyFullData`. Screenshot: sparse candles + large right void.  

**EVIDENCE:** CDP `U7` / `U7b`; `tipLagBars≈3001`; `mode=history`; screenshots above.  

**CLASSIFICATION:** **Paint/Camera** (clamp) + **Capacity/Lifetime** (tip trimmed under pressure) + Product (return-to-live).  

**REGRESSION:** Wave 1 (no Boot FreshLive) held; U7 still fails.

---

## Summary table

| ID | STATUS |
|----|--------|
| U1 | PASS |
| U2 | PASS |
| U3 | FAIL |
| U4 | FAIL |
| U5 | FAIL |
| U6 | PASS |
| U7 | FAIL |

**Gate 3:** FAIL (4 / 7)

---

## Root chain (proven)

```text
left roam prepends
  → store grows past HARD_CAP (RN keeps left; FROM_NEWEST may drop market tip)
  → ChartCompositor.extractWindow(..., RENDER_WINDOW_LIMIT=15000, VIEW)
  → LWC setData ≈ 15k series (window-local logical space)
  → visible range / preserve interacts poorly once store ≫ 15k
  → camera clamps near ~15000; right void; few stuck candles
  → user cannot return to true live tip (U7)
```

**Not** a reintroduction of commit-paired TARGET amputate (S6).  
**Not** false `hasMore` EOF (Wave 3).

---

## Classification of failure (for repair)

| # | Kind | Applies? |
|---|------|----------|
| 1 | Genuine Working Set Contract (retention VIEW⊆store) | **No** for U6/roam retention |
| 2 | CameraCommit / preservation | **Yes** — range clamp / void |
| 3 | Hydration intent/completion | **No** — chunks loaded; hasMore true |
| 4 | Paint / window selection | **Yes — primary** (`RENDER_WINDOW_LIMIT` vs store) |
| 5 | Performance limitation | Possible contributor at 39k; not required to explain 15k clamp |
| 6 | New product issue | Return-to-live after tip prune |
| 7 | Test noise | Partial on U2 first metric; terminal FAIL is real |

---

## Minimal repair plan (NO CODE this gate)

1. **Paint/camera seam (smallest):** On every windowed `setData`, re-bind visible logical range from **committed VIEW times** into the **painted series index space** (DataResolve already exists). Never leave LWC logical coordinates implying a store longer than the painted series.  
2. **Until LOD/Capacity exists:** Prefer painting a window that covers VIEW **and** does not strand the user at a soft 15k “wall,” or temporarily paint full snapshot when `barCount` is in the 15k–40k band if remap is incomplete.  
3. **Do not** raise HARD_CAP/TARGET as the fix; do not start RESET_LIVE / ReplayDAG / LOD programmes in the same change.  
4. **U7 follow-up (separate, if still broken after 1–2):** tip drop under `FROM_NEWEST` while `windowMode=history` blocks live append — product/Lifetime policy, not a silent CameraCommit.

STOP after this report — no implementation in this gate.

---

## Corrections noted

- Orchestrator is **not** on `window`; real loads go through Boot `scheduleHistoryLoad` + LWC range changes. Direct `noteLeftHistoryIntent` from page is unavailable without Boot exposure.  
- U2 must not fail merely because `firstTimeSec` moves older when history arrives during zoom.  
- “Store > 16k” alone is insufficient for Gate 3; paint/camera fidelity is the acceptance question.
