package strategy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MatrixConfigPath is the default location for persisted scoring matrix toggles (project root relative).
const MatrixConfigPath = "server/config/matrix.json"

// ScoringMatrix toggles individual scoring rule contributions.
type ScoringMatrix struct {
	UseRSX              bool `json:"useRSX"`
	UseWozduhCross      bool `json:"useWozduhCross"`
	UseRedCross         bool `json:"useRedCross"`
	UseGeometry         bool `json:"useGeometry"`
	UseGeometryBounce   bool `json:"useGeometryBounce"`
	UseGeometryTriangle bool `json:"useGeometryTriangle"`
	UseTrendlines       bool `json:"useTrendlines"`
	UseHTFOscillators   bool `json:"useHTFOscillators"`
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

// DefaultScoringMatrix returns the conservative startup state (all rules disabled).
func DefaultScoringMatrix() ScoringMatrix {
	return ScoringMatrix{}
}

var (
	scoringMatrixMu sync.RWMutex
	scoringMatrix   = DefaultScoringMatrix()
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

// ResetScoringMatrix restores the conservative default (all rules disabled).
func ResetScoringMatrix() {
	scoringMatrixMu.Lock()
	defer scoringMatrixMu.Unlock()
	scoringMatrix = DefaultScoringMatrix()
}

// SaveMatrixConfig serializes the matrix to JSON and writes it to configPath.
func SaveMatrixConfig(matrix ScoringMatrix, configPath string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(matrix, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0o644)
}

// LoadMatrixConfig reads JSON from filepath into a ScoringMatrix.
// Missing or empty files return DefaultScoringMatrix().
func LoadMatrixConfig(configPath string) (ScoringMatrix, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultScoringMatrix(), nil
		}
		return ScoringMatrix{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return DefaultScoringMatrix(), nil
	}
	var matrix ScoringMatrix
	if err := json.Unmarshal(data, &matrix); err != nil {
		return ScoringMatrix{}, err
	}
	return matrix, nil
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
		!m.UseTrendlines &&
		!m.UseHTFOscillators
}

// ScoringMatrixEntrySourcesEnabled reports whether any entry signal source is active (global).
func ScoringMatrixEntrySourcesEnabled() bool {
	return ScoringMatrixEntrySourcesEnabledFor(scoringMatrixSnapshot())
}

// ScoringMatrixEntrySourcesEnabledFor reports whether any entry source is active in the given matrix.
func ScoringMatrixEntrySourcesEnabledFor(m ScoringMatrix) bool {
	return m.UseRSX || m.UseWozduhCross || m.UseTrendlines
}
