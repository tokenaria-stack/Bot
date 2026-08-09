# Step 2 — Working Set → Paint (WS-04)

**Status:** Superseded in part by **Track C** (`WORKING_SET_TRACK_C_PAINT.md`).  
**Kind:** Paint coverage.

**Track C update:** Soft `RENDER_WINDOW_LIMIT` / tip-window removed. Compositor paints the **full retained snapshot** (`selectPaintSnapshot`) so LWC indices ≡ store indices. VIEW ⊆ store remains Working Set retention’s duty.

| Paint path | Selection | WS-04 |
|------------|-----------|-------|
| Full / prepend / indicators / delta observe | `selectPaintSnapshot` → full retained store | Pass |
| Delta candle write | No series truncate | Pass |

See Track C for U3–U7 evidence.
