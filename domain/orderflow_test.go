package domain

import "testing"

func TestTickBuffer_ringOverwrite(t *testing.T) {
	buf := NewTickBuffer(3)
	for i := 1; i <= 5; i++ {
		buf.Add(AggTrade{Price: float64(i), Quantity: 1, Time: int64(i * 1000)})
	}
	all := buf.All()
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	if all[0].Price != 3 || all[2].Price != 5 {
		t.Fatalf("unexpected ring contents: %+v", all)
	}
}

func TestTickBuffer_Before(t *testing.T) {
	buf := NewTickBuffer(10)
	buf.Add(AggTrade{Time: 1000, Price: 1})
	buf.Add(AggTrade{Time: 2000, Price: 2})
	buf.Add(AggTrade{Time: 3000, Price: 3})

	before := buf.Before(2500)
	if len(before) != 2 || before[1].Time != 2000 {
		t.Fatalf("Before: %+v", before)
	}
}

func TestLiquidationBuffer_Add(t *testing.T) {
	buf := NewLiquidationBuffer(2)
	buf.Add(Liquidation{Price: 100, Side: "SELL", Time: 1})
	buf.Add(Liquidation{Price: 101, Side: "BUY", Time: 2})
	buf.Add(Liquidation{Price: 102, Side: "SELL", Time: 3})

	all := buf.All()
	if len(all) != 2 || all[0].Price != 101 {
		t.Fatalf("liquidations: %+v", all)
	}
}
