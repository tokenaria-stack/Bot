package forecast

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadTape decodes a completed FeatureTape and refuses any structural or
// integrity mismatch. expectMarket / expectPlan, when non-nil, must match the header.
func ReadTape(path string, expectMarket *MarketKey, expectPlan *Digest) (TapeHeader, []TapeRow, TapeFooter, error) {
	f, err := os.Open(path)
	if err != nil {
		return TapeHeader{}, nil, TapeFooter{}, err
	}
	defer f.Close()
	return decodeTape(f, expectMarket, expectPlan)
}

func decodeTape(r io.Reader, expectMarket *MarketKey, expectPlan *Digest) (TapeHeader, []TapeRow, TapeFooter, error) {
	br := bufio.NewReader(r)
	line, err := readJSONLLine(br)
	if err != nil {
		return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape missing header: %w", err)
	}
	var kind struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(line, &kind); err != nil {
		return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape header: %w", err)
	}
	if kind.Kind != tapeKindHeader {
		return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape first record must be header, got %q", kind.Kind)
	}
	var hj tapeHeaderJSON
	if err := json.Unmarshal(line, &hj); err != nil {
		return TapeHeader{}, nil, TapeFooter{}, err
	}
	plan, err := ParseDigestHex(hj.PlanDigest)
	if err != nil {
		return TapeHeader{}, nil, TapeFooter{}, err
	}
	hdr := TapeHeader{
		FormatVersion: hj.FormatVersion,
		Market:        marketFromJSON(hj.Market),
		PlanDigest:    plan,
		FeatureIDs:    append([]FeatureID(nil), hj.FeatureIDs...),
		VectorLen:     hj.VectorLen,
	}
	if err := validateHeader(hdr); err != nil {
		return TapeHeader{}, nil, TapeFooter{}, err
	}
	if expectMarket != nil && *expectMarket != hdr.Market {
		return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape MarketKey mismatch")
	}
	if expectPlan != nil && *expectPlan != hdr.PlanDigest {
		return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape PlanDigest mismatch")
	}

	content := newContentHasher()
	content.header(hdr)
	var rows []TapeRow
	var lastAt int64
	sawFooter := false
	var footer TapeFooter

	for {
		line, err = readJSONLLine(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return TapeHeader{}, nil, TapeFooter{}, err
		}
		if err := json.Unmarshal(line, &kind); err != nil {
			return TapeHeader{}, nil, TapeFooter{}, err
		}
		if sawFooter {
			return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape record after footer")
		}
		switch kind.Kind {
		case tapeKindHeader:
			return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: duplicate feature-tape header")
		case tapeKindRow:
			var rj tapeRowJSON
			if err := json.Unmarshal(line, &rj); err != nil {
				return TapeHeader{}, nil, TapeFooter{}, err
			}
			ready := Ready(rj.Ready)
			vals := rj.Values
			if !ready {
				if vals != nil {
					return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: Ready=false must omit Values")
				}
			} else if err := validateReadyValues(hdr.VectorLen, ready, vals); err != nil {
				return TapeHeader{}, nil, TapeFooter{}, err
			}
			if len(rows) > 0 && rj.At <= lastAt {
				return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape At must be strictly increasing")
			}
			row := TapeRow{At: rj.At, Ready: ready}
			if ready {
				row.Values = append([]float64(nil), vals...)
			}
			rows = append(rows, row)
			content.row(row.At, row.Ready, row.Values)
			lastAt = row.At
		case tapeKindFooter:
			var fj tapeFooterJSON
			if err := json.Unmarshal(line, &fj); err != nil {
				return TapeHeader{}, nil, TapeFooter{}, err
			}
			src, err := ParseDigestHex(fj.SourceRangeDigest)
			if err != nil {
				return TapeHeader{}, nil, TapeFooter{}, err
			}
			gotContent, err := ParseDigestHex(fj.ContentDigest)
			if err != nil {
				return TapeHeader{}, nil, TapeFooter{}, err
			}
			if fj.RowCount != len(rows) {
				return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape RowCount %d != decoded rows %d", fj.RowCount, len(rows))
			}
			if len(rows) == 0 {
				return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: refuse empty feature-tape")
			}
			if fj.FirstAt != rows[0].At || fj.LastAt != rows[len(rows)-1].At {
				return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape FirstAt/LastAt mismatch")
			}
			var zero Digest
			if src == zero {
				return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: SourceRangeDigest is required")
			}
			content.meta(src, fj.RowCount, fj.FirstAt, fj.LastAt)
			want := content.sum()
			if want != gotContent {
				return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape ContentDigest mismatch")
			}
			footer = TapeFooter{
				SourceRangeDigest: src,
				RowCount:          fj.RowCount,
				FirstAt:           fj.FirstAt,
				LastAt:            fj.LastAt,
				ContentDigest:     gotContent,
			}
			sawFooter = true
		default:
			return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: unknown feature-tape kind %q", kind.Kind)
		}
	}
	if !sawFooter {
		return TapeHeader{}, nil, TapeFooter{}, fmt.Errorf("forecast: feature-tape missing footer")
	}
	return hdr, rows, footer, nil
}

func readJSONLLine(br *bufio.Reader) ([]byte, error) {
	line, err := br.ReadBytes('\n')
	if err != nil && !(err == io.EOF && len(bytes.TrimSpace(line)) > 0) {
		if err == io.EOF && len(bytes.TrimSpace(line)) == 0 {
			return nil, io.EOF
		}
		return nil, err
	}
	line = bytes.TrimRight(line, "\r\n")
	if len(bytes.TrimSpace(line)) == 0 {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("forecast: empty feature-tape line")
	}
	if !json.Valid(line) && !strings.HasPrefix(strings.TrimSpace(string(line)), "{") {
		return nil, fmt.Errorf("forecast: feature-tape line is not JSON")
	}
	return line, nil
}
