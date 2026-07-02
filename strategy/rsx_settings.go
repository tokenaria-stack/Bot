package strategy

import (
	"encoding/json"
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
	Length               int     `json:"length"`
	DivLookback          int     `json:"div_lookback"`
	SignalLength         int     `json:"signal_length"`
	Source               string  `json:"source"`       // "close" или "hlc3"
	PivotRadius          int     `json:"pivot_radius"` // Для фрактальных дивергенций
	DivMethod            string  `json:"div_method"`   // "tv" (TradingView) или "fractal"
	MinPriceDeltaRatio   float64 `json:"min_price_delta_ratio"`
	MinOscDelta          float64 `json:"min_osc_delta"`
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
	rsxSettingsInstance = &RSXSettings{}
	*rsxSettingsInstance = defaultRSXSettings()
}

func defaultRSXSettings() RSXSettings {
	return RSXSettings{
		Length:       DefaultRSXLength,
		DivLookback:  RSXLookbackDefault,
		SignalLength: DefaultRSXSignalLength,
		Source:       "close",
		PivotRadius:  DefaultRSXPivotRadius,
		DivMethod:    "tv",
	}
}

func mergeRSXSettings(base, update RSXSettings) RSXSettings {
	out := base
	if update.Length > 0 {
		out.Length = clampInt(update.Length, MinRSXLength, MaxRSXLength, DefaultRSXLength)
	}
	if update.DivLookback > 0 {
		out.DivLookback = clampInt(update.DivLookback, MinRSXDivLookback, MaxRSXDivLookback, RSXLookbackDefault)
	}
	if update.SignalLength > 0 {
		out.SignalLength = clampInt(update.SignalLength, MinRSXSignalLength, MaxRSXSignalLength, DefaultRSXSignalLength)
	}
	if update.Source != "" {
		out.Source = normalizeRSXSource(update.Source)
	}
	if update.PivotRadius > 0 {
		out.PivotRadius = clampInt(update.PivotRadius, MinRSXPivotRadius, MaxRSXPivotRadius, DefaultRSXPivotRadius)
	}
	if update.DivMethod != "" {
		out.DivMethod = normalizeRSXDivMethod(update.DivMethod)
	}
	if update.MinPriceDeltaRatio > 0 {
		out.MinPriceDeltaRatio = update.MinPriceDeltaRatio
	}
	if update.MinOscDelta > 0 {
		out.MinOscDelta = update.MinOscDelta
	}
	return out
}

// NormalizeRSXSettings clamps indicator settings without mutating the global singleton.
func NormalizeRSXSettings(update RSXSettings) RSXSettings {
	normalized := mergeRSXSettings(defaultRSXSettings(), update)
	if normalized.PivotRadius <= 0 {
		normalized.PivotRadius = DefaultRSXPivotRadius
	}
	return normalized
}

// UnmarshalJSON accepts pivot_radius and pivotRadius keys from dashboard payloads.
func (s *RSXSettings) UnmarshalJSON(data []byte) error {
	type alias RSXSettings
	aux := struct {
		alias
		PivotRadiusCamel int `json:"pivotRadius"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*s = RSXSettings(aux.alias)
	if s.PivotRadius <= 0 && aux.PivotRadiusCamel > 0 {
		s.PivotRadius = aux.PivotRadiusCamel
	}
	return nil
}

// ApplyRSXSettings обновляет настройки с clamp и возвращает применённые значения.
func ApplyRSXSettings(update RSXSettings) RSXSettings {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.Lock()
	defer rsxSettingsMutex.Unlock()

	normalized := mergeRSXSettings(*rsxSettingsInstance, update)
	*rsxSettingsInstance = normalized
	return normalized
}

// ResetRSXSettings restores dashboard defaults (tests).
func ResetRSXSettings() {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.Lock()
	defer rsxSettingsMutex.Unlock()
	*rsxSettingsInstance = defaultRSXSettings()
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
