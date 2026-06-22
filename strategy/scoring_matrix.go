package strategy

import "sync"

// ScoringMatrix toggles individual scoring rule contributions.
type ScoringMatrix struct {
	UseRSX              bool `json:"useRSX"`
	UseWozduhCross      bool `json:"useWozduhCross"`
	UseRedCross         bool `json:"useRedCross"`
	UseGeometry         bool `json:"useGeometry"`
	UseGeometryBounce   bool `json:"useGeometryBounce"`
	UseGeometryTriangle bool `json:"useGeometryTriangle"`
	UseTrendlines       bool `json:"useTrendlines"`
	UseDivergence       bool `json:"useDivergence"`
	UseFib              bool `json:"useFib"`
	UseExpRegime        bool `json:"useExpRegime"`
	UseJurikTrend       bool `json:"useJurikTrend"`
	UseWozduhSpike      bool `json:"useWozduhSpike"`
	UseAD               bool `json:"useAD"`
	UseAOCross          bool `json:"useAOCross"`
}

// ScoringMatrixSettings is the JSON DTO name for settings.matrix from the dashboard.
type ScoringMatrixSettings = ScoringMatrix

var defaultScoringMatrix = ScoringMatrix{
	UseRSX:              true,
	UseWozduhCross:      true,
	UseRedCross:         true,
	UseGeometry:         true,
	UseGeometryBounce:   true,
	UseGeometryTriangle: true,
	UseTrendlines:       true,
	UseDivergence:       true,
	UseFib:              true,
	UseExpRegime:        true,
	UseJurikTrend:       true,
	UseWozduhSpike:      true,
	UseAD:               true,
	UseAOCross:          true,
}

var (
	scoringMatrixMu sync.RWMutex
	scoringMatrix   = defaultScoringMatrix
)

// GetScoringMatrix returns a copy of the active scoring toggles.
func GetScoringMatrix() ScoringMatrix {
	scoringMatrixMu.RLock()
	defer scoringMatrixMu.RUnlock()
	return scoringMatrix
}

// SetScoringMatrix replaces all scoring toggles.
func SetScoringMatrix(update ScoringMatrix) {
	scoringMatrixMu.Lock()
	defer scoringMatrixMu.Unlock()
	scoringMatrix = update
}

func scoringMatrixSnapshot() ScoringMatrix {
	scoringMatrixMu.RLock()
	defer scoringMatrixMu.RUnlock()
	return scoringMatrix
}

// ResetScoringMatrix restores all scoring rules to enabled.
func ResetScoringMatrix() {
	scoringMatrixMu.Lock()
	defer scoringMatrixMu.Unlock()
	scoringMatrix = defaultScoringMatrix
}

// ScoringMatrixFullyDisabled reports whether every matrix toggle is off (global snapshot).
func ScoringMatrixFullyDisabled() bool {
	return ScoringMatrixFullyDisabledFor(scoringMatrixSnapshot())
}

// ScoringMatrixFullyDisabledFor reports whether every toggle is off in the given matrix.
func ScoringMatrixFullyDisabledFor(m ScoringMatrix) bool {
	return !m.UseRSX &&
		!m.UseWozduhCross &&
		!m.UseRedCross &&
		!m.UseGeometry &&
		!m.UseGeometryBounce &&
		!m.UseGeometryTriangle &&
		!m.UseDivergence &&
		!m.UseFib &&
		!m.UseExpRegime &&
		!m.UseJurikTrend &&
		!m.UseWozduhSpike &&
		!m.UseAD &&
		!m.UseAOCross &&
		!m.UseTrendlines
}

// ScoringMatrixEntrySourcesEnabled reports whether any entry signal source is active (global).
func ScoringMatrixEntrySourcesEnabled() bool {
	return ScoringMatrixEntrySourcesEnabledFor(scoringMatrixSnapshot())
}

// ScoringMatrixEntrySourcesEnabledFor reports whether any entry source is active in the given matrix.
func ScoringMatrixEntrySourcesEnabledFor(m ScoringMatrix) bool {
	return m.UseRSX || m.UseWozduhCross || m.UseTrendlines
}
