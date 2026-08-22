package market

import "trading_bot/exchange"

// LiveKlineRAMCap is the max closed bars kept in RAM per live native/derived Frame.
const LiveKlineRAMCap = 3000

// MicroKlineRAMCap is the fixed working-set cap for live 1s (chart store contract).
const MicroKlineRAMCap = 9000

// FrameBootKlineLimit is how many bars each Frame loads from SQLite/REST at process start.
const FrameBootKlineLimit = 400

// GetKlinesTail returns a copy of the last maxBars candles (or all when shorter).
func (a *Frame) GetKlinesTail(maxBars int) []exchange.Kline {
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

func (a *Frame) ramBarCap() int {
	if a != nil && exchange.IsLiveSecond(a.timeframe) {
		return MicroKlineRAMCap
	}
	return LiveKlineRAMCap
}
