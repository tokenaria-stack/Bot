package domain

import (
	"fmt"
	"sync"
	"time"
)

const DefaultSessionCapital = 10000.0

// ClosedTrade is one completed round-trip position (live or paper).
type ClosedTrade struct {
	EntryTime     int64   `json:"entryTime"`
	ExitTime      int64   `json:"exitTime"`
	Side          string  `json:"side"` // LONG or SHORT
	EntryPrice    float64 `json:"entryPrice"`
	ExitPrice     float64 `json:"exitPrice"`
	StopLossPrice float64 `json:"stopLossPrice,omitempty"`
	Fee           float64 `json:"fee,omitempty"`
	SlippagePct   float64 `json:"slippagePct,omitempty"`
	PnL           float64 `json:"pnl"` // percent return on balance before exit
	PnLDollar     float64 `json:"pnlDollar,omitempty"`
	ExitReason    string  `json:"exitReason"`
	Duration      string  `json:"duration"`
}

// EquityPoint is one balance snapshot for the equity curve (time in Unix seconds).
type EquityPoint struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// SessionStats aggregates closed trades for a trading mode.
type SessionStats struct {
	Mode           string        `json:"mode"`
	TotalTrades    int           `json:"totalTrades"`
	WinRate        float64       `json:"winRate"`
	NetProfit      float64       `json:"netProfit"`
	MaxDrawdown    float64       `json:"maxDrawdown"`
	ProfitFactor   float64       `json:"profitFactor"`
	RecoveryFactor float64       `json:"recoveryFactor"`
	Trades         []ClosedTrade `json:"trades"`
	EquityCurve    []EquityPoint `json:"equityCurve"`
}

// TradeHistoryStore keeps live and paper closed trades in separate buffers.
type TradeHistoryStore struct {
	mu            sync.RWMutex
	RealTrades    []ClosedTrade
	VirtualTrades []ClosedTrade
}

// NewTradeHistoryStore creates an empty session trade history store.
func NewTradeHistoryStore() *TradeHistoryStore {
	return &TradeHistoryStore{}
}

// AppendVirtual records a paper / sandbox closed trade.
func (s *TradeHistoryStore) AppendVirtual(trade ClosedTrade) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.VirtualTrades = append(s.VirtualTrades, trade)
	s.mu.Unlock()
}

// AppendReal records a live exchange closed trade.
func (s *TradeHistoryStore) AppendReal(trade ClosedTrade) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.RealTrades = append(s.RealTrades, trade)
	s.mu.Unlock()
}

// StatsForMode returns aggregated metrics for live or paper.
func (s *TradeHistoryStore) StatsForMode(mode string) SessionStats {
	if s == nil {
		return SessionStats{Mode: mode}
	}
	s.mu.RLock()
	var trades []ClosedTrade
	switch mode {
	case "live":
		trades = cloneClosedTrades(s.RealTrades)
	case "paper":
		trades = cloneClosedTrades(s.VirtualTrades)
	default:
		s.mu.RUnlock()
		return SessionStats{Mode: mode}
	}
	s.mu.RUnlock()

	stats := ComputeSessionStats(DefaultSessionCapital, trades)
	stats.Mode = mode
	return stats
}

func cloneClosedTrades(in []ClosedTrade) []ClosedTrade {
	if len(in) == 0 {
		return nil
	}
	out := make([]ClosedTrade, len(in))
	copy(out, in)
	return out
}

// ComputeSessionStats builds equity curve and performance metrics from closed trades.
func ComputeSessionStats(initial float64, trades []ClosedTrade) SessionStats {
	if initial <= 0 {
		initial = DefaultSessionCapital
	}

	total := len(trades)
	wins := 0
	var grossProfit, grossLoss float64
	balance := initial
	peak := initial
	maxDrawdownPct := 0.0
	maxDrawdownUSD := 0.0

	equity := []EquityPoint{{Time: time.Now().Unix(), Value: initial}}
	if total > 0 {
		equity[0].Time = trades[0].EntryTime
	}

	for _, t := range trades {
		if t.PnLDollar > 0 {
			wins++
			grossProfit += t.PnLDollar
		} else if t.PnLDollar < 0 {
			grossLoss += -t.PnLDollar
		}

		balance += t.PnLDollar
		equity = append(equity, EquityPoint{Time: t.ExitTime, Value: balance})
		peak, maxDrawdownPct, maxDrawdownUSD = updateSessionDrawdown(balance, peak, maxDrawdownPct, maxDrawdownUSD)
	}

	netProfit := 0.0
	if initial > 0 {
		netProfit = (balance - initial) / initial * 100
	}

	winRate := 0.0
	if total > 0 {
		winRate = float64(wins) / float64(total) * 100
	}

	profitFactor := 0.0
	switch {
	case grossLoss > 0:
		profitFactor = grossProfit / grossLoss
	case grossProfit > 0:
		profitFactor = profitFactorOrNet(profitFactor, netProfit)
	}

	recoveryFactor := 0.0
	if maxDrawdownUSD > 0 {
		recoveryFactor = (balance - initial) / maxDrawdownUSD
	}

	return SessionStats{
		TotalTrades:    total,
		WinRate:        winRate,
		NetProfit:      netProfit,
		MaxDrawdown:    maxDrawdownPct,
		ProfitFactor:   profitFactor,
		RecoveryFactor: recoveryFactor,
		Trades:         trades,
		EquityCurve:    equity,
	}
}

func profitFactorOrNet(_, netProfit float64) float64 {
	if netProfit > 0 {
		return netProfit
	}
	return 0
}

func updateSessionDrawdown(balance, peak, maxPct, maxUSD float64) (float64, float64, float64) {
	if balance > peak {
		peak = balance
	}
	if peak <= 0 {
		return peak, maxPct, maxUSD
	}
	ddUSD := peak - balance
	ddPct := ddUSD / peak * 100
	if ddPct > maxPct {
		maxPct = ddPct
	}
	if ddUSD > maxUSD {
		maxUSD = ddUSD
	}
	return peak, maxPct, maxUSD
}

// FormatTradeDuration formats human-readable trade duration.
func FormatTradeDuration(entrySec, exitSec int64) string {
	if entrySec <= 0 || exitSec <= 0 || exitSec < entrySec {
		return "—"
	}
	d := time.Duration(exitSec-entrySec) * time.Second
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours()/24), int(d.Hours())%24)
}

// DisplaySideFromEntry maps BUY/SELL entry side to LONG/SHORT label.
func DisplaySideFromEntry(entrySide string) string {
	if entrySide == "SELL" {
		return "SHORT"
	}
	return "LONG"
}
