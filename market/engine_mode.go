package market

import (
	"strings"
	"sync/atomic"
)

// EngineMode separates the chart delivery server from the live trading bot (Shot 9F).
type EngineMode string

const (
	// EngineModeChartOnly delivers OHLCV + DAG plots.
	EngineModeChartOnly EngineMode = "ChartOnly"
	// EngineModeLive starts Runtime.Run. DAG/facts still run on the tick path.
	EngineModeLive EngineMode = "Live"
)

var engineMode atomic.Value // stores EngineMode

func init() {
	engineMode.Store(EngineModeChartOnly)
}

// SetEngineMode installs the process-wide engine mode. Unknown values fall back to ChartOnly.
func SetEngineMode(mode EngineMode) {
	engineMode.Store(NormalizeEngineMode(string(mode)))
}

// GetEngineMode returns the current process-wide engine mode.
func GetEngineMode() EngineMode {
	if v := engineMode.Load(); v != nil {
		if m, ok := v.(EngineMode); ok {
			return m
		}
	}
	return EngineModeChartOnly
}

// EngineAllowsStrategies reports whether Live-only process wiring may run (Runtime.Run).
// DAG chart indicators always run regardless of this gate.
func EngineAllowsStrategies() bool {
	return GetEngineMode() == EngineModeLive
}

// NormalizeEngineMode maps env/config strings to a canonical EngineMode (default ChartOnly).
func NormalizeEngineMode(raw string) EngineMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "live", "trading", "full":
		return EngineModeLive
	case "chartonly", "chart_only", "chart-only", "delivery", "":
		return EngineModeChartOnly
	default:
		return EngineModeChartOnly
	}
}
