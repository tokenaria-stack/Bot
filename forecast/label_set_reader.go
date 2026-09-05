package forecast

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ReadLabelSet decodes a completed LabelSet and refuses any structural or
// integrity mismatch. Optional expect fields must match when non-nil.
func ReadLabelSet(path string, expect *LabelExpect) (LabelHeader, []LabelRow, LabelFooter, error) {
	f, err := os.Open(path)
	if err != nil {
		return LabelHeader{}, nil, LabelFooter{}, err
	}
	defer f.Close()
	return decodeLabelSet(f, expect)
}

func decodeLabelSet(r io.Reader, expect *LabelExpect) (LabelHeader, []LabelRow, LabelFooter, error) {
	br := bufio.NewReader(r)
	line, err := readJSONLLine(br)
	if err != nil {
		return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set missing header: %w", err)
	}
	var kind struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(line, &kind); err != nil {
		return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set header: %w", err)
	}
	if kind.Kind != labelKindHeader {
		return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set first record must be header, got %q", kind.Kind)
	}
	var hj labelHeaderJSON
	if err := json.Unmarshal(line, &hj); err != nil {
		return LabelHeader{}, nil, LabelFooter{}, err
	}
	target, err := ParseDigestHex(hj.TargetDigest)
	if err != nil {
		return LabelHeader{}, nil, LabelFooter{}, err
	}
	plan, err := ParseDigestHex(hj.FeatureTapePlanDigest)
	if err != nil {
		return LabelHeader{}, nil, LabelFooter{}, err
	}
	tapeSrc, err := ParseDigestHex(hj.FeatureTapeSourceRangeDigest)
	if err != nil {
		return LabelHeader{}, nil, LabelFooter{}, err
	}
	tapeContent, err := ParseDigestHex(hj.FeatureTapeContentDigest)
	if err != nil {
		return LabelHeader{}, nil, LabelFooter{}, err
	}
	hdr := LabelHeader{
		FormatVersion:                hj.FormatVersion,
		Market:                       marketFromJSON(hj.Market),
		TargetDigest:                 target,
		LabelLogicVersion:            LogicVersion(hj.LabelLogicVersion),
		FeatureTapePlanDigest:        plan,
		FeatureTapeSourceRangeDigest: tapeSrc,
		FeatureTapeContentDigest:     tapeContent,
	}
	if err := validateLabelHeader(hdr); err != nil {
		return LabelHeader{}, nil, LabelFooter{}, err
	}
	if expect != nil {
		if expect.Market != nil && *expect.Market != hdr.Market {
			return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set MarketKey mismatch")
		}
		if expect.Target != nil && *expect.Target != hdr.TargetDigest {
			return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set TargetDigest mismatch")
		}
		if expect.TapePlan != nil && *expect.TapePlan != hdr.FeatureTapePlanDigest {
			return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set FeatureTape PlanDigest mismatch")
		}
		if expect.TapeSource != nil && *expect.TapeSource != hdr.FeatureTapeSourceRangeDigest {
			return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set FeatureTape SourceRangeDigest mismatch")
		}
		if expect.TapeContent != nil && *expect.TapeContent != hdr.FeatureTapeContentDigest {
			return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set FeatureTape ContentDigest mismatch")
		}
	}

	content := newLabelContentHasher()
	content.header(hdr)
	var rows []LabelRow
	var lastAt int64
	sawFooter := false
	var footer LabelFooter

	for {
		line, err = readJSONLLine(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return LabelHeader{}, nil, LabelFooter{}, err
		}
		if err := json.Unmarshal(line, &kind); err != nil {
			return LabelHeader{}, nil, LabelFooter{}, err
		}
		if sawFooter {
			return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set record after footer")
		}
		switch kind.Kind {
		case labelKindHeader:
			return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: duplicate label-set header")
		case labelKindRow:
			var rj labelRowJSON
			if err := json.Unmarshal(line, &rj); err != nil {
				return LabelHeader{}, nil, LabelFooter{}, err
			}
			row := LabelRow{
				At:      rj.At,
				Outcome: TargetOutcome(rj.Outcome),
				HitAt:   rj.HitAt,
				Reason:  LabelReason(rj.Reason),
			}
			if err := validateLabelRow(row); err != nil {
				return LabelHeader{}, nil, LabelFooter{}, err
			}
			if len(rows) > 0 && row.At <= lastAt {
				return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set At must be strictly increasing")
			}
			rows = append(rows, row)
			content.row(row)
			lastAt = row.At
		case labelKindFooter:
			var fj labelFooterJSON
			if err := json.Unmarshal(line, &fj); err != nil {
				return LabelHeader{}, nil, LabelFooter{}, err
			}
			src, err := ParseDigestHex(fj.LabelSourceRangeDigest)
			if err != nil {
				return LabelHeader{}, nil, LabelFooter{}, err
			}
			gotContent, err := ParseDigestHex(fj.ContentDigest)
			if err != nil {
				return LabelHeader{}, nil, LabelFooter{}, err
			}
			if fj.RowCount != len(rows) {
				return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set RowCount %d != decoded rows %d", fj.RowCount, len(rows))
			}
			if len(rows) == 0 {
				return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: refuse empty label-set")
			}
			if fj.FirstAt != rows[0].At || fj.LastAt != rows[len(rows)-1].At {
				return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set FirstAt/LastAt mismatch")
			}
			var zero Digest
			if src == zero {
				return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: LabelSourceRangeDigest is required")
			}
			content.meta(src, fj.RowCount, fj.FirstAt, fj.LastAt)
			want := content.sum()
			if want != gotContent {
				return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set ContentDigest mismatch")
			}
			footer = LabelFooter{
				LabelSourceRangeDigest: src,
				RowCount:               fj.RowCount,
				FirstAt:                fj.FirstAt,
				LastAt:                 fj.LastAt,
				ContentDigest:          gotContent,
			}
			sawFooter = true
		default:
			return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: unknown label-set kind %q", kind.Kind)
		}
	}
	if !sawFooter {
		return LabelHeader{}, nil, LabelFooter{}, fmt.Errorf("forecast: label-set missing footer")
	}
	return hdr, rows, footer, nil
}
