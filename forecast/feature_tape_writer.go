package forecast

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TapeWriter is a dumb JSONL FeatureTape encoder. It does not calculate features.
type TapeWriter struct {
	path    string
	tmpPath string
	f       *os.File
	bw      *bufio.Writer
	enc     *json.Encoder
	hdr     TapeHeader
	content *contentHasher
	count   int
	firstAt int64
	lastAt  int64
	closed  bool
}

// CreateTapeWriter writes a header to a sibling temp file. finalPath must not exist.
func CreateTapeWriter(finalPath string, hdr TapeHeader) (*TapeWriter, error) {
	if finalPath == "" {
		return nil, fmt.Errorf("forecast: feature-tape path is required")
	}
	if err := validateHeader(hdr); err != nil {
		return nil, err
	}
	if err := tapeDirOk(finalPath); err != nil {
		return nil, err
	}
	if _, err := os.Stat(finalPath); err == nil {
		return nil, fmt.Errorf("forecast: refuse overwrite of existing feature-tape %s", finalPath)
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
	w := &TapeWriter{
		path:    finalPath,
		tmpPath: tmpPath,
		f:       f,
		bw:      bw,
		enc:     enc,
		hdr:     hdr,
		content: newContentHasher(),
	}
	w.content.header(hdr)
	if err := enc.Encode(tapeHeaderJSON{
		Kind:          tapeKindHeader,
		FormatVersion: hdr.FormatVersion,
		Market:        marketJSON(hdr.Market),
		PlanDigest:    hdr.PlanDigest.String(),
		FeatureIDs:    append([]FeatureID(nil), hdr.FeatureIDs...),
		VectorLen:     hdr.VectorLen,
	}); err != nil {
		w.abort()
		return nil, err
	}
	return w, nil
}

// WriteRow appends one source-bar observation. ready=false requires values == nil.
func (w *TapeWriter) WriteRow(at int64, ready Ready, values []float64) error {
	if w == nil || w.closed {
		return fmt.Errorf("forecast: feature-tape writer is closed")
	}
	if err := validateReadyValues(w.hdr.VectorLen, ready, values); err != nil {
		w.abort()
		return err
	}
	if w.count > 0 && at <= w.lastAt {
		w.abort()
		return fmt.Errorf("forecast: feature-tape At must be strictly increasing (got %d after %d)", at, w.lastAt)
	}
	var stored []float64
	if ready {
		stored = append([]float64(nil), values...)
	}
	rec := tapeRowJSON{Kind: tapeKindRow, At: at, Ready: bool(ready), Values: stored}
	if err := w.enc.Encode(rec); err != nil {
		w.abort()
		return err
	}
	w.content.row(at, ready, stored)
	if w.count == 0 {
		w.firstAt = at
	}
	w.lastAt = at
	w.count++
	return nil
}

// Finish writes the footer, flushes, closes, and atomically renames temp → final.
func (w *TapeWriter) Finish(sourceRange Digest) error {
	if w == nil || w.closed {
		return fmt.Errorf("forecast: feature-tape writer is closed")
	}
	if w.count == 0 {
		w.abort()
		return fmt.Errorf("forecast: refuse empty feature-tape")
	}
	var zero Digest
	if sourceRange == zero {
		w.abort()
		return fmt.Errorf("forecast: SourceRangeDigest is required")
	}
	w.content.meta(sourceRange, w.count, w.firstAt, w.lastAt)
	content := w.content.sum()
	if err := w.enc.Encode(tapeFooterJSON{
		Kind:              tapeKindFooter,
		SourceRangeDigest: sourceRange.String(),
		RowCount:          w.count,
		FirstAt:           w.firstAt,
		LastAt:            w.lastAt,
		ContentDigest:     content.String(),
	}); err != nil {
		w.abort()
		return err
	}
	if err := w.bw.Flush(); err != nil {
		w.abort()
		return err
	}
	if err := w.f.Close(); err != nil {
		w.closed = true
		_ = os.Remove(w.tmpPath)
		return err
	}
	w.f = nil
	w.closed = true
	if err := os.Rename(w.tmpPath, w.path); err != nil {
		_ = os.Remove(w.tmpPath)
		return err
	}
	return nil
}

// Abort closes and removes the temp file without producing a final artifact.
func (w *TapeWriter) Abort() {
	w.abort()
}

func (w *TapeWriter) abort() {
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

func tapeDirOk(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	st, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("forecast: feature-tape parent is not a directory")
	}
	return nil
}
