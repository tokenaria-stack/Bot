package forecast

import (
	"fmt"

	"trading_bot/data"
)

// HorizonEnd is the open time of the last market bar in the worst-case
// target path of H bars after At (t+1 … t+H). Training against a block
// that starts at V is legal only when HorizonEnd < V (strict).
func HorizonEnd(at int64, timeframe string, bars int) (int64, error) {
	if bars < 0 {
		return 0, fmt.Errorf("forecast: HorizonEnd bars must be >= 0")
	}
	t := at
	for i := 0; i < bars; i++ {
		next, err := data.NextBarOpen(t, timeframe)
		if err != nil {
			return 0, err
		}
		if next <= t {
			return 0, fmt.Errorf("forecast: HorizonEnd NextBarOpen did not advance from %d", t)
		}
		t = next
	}
	return t, nil
}
