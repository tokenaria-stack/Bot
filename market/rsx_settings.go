package market

import (
	"encoding/json"
	"log"
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

// RSXSettings is engine-owned RSX / divergence configuration (ADR-012).
type RSXSettings struct {
	Length             int     `json:"length"`
	DivLookback        int     `json:"div_lookback"`
	SignalLength       int     `json:"signal_length"`
	Source             string  `json:"source"`       // "close" or "hlc3" (default hlc3)
	PivotRadius        int     `json:"pivot_radius"` // fractal divergence
	DivMethod          string  `json:"div_method"`   // "tv" or "fractal"
	MinPriceDeltaRatio float64 `json:"min_price_delta_ratio"`
	MinOscDelta        float64 `json:"min_osc_delta"`
}

// RSXApplyResult is the engine response after a settings apply (menu / API).
type RSXApplyResult struct {
	Settings   RSXSettings `json:"settings"`
	Generation int64       `json:"generation"`
	Changed    bool        `json:"changed"`
}

var (
	rsxSettingsInstance *RSXSettings
	rsxSettingsOnce     sync.Once
	rsxSettingsMutex    sync.RWMutex
	rsxSettingsGen      int64
)

// GetRSXSettings returns a copy of the engine RSX SSOT.
func GetRSXSettings() RSXSettings {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.RLock()
	defer rsxSettingsMutex.RUnlock()
	return *rsxSettingsInstance
}

// RSXSettingsGeneration increments on each successful commit (projection epoch hint).
func RSXSettingsGeneration() int64 {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.RLock()
	defer rsxSettingsMutex.RUnlock()
	return rsxSettingsGen
}

func initRSXSettingsDefaults() {
	rsxSettingsInstance = &RSXSettings{}
	*rsxSettingsInstance = defaultRSXSettings()
	rsxSettingsGen = 1
}

func defaultRSXSettings() RSXSettings {
	return RSXSettings{
		Length:       DefaultRSXLength,
		DivLookback:  RSXLookbackDefault,
		SignalLength: DefaultRSXSignalLength,
		Source:       "hlc3",
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

// ApplyRSXSettings merges update into engine SSOT (compare-before-mutate).
// On change: commit, bump generation, autosave to disk.
func ApplyRSXSettings(update RSXSettings) RSXApplyResult {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.Lock()
	old := *rsxSettingsInstance
	normalized := NormalizeRSXSettings(mergeRSXSettings(old, update))
	if RSXSettingsEqual(old, normalized) {
		gen := rsxSettingsGen
		rsxSettingsMutex.Unlock()
		return RSXApplyResult{Settings: old, Generation: gen, Changed: false}
	}
	*rsxSettingsInstance = normalized
	rsxSettingsGen++
	gen := rsxSettingsGen
	rsxSettingsMutex.Unlock()
	if err := saveRSXSettingsToDisk(normalized); err != nil {
		log.Printf("[RSX] autosave failed: %v", err)
	}
	return RSXApplyResult{Settings: normalized, Generation: gen, Changed: true}
}

// replaceRSXSettingsForBoot forces SSOT from disk without treating as a user edit
// (no autosave write-back; generation bumped once).
func replaceRSXSettingsForBoot(settings RSXSettings) {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.Lock()
	defer rsxSettingsMutex.Unlock()
	normalized := NormalizeRSXSettings(settings)
	*rsxSettingsInstance = normalized
	rsxSettingsGen++
}

// ResetRSXSettings restores dashboard defaults (tests). Does not touch the autosave file.
func ResetRSXSettings() {
	rsxSettingsOnce.Do(initRSXSettingsDefaults)
	rsxSettingsMutex.Lock()
	defer rsxSettingsMutex.Unlock()
	*rsxSettingsInstance = defaultRSXSettings()
	rsxSettingsGen++
}

// RSXImpactOfChange is the RSX implementation of ImpactOfChange (ADR-013).
// Only RSX implements this in B1; Wozduh/ATR will add their own later.
func RSXImpactOfChange(old, new RSXSettings) ChangeImpact {
	a := NormalizeRSXSettings(old)
	b := NormalizeRSXSettings(new)
	if RSXSettingsEqual(a, b) {
		return ChangeImpactProjectionOnly
	}
	if a.Length != b.Length ||
		a.SignalLength != b.SignalLength ||
		normalizeRSXSource(a.Source) != normalizeRSXSource(b.Source) {
		return ChangeImpactIndicatorReplay
	}
	if normalizeRSXDivMethod(a.DivMethod) != normalizeRSXDivMethod(b.DivMethod) ||
		a.PivotRadius != b.PivotRadius ||
		a.DivLookback != b.DivLookback ||
		a.MinPriceDeltaRatio != b.MinPriceDeltaRatio ||
		a.MinOscDelta != b.MinOscDelta {
		return ChangeImpactAnnotationOnly
	}
	return ChangeImpactProjectionOnly
}

// RSXNeedsStreamingReplay reports whether math engines must cold-replay klines.
func RSXNeedsStreamingReplay(prev, next RSXSettings) bool {
	return RSXImpactOfChange(prev, next) == ChangeImpactIndicatorReplay
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
