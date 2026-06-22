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
Layer 1  indicators/     ✅ Pipeline Baseline (streaming math, no talib)
Layer 2  strategy/       🔜 Context analytics — Elliott, ZigZag wiring, streaming ChiefAnalyst
Layer 3  strategy/       🔜 Falcon strategy — strategy/falcon.go on top of Layer 2 reports
         master.go       ✅ FSM + execution (current production path)
```

| Layer | Status | Next step |
|-------|--------|-----------|
| **1 — Indicators** | ✅ Ready | Consumed by analyst via `*Values` today; migrate to live `Update` instances in Layer 2 |
| **2 — Context / Elliott** | 🔜 | Wire `ZigZag`, `CalcRetracements`, streaming RSI/MACD into `ChiefAnalyst`; Elliott wave context |
| **3 — Falcon** | 🔜 | Create `strategy/falcon.go`: entry/exit rules on Layer 2 context + `MasterGeneral` FSM |

**Current integration:** `strategy/analyst.go` uses batch wrappers (`RSIValues`, `MACDValues`, `ATRValues`, `AOValuesFromKlines`) for `GenerateMarketReport()`. This is compatible with Layer 1 and safe for production; Layer 2 replaces batch replay with persistent streaming state per timeframe.

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
├── MEMORY.md              # Full architecture & FSM rules
├── main.go
├── config/
├── exchange/              # Binance Futures fapi + fstream
├── execution/             # Position sizing
├── indicators/            # Layer 1 — streaming math
├── vector_db/             # Qdrant pattern memory
└── strategy/
    ├── analyst.go         # ChiefAnalyst (Layer 2 consumer)
    ├── master.go          # MasterGeneral FSM
    └── risk.go            # Signal validation profiles
```

## Verification (baseline)

```bash
go build ./...
go test ./indicators/...
grep -r go-talib indicators/   # must return empty
```

Last baseline lock: **Pipeline Baseline v1** — all `indicators/` modules streaming-ready, `go-talib` removed project-wide.
