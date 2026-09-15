package data

import (
	"path/filepath"
	"testing"
)

func TestAssignCanonicalVolume_ExactPKLowers(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "assign_vol.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	open := int64(1_700_000_000_000)
	if err := SaveKlines("BTCUSDT", "15m", []Candle{{
		OpenTime: open, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 1663, CloseTime: open + 899_999,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := AssignCanonicalVolume("BTCUSDT", "15m", open, 1392); err != nil {
		t.Fatal(err)
	}
	got, err := LoadKlines("BTCUSDT", "15m", open, open+1, 0)
	if err != nil || len(got) != 1 || got[0].Volume != 1392 || got[0].High != 2 {
		t.Fatalf("%v %v", got, err)
	}
	if err := AssignCanonicalVolume("BTCUSDT", "15m", open+1, 1); err == nil {
		t.Fatal("wrong open_time must refuse")
	}
}

func TestAssignCanonicalVolume_ExactPKOnly(t *testing.T) {
	resetDBConnection(filepath.Join(t.TempDir(), "assign_vol_pk.db"))
	if err := InitDB(); err != nil {
		t.Fatal(err)
	}
	a := int64(1_700_000_000_000)
	b := a + 900_000
	if err := SaveKlines("BTCUSDT", "15m", []Candle{
		{OpenTime: a, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 10, CloseTime: a + 899_999},
		{OpenTime: b, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 20, CloseTime: b + 899_999},
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveKlines("BTCUSDT", "1h", []Candle{
		{OpenTime: a, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 99, CloseTime: a + 3_599_999},
	}); err != nil {
		t.Fatal(err)
	}
	if err := AssignCanonicalVolume("BTCUSDT", "15m", a, 11); err != nil {
		t.Fatal(err)
	}
	got, err := LoadKlines("BTCUSDT", "15m", a, b+1, 0)
	if err != nil || len(got) != 2 || got[0].Volume != 11 || got[1].Volume != 20 {
		t.Fatalf("15m sibling mutated: %v %v", got, err)
	}
	h, err := LoadKlines("BTCUSDT", "1h", a, a+1, 0)
	if err != nil || len(h) != 1 || h[0].Volume != 99 {
		t.Fatalf("1h mutated: %v %v", h, err)
	}
}
