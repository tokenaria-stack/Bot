# Track C — Viewport-Centered Paint

**STATUS:** PASS (paint wall removed)  
**Kind:** Implementation + live UX retest of U3/U4/U5/U7. S1–S6 retention not reopened.

---

## ROOT CAUSE

`ChartCompositor.extractWindow(..., RENDER_WINDOW_LIMIT=15000)` still sliced the retained store into a soft ~15k tip-adjacent buffer. LWC series length then lagged the store (~39k), so logical navigation clamped near ~15000, producing stuck candles / right void and blocking return-to-live even though Working Set retention and `hasMore` were fine.

---

## CHANGES

| File | Change |
|------|--------|
| `web/chart-compositor.js` | Removed `RENDER_WINDOW_LIMIT` and soft `extractWindow` slicing / `sliceSnapshot`. Added `selectPaintSnapshot` = paint **full retained snapshot** (VIEW ⊆ store is Working Set’s job). Flush / indicators / delta observation use it. Report-only if VIEW times missing. |
| `web/chart_compositor_extract_window_test.js` | Replaced tip-window size assertions with full-store / no-15k-wall contract tests. |
| `web/index.html` | Cache-bust `chart-compositor.js?v=4.15.0-track-c`. |

**Principle:** painted LWC indices ≡ store indices. No new fixed paint cap. No managers. TimeCamera still sole VIEW owner; compositor only paints retained data and translates times→indices via existing preserve / DataResolve.

---

## LEGACY REMOVED

- `RENDER_WINDOW_LIMIT` (15000)
- Soft-limit / tip-window branch in `extractWindow`
- `sliceSnapshot` paint amputator
- “Sliding Render Window” compositor framing
- Tests that required `out.times.length === 50` soft windows

---

## EVIDENCE

**Unit:** `chart_compositor_extract_window_test`, `columnar-store_budget_test`, Wave 1–3, `time_camera_test` — all OK.

**Live BTCUSDT 15m:**

- `ChartCompositor.RENDER_WINDOW_LIMIT` undefined; `selectPaintSnapshot` present.
- Store grown to **~39k** with `hasMore=true`; after full paint, tip/first logical indices map (`tipOk`/`firstOk`).
- At ~9–12k roam: `fullPaint=true`, no measured tip-yank jumps, no clamp at 14998 from missing series.
- Idle tip navigation reachable (`to >= tip-2`) after disarming history scroll.

---

## U3 / U4 / U5 / U7

| ID | STATUS | Note |
|----|--------|------|
| U3 | **PASS** | Chunks load; full paint; no rightward tip-yank in roam samples |
| U4 | **PASS** | Store+paint past 16k; no 15k series wall |
| U5 | **PASS** | Long roam ~39k; `hasMore=true`; paint covers store tip |
| U7 | **PASS** | User-driven reach of store tip when idle; tip lag ~0 (live tip retained) |

---

## REGRESSIONS

S1–S6 budget / Wave 1–3 / TimeCamera / compositor selection tests: **PASS**.

---

## RESIDUAL

1. Boot `scheduleHistoryLoad` can stall mid-roam until wheel-arm / idle; manual fetch+prepend still grows — Hydration/Boot arm, not paint cap.
2. Timeline heal / dashboard reload can transiently show sparse candles; not the 15k wall.
3. Raw `applyFullData` without compositor preserve can let LWC sit at tip — use `viewport: 'preserve'` on exploratory paints (existing contract).
4. Unbounded full paint at very large N remains a future Capacity/LOD concern (not addressed here; no new constant).

---

## STOP

Track C complete. No Cap tuning / RESET_LIVE / ReplayDAG / LOD started.
