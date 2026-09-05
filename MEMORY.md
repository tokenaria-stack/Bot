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
| Packages | `market/` (state), `decision/` (contracts), `strategy/` = beacon |
| Import DAG | `exchange → market → decision → execution` |
| Timestamp | **#83 PASS** — tag `TS_CONTRACT_CLEAN` (Go A–D+E2, FE F2/F3/F5a–F5f) |
| Chart | **Frozen** — `CHART_FROZEN` + HISTORY-IDLE-PUMP-1 ✅ + SPARSE-LIVE-INGEST-1 ✅ + **SPARSE-ADR010-TIP-1 ✅** |
| NEXT | **LABEL-SET-1B** (when asked). LABEL-SET-1A frozen `690d0be` + `1433626`. ATR-TRUTH-1 frozen `84124a0`. FeatureTape 1B frozen `6715718` |
| RSX | **RSX-SIGNAL-1.1 frozen** (`b4ac2ae`) + **2A/2A.1** (`39d6f78`) + **2B** (`3f20255`) + **LEGACY-SCORE-CLEAN-1** (`08e5f28`) + **SLOT-CLEAN-1** (`af06ea8`) + **FALCON-SCORE-CLEAN-1** + **RSX-SIGNAL-3 frozen** (`c856fef`) + **RSX-VISIBILITY-1 frozen** (`749912f`) |
| Wozduh | **WOZDUH-WIRE-1 frozen** (`0c2ecce`) + **WOZDUH-ACTIVE-1A frozen** (`2cd4ca4`) + **WOZDUH-ACTIVE-1B frozen** (`1b724ef`) |

Update the owning SSOT file — do not duplicate content here.
