# Trading Bot

Enterprise multi-timeframe crypto trading bot for **Binance USDⓈ-M Futures**.

Go data plane + DAG indicators + DDR Lightweight Charts frontend.  
Default: `ENGINE_MODE=ChartOnly` (charts / data plane only).

## Documentation (SSOT map)

| Document | Purpose | When to read |
|----------|---------|--------------|
| [`.cursor/rules/jeweler-protocol.mdc`](.cursor/rules/jeweler-protocol.mdc) | Engineering laws | Always (Cursor always-on) |
| [`.cursor/rules/senior-quant-architect.mdc`](.cursor/rules/senior-quant-architect.mdc) | Engineer role | Always (Cursor always-on) |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Current system | Before new modules |
| [`docs/OPEN_DEBTS.md`](docs/OPEN_DEBTS.md) | Backlog / NEXT | Before planning work |
| [`docs/HISTORY.md`](docs/HISTORY.md) | Completed phases | On request / regressions |
| [`docs/DECISIONS.md`](docs/DECISIONS.md) | Why decisions were made | On request |
| [`MEMORY.md`](MEMORY.md) | Index only | Pointers |

## Quick start

```bash
cp .env.example .env   # Futures keys
go build ./...
go test ./market/ ./server/ ./server/wire/ ./execution/ ./core/ ./indicators/ -count=1 -skip GoldenAudit
go run .               # dashboard :8080
```

## Package layout

```
exchange/     transport + Ingress (Authority, merge)
data/         SQLite + PersistenceQueue
market/       Frame, Runtime, streaming/snapshot, Boot, falcon bus
decision/     ScoreDecision / ScoreFactor contracts
execution/    position sizing
indicators/   streaming math (no go-talib)
core/         DAG runner + nodes
server/       HTTP/WS projection
web/          DDR charts (boot.js)
strategy/     doc.go beacon only
vector_db/    Qdrant socket
```

**Import DAG:** `exchange → market → decision → execution`

## Indicators baseline

Streaming-first (`Indicator` / `CandleIndicator`), O(1) memory, no `go-talib`.  
Details: `indicators/` + `docs/ARCHITECTURE.md`.

## Status

- **Core 5.0** data plane (Phases A–G) ✅  
- **Core 6.0** documentation cleanup ✅  
- **NEXT:** #76 ScoreNodes, #67 Live Confirm — see `docs/OPEN_DEBTS.md`
