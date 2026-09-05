package market

import (
	"fmt"

	"trading_bot/exchange"
	"trading_bot/forecast"
)

// DumpFeatureTape writes one immutable FeatureTape from a materialized closed-bar
// slice. One O(N) Frame replay; frozen 1A Fill only. bars must already be a
// coherent ordered view (caller materializes; this function does not page SQLite).
func DumpFeatureTape(path string, key forecast.MarketKey, plan forecast.FeaturePlan, settings RSXSettings, bars []exchange.Kline) error {
	if err := key.Validate(); err != nil {
		return err
	}
	if len(bars) == 0 {
		return fmt.Errorf("market: refuse empty feature-tape source")
	}
	planID, err := plan.Identity()
	if err != nil {
		return err
	}
	hdr := forecast.TapeHeader{
		FormatVersion: forecast.FeatureTapeFormatV1,
		Market:        key,
		PlanDigest:    planID.Digest,
		FeatureIDs:    append([]forecast.FeatureID(nil), plan.Schema...),
		VectorLen:     plan.VectorLen(),
	}
	frame := NewFrame(nil, key.Timeframe, ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	frame.ApplyBacktestRSXConfig(settings)
	ev, err := BindFeatureEvaluator(frame, plan)
	if err != nil {
		return err
	}
	defer ev.Unbind()

	w, err := forecast.CreateTapeWriter(path, hdr)
	if err != nil {
		return err
	}
	src := forecast.NewSourceRangeHasher(key)
	for _, k := range bars {
		src.Add(forecast.CanonicalClosedBar{
			OpenTime: k.OpenTime,
			Open:     k.Open,
			High:     k.High,
			Low:      k.Low,
			Close:    k.Close,
			Volume:   k.Volume,
		})
		frame.UpdateKlineTick(k, true)
		if ev.LastBarOpenTime() != k.OpenTime {
			w.Abort()
			return fmt.Errorf("market: feature-tape At off-by-one: source %d frame %d", k.OpenTime, ev.LastBarOpenTime())
		}
		ready, dst, err := ev.FillOwned()
		if err != nil {
			w.Abort()
			return err
		}
		var values []float64
		if ready {
			values = dst
		}
		if err := w.WriteRow(k.OpenTime, ready, values); err != nil {
			return err
		}
	}
	return w.Finish(src.Sum())
}
