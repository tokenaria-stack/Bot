package execution

import (
	"fmt"
	"log"
	"math"
)

// TradeSignal is an abstract signal from the strategy layer (intent only).
type TradeSignal struct {
	Symbol   string
	Side     string  // "BUY" or "SELL"
	Price    float64 // entry price
	StopLoss float64 // SL level (required for risk sizing)
}

// OrderRequest is a sized order ready for the exchange layer.
type OrderRequest struct {
	Symbol   string
	Side     string
	Quantity float64 // position size in base asset (e.g. BTC)
	Leverage int
	StopLoss float64
}

// RiskManager protects deposit and computes position sizing.
type RiskManager struct {
	RiskPerTradePct float64
	MaxLeverage     int
}

// NewRiskManager creates a risk manager with per-trade risk % and max leverage cap.
func NewRiskManager(riskPct float64, maxLev int) *RiskManager {
	return &RiskManager{
		RiskPerTradePct: riskPct,
		MaxLeverage:     maxLev,
	}
}

// CalculateTargetQuantity returns position size (base asset qty) and required leverage.
// lotMod scales the dollar risk budget (from ScoreDecision volatility modifier).
func CalculateTargetQuantity(
	balance, riskPerTradePct, entryPrice, stopLossPrice, lotMod, maxLeverage float64,
) (float64, float64) {
	if balance <= 0 || entryPrice <= 0 || stopLossPrice <= 0 || entryPrice == stopLossPrice {
		return 0, 0
	}
	if maxLeverage <= 0 {
		maxLeverage = 1
	}

	riskAmount := balance * (riskPerTradePct / 100.0)
	if lotMod > 0 {
		riskAmount *= lotMod
	}

	slDistancePct := math.Abs(entryPrice-stopLossPrice) / entryPrice
	if slDistancePct == 0 {
		return 0, 0
	}

	positionSizeUSDT := riskAmount / slDistancePct

	requiredLeverage := positionSizeUSDT / balance
	if requiredLeverage > maxLeverage {
		positionSizeUSDT = balance * maxLeverage
		requiredLeverage = maxLeverage
	}
	if requiredLeverage < 1 {
		requiredLeverage = 1
	}

	qty := positionSizeUSDT / entryPrice
	return qty, math.Ceil(requiredLeverage)
}

// EvaluateSignal converts a raw signal into a sized order using stop-distance sizing.
func (rm *RiskManager) EvaluateSignal(sig TradeSignal, availableBalance, lotMod float64) (*OrderRequest, error) {
	if availableBalance <= 0 {
		return nil, fmt.Errorf("insufficient balance: %.2f", availableBalance)
	}
	if sig.Price <= 0 || sig.StopLoss <= 0 {
		return nil, fmt.Errorf("invalid price or stop loss")
	}

	maxLev := float64(rm.MaxLeverage)
	if maxLev <= 0 {
		maxLev = 1
	}

	qty, lev := CalculateTargetQuantity(
		availableBalance,
		rm.RiskPerTradePct,
		sig.Price,
		sig.StopLoss,
		lotMod,
		maxLev,
	)
	if qty <= 0 {
		return nil, fmt.Errorf("position size is zero")
	}

	log.Printf("[RiskManager] Evaluated %s: Entry=%.2f, SL=%.2f. Qty=%.8f, Lev=%.0fx",
		sig.Side, sig.Price, sig.StopLoss, qty, lev)

	return &OrderRequest{
		Symbol:   sig.Symbol,
		Side:     sig.Side,
		Quantity: qty,
		Leverage: int(lev),
		StopLoss: sig.StopLoss,
	}, nil
}
