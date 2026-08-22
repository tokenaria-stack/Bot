package market

import "trading_bot/exchange"

// AttachLiveSecondFrames adds empty RAM Frames for activated second TFs (1s).
// No SQLite, no REST — history starts empty and grows from aggTrade.
func AttachLiveSecondFrames(frames map[string]*Frame, chaos ChaosConfig) {
	if frames == nil {
		return
	}
	for _, e := range exchange.LiveSeconds() {
		if frames[e.Name] != nil {
			continue
		}
		frames[e.Name] = NewFrame(nil, e.Name, chaos)
	}
}
