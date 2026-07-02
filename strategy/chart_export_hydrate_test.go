package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestMarkerChartExportPointsAlignedAfterBoot(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 120)
	base := int64(1_700_000_000_000)
	for i := range klines {
		price := 50000.0 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime: base + int64(i)*60_000,
			Open:     price,
			High:     price + 10,
			Low:      price - 10,
			Close:    price + 5,
			Volume:   100,
		}
	}

	m := NewMarker(klines, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	m.mu.RLock()
	n := len(m.klines)
	pts := len(m.chartExportPoints)
	m.mu.RUnlock()
	if n != len(klines) {
		t.Fatalf("klines len = %d, want %d", n, len(klines))
	}
	if pts != n {
		t.Fatalf("chartExportPoints len = %d, want %d (klines)", pts, n)
	}

	window := klines[len(klines)-50:]
	result, ok := ExportChartSeriesForWindow(m, window, GetRSXSettings())
	if !ok || result == nil || len(result.ChartPoints) == 0 {
		t.Fatal("expected export from hydrated marker RAM")
	}
}

func TestLoadHistoricalKlinesHydratesChartExportPoints(t *testing.T) {
	t.Parallel()

	m := NewMarker(nil, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	backfill := make([]exchange.Kline, 80)
	base := int64(1_700_000_000_000)
	for i := range backfill {
		price := 48000.0 + float64(i)
		backfill[i] = exchange.Kline{
			OpenTime: base + int64(i)*60_000,
			Open:     price,
			High:     price + 10,
			Low:      price - 10,
			Close:    price + 5,
			Volume:   50,
		}
	}
	m.LoadHistoricalKlines(backfill)

	m.mu.RLock()
	n := len(m.klines)
	pts := len(m.chartExportPoints)
	m.mu.RUnlock()
	if n == 0 {
		t.Fatal("expected klines after LoadHistoricalKlines")
	}
	if pts != n {
		t.Fatalf("chartExportPoints len = %d, want %d after LoadHistoricalKlines", pts, n)
	}
}

func TestTrimKlinesToCapPreservesDataBusAlignment(t *testing.T) {
	t.Parallel()

	base := int64(1_700_000_000_000)
	klines := make([]exchange.Kline, LiveKlineRAMCap)
	for i := range klines {
		price := 50000.0 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime: base + int64(i)*180_000,
			Open:     price,
			High:     price + 10,
			Low:      price - 10,
			Close:    price + 5,
			Volume:   100,
		}
	}

	m := NewMarker(klines, nil, "3m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})

	for i := 0; i < 10; i++ {
		idx := LiveKlineRAMCap + i
		price := 50000.0 + float64(idx)
		m.UpdateKlineTick(exchange.Kline{
			OpenTime: base + int64(idx)*180_000,
			Open:     price,
			High:     price + 10,
			Low:      price - 10,
			Close:    price + 5,
			Volume:   100,
		}, true)
	}

	m.mu.RLock()
	n := len(m.klines)
	if n != LiveKlineRAMCap {
		t.Fatalf("klines len = %d, want %d after trim", n, LiveKlineRAMCap)
	}
	if len(m.chartExportPoints) != n {
		t.Fatalf("chartExportPoints len = %d, want %d", len(m.chartExportPoints), n)
	}
	if len(m.JurikLines) != n {
		t.Fatalf("JurikLines len = %d, want %d", len(m.JurikLines), n)
	}
	if len(m.WozduhRed) != n {
		t.Fatalf("WozduhRed len = %d, want %d", len(m.WozduhRed), n)
	}
	m.mu.RUnlock()

	window := m.GetKlinesTail(100)
	result, ok := ExportChartSeriesForWindow(m, window, GetRSXSettings())
	if !ok || result == nil || len(result.ChartPoints) == 0 {
		t.Fatal("expected export after RAM cap trim")
	}
}

func TestLayer2RestoreMergesSnapWithLiveTail(t *testing.T) {
	t.Parallel()

	klines := make([]exchange.Kline, 10)
	base := int64(1_700_000_000_000)
	for i := range klines {
		price := 50000.0 + float64(i)
		klines[i] = exchange.Kline{
			OpenTime: base + int64(i)*60_000,
			Open:     price,
			High:     price + 10,
			Low:      price - 10,
			Close:    price + 5,
			Volume:   100,
		}
	}

	m := NewMarker(klines, nil, "3m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})

	m.UpdateKlineTick(klines[9], true)

	m.UpdateKlineTick(exchange.Kline{
		OpenTime: base + 10*60_000,
		Open:     50010,
		High:     50020,
		Low:      50000,
		Close:    50015,
		Volume:   50,
	}, false)

	m.UpdateKlineTick(exchange.Kline{
		OpenTime: base + 10*60_000,
		Open:     50010,
		High:     50025,
		Low:      50000,
		Close:    50020,
		Volume:   55,
	}, false)

	m.mu.RLock()
	n := len(m.klines)
	if n != 11 {
		t.Fatalf("klines len = %d, want 11", n)
	}
	if len(m.chartExportPoints) != n {
		t.Fatalf("chartExportPoints len = %d, want %d after layer2 restore", len(m.chartExportPoints), n)
	}
	if len(m.JurikLines) != n {
		t.Fatalf("JurikLines len = %d, want %d after layer2 restore", len(m.JurikLines), n)
	}
	m.mu.RUnlock()

	window := m.GetKlinesTail(11)
	if _, ok := ExportChartSeriesForWindow(m, window, GetRSXSettings()); !ok {
		t.Fatal("expected export after layer2 restore merge")
	}
}
