package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trading_bot/forecast/starlens"
	"trading_bot/market"
)

const (
	lensSourceAll       = "all"
	lensSourceUp        = "up"
	lensSourceDown      = "down"
	lensSourceSurvived  = "survived"
	lensSourceStopFirst = "stop_first"
	lensSourceUnordered = "unordered"
	lensSourceDiscovery = "discovery"
	lensSourceHoldout   = "holdout"
)

type lensStore struct {
	rows    []starlens.Row
	digest  string
	sources map[string]starlens.Population
	saved   []starlens.Population
}

var (
	lensOnce  sync.Once
	lensBody  *lensStore
	lensErr   error
	lensSaved sync.Mutex
)

func loadLens() (*lensStore, error) {
	lensOnce.Do(func() {
		lensBody, lensErr = readLens()
	})
	return lensBody, lensErr
}

func readLens() (*lensStore, error) {
	path, err := starlens.FindOutcomeFile()
	if err != nil {
		return nil, err
	}
	digest, rows, err := starlens.ReadOutcomeFile(path)
	if err != nil {
		return nil, err
	}
	if digest != starlens.OutcomeDigest {
		return nil, fmt.Errorf("star lens: outcome digest %s", digest)
	}
	if err := starlens.AttachFrozenCoordinates(rows); err != nil {
		return nil, err
	}
	universe, err := starlens.ResearchUniverse(rows)
	if err != nil {
		return nil, err
	}
	sources := map[string]starlens.Population{lensSourceAll: universe}
	named := []struct {
		id      string
		clauses []starlens.Clause
	}{
		{lensSourceUp, []starlens.Clause{starlens.Side("up")}},
		{lensSourceDown, []starlens.Clause{starlens.Side("down")}},
		{lensSourceSurvived, []starlens.Clause{starlens.Status("survived")}},
		{lensSourceStopFirst, []starlens.Clause{starlens.Status("stop first")}},
		{lensSourceUnordered, []starlens.Clause{starlens.Status("unordered")}},
	}
	for _, item := range named {
		got, err := starlens.Evaluate(universe, rows, item.clauses)
		if err != nil {
			return nil, err
		}
		sources[item.id] = starlens.Population{
			ID:         "source:" + item.id,
			ParentID:   universe.ID,
			Operation:  "source",
			Provenance: universe.Provenance,
			Members:    got.Pass,
		}
	}
	var discovery, holdout []starlens.StarRef
	for _, member := range universe.Members {
		if member.DecisionAt < market.DiscoveryHoldoutAt {
			discovery = append(discovery, member)
		} else {
			holdout = append(holdout, member)
		}
	}
	sources[lensSourceDiscovery] = starlens.Population{
		ID: "source:discovery", ParentID: universe.ID, Operation: "source",
		Provenance: universe.Provenance, Members: discovery,
	}
	sources[lensSourceHoldout] = starlens.Population{
		ID: "source:holdout", ParentID: universe.ID, Operation: "source",
		Provenance: universe.Provenance, Members: holdout,
	}
	store := &lensStore{rows: rows, digest: digest, sources: sources}
	if err := store.loadSaved(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *lensStore) loadSaved() error {
	dir := filepath.Join(filepath.Dir(inspectionArtifact("star_sl_outcome.json")), "populations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		pop, err := starlens.LoadPopulation(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		s.saved = append(s.saved, pop)
	}
	return nil
}

func (s *lensStore) parent(id string) (starlens.Population, error) {
	if pop, ok := s.sources[id]; ok {
		return pop, nil
	}
	for _, pop := range s.saved {
		if pop.ID == id {
			return pop, nil
		}
	}
	return starlens.Population{}, fmt.Errorf("star lens: unknown population %s", id)
}

func periodCounts(members []starlens.StarRef) (discovery, holdout int) {
	for _, member := range members {
		if member.DecisionAt < market.DiscoveryHoldoutAt {
			discovery++
		} else {
			holdout++
		}
	}
	return discovery, holdout
}

type lensSourceInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
	Discovery int    `json:"discovery"`
	Holdout   int    `json:"holdout"`
	ParentID  string `json:"parentId,omitempty"`
	Operation string `json:"operation"`
	SavedAt   string `json:"savedAt,omitempty"`
}

func lensSourceName(id string) string {
	switch id {
	case lensSourceAll:
		return "All Stars"
	case lensSourceUp:
		return "Up"
	case lensSourceDown:
		return "Down"
	case lensSourceSurvived:
		return "Survived"
	case lensSourceStopFirst:
		return "Stop First"
	case lensSourceUnordered:
		return "Unordered"
	case lensSourceDiscovery:
		return "Discovery"
	case lensSourceHoldout:
		return "Holdout"
	default:
		return id
	}
}

func (d *DashboardServer) handleStarLensSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store, err := loadLens()
	if err != nil {
		http.Error(w, "star lens artifact missing", http.StatusNotFound)
		return
	}
	order := []string{
		lensSourceAll, lensSourceUp, lensSourceDown,
		lensSourceSurvived, lensSourceStopFirst, lensSourceUnordered,
		lensSourceDiscovery, lensSourceHoldout,
	}
	out := make([]lensSourceInfo, 0, len(order)+len(store.saved))
	for _, id := range order {
		pop := store.sources[id]
		disc, hold := periodCounts(pop.Members)
		out = append(out, lensSourceInfo{
			ID: id, Name: lensSourceName(id), Count: len(pop.Members),
			Discovery: disc, Holdout: hold, ParentID: pop.ParentID, Operation: pop.Operation,
		})
	}
	lensSaved.Lock()
	saved := append([]starlens.Population(nil), store.saved...)
	lensSaved.Unlock()
	for _, pop := range saved {
		disc, hold := periodCounts(pop.Members)
		name := pop.ID
		if len(name) > 8 {
			name = name[:8]
		}
		out = append(out, lensSourceInfo{
			ID: pop.ID, Name: name, Count: len(pop.Members),
			Discovery: disc, Holdout: hold, ParentID: pop.ParentID,
			Operation: pop.Operation, SavedAt: pop.SavedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"outcomeDigest": store.digest,
		"holdoutAt":     market.DiscoveryHoldoutAt,
		"populations":   out,
	})
}

func (d *DashboardServer) handleStarLensCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := loadLens(); err != nil {
		http.Error(w, "star lens artifact missing", http.StatusNotFound)
		return
	}
	cat := starlens.ResearchCatalog()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":       len(cat),
		"coordinates": cat,
	})
}

type lensEvalRequest struct {
	Source        string            `json:"source"`
	Clauses       []starlens.Clause `json:"clauses"`
	SelectedField string            `json:"selectedField"`
	ViewFrom      int64             `json:"viewFrom"`
	ViewTo        int64             `json:"viewTo"`
}

type lensHistBin struct {
	From     float64 `json:"from"`
	To       float64 `json:"to"`
	Count    int     `json:"count"`
	OpenLow  bool    `json:"openLow,omitempty"`
	OpenHigh bool    `json:"openHigh,omitempty"`
}

type lensNumberWire struct {
	Field    string        `json:"field"`
	Base     int           `json:"base"`
	Missing  int           `json:"missing"`
	Observed int           `json:"observed"`
	Min      float64       `json:"min"`
	Max      float64       `json:"max"`
	RangeOK  bool          `json:"rangeOk"`
	Hist     []lensHistBin `json:"hist"`
}

type lensEventWire struct {
	Name       string  `json:"name"`
	Base       int     `json:"base"`
	Reached    int     `json:"reached"`
	NotReached int     `json:"notReached"`
	Undefined  int     `json:"undefined"`
	Rate       float64 `json:"rate"`
	RateOK     bool    `json:"rateOk"`
}

func wireNumber(field string, pic starlens.NumberPicture) (lensNumberWire, error) {
	out := lensNumberWire{Field: field, Base: pic.Base, Missing: pic.Missing, Observed: len(pic.Values)}
	if len(pic.Values) == 0 {
		return out, nil
	}
	out.Min, out.Max = pic.Values[0], pic.Values[0]
	for _, v := range pic.Values {
		if v < out.Min {
			out.Min = v
		}
		if v > out.Max {
			out.Max = v
		}
	}
	out.RangeOK = true
	hist, err := starlens.DisplayHistogram(pic.Values)
	if err != nil {
		return out, err
	}
	out.Hist = make([]lensHistBin, len(hist))
	sum := 0
	for i, b := range hist {
		out.Hist[i] = lensHistBin{From: b.From, To: b.To, Count: b.Count, OpenLow: b.OpenLow, OpenHigh: b.OpenHigh}
		sum += b.Count
	}
	if sum != out.Observed {
		return out, fmt.Errorf("starlens: histogram sum %d observed %d", sum, out.Observed)
	}
	return out, nil
}

func wireEvent(name string, pic starlens.EventPicture) lensEventWire {
	return lensEventWire{
		Name: name, Base: pic.Base, Reached: pic.Reached, NotReached: pic.NotReached,
		Undefined: pic.Undefined, Rate: pic.Rate, RateOK: pic.RateOK,
	}
}

func (d *DashboardServer) handleStarLensEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store, err := loadLens()
	if err != nil {
		http.Error(w, "star lens artifact missing", http.StatusNotFound)
		return
	}
	var req lensEvalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		req.Source = lensSourceAll
	}
	parent, err := store.parent(req.Source)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := starlens.Evaluate(parent, store.rows, req.Clauses)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fields := []string{
		starlens.FieldMFEATR, starlens.FieldMAEATR,
		starlens.FieldMFEPercent, starlens.FieldMAEPercent,
	}
	numbers := make([]lensNumberWire, 0, len(fields))
	for _, field := range fields {
		pic, err := result.NumberPicture(field)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		wire, err := wireNumber(field, pic)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		numbers = append(numbers, wire)
	}
	if req.SelectedField != "" {
		extra := true
		for _, field := range fields {
			if field == req.SelectedField {
				extra = false
				break
			}
		}
		if extra {
			pic, err := result.NumberPicture(req.SelectedField)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			wire, err := wireNumber(req.SelectedField, pic)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			numbers = append(numbers, wire)
		}
	}
	r1, err := result.RPicture(1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r2, err := result.RPicture(2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r3, err := result.RPicture(3)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	discPop, holdPop := periodCounts(parent.Members)
	discPass, holdPass := periodCounts(result.Pass)
	view := 0
	if req.ViewTo > req.ViewFrom {
		view = starlens.ViewCount(result.Pass, req.ViewFrom, req.ViewTo)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"source":        req.Source,
		"parentId":      parent.ID,
		"population":    result.Population,
		"lensPass":      result.LensPass,
		"chartView":     view,
		"discovery":     discPop,
		"holdout":       holdPop,
		"passDiscovery": discPass,
		"passHoldout":   holdPass,
		"numbers":       numbers,
		"events": []lensEventWire{
			wireEvent("1R / 0.15", r1),
			wireEvent("2R / 0.15", r2),
			wireEvent("3R / 0.15", r3),
			wireEvent("stop / 0.15", result.StopPicture()),
		},
		"status": result.StatusPicture().Counts,
		"pass":   result.Pass,
	})
}

type lensSaveRequest struct {
	Source  string            `json:"source"`
	Clauses []starlens.Clause `json:"clauses"`
}

func (d *DashboardServer) handleStarLensSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store, err := loadLens()
	if err != nil {
		http.Error(w, "star lens artifact missing", http.StatusNotFound)
		return
	}
	var req lensSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	parent, err := store.parent(req.Source)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dir := filepath.Join(filepath.Dir(inspectionArtifact("star_sl_outcome.json")), "populations")
	tmp, err := os.CreateTemp(os.TempDir(), "star-lens-*.json")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmp.Close()
	child, err := starlens.SaveChild(parent, store.rows, req.Clauses, tmp.Name(), time.Now().UTC())
	if err != nil {
		_ = os.Remove(tmp.Name())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := starlens.Certify(parent, store.rows, child); err != nil {
		_ = os.Remove(tmp.Name())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dest := filepath.Join(dir, child.ID+".json")
	body, err := os.ReadFile(tmp.Name())
	_ = os.Remove(tmp.Name())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lensSaved.Lock()
	store.saved = append(store.saved, child)
	lensSaved.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(child)
}
