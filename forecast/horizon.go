package forecast

import (
	"fmt"

	"trading_bot/ml"
)

// HorizonEnd is the open time of the last market bar in the worst-case
// target path of H bars after At (t+1 … t+H). Training against a block
// that starts at V is legal only when HorizonEnd < V (strict).
func HorizonEnd(at int64, timeframe string, bars int) (int64, error) {
	t, err := ml.HorizonEnd(at, timeframe, bars)
	if err != nil {
		return 0, fmt.Errorf("forecast: %v", err)
	}
	return t, nil
}
