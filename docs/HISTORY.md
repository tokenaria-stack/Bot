# History (Completed Phases)

**SSOT for:** what was done in the past.  
**Read only on request** (regression archaeology, "how did we fix X?").  
Do **not** load this file for routine feature work — use `ARCHITECTURE.md` + `OPEN_DEBTS.md`.

Full pre-Core-6.0 Russian chronicle lived in `MEMORY.md`; git history retains it. This file is the English condensed canon.

---

## Core 6.0 — Documentation Cleanup (Jul 2026) ✅

- Split knowledge SSOT: Protocol / Role (always-on) vs Architecture / Debts / History / Decisions.
- Controlled English rewrite.
- `MEMORY.md` reduced to index.

---

## Core 5.0 — Data Plane SSOT (Phases A–G) ✅

**Goal:** one candle = one lifecycle = one canonical version. Trigger: Golden Audit volume drift + boot race + merge duplication (#19).

| Phase | Summary |
|-------|---------|
| **A Ingress SSOT** | `exchange/ingress.go`: Authority, merge, Validate, metrics, edge ledger. Deleted duplicate merges. |
| **B Boundary + WAL** | REST Grace 5s; monotonic UPSERT firewall; WAL checkpoint. |
| **C Boot FSM** | WS first → load → reconcile via `routeTick` → live. |
| **D Bar Source Seam** | Purge micro-candles; tick TF menu left as socket for TickBarBuilder. |
| **E Repair + repo** | `cmd/repair_volumes`; cleanup binaries/backups; `.gitignore`. |
| **F Legacy purge** | Delete ScoreEngine/trade FSM/matrix/risk/A-B CLI; keep sockets; ChartOnly default. |
| **G Rebrand** | `Frame`/`Runtime`; `market/` + `decision/`; `strategy/` beacon; DAG audit. |

---

## Core 4.4–4.10 — Continuity chain ✅

Trigger: chart holes + RSX tip spike vs TradingView at History/Live boundary.

| Shot | Summary |
|------|---------|
| 4.3 | Golden Audit: OHLC+RSX bit-identical vs REST; volume differed |
| 4.4–4.5 | FE gap-detect + WS reconnect → `loadDashboard` (no silent glue) |
| 4.6 | Tracer: bad `line_rsx` already wrong on WS |
| 4.7–4.8 | Root cause: double DAG commit on bar boundary; `lastCommittedOpenTime` fix |
| 4.9 | TF interval shim was hardcoded 60s → false gap storms |
| 4.10 | Cold boot camera: width-independent `applyOptions` only |

---

## Core 4.0 — Great Purge ✅

Wozduh math → DAG bus (`WozduhNode` + slots); Falcon Evaluate gated; tip via plots; TV floating UI (legend chrome only). Keep `falcon.go` until ScoreNodes (#76).

---

## Core 3.5 — Projection (11A–11E) ✅

Tip Ownership; `projectionEpoch`; Atomic TF handoff; Sticky Live Edge / Microscope; delta integrity. **TF mechanics CLOSED.**

---

## Core 3.0 — Frontend (10A–10B) ✅

`ScaleController` Auto/Log SSOT; zero-gap WS-first handoff around monolith replace.

---

## Core 2.3 — Data Foundation (Shots 9A–9J) ✅

| Shot | Summary |
|------|---------|
| 9A | `HistoryProvider.GetWindow` = SQLite ∪ RAM |
| 9B | Atomic `BroadcastChartTick` all TFs |
| 9C | `PersistenceQueue` sole runtime UPSERT |
| 9D | SQLite tip catch-up independent of RAM |
| 9E | Sterile `FetchClosedRange`; delete synthesize path |
| 9F | `EngineMode` ChartOnly \| Live |
| 9H–9J | Navigators/annotations/header from DAG; Falcon off delivery path |

**UI paint laws (still current):** ChartAdapter-only LWC; paint via window; RenderScheduler sole initiator; Store → Window → Geometry → Series → Adapter.

---

## Core 2.0 — DDR / DAG shadow ✅

DAG dual-write; columnar history hydration; frontend DDR cutover; projector annotations path.

---

## Earlier frontend SSOT (Phase 19.5–20 / 28)

Pre-fetch assembly; ViewportManager time anchors; store seal guards; backtest view-plane split; black screen root cause = HTML tab nesting (`#tab-backtest` under hidden `#tab-live`).

---

## How to use this file

- Hunting a regression → search phase/shot name, then open the files listed in that era's commits.
- Asking "why?" → prefer `DECISIONS.md`; use History for chronology.
- Closed debt numbers → mentioned in phase tables; open ones live only in `OPEN_DEBTS.md`.
