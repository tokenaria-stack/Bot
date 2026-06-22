package indicators

// Package indicators provides streaming (tick-by-tick) mathematical primitives
// for the trading bot pipeline architecture (Layer 1).
//
// # Pipeline Baseline
//
// Every indicator updates in O(1) memory per tick via Update or UpdateCandle.
// Indicators compose: the output of one module can feed Update on another
// (e.g. MACD(RSI(price))).
//
// Core interfaces:
//   - Indicator        — Update(val float64) float64 + Value() float64
//   - CandleIndicator  — UpdateCandle(high, low, close float64) float64 + Value()
//
// Batch helpers (*Values functions) replay historical series through streaming
// structs for backtesting and ChiefAnalyst warm-up. They must not be used on
// the live tick path once Layer 2 streaming wiring is complete.
//
// Rules:
//   - No go-talib or external TA dependencies in this package.
//   - No trading logic, exchange calls, or strategy decisions.
//   - Windowed indicators use ring buffers or running sums (O(1) memory).
