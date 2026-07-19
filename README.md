# Trading Bot

Institutional **streaming-first** market analysis engine for **Binance USDⓈ-M Futures**.

- NOT a TradingView clone
- NOT a retail signal bot
- NOT an indicator dump

Primary goal: deterministic market state (`market.Frame` / `Runtime`).  
Go data plane + DAG indicators + DDR Lightweight Charts.  
Default: `ENGINE_MODE=ChartOnly`.

## Read order

1. Protocol — [`.cursor/rules/jeweler-protocol.mdc`](.cursor/rules/jeweler-protocol.mdc) *(always-on)*
2. Role — [`.cursor/rules/senior-quant-architect.mdc`](.cursor/rules/senior-quant-architect.mdc) *(always-on)*
3. Architecture — [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
4. Open debts — [`docs/OPEN_DEBTS.md`](docs/OPEN_DEBTS.md)
5. Decisions — [`docs/DECISIONS.md`](docs/DECISIONS.md) *(on request)*
6. History — [`docs/HISTORY.md`](docs/HISTORY.md) *(on request)*

[`MEMORY.md`](MEMORY.md) is a short index alias — prefer this README as the entry point.

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

## Status

- **Core 5.0** data plane (Phases A–G) ✅
- **Core 6.0 / 6.1** documentation OS ✅
- **NEXT:** #76 ScoreNodes, #67 Live Confirm — [`docs/OPEN_DEBTS.md`](docs/OPEN_DEBTS.md)
