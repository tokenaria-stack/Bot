package forecast

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// LabelWriter is a dumb JSONL LabelSet encoder. It does not calculate labels.
type LabelWriter struct {
	path    string
	tmpPath string
	f       *os.File
	bw      *bufio.Writer
	enc     *json.Encoder
	hdr     LabelHeader
	content *labelContentHasher
	count   int
	firstAt int64
	lastAt  int64
	closed  bool
}

// CreateLabelWriter writes a header to a sibling temp file. finalPath must not exist.
func CreateLabelWriter(finalPath string, hdr LabelHeader) (*LabelWriter, error) {
	if finalPath == "" {
		return nil, fmt.Errorf("forecast: label-set path is required")
	}
	if err := validateLabelHeader(hdr); err != nil {
		return nil, err
	}
	if err := tapeDirOk(finalPath); err != nil {
		return nil, err
	}
	if _, err := os.Stat(finalPath); err == nil {
		return nil, fmt.Errorf("forecast: refuse overwrite of existing label-set %s", finalPath)
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
	w := &LabelWriter{
		path:    finalPath,
		tmpPath: tmpPath,
		f:       f,
		bw:      bw,
		enc:     enc,
		hdr:     hdr,
		content: newLabelContentHasher(),
	}
	w.content.header(hdr)
	if err := enc.Encode(labelHeaderJSON{
		Kind:                         labelKindHeader,
		FormatVersion:                hdr.FormatVersion,
		Market:                       marketJSON(hdr.Market),
		TargetDigest:                 hdr.TargetDigest.String(),
		LabelLogicVersion:            string(hdr.LabelLogicVersion),
		FeatureTapePlanDigest:        hdr.FeatureTapePlanDigest.String(),
		FeatureTapeSourceRangeDigest: hdr.FeatureTapeSourceRangeDigest.String(),
		FeatureTapeContentDigest:     hdr.FeatureTapeContentDigest.String(),
	}); err != nil {
		w.abort()
		return nil, err
	}
	return w, nil
}

// WriteRow appends one candidate outcome. At must be strictly increasing.
func (w *LabelWriter) WriteRow(row LabelRow) error {
	if w == nil || w.closed {
		return fmt.Errorf("forecast: label-set writer is closed")
	}
	if err := validateLabelRow(row); err != nil {
		w.abort()
		return err
	}
	if w.count > 0 && row.At <= w.lastAt {
		w.abort()
		return fmt.Errorf("forecast: label-set At must be strictly increasing (got %d after %d)", row.At, w.lastAt)
	}
	if err := w.enc.Encode(labelRowJSON{
		Kind:    labelKindRow,
		At:      row.At,
		Outcome: string(row.Outcome),
		HitAt:   row.HitAt,
		Reason:  string(row.Reason),
	}); err != nil {
		w.abort()
		return err
	}
	w.content.row(row)
	if w.count == 0 {
		w.firstAt = row.At
	}
	w.lastAt = row.At
	w.count++
	return nil
}

// Finish writes the footer, flushes, closes, and atomically renames temp → final.
func (w *LabelWriter) Finish(sourceRange Digest) error {
	if w == nil || w.closed {
		return fmt.Errorf("forecast: label-set writer is closed")
	}
	if w.count == 0 {
		w.abort()
		return fmt.Errorf("forecast: refuse empty label-set")
	}
	var zero Digest
	if sourceRange == zero {
		w.abort()
		return fmt.Errorf("forecast: LabelSourceRangeDigest is required")
	}
	w.content.meta(sourceRange, w.count, w.firstAt, w.lastAt)
	content := w.content.sum()
	if err := w.enc.Encode(labelFooterJSON{
		Kind:                   labelKindFooter,
		LabelSourceRangeDigest: sourceRange.String(),
		RowCount:               w.count,
		FirstAt:                w.firstAt,
		LastAt:                 w.lastAt,
		ContentDigest:          content.String(),
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
func (w *LabelWriter) Abort() {
	w.abort()
}

func (w *LabelWriter) abort() {
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

// GenerateLabelSet writes one immutable LabelSet for a FeatureTape + primary bars + TargetSpec.
func GenerateLabelSet(finalPath, tapePath string, spec TargetSpec, bars []CanonicalClosedBar, expect *LabelExpect) error {
	hdr, rows, src, err := BuildLabelSet(tapePath, spec, bars, expect)
	if err != nil {
		return err
	}
	return writeLabelSet(finalPath, hdr, rows, src)
}

func writeLabelSet(finalPath string, hdr LabelHeader, rows []LabelRow, src Digest) error {
	w, err := CreateLabelWriter(finalPath, hdr)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.WriteRow(row); err != nil {
			return err
		}
	}
	return w.Finish(src)
}
