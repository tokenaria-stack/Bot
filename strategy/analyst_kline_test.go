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
