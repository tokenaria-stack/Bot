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
	RiskPerTradePct float64 // % of balance risked per trade (e.g. 1.0 = 1%)
	MaxLeverage     int
}

// NewRiskManager creates a risk manager with per-trade risk % and max leverage cap.
func NewRiskManager(riskPct float64, maxLev int) *RiskManager {
	return &RiskManager{
		RiskPerTradePct: riskPct,
		MaxLeverage:     maxLev,
	}
}

// EvaluateSignal converts a raw signal into a sized order using stop-distance sizing.
func (rm *RiskManager) EvaluateSignal(sig TradeSignal, availableBalance float64) (*OrderRequest, error) {
	if availableBalance <= 0 {
		return nil, fmt.Errorf("insufficient balance: %.2f", availableBalance)
	}
	if sig.Price <= 0 || sig.StopLoss <= 0 {
		return nil, fmt.Errorf("invalid price or stop loss")
	}

	slDistancePct := math.Abs(sig.Price-sig.StopLoss) / sig.Price
	if slDistancePct == 0 {
		return nil, fmt.Errorf("stop loss cannot be equal to entry price")
	}

	riskAmount := availableBalance * (rm.RiskPerTradePct / 100.0)
	positionSizeUSDT := riskAmount / slDistancePct

	leverage := int(math.Ceil(positionSizeUSDT / availableBalance))
	if leverage < 1 {
		leverage = 1
	}

	if leverage > rm.MaxLeverage {
		log.Printf("[RiskManager] Warning: Required leverage %d exceeds MaxLeverage %d. Capping.", leverage, rm.MaxLeverage)
		leverage = rm.MaxLeverage
		positionSizeUSDT = availableBalance * float64(leverage)
	}

	qty := positionSizeUSDT / sig.Price

	log.Printf("[RiskManager] Evaluated %s: Entry=%.2f, SL=%.2f (Dist: %.2f%%). Risking %.2f USDT. Qty=%.4f, Lev=%dx",
		sig.Side, sig.Price, sig.StopLoss, slDistancePct*100, riskAmount, qty, leverage)

	return &OrderRequest{
		Symbol:   sig.Symbol,
		Side:     sig.Side,
		Quantity: qty,
		Leverage: leverage,
		StopLoss: sig.StopLoss,
	}, nil
}
