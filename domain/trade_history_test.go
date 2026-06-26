package domain

import "testing"

func TestTradeHistoryStore_SeparatesModes(t *testing.T) {
	t.Parallel()

	store := NewTradeHistoryStore()
	store.AppendVirtual(ClosedTrade{Side: "LONG", PnLDollar: 10, PnL: 0.1, ExitTime: 100})
	store.AppendReal(ClosedTrade{Side: "SHORT", PnLDollar: -5, PnL: -0.05, ExitTime: 200})

	paper := store.StatsForMode("paper")
	live := store.StatsForMode("live")

	if len(paper.Trades) != 1 || paper.Trades[0].Side != "LONG" {
		t.Fatalf("paper trades: %+v", paper.Trades)
	}
	if len(live.Trades) != 1 || live.Trades[0].Side != "SHORT" {
		t.Fatalf("live trades: %+v", live.Trades)
	}
}

func TestComputeSessionStats(t *testing.T) {
	t.Parallel()

	stats := ComputeSessionStats(10000, []ClosedTrade{
		{PnLDollar: 100, PnL: 1, ExitTime: 10},
		{PnLDollar: -50, PnL: -0.5, ExitTime: 20},
	})
	if stats.TotalTrades != 2 {
		t.Fatalf("total=%d", stats.TotalTrades)
	}
	if stats.WinRate != 50 {
		t.Fatalf("winRate=%f", stats.WinRate)
	}
	if stats.NetProfit <= 0 {
		t.Fatalf("netProfit=%f", stats.NetProfit)
	}
	if len(stats.EquityCurve) < 3 {
		t.Fatalf("equity len=%d", len(stats.EquityCurve))
	}
}
