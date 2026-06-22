package strategy

import (
	"strings"
	"sync"
)

const (
	RSXLookbackDefault     = 90
	DefaultRSXLength       = 14
	MinRSXLength           = 3
	MaxRSXLength           = 100
	DefaultRSXSignalLength = 9
	MinRSXSignalLength     = 2
	MaxRSXSignalLength     = 50
	MinRSXDivLookback      = 10
	MaxRSXDivLookback      = 200
	MinRSXPivotRadius      = 1
	MaxRSXPivotRadius      = 10
	DefaultRSXPivotRadius  = 2
)

// RSXSettings хранит конфигурацию индикатора RSX и поиска дивергенций.
type RSXSettings struct {
	Length       int    `json:"length"`
	DivLookback  int    `json:"div_lookback"`
	SignalLength int    `json:"signal_length"`
	Source       string `json:"source"`       // "close" или "hlc3"
	PivotRadius  int    `json:"pivot_radius"` // Для фрактальных дивергенций
	DivMethod    string `json:"div_method"`   // "tv" (TradingView) или "fractal"
}

var (
	rsxSettingsInstance *RSXSettings
	rsxSettingsOnce     sync.Once
	rsxSettingsMutex    sync.RWMutex
)

// GetRSXSettings возвращает копию текущих настроек.
func GetRSXSettings() RSXSettings {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.RLock()
	defer rsxSettingsMutex.RUnlock()
	return *rsxSettingsInstance
}

func initRSXSettingsDefaults() {
	rsxSettingsInstance = &RSXSettings{
		Length:       DefaultRSXLength,
		DivLookback:  RSXLookbackDefault,
		SignalLength: DefaultRSXSignalLength,
		Source:       "close",
		PivotRadius:  DefaultRSXPivotRadius,
		DivMethod:    "tv",
	}
}

// ApplyRSXSettings обновляет настройки с clamp и возвращает применённые значения.
func ApplyRSXSettings(update RSXSettings) RSXSettings {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.Lock()
	defer rsxSettingsMutex.Unlock()

	if update.Length > 0 {
		rsxSettingsInstance.Length = clampInt(update.Length, MinRSXLength, MaxRSXLength, DefaultRSXLength)
	}
	if update.DivLookback > 0 {
		rsxSettingsInstance.DivLookback = clampInt(update.DivLookback, MinRSXDivLookback, MaxRSXDivLookback, RSXLookbackDefault)
	}
	if update.SignalLength > 0 {
		rsxSettingsInstance.SignalLength = clampInt(update.SignalLength, MinRSXSignalLength, MaxRSXSignalLength, DefaultRSXSignalLength)
	}
	if update.Source != "" {
		rsxSettingsInstance.Source = normalizeRSXSource(update.Source)
	}
	if update.PivotRadius > 0 {
		rsxSettingsInstance.PivotRadius = clampInt(update.PivotRadius, MinRSXPivotRadius, MaxRSXPivotRadius, DefaultRSXPivotRadius)
	}
	if update.DivMethod != "" {
		rsxSettingsInstance.DivMethod = normalizeRSXDivMethod(update.DivMethod)
	}

	return *rsxSettingsInstance
}

// ResetRSXSettings restores dashboard defaults (tests).
func ResetRSXSettings() {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.Lock()
	defer rsxSettingsMutex.Unlock()
	*rsxSettingsInstance = RSXSettings{
		Length:       DefaultRSXLength,
		DivLookback:  RSXLookbackDefault,
		SignalLength: DefaultRSXSignalLength,
		Source:       "close",
		PivotRadius:  DefaultRSXPivotRadius,
		DivMethod:    "tv",
	}
}

// RSXPivotRadius returns the active fractal pivot radius from settings.
func RSXPivotRadius() int {
	r := GetRSXSettings().PivotRadius
	if r <= 0 {
		return DefaultRSXPivotRadius
	}
	return r
}

// RSXUsesFractalDiv reports whether fractal (classic pivot) divergence mode is active.
func RSXUsesFractalDiv() bool {
	return normalizeRSXDivMethod(GetRSXSettings().DivMethod) == "fractal"
}

// RSXSourcePrice returns the scalar fed into Jurik RSX for the current bar.
func RSXSourcePrice(high, low, close float64, source string) float64 {
	if normalizeRSXSource(source) == "close" {
		return close
	}
	return (high + low + close) / 3.0
}

func normalizeRSXSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "close":
		return "close"
	default:
		return "hlc3"
	}
}

func normalizeRSXDivMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "fractal":
		return "fractal"
	default:
		return "tv"
	}
}

func clampInt(val, min, max, fallback int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	if val <= 0 {
		return fallback
	}
	return val
}
