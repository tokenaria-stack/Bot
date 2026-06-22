package exchange

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/adshao/go-binance/v2/futures"
)

var (
	MainnetBaseURL = futures.BaseApiMainUrl
	TestnetBaseURL = futures.BaseApiTestnetUrl
)

// SymbolLimits stores parsed trading constraints for a symbol.
type SymbolLimits struct {
	TickSize float64
	StepSize float64
}

// Normalizer caches exchange symbol limits and formats order values safely.
type Normalizer struct {
	mu     sync.RWMutex
	limits map[string]SymbolLimits
}

// NewNormalizer creates an empty thread-safe normalizer.
func NewNormalizer() *Normalizer {
	return &Normalizer{
		limits: make(map[string]SymbolLimits),
	}
}

// LoadLimitsFromFutures fetches GET /fapi/v1/exchangeInfo and caches tick/step sizes.
func (n *Normalizer) LoadLimitsFromFutures(ctx context.Context, client *futures.Client) error {
	info, err := client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return fmt.Errorf("fetch futures exchangeInfo: %w", err)
	}

	parsed := make(map[string]SymbolLimits, len(info.Symbols))
	for _, symbol := range info.Symbols {
		priceFilter := symbol.PriceFilter()
		lotFilter := symbol.LotSizeFilter()
		if priceFilter == nil || lotFilter == nil {
			continue
		}

		tickSize, err := strconv.ParseFloat(priceFilter.TickSize, 64)
		if err != nil {
			return fmt.Errorf("parse tickSize for %s: %w", symbol.Symbol, err)
		}
		stepSize, err := strconv.ParseFloat(lotFilter.StepSize, 64)
		if err != nil {
			return fmt.Errorf("parse stepSize for %s: %w", symbol.Symbol, err)
		}
		if tickSize <= 0 || stepSize <= 0 {
			continue
		}

		parsed[symbol.Symbol] = SymbolLimits{
			TickSize: tickSize,
			StepSize: stepSize,
		}
	}

	n.mu.Lock()
	n.limits = parsed
	n.mu.Unlock()

	return nil
}

// FormatPrice formats a price according to the symbol tick size.
func (n *Normalizer) FormatPrice(symbol string, price float64) (string, error) {
	limits, err := n.symbolLimits(symbol)
	if err != nil {
		return "", err
	}

	aligned := truncateToStep(price, limits.TickSize)
	return formatWithStep(aligned, limits.TickSize), nil
}

// FormatQuantity formats a quantity rounded strictly down to the symbol step size.
func (n *Normalizer) FormatQuantity(symbol string, qty float64) (string, error) {
	limits, err := n.symbolLimits(symbol)
	if err != nil {
		return "", err
	}

	truncated := truncateToStep(qty, limits.StepSize)
	return formatWithStep(truncated, limits.StepSize), nil
}

func (n *Normalizer) symbolLimits(symbol string) (SymbolLimits, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	limits, ok := n.limits[symbol]
	if !ok {
		return SymbolLimits{}, fmt.Errorf("symbol limits not found for %q", symbol)
	}

	return limits, nil
}

func truncateToStep(value, step float64) float64 {
	if step <= 0 {
		return value
	}

	return math.Floor(value/step) * step
}

func formatWithStep(value, step float64) string {
	precision := decimalPlacesFromStep(step)
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func decimalPlacesFromStep(step float64) int {
	if step <= 0 {
		return 8
	}

	stepText := strings.TrimRight(strconv.FormatFloat(step, 'f', -1, 64), "0")
	stepText = strings.TrimRight(stepText, ".")

	dot := strings.IndexByte(stepText, '.')
	if dot < 0 {
		return 0
	}

	return len(stepText) - dot - 1
}
