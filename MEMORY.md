# MEMORY — Documentation Index

**Index only (Core 6.1).** Not an architecture or rules SSOT.  
Prefer [`README.md`](README.md) as the entry point.

**Agents:** phrases like «сохрани в памяти» / «update MEMORY» mean update the **SSOT map**
(see Role rule). Do **not** turn this file back into an encyclopedia.

## Read order

Default (always / start here):

1. `.cursor/rules/jeweler-protocol.mdc` — laws
2. `.cursor/rules/senior-quant-architect.mdc` — role + memory routing
3. [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — current system
4. [`docs/OPEN_DEBTS.md`](docs/OPEN_DEBTS.md) — NEXT / backlog
5. This file — snapshot pointers only

On request / only when the task needs them:

- [`docs/DECISIONS.md`](docs/DECISIONS.md) — why a choice exists
- [`docs/HISTORY.md`](docs/HISTORY.md) — completed phases
- [`docs/WORKING_SET_CONTRACT.md`](docs/WORKING_SET_CONTRACT.md) — VIEW ↔ store ↔ paint law
- [`docs/CACHE_LIFETIME_CONTRACT.md`](docs/CACHE_LIFETIME_CONTRACT.md) — lifetime law
- Other closed `WORKING_SET_*` / `TRACK_B_*` / acceptance/evidence files — [`docs/archive/`](docs/archive/) (not default reading)

## Snapshot

| Item | Value |
|------|-------|
| Data plane | Core 5.0 Phases A–G ✅ |
| Docs | Core 6.0/6.1 + **#69A/69C** + **#80/#81 Timeline Publish Gate** ✅ |
| Default mode | `ENGINE_MODE=ChartOnly` |
| Packages | `market/` (state), `decision/` (research opinion), `strategy/` = beacon |
| Import DAG | live `exchange → market → server/web`; research `forecast → decision → decisionresearch` |
| Timestamp | **#83 PASS** — tag `TS_CONTRACT_CLEAN` (Go A–D+E2, FE F2/F3/F5a–F5f) |
| Chart | **Frozen** — `CHART_FROZEN` + HISTORY-IDLE-PUMP-1 ✅ + **WOZDUH-X-AND-CAMERA-LEAK-1** + **WOZDUH-TIME-INDEX-PIN-1** accepted. **HISTORY-ZOOM-OUT-EDGE-1 parked.** **PRICE-SERIES-STYLE-1.1** `71c929b`. |
| Chapter close | Frozen only after commit and push, with local HEAD equal to origin. The order lives in the role rule. |
| NEXT | **POPULATION-OVERLAY-PERFORMANCE-V1 frozen `18609b1`.** **RESPONSIVE-HISTOGRAM-V2 frozen `1399186`.** Discovery join `72f653c3cbf51a22cd9c3ae2bc555e9535a845f709a696ee9d1bdab1977adb8b` stays empty of named regions. Matrix digest `8c08c2a643023ee04985d2e5fed6d1cbbf285a4148e216681088f4ec49c3ec8a`. Schema 3 stays `934e2e0d584297b50f199cd102511f6ecfb93a034ce94d9b27d6046675d8eee7`. Excursion timing frozen (digest `8ca97eed20854c0503630f4880dd675f323ccca0eb6a883071e191af8bc08bd1`). Certified **PricePath** `1317724`. **RSX-COMPUTE-DEDUP-AUDIT** before the next RSX research-engine use. Chart polish parked. DATA-1B is ledger. TP-STOP-FACT-GAP-1 stays on the Brain-3 ledger. |
| RSX | **RSX-TV-ONE-BRAIN-1 frozen** `4688160`. **MARKET-RSX-PARITY-1 closed.** **FEATURE-TAPE-RSX-REGEN-1 closed.** **RESEARCH-DATASET-1 frozen** `f311203`. **VALIDATION-PLAN-1 frozen** `0737c59`. **OOF-MATRIX-1 frozen** `d749042`. **MODEL-FIT-1 frozen** `0d60270`+`29cb928`. **CALIBRATION-1 frozen** `4c8c08e`+`b550613`. **RANK-1 frozen** `cc49528`. **RECIPE-FREEZE-1 frozen** `c2d278a`. **FINAL-MODEL-FIT-1 frozen** `9cb03f5`. **FORECAST-BUNDLE-1 frozen** `a9e1228`. **OOF-FORECAST-EVIDENCE-1 frozen** `124f273`. **DECISION-VALIDATION-PLAN-1 frozen** `a0da055`. **DECISION-CONTRACT-1 frozen** `b7a76b4`. **DECISION-RESEARCH-1 frozen** `789ddcd`. Do not reopen RSX unless regression. |
| Wozduh | **WOZDUH-WIRE-1 / ACTIVE-1A / ACTIVE-1B frozen.** **WOZDUH-NUMERIC-NAMES-1** `4cb5bdc`. **WOZDUH-VOLCROSS-REMOVE-1** `b7202ab`. **WOZDUH-COLOR-OVERRIDES-1** `1c7ae3d`. **WOZDUH-STYLE-SECTION-1** `b55b64f`. **CHANNEL-SPLIT-FILLS-1** `a02aa98`. **WOZDUH-PANE-AUTOSCALE-OWNER-1** `1a409e9`. **HYDRATION-OWNERSHIP-1** `c254253`. **CROSSHAIR-PANE-HOST-1** `87fd5a8`. **RSX-PANE-AUTOSCALE-OWNER-1** `01e2a7d`. **WOZDUH-CROSSOVER-PAINT-1 frozen** `a3cab92`. **WOZDUH-FAMILY-EYE-1 frozen** `b9ae83b`. |

Update the owning SSOT file — do not duplicate content here.
