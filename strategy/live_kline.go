package strategy

import "trading_bot/exchange"

// LiveKlineRAMCap is the max closed bars kept in RAM per live Marker (ring buffer).
const LiveKlineRAMCap = 3000

// AnalystBootKlineLimit is how many bars each analyst loads from SQLite/REST at process start.
const AnalystBootKlineLimit = 400

// GetKlinesTail returns a copy of the last maxBars candles (or all when shorter).
func (a *Marker) GetKlinesTail(maxBars int) []exchange.Kline {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if maxBars <= 0 || len(a.klines) == 0 {
		return nil
	}
	start := 0
	if len(a.klines) > maxBars {
		start = len(a.klines) - maxBars
	}
	out := make([]exchange.Kline, len(a.klines)-start)
	copy(out, a.klines[start:])
	return out
}
