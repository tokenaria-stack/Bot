package starlens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SaveChild evaluates the lens and writes a new population file.
// The parent value is not modified. The outcome file is not modified.
// savedAt is recorded and is not part of the child id.
func SaveChild(parent Population, table []Row, clauses []Clause, path string, savedAt time.Time) (Population, error) {
	if parent.ID == "" {
		return Population{}, fmt.Errorf("starlens: parent has no id")
	}
	if parent.Provenance.OutcomeDigest == "" {
		return Population{}, fmt.Errorf("starlens: outcome digest is required")
	}
	result, err := Evaluate(parent, table, clauses)
	if err != nil {
		return Population{}, err
	}
	id, err := populationID(parent.ID, "lens", parent.Provenance, clauses)
	if err != nil {
		return Population{}, err
	}
	child := Population{
		ID: id, ParentID: parent.ID, Operation: "lens",
		SavedAt:    SavedAtTime(savedAt),
		Provenance: parent.Provenance,
		Clauses:    append([]Clause(nil), clauses...),
		Members:    result.Pass,
	}
	if err := writePopulation(path, child); err != nil {
		return Population{}, err
	}
	return child, nil
}

func writePopulation(path string, pop Population) error {
	body, err := json.MarshalIndent(pop, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// LoadPopulation reads a child or a universe file.
// It does not compare the members with a fresh evaluation.
func LoadPopulation(path string) (Population, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Population{}, err
	}
	var pop Population
	if err := json.Unmarshal(body, &pop); err != nil {
		return Population{}, err
	}
	if pop.ID == "" || pop.Operation == "" {
		return Population{}, fmt.Errorf("starlens: population file %s has no id", path)
	}
	return pop, nil
}

// Certify replays the child clauses against the parent.
// A mismatch is returned. The child file is not rewritten.
func Certify(parent Population, table []Row, child Population) error {
	if child.Operation != "lens" {
		return fmt.Errorf("starlens: certify operation %q", child.Operation)
	}
	if child.ParentID != parent.ID {
		return fmt.Errorf("starlens: child parent %s", child.ParentID)
	}
	if child.Provenance != parent.Provenance {
		return fmt.Errorf("starlens: child provenance differs from the parent")
	}
	wantID, err := populationID(parent.ID, "lens", parent.Provenance, child.Clauses)
	if err != nil {
		return err
	}
	if child.ID != wantID {
		return fmt.Errorf("starlens: child id does not match the clauses")
	}
	result, err := Evaluate(parent, table, child.Clauses)
	if err != nil {
		return err
	}
	if !sameRefs(result.Pass, child.Members) {
		return fmt.Errorf("starlens: replayed members differ from the stored child")
	}
	return nil
}

func sameRefs(a, b []StarRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
