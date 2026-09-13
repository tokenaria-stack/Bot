package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type structuralStopSnapshot struct {
	WickOwner   string            `json:"wick_owner"`
	LongSample  []int             `json:"long_sample"`
	Assignments []json.RawMessage `json:"assignments"`
	Report      string            `json:"report"`
	Notes       []string          `json:"notes"`
}

func researchStructuralStopFile(name string) string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join("research", "structuralstop1", name)
	}
	dir := wd
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, "research", "structuralstop1", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(wd, "research", "structuralstop1", name)
}

func structuralStopPath() string {
	s0 := researchStructuralStopFile("snapshot_s0.json")
	if _, err := os.Stat(s0); err == nil {
		return s0
	}
	return researchStructuralStopFile("snapshot.json")
}

func loadJSONRowsByOrdinal(name string, ordinal int) map[string]any {
	raw, err := os.ReadFile(researchStructuralStopFile(name))
	if err != nil {
		return nil
	}
	var doc struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	for _, row := range doc.Rows {
		switch v := row["ordinal"].(type) {
		case float64:
			if int(v) == ordinal {
				return row
			}
		}
	}
	return nil
}

func loadPriceSwingLawRow(ordinal int) map[string]any {
	return loadJSONRowsByOrdinal("price_swing_law.json", ordinal)
}

func loadSignificanceRow(ordinal int) map[string]any {
	return loadJSONRowsByOrdinal("structural_significance.json", ordinal)
}

func loadStructuralStopSnapshot() (structuralStopSnapshot, error) {
	raw, err := os.ReadFile(structuralStopPath())
	if err != nil {
		return structuralStopSnapshot{}, err
	}
	var snap structuralStopSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return structuralStopSnapshot{}, err
	}
	return snap, nil
}

func (d *DashboardServer) handleStructuralStop1(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	snap, err := loadStructuralStopSnapshot()
	if err != nil {
		http.Error(w, "structural-stop-1 snapshot missing (run STRUCTURAL_STOP_1=1)", http.StatusNotFound)
		return
	}
	idxStr := r.URL.Query().Get("sample")
	if idxStr == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"wick_owner":   snap.WickOwner,
			"sample_count": len(snap.LongSample),
			"long_sample":  snap.LongSample,
			"notes":        snap.Notes,
			"report":       snap.Report,
			"judgments":    []string{"AGREE", "TOO_TIGHT", "TOO_FAR", "WRONG_PIVOT", "NO_CLEAR_STRUCTURE"},
		})
		return
	}
	i, err := strconv.Atoi(idxStr)
	if err != nil || i < 0 || i >= len(snap.LongSample) {
		http.Error(w, "bad sample", http.StatusBadRequest)
		return
	}
	ai := snap.LongSample[i]
	if ai < 0 || ai >= len(snap.Assignments) {
		http.Error(w, "sample index", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"wick_owner":   snap.WickOwner,
		"sample_i":     i,
		"sample_count": len(snap.LongSample),
		"assignment":   json.RawMessage(snap.Assignments[ai]),
		"price_swing":  loadPriceSwingLawRow(i + 1),
		"significance": loadSignificanceRow(i + 1),
		"judgments":    []string{"AGREE", "TOO_TIGHT", "TOO_FAR", "WRONG_PIVOT", "NO_CLEAR_STRUCTURE"},
	})
}
