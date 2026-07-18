# Trading Bot — Pipeline Baseline

Enterprise MTF crypto trading bot for **Binance USDⓈ-M Futures**.  
Architecture reference: [`MEMORY.md`](MEMORY.md) (full system rules and FSM).

## Pipeline Architecture (Layer 1) — ✅ Baseline Locked

**Layer 1 is complete.** All math in `indicators/` is **streaming-first**: each tick updates state in **O(1) memory** (ring buffers, running sums, or single previous-value stores). No `go-talib`. No batch-only dependencies on the live path.

### Core interfaces

| Interface | Contract | Use when |
|-----------|----------|----------|
| `Indicator` | `Update(val float64) float64`, `Value() float64` | Single-value streams (price, RSI output, hl2, …) |
| `CandleIndicator` | `UpdateCandle(h, l, c float64) float64`, `Value() float64` | OHLC-based (ATR, Stochastic, AD, …) |

**Composability:** any `Indicator` output can feed another `Indicator`:

```go
rsi  := indicators.NewRSI(14)
macd := indicators.NewMACD(12, 26, 9)

for _, price := range prices {
    macdLine := macd.Update(rsi.Update(price))
}
```

### Module inventory (`indicators/`)

| File | Streaming types | Batch wrappers | Memory |
|------|-----------------|----------------|--------|
| `types.go` | `Indicator`, `CandleIndicator` | — | — |
| `ma.go` | `SMA`, `EMA`, `RMA` | `SMAValues`, `EMAValues` | Ring / prev |
| `utils.go` | `RollingSum`, `RollingStDev` | `ExtractPrices` | Ring |
| `oscillators.go` | `RSI`, `MACD`, `Stochastic` | `RSIValues`, `MACDValues`, `StochValues` | RMA / EMA / ring |
| `volatility.go` | `BollingerBands`, `ATR` | `BollingerBandsValues`, `ATRValues` | SMA+StDev / RMA |
| `chaos.go` | `AO`, `WilliamsFractals` | `AOValues`, `AOValuesFromKlines`, `WilliamsFractalPeaks` | SMA / 5-bar ring |
| `volume.go` | `AD`, `CumSum`, `VolumeWeightedEMA` | — | cumulative / dual EMA |
| `zigzag.go` | `ZigZag` (ATR + fractals + RSI sensitivity) | — | delegates to ATR/fractals |
| `fibonacci.go` | — | `CalcRetracements` | pure function |
| `peaks.go` | — | `FindExtremes`, `FilterPeaksByATR` | batch analysis |
| `divergence.go` | — | divergence detectors (classes A/B/C) | batch analysis |

### Extended APIs (not `Indicator`, by design)

| Type | Entry method | Notes |
|------|--------------|-------|
| `AO` | `Update(hl2 float64)` | Input is median price `(H+L)/2`; satisfies `Indicator` |
| `WilliamsFractals` | `UpdateCandle(high, low) FractalStatus` | 5-candle ring; returns fractal flags for center bar |
| `ZigZag` | `UpdateCandle(h, l, c, rsi) ZigZagUpdate` | Adaptive ATR filter; `Value()` = last node price |
| `VolumeWeightedEMA` | `Update(price, volume float64)` | `EMA(P×V) / EMA(V)` |
| `MACD` | + `Signal()`, `Histogram()` | Extra outputs beyond `Value()` |
| `Stochastic` | + `K()`, `D()` | `Value()` = slow %K |
| `BollingerBands` | + `Bands()` | `Value()` = middle band |

### Standard for new indicators

1. Implement `Indicator` or `CandleIndicator` with **O(1) memory**.
2. Use ring buffers (`SMA`, `RollingStDev`, `Stochastic`) or prev-value stores (`EMA`, `RMA`).
3. Export `NewXxx(...)` constructor and compile-time check: `var _ Indicator = (*Xxx)(nil)`.
4. Add `XxxValues(...)` batch wrapper only for history replay / tests — not for live ticks.
5. **No** trading logic, exchange imports, or external TA libraries.

---

## Layer roadmap

```
indicators/     ✅ Streaming math (no talib)
market/         ✅ Frame + Runtime + streaming/snapshot (data plane — "what is happening?")
decision/       🔜 ScoreDecision/ScoreFactor contracts; future strategies ("what to do?")
execution/      ✅ Position sizing sockets
strategy/       🏷 doc.go beacon only (Phase F purged legacy)
```

| Package | Status | Role |
|---------|--------|------|
| **indicators** | ✅ | Streaming math; consumed by `market.Frame` |
| **market** | ✅ | `Frame` / `Runtime` / Boot / MTF / chart replay |
| **decision** | 🔜 sockets | Contracts only until #76 ScoreNodes + new strategies |
| **strategy** | placeholder | No active code — see `decision/` |

**Import DAG:** `exchange → market → decision → execution` (one-way).

---

## Quick start

```bash
cp .env.example .env   # Futures Testnet keys
go build ./...
go test ./...
go run .
```

## Project layout

```
trading_bot/
├── README.md              # This file — Pipeline Baseline
├── MEMORY.md              # Full architecture (Core 5.0 Phases A–G)
├── main.go
├── config/
├── exchange/              # Transport + ingress (Bar Source Seam)
├── market/                # Frame, Runtime, streaming/snapshot, falcon bus
├── decision/              # ScoreDecision / ScoreFactor contracts
├── execution/             # Position sizing
├── indicators/            # Streaming math
├── vector_db/             # Qdrant pattern memory (socket)
├── server/                # HTTP/WS projection
└── strategy/              # doc.go beacon only
```

## Verification (baseline)

```bash
go build ./...
go test ./indicators/...
grep -r go-talib indicators/   # must return empty
```

Last baseline lock: **Pipeline Baseline v1** — all `indicators/` modules streaming-ready, `go-talib` removed project-wide.
