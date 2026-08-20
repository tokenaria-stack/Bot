package exchange

import (
	"path/filepath"
	"testing"

	"trading_bot/data"
)

func TestLoadContinuousContractBeforeEnd_GapYieldsNewBars(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "cc_gap.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}

	step := int64(60_000)
	base := int64(1_700_000_000_000) // post-futures genesis
	olderN := 30
	older := make([]data.Candle, olderN)
	for i := 0; i < olderN; i++ {
		open := base + int64(i)*step
		older[i] = data.Candle{
			OpenTime: open, Open: 1, High: 2, Low: 1, Close: float64(i),
			Volume: 1, CloseTime: open + step - 1,
		}
	}
	if err := data.SaveKlines("BTCUSDT", "1m", older); err != nil {
		t.Fatal(err)
	}
	boundary := older[olderN-1].OpenTime + step*8_000
	if err := data.SaveKlines("BTCUSDT", "1m", []data.Candle{{
		OpenTime: boundary, Open: 9, High: 10, Low: 8, Close: 9.5,
		Volume: 1, CloseTime: boundary + step - 1,
	}}); err != nil {
		t.Fatal(err)
	}

	const want = 12
	// Old contract: time-span LoadContinuousContractFromDB collapses to boundary.
	spanStart := boundary - step*int64(want)
	span, err := LoadContinuousContractFromDB("BTCUSDT", "1m", spanStart, boundary, want)
	if err != nil {
		t.Fatal(err)
	}
	if len(span) != 1 {
		t.Fatalf("time-span control len=%d want 1", len(span))
	}

	got, err := LoadContinuousContractBeforeEnd("BTCUSDT", "1m", boundary, want)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != want {
		t.Fatalf("before-end len=%d want %d", len(got), want)
	}

	// FE progress: bars strictly older than the store boundary.
	newBars := 0
	for _, c := range got {
		if c.OpenTime < boundary {
			newBars++
		}
	}
	if newBars != want-1 {
		t.Fatalf("newBars=%d want %d (exclude boundary overlap)", newBars, want-1)
	}
}

func TestLoadContinuousContractBeforeEnd_TrueBeginning(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "cc_eof.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	step := int64(60_000)
	base := int64(1_700_000_000_000)
	rows := make([]data.Candle, 4)
	for i := range rows {
		open := base + int64(i)*step
		rows[i] = data.Candle{
			OpenTime: open, Open: 1, High: 2, Low: 1, Close: float64(i),
			Volume: 1, CloseTime: open + step - 1,
		}
	}
	if err := data.SaveKlines("BTCUSDT", "1m", rows); err != nil {
		t.Fatal(err)
	}
	got, err := LoadContinuousContractBeforeEnd("BTCUSDT", "1m", rows[3].OpenTime, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("len=%d want 4", len(got))
	}
}

func TestLoadContinuousContractBeforeEnd_NoSpotPadInFuturesEraWindow(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "cc_era.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}
	step := int64(60_000)
	spotOpen := BinanceFuturesGenesisMs - step
	if err := data.SaveKlines(SpotStorageSymbol("BTCUSDT"), "1m", []data.Candle{{
		OpenTime: spotOpen, CloseTime: spotOpen + step - 1, Close: 111,
		Open: 111, High: 111, Low: 111, Volume: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	end := int64(1_672_765_200_000) // 2023-01-03 — wholly post-genesis for 12×1m
	got, err := LoadContinuousContractBeforeEnd("BTCUSDT", "1m", end, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len=%d want 0 (must not fill 2023 request from 2019 spot)", len(got))
	}
}
