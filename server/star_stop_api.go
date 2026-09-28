package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type starStopFile struct {
	Digest          string        `json:"digest"`
	Count           int           `json:"count"`
	Valid           int           `json:"valid"`
	NoStructure     int           `json:"noStructure"`
	NoATR           int           `json:"noAtr"`
	NoEntry         int           `json:"noEntry"`
	InvalidGeometry int           `json:"invalidGeometry"`
	Rows            []starStopRow `json:"rows"`
}

type starStopRow struct {
	DecisionAt       int64      `json:"decisionAt"`
	StarClose        int64      `json:"starClose"`
	Side             string     `json:"side"`
	Signal           float64    `json:"signal"`
	EntryAt          int64      `json:"entryAt,omitempty"`
	EntryPrice       float64    `json:"entryPrice,omitempty"`
	SwingAt          int64      `json:"swingAt,omitempty"`
	SwingWick        float64    `json:"swingWick,omitempty"`
	SwingConfirmedAt int64      `json:"swingConfirmedAt,omitempty"`
	ATR              float64    `json:"atr,omitempty"`
	ATROK            bool       `json:"atrOk"`
	StopPrice        float64    `json:"stopPrice,omitempty"`
	DistanceATR      float64    `json:"distanceAtr,omitempty"`
	Status           string     `json:"status"`
	H1               starStopTF `json:"h1"`
	H4               starStopTF `json:"h4"`
}

type starStopTF struct {
	Present    bool    `json:"present"`
	OpenTime   int64   `json:"openTime,omitempty"`
	CloseTime  int64   `json:"closeTime,omitempty"`
	Vwema      float64 `json:"vwema,omitempty"`
	Slope      float64 `json:"slope,omitempty"`
	Width      float64 `json:"width,omitempty"`
	Distance   float64 `json:"distance,omitempty"`
	ValuesOK   bool    `json:"valuesOk"`
	SlopeOK    bool    `json:"slopeOk"`
	WidthOK    bool    `json:"widthOk"`
	DistanceOK bool    `json:"distanceOk"`
}

func starStopArtifactPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join("research", "starstop", "star_stops.json")
	}
	dir := wd
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, "research", "starstop", "star_stops.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(wd, "research", "starstop", "star_stops.json")
}

func loadStarStops() (starStopFile, error) {
	raw, err := os.ReadFile(starStopArtifactPath())
	if err != nil {
		return starStopFile{}, err
	}
	var doc starStopFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return starStopFile{}, err
	}
	return doc, nil
}

func findStarByDecision(rows []starStopRow, at int64) int {
	lo, hi := 0, len(rows)
	for lo < hi {
		mid := (lo + hi) / 2
		if rows[mid].DecisionAt < at {
			lo = mid + 1
			continue
		}
		hi = mid
	}
	if lo < len(rows) && rows[lo].DecisionAt == at {
		return lo
	}
	return -1
}

func (d *DashboardServer) handleStarStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	doc, err := loadStarStops()
	if err != nil || doc.Count != len(doc.Rows) || doc.Count == 0 {
		http.Error(w, "star-stop artifact missing", http.StatusNotFound)
		return
	}
	idx := 0
	if raw := r.URL.Query().Get("at"); raw != "" {
		at, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || at <= 0 {
			http.Error(w, "bad time", http.StatusBadRequest)
			return
		}
		idx = findStarByDecision(doc.Rows, at)
		if idx < 0 {
			http.Error(w, "not a star", http.StatusNotFound)
			return
		}
	} else if raw := r.URL.Query().Get("index"); raw != "" {
		idx, err = strconv.Atoi(raw)
		if err != nil || idx < 0 || idx >= doc.Count {
			http.Error(w, "bad index", http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"index":           idx,
		"count":           doc.Count,
		"digest":          doc.Digest,
		"valid":           doc.Valid,
		"noStructure":     doc.NoStructure,
		"noAtr":           doc.NoATR,
		"noEntry":         doc.NoEntry,
		"invalidGeometry": doc.InvalidGeometry,
		"row":             doc.Rows[idx],
	})
}
