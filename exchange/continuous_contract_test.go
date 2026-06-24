package exchange

import (
	"path/filepath"
	"testing"

	"trading_bot/data"
)

func TestSplitRangeAtGenesis_SpotOnly(t *testing.T) {
	t.Parallel()

	start := BinanceFuturesGenesisMs - 10_000
	end := BinanceFuturesGenesisMs - 1
	segs := splitRangeAtGenesis(start, end)
	if len(segs) != 1 || !segs[0].spotStorage {
		t.Fatalf("segs = %+v, want single spot segment", segs)
	}
	if segs[0].start != start || segs[0].end != end {
		t.Fatalf("spot segment = %+v", segs[0])
	}
}

func TestSplitRangeAtGenesis_FuturesOnly(t *testing.T) {
	t.Parallel()

	start := BinanceFuturesGenesisMs
	end := BinanceFuturesGenesisMs + 10_000
	segs := splitRangeAtGenesis(start, end)
	if len(segs) != 1 || segs[0].spotStorage {
		t.Fatalf("segs = %+v, want single futures segment", segs)
	}
}

func TestSplitRangeAtGenesis_CrossGenesis(t *testing.T) {
	t.Parallel()

	start := BinanceFuturesGenesisMs - 5_000
	end := BinanceFuturesGenesisMs + 5_000
	segs := splitRangeAtGenesis(start, end)
	if len(segs) != 2 {
		t.Fatalf("len(segs) = %d, want 2", len(segs))
	}
	if !segs[0].spotStorage || segs[0].end != BinanceFuturesGenesisMs-1 {
		t.Fatalf("spot segment = %+v", segs[0])
	}
	if segs[1].spotStorage || segs[1].start != BinanceFuturesGenesisMs {
		t.Fatalf("futures segment = %+v", segs[1])
	}
}

func TestNormalizeContinuousContractRange(t *testing.T) {
	t.Parallel()

	t.Run("start zero clamps to spot genesis", func(t *testing.T) {
		end := BinanceFuturesGenesisMs + 1000
		start, gotEnd, ok := normalizeContinuousContractRange(0, end)
		if !ok {
			t.Fatal("expected ok")
		}
		if start != BinanceSpotGenesisMs {
			t.Fatalf("start = %d, want %d", start, BinanceSpotGenesisMs)
		}
		if gotEnd != end {
			t.Fatalf("end = %d, want %d", gotEnd, end)
		}
	})

	t.Run("pre spot era returns not ok", func(t *testing.T) {
		_, _, ok := normalizeContinuousContractRange(0, BinanceSpotGenesisMs-1)
		if ok {
			t.Fatal("expected not ok for pre-spot window")
		}
	})

	t.Run("futures era unchanged", func(t *testing.T) {
		start := BinanceFuturesGenesisMs + 1000
		end := start + 5000
		gotStart, gotEnd, ok := normalizeContinuousContractRange(start, end)
		if !ok || gotStart != start || gotEnd != end {
			t.Fatalf("got start=%d end=%d ok=%v", gotStart, gotEnd, ok)
		}
	})
}

func TestNormalizeSpotRange(t *testing.T) {
	t.Parallel()

	start, end, ok := normalizeSpotRange(0, BinanceFuturesGenesisMs)
	if !ok {
		t.Fatal("expected ok")
	}
	if start != BinanceSpotGenesisMs {
		t.Fatalf("start = %d, want spot genesis", start)
	}
	if end != BinanceFuturesGenesisMs {
		t.Fatalf("end = %d", end)
	}
}

func TestSpotStorageSymbol(t *testing.T) {
	t.Parallel()

	if got := SpotStorageSymbol("btcusdt.p"); got != "BTCUSDT_SPOT" {
		t.Fatalf("SpotStorageSymbol = %q, want BTCUSDT_SPOT", got)
	}
}

func TestLoadContinuousContractFromDB_StitchesWithoutDuplicateAtGenesis(t *testing.T) {
	data.ResetDBForTest(filepath.Join(t.TempDir(), "cc.db"))
	if err := data.InitDB(); err != nil {
		t.Fatal(err)
	}

	symbol := "BTCUSDT"
	interval := "1d"
	step := int64(24 * 60 * 60 * 1000)

	spotLast := BinanceFuturesGenesisMs - step
	spotRows := []data.Candle{
		{OpenTime: spotLast - step, CloseTime: spotLast - 1, Close: 9000},
		{OpenTime: spotLast, CloseTime: BinanceFuturesGenesisMs - 1, Close: 9100},
	}
	futRows := []data.Candle{
		{OpenTime: BinanceFuturesGenesisMs, CloseTime: BinanceFuturesGenesisMs + step - 1, Close: 9200},
		{OpenTime: BinanceFuturesGenesisMs + step, CloseTime: BinanceFuturesGenesisMs + 2*step - 1, Close: 9300},
	}

	if err := data.SaveKlines(SpotStorageSymbol(symbol), interval, spotRows); err != nil {
		t.Fatal(err)
	}
	if err := data.SaveKlines(symbol, interval, futRows); err != nil {
		t.Fatal(err)
	}

	start := spotLast - step
	end := BinanceFuturesGenesisMs + step
	got, err := LoadContinuousContractFromDB(symbol, interval, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4 stitched bars without duplicate", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].OpenTime <= got[i-1].OpenTime {
			t.Fatalf("non-monotonic stitch at %d: %+v then %+v", i, got[i-1], got[i])
		}
	}
	if got[1].OpenTime != spotLast || got[2].OpenTime != BinanceFuturesGenesisMs {
		t.Fatalf("genesis seam = %+v | %+v", got[1], got[2])
	}
}

func TestDedupeDataCandlesByOpenTime_PrefersLast(t *testing.T) {
	t.Parallel()

	in := []data.Candle{
		{OpenTime: BinanceFuturesGenesisMs, Close: 1},
		{OpenTime: BinanceFuturesGenesisMs, Close: 2},
	}
	out := dedupeDataCandlesByOpenTime(in)
	if len(out) != 1 || out[0].Close != 2 {
		t.Fatalf("dedupe = %+v", out)
	}
}
