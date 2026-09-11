package forecast

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type tape2HeaderJSON struct {
	Kind           string         `json:"kind"`
	FormatVersion  string         `json:"format_version"`
	SpecDigest     string         `json:"spec_digest"`
	PlanDigest     string         `json:"plan_digest"`
	FeaturesDigest string         `json:"features_digest"`
	AnalysisDigest string         `json:"analysis_digest"`
	TargetDigest   string         `json:"target_digest"`
	Primary        tapeMarketJSON `json:"primary"`
	HTF1h          tapeMarketJSON `json:"htf_1h"`
	HTF4h          tapeMarketJSON `json:"htf_4h"`
	FeatureIDs     []FeatureID    `json:"feature_ids"`
	VectorLen      int            `json:"vector_len"`
	Q              int            `json:"q"`
	Demand         HistoryDemand  `json:"history_demand"`
	PrimarySource  string         `json:"primary_source_digest"`
	HTF1hSource    string         `json:"htf_1h_source_digest"`
	HTF4hSource    string         `json:"htf_4h_source_digest"`
}

type tape2RowJSON struct {
	Kind   string    `json:"kind"`
	At     int64     `json:"at"`
	Ready  bool      `json:"ready"`
	Reason string    `json:"reason,omitempty"`
	Values []float64 `json:"values,omitempty"`
}

type tape2FooterJSON struct {
	Kind          string `json:"kind"`
	RowCount      int    `json:"row_count"`
	ReadyCount    int    `json:"ready_count"`
	NotReadyCount int    `json:"not_ready_count"`
	FirstAt       int64  `json:"first_at"`
	LastAt        int64  `json:"last_at"`
	ContentDigest string `json:"content_digest"`
}

// Tape2Writer encodes feature-tape-v2 JSONL. It does not calculate features.
type Tape2Writer struct {
	path      string
	tmpPath   string
	f         *os.File
	bw        *bufio.Writer
	enc       *json.Encoder
	hdr       Tape2Header
	content   *contentHasher
	count     int
	readyN    int
	notReadyN int
	firstAt   int64
	lastAt    int64
	closed    bool
}

func CreateTape2Writer(finalPath string, hdr Tape2Header) (*Tape2Writer, error) {
	if finalPath == "" {
		return nil, fmt.Errorf("forecast: feature-tape-v2 path is required")
	}
	if err := validateTape2Header(hdr); err != nil {
		return nil, err
	}
	if err := tapeDirOk(finalPath); err != nil {
		return nil, err
	}
	if _, err := os.Stat(finalPath); err == nil {
		return nil, fmt.Errorf("forecast: refuse overwrite of existing feature-tape-v2 %s", finalPath)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	tmpPath := finalPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	w := &Tape2Writer{
		path:    finalPath,
		tmpPath: tmpPath,
		f:       f,
		bw:      bw,
		enc:     enc,
		hdr:     hdr,
		content: newTape2ContentHasher(),
	}
	w.content.header2(hdr)
	if err := enc.Encode(tape2HeaderJSON{
		Kind:           tapeKindHeader,
		FormatVersion:  hdr.FormatVersion,
		SpecDigest:     hdr.SpecDigest.String(),
		PlanDigest:     hdr.PlanDigest.String(),
		FeaturesDigest: hdr.FeaturesDigest.String(),
		AnalysisDigest: hdr.AnalysisDigest.String(),
		TargetDigest:   hdr.TargetDigest.String(),
		Primary:        marketJSON(hdr.Primary),
		HTF1h:          marketJSON(hdr.HTF1h),
		HTF4h:          marketJSON(hdr.HTF4h),
		FeatureIDs:     append([]FeatureID(nil), hdr.FeatureIDs...),
		VectorLen:      hdr.VectorLen,
		Q:              hdr.Q,
		Demand:         hdr.Demand,
		PrimarySource:  hdr.PrimarySource.String(),
		HTF1hSource:    hdr.HTF1hSource.String(),
		HTF4hSource:    hdr.HTF4hSource.String(),
	}); err != nil {
		w.abort()
		return nil, err
	}
	return w, nil
}

func (w *Tape2Writer) WriteRow(at int64, ready Ready, reason NotReadyReason, values []float64) error {
	if w == nil || w.closed {
		return fmt.Errorf("forecast: feature-tape-v2 writer is closed")
	}
	if err := validateTape2Row(ready, reason, values); err != nil {
		w.abort()
		return err
	}
	if w.count > 0 && at <= w.lastAt {
		w.abort()
		return fmt.Errorf("forecast: feature-tape-v2 At must be strictly increasing")
	}
	var stored []float64
	if ready {
		stored = append([]float64(nil), values...)
	}
	rec := tape2RowJSON{Kind: tapeKindRow, At: at, Ready: bool(ready), Values: stored}
	if !ready {
		rec.Reason = string(reason)
	}
	if err := w.enc.Encode(rec); err != nil {
		w.abort()
		return err
	}
	w.content.row2(at, ready, reason, stored)
	if w.count == 0 {
		w.firstAt = at
	}
	w.lastAt = at
	w.count++
	if ready {
		w.readyN++
	} else {
		w.notReadyN++
	}
	return nil
}

func (w *Tape2Writer) Finish() (Tape2Footer, error) {
	var empty Tape2Footer
	if w == nil || w.closed {
		return empty, fmt.Errorf("forecast: feature-tape-v2 writer is closed")
	}
	if w.count == 0 {
		w.abort()
		return empty, fmt.Errorf("forecast: refuse empty feature-tape-v2")
	}
	w.content.meta2(w.count, w.readyN, w.notReadyN, w.firstAt, w.lastAt)
	content := w.content.sum()
	foot := Tape2Footer{
		RowCount: w.count, ReadyCount: w.readyN, NotReadyCount: w.notReadyN,
		FirstAt: w.firstAt, LastAt: w.lastAt, ContentDigest: content,
	}
	if err := w.enc.Encode(tape2FooterJSON{
		Kind: tapeKindFooter, RowCount: foot.RowCount, ReadyCount: foot.ReadyCount,
		NotReadyCount: foot.NotReadyCount, FirstAt: foot.FirstAt, LastAt: foot.LastAt,
		ContentDigest: content.String(),
	}); err != nil {
		w.abort()
		return empty, err
	}
	if err := w.bw.Flush(); err != nil {
		w.abort()
		return empty, err
	}
	if err := w.f.Close(); err != nil {
		w.closed = true
		_ = os.Remove(w.tmpPath)
		return empty, err
	}
	w.f = nil
	w.closed = true
	if err := os.Rename(w.tmpPath, w.path); err != nil {
		_ = os.Remove(w.tmpPath)
		return empty, err
	}
	return foot, nil
}

func (w *Tape2Writer) Abort() { w.abort() }

func (w *Tape2Writer) abort() {
	if w == nil || w.closed {
		return
	}
	w.closed = true
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
	if w.tmpPath != "" {
		_ = os.Remove(w.tmpPath)
	}
}

func ReadTape2(path string) (Tape2Header, []FeatureRow2, Tape2Footer, error) {
	f, err := os.Open(path)
	if err != nil {
		return Tape2Header{}, nil, Tape2Footer{}, err
	}
	defer f.Close()
	return decodeTape2(f)
}

func decodeTape2(r io.Reader) (Tape2Header, []FeatureRow2, Tape2Footer, error) {
	br := bufio.NewReader(r)
	line, err := readJSONLLine(br)
	if err != nil {
		return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 missing header: %w", err)
	}
	var kind struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(line, &kind); err != nil {
		return Tape2Header{}, nil, Tape2Footer{}, err
	}
	if kind.Kind != tapeKindHeader {
		return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 first record must be header")
	}
	var hj tape2HeaderJSON
	if err := json.Unmarshal(line, &hj); err != nil {
		return Tape2Header{}, nil, Tape2Footer{}, err
	}
	hdr, err := headerFromJSON2(hj)
	if err != nil {
		return Tape2Header{}, nil, Tape2Footer{}, err
	}
	if err := validateTape2Header(hdr); err != nil {
		return Tape2Header{}, nil, Tape2Footer{}, err
	}
	content := newTape2ContentHasher()
	content.header2(hdr)
	var rows []FeatureRow2
	var lastAt int64
	sawFooter := false
	var footer Tape2Footer
	for {
		line, err = readJSONLLine(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return Tape2Header{}, nil, Tape2Footer{}, err
		}
		if err := json.Unmarshal(line, &kind); err != nil {
			return Tape2Header{}, nil, Tape2Footer{}, err
		}
		switch kind.Kind {
		case tapeKindRow:
			if sawFooter {
				return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 row after footer")
			}
			var rj tape2RowJSON
			if err := json.Unmarshal(line, &rj); err != nil {
				return Tape2Header{}, nil, Tape2Footer{}, err
			}
			ready := Ready(rj.Ready)
			reason := NotReadyReason(rj.Reason)
			if err := validateTape2Row(ready, reason, rj.Values); err != nil {
				return Tape2Header{}, nil, Tape2Footer{}, err
			}
			if len(rows) > 0 && rj.At <= lastAt {
				return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 At not increasing")
			}
			row := FeatureRow2{At: rj.At, Ready: ready, Reason: reason}
			if ready {
				v, err := FeatureVector2FromSlice(rj.Values)
				if err != nil {
					return Tape2Header{}, nil, Tape2Footer{}, err
				}
				row.Values = v
			}
			content.row2(rj.At, ready, reason, rj.Values)
			rows = append(rows, row)
			lastAt = rj.At
		case tapeKindFooter:
			if sawFooter {
				return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 duplicate footer")
			}
			var fj tape2FooterJSON
			if err := json.Unmarshal(line, &fj); err != nil {
				return Tape2Header{}, nil, Tape2Footer{}, err
			}
			cd, err := ParseDigestHex(fj.ContentDigest)
			if err != nil {
				return Tape2Header{}, nil, Tape2Footer{}, err
			}
			footer = Tape2Footer{
				RowCount: fj.RowCount, ReadyCount: fj.ReadyCount, NotReadyCount: fj.NotReadyCount,
				FirstAt: fj.FirstAt, LastAt: fj.LastAt, ContentDigest: cd,
			}
			sawFooter = true
		default:
			return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 unknown kind %q", kind.Kind)
		}
	}
	if !sawFooter {
		return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 missing footer")
	}
	readyN, notN := 0, 0
	for _, row := range rows {
		if row.Ready {
			readyN++
		} else {
			notN++
		}
	}
	if footer.RowCount != len(rows) || footer.ReadyCount != readyN || footer.NotReadyCount != notN {
		return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 footer counts mismatch")
	}
	if len(rows) == 0 || footer.FirstAt != rows[0].At || footer.LastAt != rows[len(rows)-1].At {
		return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 footer At mismatch")
	}
	content.meta2(footer.RowCount, footer.ReadyCount, footer.NotReadyCount, footer.FirstAt, footer.LastAt)
	if content.sum() != footer.ContentDigest {
		return Tape2Header{}, nil, Tape2Footer{}, fmt.Errorf("forecast: feature-tape-v2 ContentDigest mismatch")
	}
	return hdr, rows, footer, nil
}

func headerFromJSON2(hj tape2HeaderJSON) (Tape2Header, error) {
	parse := func(s string) (Digest, error) { return ParseDigestHex(s) }
	spec, err := parse(hj.SpecDigest)
	if err != nil {
		return Tape2Header{}, err
	}
	plan, err := parse(hj.PlanDigest)
	if err != nil {
		return Tape2Header{}, err
	}
	feat, err := parse(hj.FeaturesDigest)
	if err != nil {
		return Tape2Header{}, err
	}
	an, err := parse(hj.AnalysisDigest)
	if err != nil {
		return Tape2Header{}, err
	}
	tgt, err := parse(hj.TargetDigest)
	if err != nil {
		return Tape2Header{}, err
	}
	ps, err := parse(hj.PrimarySource)
	if err != nil {
		return Tape2Header{}, err
	}
	h1, err := parse(hj.HTF1hSource)
	if err != nil {
		return Tape2Header{}, err
	}
	h4, err := parse(hj.HTF4hSource)
	if err != nil {
		return Tape2Header{}, err
	}
	return Tape2Header{
		FormatVersion:  hj.FormatVersion,
		SpecDigest:     spec,
		PlanDigest:     plan,
		FeaturesDigest: feat,
		AnalysisDigest: an,
		TargetDigest:   tgt,
		Primary:        marketFromJSON(hj.Primary),
		HTF1h:          marketFromJSON(hj.HTF1h),
		HTF4h:          marketFromJSON(hj.HTF4h),
		FeatureIDs:     append([]FeatureID(nil), hj.FeatureIDs...),
		VectorLen:      hj.VectorLen,
		Q:              hj.Q,
		Demand:         hj.Demand,
		PrimarySource:  ps,
		HTF1hSource:    h1,
		HTF4hSource:    h4,
	}, nil
}
