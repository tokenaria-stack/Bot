package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestMarker_UpdateKlineTick_RolloverPerMinute(t *testing.T) {
	m := NewMarker(nil, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})

	bar1 := exchange.Kline{
		OpenTime:  1_700_000_040_000,
		CloseTime: 1_700_000_099_999,
		Open:      100,
		High:      101,
		Low:       99,
		Close:     100.5,
		Volume:    10,
	}
	m.UpdateKlineTick(bar1, false)

	bar1Live := exchange.Kline{
		OpenTime:  1_700_000_040_000,
		CloseTime: 1_700_000_099_999,
		Open:      100,
		High:      105,
		Low:       98,
		Close:     104,
		Volume:    20,
	}
	m.UpdateKlineTick(bar1Live, false)

	bar1Closed := bar1Live
	m.UpdateKlineTick(bar1Closed, true)

	bar2 := exchange.Kline{
		OpenTime:  1_700_000_100_000,
		CloseTime: 1_700_000_159_999,
		Open:      104,
		High:      106,
		Low:       103,
		Close:     105,
		Volume:    8,
	}
	m.UpdateKlineTick(bar2, false)

	klines := m.GetKlines()
	if len(klines) != 2 {
		t.Fatalf("klines len = %d, want 2 distinct 1m bars", len(klines))
	}
	if klines[0].OpenTime != bar1.OpenTime {
		t.Fatalf("first OpenTime = %d, want %d", klines[0].OpenTime, bar1.OpenTime)
	}
	if klines[1].OpenTime != bar2.OpenTime {
		t.Fatalf("second OpenTime = %d, want %d", klines[1].OpenTime, bar2.OpenTime)
	}
}

func TestMarker_UpdateKlineTick_SecondsCoerced(t *testing.T) {
	m := NewMarker(nil, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})

	m.UpdateKlineTick(exchange.Kline{
		OpenTime: 1_700_000_040,
		Open:     100, High: 101, Low: 99, Close: 100, Volume: 1,
	}, false)
	m.UpdateKlineTick(exchange.Kline{
		OpenTime: 1_700_000_100,
		Open:     101, High: 102, Low: 100, Close: 101, Volume: 1,
	}, true)

	if len(m.GetKlines()) != 2 {
		t.Fatalf("expected 2 bars after second-normalized rollover, got %d", len(m.GetKlines()))
	}
}

func warmupMarkerBars(m *Marker, n int, startMs int64, stepMs int64) {
	for i := 0; i < n; i++ {
		base := float64(100 + i)
		m.UpdateKlineTick(exchange.Kline{
			OpenTime:  startMs + int64(i)*stepMs,
			CloseTime: startMs + int64(i+1)*stepMs - 1,
			Open:      base,
			High:      base + 2,
			Low:       base - 1,
			Close:     base + 0.5,
			Volume:    10 + float64(i),
		}, true)
	}
}

func markerJurikRSX(m *Marker) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.falconSignals.JurikRSX
}

func markerLatestAO(m *Marker) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latestAO
}

func markerADValue(m *Marker) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ad.Value()
}

func markerVolATR(m *Marker) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.volatilityState.ATR
}

func markerRSXBarCount(m *Marker) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.JurikLines)
}

func markerDivScore(m *Marker) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.divSignal.Score
}

func assertIntraBarStable(t *testing.T, name string, want, got float64) {
	t.Helper()
	const eps = 1e-9
	if want == 0 && got == 0 {
		return
	}
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > eps {
		t.Fatalf("intra-bar %s = %v, want %v (single evaluate on final OHLC)", name, got, want)
	}
}

func TestMarker_UpdateKlineTick_IntraBarDoesNotCompoundFalcon(t *testing.T) {
	const stepMs = int64(60_000)
	startMs := int64(1_700_000_000_000)

	history := NewMarker(nil, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	warmupMarkerBars(history, 59, startMs, stepMs)

	lastOpen := startMs + 58*stepMs
	final := exchange.Kline{
		OpenTime:  lastOpen,
		CloseTime: lastOpen + stepMs - 1,
		Open:      158,
		High:      161,
		Low:       157,
		Close:     160.5,
		Volume:    42,
	}

	single := NewMarker(nil, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	warmupMarkerBars(single, 59, startMs, stepMs)
	single.UpdateKlineTick(final, false)
	wantRSX := markerJurikRSX(single)

	intra := NewMarker(nil, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	warmupMarkerBars(intra, 59, startMs, stepMs)
	ticks := []exchange.Kline{
		{OpenTime: lastOpen, CloseTime: lastOpen + stepMs - 1, Open: 158, High: 159, Low: 157, Close: 158.2, Volume: 10},
		{OpenTime: lastOpen, CloseTime: lastOpen + stepMs - 1, Open: 158, High: 160, Low: 156.5, Close: 159.8, Volume: 20},
		{OpenTime: lastOpen, CloseTime: lastOpen + stepMs - 1, Open: 158, High: 161, Low: 157, Close: 160.5, Volume: 42},
	}
	for _, tick := range ticks {
		intra.UpdateKlineTick(tick, false)
	}
	gotRSX := markerJurikRSX(intra)

	const eps = 1e-9
	if wantRSX == 0 && gotRSX == 0 {
		return
	}
	diff := gotRSX - wantRSX
	if diff < 0 {
		diff = -diff
	}
	if diff > eps {
		t.Fatalf("intra-bar RSX = %v, want %v (single evaluate on final OHLC)", gotRSX, wantRSX)
	}
}

func TestMarker_UpdateKlineTick_IntraBarDoesNotCompoundLayer2(t *testing.T) {
	const stepMs = int64(60_000)
	startMs := int64(1_700_000_000_000)

	lastOpen := startMs + 58*stepMs
	final := exchange.Kline{
		OpenTime:  lastOpen,
		CloseTime: lastOpen + stepMs - 1,
		Open:      158,
		High:      161,
		Low:       157,
		Close:     160.5,
		Volume:    42,
	}
	ticks := []exchange.Kline{
		{OpenTime: lastOpen, CloseTime: lastOpen + stepMs - 1, Open: 158, High: 159, Low: 157, Close: 158.2, Volume: 10},
		{OpenTime: lastOpen, CloseTime: lastOpen + stepMs - 1, Open: 158, High: 160, Low: 156.5, Close: 159.8, Volume: 20},
		final,
	}

	cfg := ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34}

	single := NewMarker(nil, nil, "1m", "", cfg)
	warmupMarkerBars(single, 59, startMs, stepMs)
	single.UpdateKlineTick(final, false)

	intra := NewMarker(nil, nil, "1m", "", cfg)
	warmupMarkerBars(intra, 59, startMs, stepMs)
	for _, tick := range ticks {
		intra.UpdateKlineTick(tick, false)
	}

	assertIntraBarStable(t, "AO", markerLatestAO(single), markerLatestAO(intra))
	assertIntraBarStable(t, "AD", markerADValue(single), markerADValue(intra))
	assertIntraBarStable(t, "ATR", markerVolATR(single), markerVolATR(intra))
}

func TestMarker_UpdateKlineTick_IntraBarDoesNotGrowRSXOrPoisonDiv(t *testing.T) {
	const stepMs = int64(60_000)
	startMs := int64(1_700_000_000_000)

	lastOpen := startMs + 58*stepMs
	final := exchange.Kline{
		OpenTime:  lastOpen,
		CloseTime: lastOpen + stepMs - 1,
		Open:      158,
		High:      161,
		Low:       157,
		Close:     160.5,
		Volume:    42,
	}
	ticks := []exchange.Kline{
		{OpenTime: lastOpen, CloseTime: lastOpen + stepMs - 1, Open: 158, High: 159, Low: 157, Close: 158.2, Volume: 10},
		{OpenTime: lastOpen, CloseTime: lastOpen + stepMs - 1, Open: 158, High: 160, Low: 156.5, Close: 159.8, Volume: 20},
		final,
	}

	cfg := ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34}

	single := NewMarker(nil, nil, "1m", "", cfg)
	warmupMarkerBars(single, 59, startMs, stepMs)
	single.UpdateKlineTick(final, false)

	intra := NewMarker(nil, nil, "1m", "", cfg)
	warmupMarkerBars(intra, 59, startMs, stepMs)
	for _, tick := range ticks {
		intra.UpdateKlineTick(tick, false)
	}

	if got, want := markerRSXBarCount(intra), markerRSXBarCount(single); got != want {
		t.Fatalf("intra-bar RSX bar count = %d, want %d", got, want)
	}
	if got, want := markerDivScore(intra), markerDivScore(single); got != want {
		t.Fatalf("intra-bar div score = %d, want %d", got, want)
	}
}
