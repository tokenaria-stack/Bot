# MEMORY — Documentation Index

**Index only (Core 6.1).** Not an architecture or rules SSOT.  
Prefer [`README.md`](README.md) as the entry point.

**Agents:** phrases like «сохрани в памяти» / «update MEMORY» mean update the **SSOT map**
(see Role rule). Do **not** turn this file back into an encyclopedia.

## Read order

1. `.cursor/rules/jeweler-protocol.mdc` — laws (always-on)
2. `.cursor/rules/senior-quant-architect.mdc` — role + checklist + memory routing (always-on)
3. [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — current system
4. [`docs/WORKING_SET_CONTRACT.md`](docs/WORKING_SET_CONTRACT.md) — Working Set normative contract (VIEW ↔ store ↔ paint)
5. [`docs/WORKING_SET_ACCEPTANCE.md`](docs/WORKING_SET_ACCEPTANCE.md) — Working Set done-criteria
6. [`docs/CACHE_LIFETIME_CONTRACT.md`](docs/CACHE_LIFETIME_CONTRACT.md) — Cache Lifetime normative contract (retention beyond VIEW)
7. [`docs/CACHE_LIFETIME_ACCEPTANCE.md`](docs/CACHE_LIFETIME_ACCEPTANCE.md) — Lifetime done-criteria
8. [`docs/OPEN_DEBTS.md`](docs/OPEN_DEBTS.md) — NEXT / backlog

On request: [`docs/DECISIONS.md`](docs/DECISIONS.md), [`docs/HISTORY.md`](docs/HISTORY.md).

## Snapshot

| Item | Value |
|------|-------|
| Data plane | Core 5.0 Phases A–G ✅ |
| Docs | Core 6.0/6.1 + **#69A/69C** + **#80/#81 Timeline Publish Gate** ✅ |
| Default mode | `ENGINE_MODE=ChartOnly` |
| Packages | `market/` (state), `decision/` (contracts), `strategy/` = beacon |
| Import DAG | `exchange → market → decision → execution` |
| NEXT | Forming-bar tip vs TV, **#76 ScoreNodes**, Lifetime Track B Step 2+ / Capacity |

Update the owning SSOT file — do not duplicate content here.
