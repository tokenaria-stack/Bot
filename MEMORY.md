# MEMORY — Documentation Index

**Index only (Core 6.1).** Not an architecture or rules SSOT.  
Prefer [`README.md`](README.md) as the entry point.

## Read order

1. `.cursor/rules/jeweler-protocol.mdc` — laws (always-on)
2. `.cursor/rules/senior-quant-architect.mdc` — role + before-coding checklist (always-on)
3. [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — current system
4. [`docs/OPEN_DEBTS.md`](docs/OPEN_DEBTS.md) — NEXT / backlog

On request: [`docs/DECISIONS.md`](docs/DECISIONS.md), [`docs/HISTORY.md`](docs/HISTORY.md).

## Snapshot

| Item | Value |
|------|-------|
| Data plane | Core 5.0 Phases A–G ✅ |
| Docs | Core 6.0 + **6.1 polish** ✅ |
| Default mode | `ENGINE_MODE=ChartOnly` |
| Packages | `market/` (state), `decision/` (contracts), `strategy/` = beacon |
| Import DAG | `exchange → market → decision → execution` |
| NEXT | **#76 ScoreNodes**, **#67 Live Confirm** |

Update the owning SSOT file — do not duplicate content here.
