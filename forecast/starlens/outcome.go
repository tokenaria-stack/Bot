package starlens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// rawRow keeps pointer fields so an omitted JSON key stays nil.
// A decoded float of 0 is not enough. The 0.15 file omits r when R is undefined,
// and it also stores a real numeric 0 on other fields.
type rawRow struct {
	DecisionAt int64  `json:"decisionAt"`
	Side       string `json:"side"`
	Status     string `json:"status"`

	EntryOK    bool     `json:"entryOk"`
	EntryPrice *float64 `json:"entryPrice"`

	ExcursionOK bool     `json:"excursionOk"`
	MFEPrice    *float64 `json:"mfePrice"`
	MAEPrice    *float64 `json:"maePrice"`
	MFEPercent  *float64 `json:"mfePercent"`
	MAEPercent  *float64 `json:"maePercent"`
	MFEAt       *int64   `json:"mfeAt"`
	MAEAt       *int64   `json:"maeAt"`

	ATRExcursion bool     `json:"atrExcursion"`
	MFEATR       *float64 `json:"mfeAtr"`
	MAEATR       *float64 `json:"maeAtr"`

	StopPrice       *float64 `json:"stopPrice"`
	StopDistance    *float64 `json:"stopDistance"`
	StopDistanceATR *float64 `json:"stopDistanceAtr"`
	R               *float64 `json:"r"`
	StopReached     bool     `json:"stopReached"`
	StopAt          *int64   `json:"stopAt"`
	Hit1R           bool     `json:"hit1R"`
	Hit2R           bool     `json:"hit2R"`
	Hit3R           bool     `json:"hit3R"`
	At1R            *int64   `json:"at1R"`
	At2R            *int64   `json:"at2R"`
	At3R            *int64   `json:"at3R"`
}

type rawFile struct {
	Digest    string   `json:"digest"`
	BufferATR float64  `json:"bufferAtr"`
	Rows      []rawRow `json:"rows"`
}

// ReadOutcomeFile reads star_sl_outcome.json.
// It does not read the capture dataset and it does not write the file back.
func ReadOutcomeFile(path string) (digest string, rows []Row, err error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var doc rawFile
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", nil, err
	}
	if doc.BufferATR != OutcomeBuffer {
		return "", nil, fmt.Errorf("starlens: outcome buffer %v", doc.BufferATR)
	}
	rows = make([]Row, len(doc.Rows))
	for i := range doc.Rows {
		row, err := convertRow(i, doc.Rows[i])
		if err != nil {
			return "", nil, err
		}
		rows[i] = row
	}
	return doc.Digest, rows, nil
}

func convertRow(index int, raw rawRow) (Row, error) {
	if raw.Side != "up" && raw.Side != "down" {
		return Row{}, fmt.Errorf("starlens: side %q at %d", raw.Side, index)
	}
	if raw.DecisionAt <= 0 {
		return Row{}, fmt.Errorf("starlens: decision time at %d", index)
	}
	row := Row{
		Index: index, DecisionAt: raw.DecisionAt, Side: raw.Side, Status: raw.Status,
		R1:   rState(raw.R, raw.Hit1R),
		R2:   rState(raw.R, raw.Hit2R),
		R3:   rState(raw.R, raw.Hit3R),
		Stop: stopState(raw.R, raw.StopReached),
	}
	if raw.EntryOK {
		row.EntryPrice = observedPtr(raw.EntryPrice)
	}
	if raw.ExcursionOK {
		row.MFEPrice = observedPtr(raw.MFEPrice)
		row.MAEPrice = observedPtr(raw.MAEPrice)
		row.MFEAt = observedTime(raw.MFEAt)
		row.MAEAt = observedTime(raw.MAEAt)
		if raw.EntryOK {
			row.MFEPercent = observedPtr(raw.MFEPercent)
			row.MAEPercent = observedPtr(raw.MAEPercent)
		}
	}
	if raw.ATRExcursion {
		row.MFEATR = observedPtr(raw.MFEATR)
		row.MAEATR = observedPtr(raw.MAEATR)
	}
	row.StopPrice = observedPositive(raw.StopPrice)
	row.StopDistance = observedPositive(raw.StopDistance)
	row.StopDistanceATR = observedPositive(raw.StopDistanceATR)
	row.R = observedPositive(raw.R)
	row.StopAt = observedTime(raw.StopAt)
	row.At1R = observedTime(raw.At1R)
	row.At2R = observedTime(raw.At2R)
	row.At3R = observedTime(raw.At3R)
	return row, nil
}

// FindOutcomeFile walks parents for research/starstop/star_sl_outcome.json.
func FindOutcomeFile() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		p := filepath.Join(dir, "research", "starstop", "star_sl_outcome.json")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("starlens: star_sl_outcome.json not found from %s", wd)
		}
		dir = parent
	}
}
