package market

import (
	"log"

	"trading_bot/data"
	"trading_bot/exchange"
)

// AttachLiveSecondFrames hydrates activated second TFs (1s) from micro_klines
// (latest MicroKlineRAMCap rows). Sparse holes are kept. No REST, no continuity check.
func AttachLiveSecondFrames(frames map[string]*Frame, symbol string, chaos ChaosConfig) {
	if frames == nil {
		return
	}
	for _, e := range exchange.LiveSeconds() {
		if frames[e.Name] != nil {
			continue
		}
		var hist []exchange.Kline
		rows, err := data.LoadLatestMicroKlines(symbol, e.Name, MicroKlineRAMCap)
		if err != nil {
			log.Printf("[Init] 1s micro hydrate: %v", err)
		} else {
			hist = exchange.KlinesFromDataCandles(rows)
		}
		frames[e.Name] = NewFrame(hist, e.Name, chaos)
		if len(hist) > 0 {
			log.Printf("[Init] Frame [%s] hydrated %d micro_klines (sparse OK)", e.Name, len(hist))
		}
	}
}
