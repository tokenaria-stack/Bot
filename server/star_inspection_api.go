package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"trading_bot/market"
)

type inspectionFile struct {
	Count  int
	Rows   []inspectionStar
	Digest string
}

var (
	inspectionOnce sync.Once
	inspectionBody inspectionFile
	inspectionErr  error
)

func inspectionArtifact(name string) string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join("research", "starstop", name)
	}
	dir := wd
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, "research", "starstop", name)
		if _, statErr := os.Stat(p); statErr == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(wd, "research", "starstop", name)
}

func loadInspection() (inspectionFile, error) {
	inspectionOnce.Do(func() {
		inspectionBody, inspectionErr = readInspection()
	})
	return inspectionBody, inspectionErr
}

func readInspection() (inspectionFile, error) {
	snapRaw, err := os.ReadFile(inspectionArtifact("star_snapshot_v3.json"))
	if err != nil {
		return inspectionFile{}, err
	}
	relRaw, err := os.ReadFile(inspectionArtifact("star_relative_v1.json"))
	if err != nil {
		return inspectionFile{}, err
	}
	pathRaw, err := os.ReadFile(inspectionArtifact("star_discovery_v1.json"))
	if err != nil {
		return inspectionFile{}, err
	}
	var snaps struct {
		Digest string                  `json:"digest"`
		Rows   []market.StarSnapshotV3 `json:"rows"`
	}
	var rels struct {
		Rows []market.StarRelations `json:"rows"`
	}
	var paths struct {
		Rows []market.PathFact `json:"rows"`
	}
	if err := json.Unmarshal(snapRaw, &snaps); err != nil {
		return inspectionFile{}, err
	}
	if err := json.Unmarshal(relRaw, &rels); err != nil {
		return inspectionFile{}, err
	}
	if err := json.Unmarshal(pathRaw, &paths); err != nil {
		return inspectionFile{}, err
	}
	rows, _, err := collectInspectionStars(snaps.Rows, rels.Rows, paths.Rows)
	if err != nil {
		return inspectionFile{}, err
	}
	return inspectionFile{Count: len(rows), Rows: rows, Digest: snaps.Digest}, nil
}

func (d *DashboardServer) handleStarInspection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	doc, err := loadInspection()
	if err != nil || doc.Count == 0 {
		http.Error(w, "inspection artifact missing", http.StatusNotFound)
		return
	}
	idx, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil || idx < 0 || idx >= doc.Count {
		http.Error(w, "bad index", http.StatusBadRequest)
		return
	}
	star := doc.Rows[idx]
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"index":        idx,
		"count":        doc.Count,
		"side":         star.Side,
		"decisionAt":   star.DecisionAt,
		"inspectionAt": star.InspectionAt,
		"holdout":      star.Holdout,
		"pathLabel":    star.PathLabel,
		"favorable":    star.Favorable,
		"adverse":      star.Adverse,
		"readings":     star.Readings,
		"schema3":      doc.Digest,
	})
}
