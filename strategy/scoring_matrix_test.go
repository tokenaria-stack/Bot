package strategy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func allEnabledScoringMatrix() ScoringMatrix {
	return ScoringMatrix{
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
}

func TestScoringMatrix_DisableRSX(t *testing.T) {
	ResetScoringMatrix()
	t.Cleanup(ResetScoringMatrix)

	m := allEnabledScoringMatrix()
	m.UseRSX = false
	SetScoringMatrix(m)

	report := longSignalReport()
	decision := scalpDecisionFromReport(context.Background(), report)
	if got := decision.LongScore; got != 65 {
		t.Fatalf("LongScore without RSX = %d, want 65 (wozduh+expansion+AO)", got)
	}

	if decision.Action != WaitAction {
		t.Fatalf("Action = %q, want WAIT (score=%d)", decision.Action, decision.Score)
	}
}

func TestScoringMatrix_DisableAll(t *testing.T) {
	ResetScoringMatrix()
	t.Cleanup(ResetScoringMatrix)

	SetScoringMatrix(ScoringMatrix{})

	decision := scalpDecisionFromReport(context.Background(), longSignalReport())
	if decision.Action != WaitAction || decision.Score != 0 {
		t.Fatalf("Action = %q score = %d, want WAIT/0", decision.Action, decision.Score)
	}
}

func TestScoringMatrix_DefaultAllDisabled(t *testing.T) {
	ResetScoringMatrix()
	t.Cleanup(ResetScoringMatrix)

	m := GetScoringMatrix()
	if m.UseRSX || m.UseWozduhCross || m.UseGeometry || m.UseDivergence {
		t.Fatalf("default matrix should be fully disabled: %+v", m)
	}
}

func TestLoadMatrixConfig_MissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "matrix.json")

	m, err := LoadMatrixConfig(path)
	if err != nil {
		t.Fatalf("LoadMatrixConfig() error = %v", err)
	}
	if m.UseRSX || m.UseWozduhCross {
		t.Fatalf("missing file should yield disabled defaults: %+v", m)
	}
}

func TestSaveAndLoadMatrixConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "matrix.json")

	want := ScoringMatrix{
		UseRSX:         true,
		UseWozduhCross: true,
		UseTrendlines:  true,
	}
	if err := SaveMatrixConfig(want, path); err != nil {
		t.Fatalf("SaveMatrixConfig() error = %v", err)
	}

	got, err := LoadMatrixConfig(path)
	if err != nil {
		t.Fatalf("LoadMatrixConfig() error = %v", err)
	}
	if got != want {
		t.Fatalf("loaded matrix = %+v, want %+v", got, want)
	}
}

func TestLoadMatrixConfig_EmptyFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "matrix.json")
	if err := os.WriteFile(path, []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadMatrixConfig(path)
	if err != nil {
		t.Fatalf("LoadMatrixConfig() error = %v", err)
	}
	if m.UseRSX || m.UseTrendlines {
		t.Fatalf("empty file should yield disabled defaults: %+v", m)
	}
}
