# MEMORY — Documentation Index

**This file is an index only (Core 6.0).** It is not an architecture or rules SSOT.

Before new modules, read in this order:

1. `.cursor/rules/jeweler-protocol.mdc` — engineering laws (always-on)
2. `.cursor/rules/senior-quant-architect.mdc` — role (always-on)
3. [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — current system
4. [`docs/OPEN_DEBTS.md`](docs/OPEN_DEBTS.md) — NEXT / backlog

On request only:

- [`docs/HISTORY.md`](docs/HISTORY.md) — completed phases
- [`docs/DECISIONS.md`](docs/DECISIONS.md) — why key choices were made
- [`README.md`](README.md) — landing / build

## Snapshot

| Item | Value |
|------|-------|
| Data plane | Core 5.0 Phases A–G ✅ |
| Docs | Core 6.0 Documentation Cleanup ✅ |
| Default mode | `ENGINE_MODE=ChartOnly` |
| Packages | `market/` (state), `decision/` (contracts), `strategy/` = beacon |
| Import DAG | `exchange → market → decision → execution` |
| NEXT | **#76 ScoreNodes**, **#67 Live Confirm** |

Do not duplicate Protocol, Architecture, or Debts here. Update the owning SSOT file instead.
