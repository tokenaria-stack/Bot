package forecast

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
)

const (
	CatBoostOOFLogitsFormatV1 = "catboost-oof-logits-v1"
	catBoostLogitsKindHeader  = "header"
	catBoostLogitsKindRow     = "row"
	catBoostLogitsKindFooter  = "footer"
)

type CatBoostOOFHeader struct {
	FormatVersion string
	AtUnit        string
	Market        MarketKey
	FeatureIDs    []FeatureID
	MatrixDigest  Digest
	SpecDigest    Digest
	FitPlanDigest Digest
	ClassOrder    [3]TargetOutcome
	Folds         []CatBoostOOFFold
}

type CatBoostOOFFold struct {
	SourceTrainBegin int
	SourceTrainEnd   int
	SourceValBegin   int
	SourceValEnd     int
	OutputBegin      int
	OutputEnd        int
	SelectedN        int
	PortableDigest   Digest
}

type CatBoostOOFRow struct {
	At      int64
	Outcome TargetOutcome
	Logits  [3]float64
}

type CatBoostOOFFooter struct {
	RowCount      int
	FirstAt       int64
	LastAt        int64
	ContentDigest Digest
}

type catBoostLogitsHeaderJSON struct {
	Kind          string                   `json:"kind"`
	FormatVersion string                   `json:"format_version"`
	AtUnit        string                   `json:"at_unit"`
	Market        tapeMarketJSON           `json:"market"`
	FeatureIDs    []FeatureID              `json:"feature_ids"`
	MatrixDigest  string                   `json:"source_oof_matrix_content_digest"`
	SpecDigest    string                   `json:"catboost_spec_digest"`
	FitPlanDigest string                   `json:"catboost_fitplan_digest"`
	ClassOrder    []TargetOutcome          `json:"class_order"`
	Folds         []catBoostLogitsFoldJSON `json:"folds"`
}

type catBoostLogitsFoldJSON struct {
	SourceTrainBegin int    `json:"source_train_begin"`
	SourceTrainEnd   int    `json:"source_train_end"`
	SourceValBegin   int    `json:"source_validation_begin"`
	SourceValEnd     int    `json:"source_validation_end"`
	OutputBegin      int    `json:"output_begin"`
	OutputEnd        int    `json:"output_end"`
	SelectedN        int    `json:"selected_tree_count"`
	PortableDigest   string `json:"portable_model_content_digest"`
}

type catBoostLogitsRowJSON struct {
	Kind    string    `json:"kind"`
	At      int64     `json:"at"`
	Outcome string    `json:"outcome"`
	Logits  []float64 `json:"logits"`
}

type catBoostLogitsFooterJSON struct {
	Kind          string `json:"kind"`
	RowCount      int    `json:"row_count"`
	FirstAt       int64  `json:"first_at"`
	LastAt        int64  `json:"last_at"`
	ContentDigest string `json:"content_digest"`
}

func hashCatBoostOOFLogits(hdr CatBoostOOFHeader, rows []CatBoostOOFRow) Digest {
	h := sha256.New()
	hashPutString(h, "CL1C")
	hashCatBoostOOFHeader(h, hdr)
	for i := range rows {
		hashPutI64(h, rows[i].At)
		hashPutString(h, string(rows[i].Outcome))
		hashPutU32(h, 3)
		for c := 0; c < 3; c++ {
			hashPutF64(h, rows[i].Logits[c])
		}
	}
	hashPutU32(h, uint32(len(rows)))
	if len(rows) > 0 {
		hashPutI64(h, rows[0].At)
		hashPutI64(h, rows[len(rows)-1].At)
	}
	var d Digest
	copy(d[:], h.Sum(nil))
	return d
}

func hashCatBoostOOFHeader(h hash.Hash, hdr CatBoostOOFHeader) {
	hashPutString(h, hdr.FormatVersion)
	hashPutString(h, hdr.AtUnit)
	hashPutMarket(h, hdr.Market)
	hashPutU32(h, uint32(len(hdr.FeatureIDs)))
	for _, id := range hdr.FeatureIDs {
		hashPutString(h, string(id))
	}
	hashPutDigest(h, hdr.MatrixDigest)
	hashPutDigest(h, hdr.SpecDigest)
	hashPutDigest(h, hdr.FitPlanDigest)
	hashPutU32(h, 3)
	for i := 0; i < 3; i++ {
		hashPutString(h, string(hdr.ClassOrder[i]))
	}
	hashPutU32(h, uint32(len(hdr.Folds)))
	for _, f := range hdr.Folds {
		hashPutU32(h, uint32(f.SourceTrainBegin))
		hashPutU32(h, uint32(f.SourceTrainEnd))
		hashPutU32(h, uint32(f.SourceValBegin))
		hashPutU32(h, uint32(f.SourceValEnd))
		hashPutU32(h, uint32(f.OutputBegin))
		hashPutU32(h, uint32(f.OutputEnd))
		hashPutU32(h, uint32(f.SelectedN))
		hashPutDigest(h, f.PortableDigest)
	}
}

func WriteCatBoostOOFLogits(path string, hdr CatBoostOOFHeader, rows []CatBoostOOFRow) (CatBoostOOFFooter, error) {
	var z CatBoostOOFFooter
	if hdr.FormatVersion != CatBoostOOFLogitsFormatV1 || hdr.AtUnit != OOFAtUnitUnixMs {
		return z, fmt.Errorf("forecast: catboost oof-logits format")
	}
	if hdr.ClassOrder != OOFClassOrder {
		return z, fmt.Errorf("forecast: CLASS_ORDER_MISMATCH")
	}
	want := hashCatBoostOOFLogits(hdr, rows)
	if _, err := os.Stat(path); err == nil {
		gotH, gotRows, gotF, err := ReadCatBoostOOFLogits(path)
		if err != nil {
			return z, fmt.Errorf("forecast: refuse existing catboost oof-logits %s: %w", path, err)
		}
		if gotF.ContentDigest == want && len(gotRows) == len(rows) && gotH.MatrixDigest == hdr.MatrixDigest && gotH.SpecDigest == hdr.SpecDigest && gotH.FitPlanDigest == hdr.FitPlanDigest {
			return gotF, errMatchExisting
		}
		return z, fmt.Errorf("forecast: refuse overwrite of different catboost oof-logits %s", path)
	} else if !os.IsNotExist(err) {
		return z, err
	}
	if err := tapeDirOk(path); err != nil {
		return z, err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return z, err
	}
	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	abort := func() { _ = f.Close(); _ = os.Remove(tmp) }
	hj := catBoostLogitsHeaderJSON{
		Kind: catBoostLogitsKindHeader, FormatVersion: hdr.FormatVersion, AtUnit: hdr.AtUnit,
		Market:     tapeMarketJSON{Venue: hdr.Market.Venue, Instrument: hdr.Market.Instrument, Contract: hdr.Market.Contract, Timeframe: hdr.Market.Timeframe},
		FeatureIDs: hdr.FeatureIDs, MatrixDigest: hdr.MatrixDigest.String(), SpecDigest: hdr.SpecDigest.String(),
		FitPlanDigest: hdr.FitPlanDigest.String(), ClassOrder: hdr.ClassOrder[:],
	}
	for _, fold := range hdr.Folds {
		hj.Folds = append(hj.Folds, catBoostLogitsFoldJSON{
			SourceTrainBegin: fold.SourceTrainBegin, SourceTrainEnd: fold.SourceTrainEnd,
			SourceValBegin: fold.SourceValBegin, SourceValEnd: fold.SourceValEnd,
			OutputBegin: fold.OutputBegin, OutputEnd: fold.OutputEnd, SelectedN: fold.SelectedN,
			PortableDigest: fold.PortableDigest.String(),
		})
	}
	if err := enc.Encode(hj); err != nil {
		abort()
		return z, err
	}
	for i := range rows {
		if math.IsNaN(rows[i].Logits[0]) || math.IsInf(rows[i].Logits[0], 0) || math.IsNaN(rows[i].Logits[1]) || math.IsInf(rows[i].Logits[1], 0) || math.IsNaN(rows[i].Logits[2]) || math.IsInf(rows[i].Logits[2], 0) {
			abort()
			return z, fmt.Errorf("forecast: NONFINITE_LOGITS")
		}
		if err := enc.Encode(catBoostLogitsRowJSON{
			Kind: catBoostLogitsKindRow, At: rows[i].At, Outcome: string(rows[i].Outcome),
			Logits: []float64{rows[i].Logits[0], rows[i].Logits[1], rows[i].Logits[2]},
		}); err != nil {
			abort()
			return z, err
		}
	}
	ft := CatBoostOOFFooter{RowCount: len(rows), ContentDigest: want}
	if len(rows) > 0 {
		ft.FirstAt = rows[0].At
		ft.LastAt = rows[len(rows)-1].At
	}
	if err := enc.Encode(catBoostLogitsFooterJSON{
		Kind: catBoostLogitsKindFooter, RowCount: ft.RowCount, FirstAt: ft.FirstAt, LastAt: ft.LastAt, ContentDigest: want.String(),
	}); err != nil {
		abort()
		return z, err
	}
	if err := bw.Flush(); err != nil {
		abort()
		return z, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return z, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return z, err
	}
	return ft, nil
}

var errMatchExisting = fmt.Errorf("forecast: catboost oof-logits MATCH")

func ReadCatBoostOOFLogits(path string) (CatBoostOOFHeader, []CatBoostOOFRow, CatBoostOOFFooter, error) {
	f, err := os.Open(path)
	if err != nil {
		return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	line, err := readJSONLLine(br)
	if err != nil {
		return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
	}
	var hj catBoostLogitsHeaderJSON
	if err := json.Unmarshal(line, &hj); err != nil {
		return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
	}
	if hj.Kind != catBoostLogitsKindHeader || hj.FormatVersion != CatBoostOOFLogitsFormatV1 {
		return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, fmt.Errorf("forecast: catboost oof-logits header")
	}
	hdr, err := catBoostLogitsHeaderFromJSON(hj)
	if err != nil {
		return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
	}
	var rows []CatBoostOOFRow
	var footer CatBoostOOFFooter
	sawFooter := false
	for {
		line, err = readJSONLLine(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
		}
		var kind struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(line, &kind); err != nil {
			return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
		}
		switch kind.Kind {
		case catBoostLogitsKindRow:
			var rj catBoostLogitsRowJSON
			if err := json.Unmarshal(line, &rj); err != nil {
				return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
			}
			if len(rj.Logits) != 3 {
				return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, fmt.Errorf("forecast: logits width")
			}
			row := CatBoostOOFRow{At: rj.At, Outcome: TargetOutcome(rj.Outcome), Logits: [3]float64{rj.Logits[0], rj.Logits[1], rj.Logits[2]}}
			rows = append(rows, row)
		case catBoostLogitsKindFooter:
			var fj catBoostLogitsFooterJSON
			if err := json.Unmarshal(line, &fj); err != nil {
				return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
			}
			cd, err := ParseDigestHex(fj.ContentDigest)
			if err != nil {
				return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, err
			}
			footer = CatBoostOOFFooter{RowCount: fj.RowCount, FirstAt: fj.FirstAt, LastAt: fj.LastAt, ContentDigest: cd}
			sawFooter = true
		default:
			return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, fmt.Errorf("forecast: unknown logits kind")
		}
	}
	if !sawFooter || footer.RowCount != len(rows) {
		return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, fmt.Errorf("forecast: catboost oof-logits incomplete")
	}
	if hashCatBoostOOFLogits(hdr, rows) != footer.ContentDigest {
		return CatBoostOOFHeader{}, nil, CatBoostOOFFooter{}, fmt.Errorf("forecast: catboost oof-logits ContentDigest mismatch")
	}
	return hdr, rows, footer, nil
}

func catBoostLogitsHeaderFromJSON(hj catBoostLogitsHeaderJSON) (CatBoostOOFHeader, error) {
	var z CatBoostOOFHeader
	md, err := ParseDigestHex(hj.MatrixDigest)
	if err != nil {
		return z, err
	}
	sd, err := ParseDigestHex(hj.SpecDigest)
	if err != nil {
		return z, err
	}
	pd, err := ParseDigestHex(hj.FitPlanDigest)
	if err != nil {
		return z, err
	}
	if len(hj.ClassOrder) != 3 {
		return z, fmt.Errorf("forecast: CLASS_ORDER_MISMATCH")
	}
	var order [3]TargetOutcome
	copy(order[:], hj.ClassOrder)
	if order != OOFClassOrder {
		return z, fmt.Errorf("forecast: CLASS_ORDER_MISMATCH")
	}
	hdr := CatBoostOOFHeader{
		FormatVersion: hj.FormatVersion, AtUnit: hj.AtUnit,
		Market:     MarketKey{Venue: hj.Market.Venue, Instrument: hj.Market.Instrument, Contract: hj.Market.Contract, Timeframe: hj.Market.Timeframe},
		FeatureIDs: hj.FeatureIDs, MatrixDigest: md, SpecDigest: sd, FitPlanDigest: pd, ClassOrder: order,
	}
	for _, f := range hj.Folds {
		pcd, err := ParseDigestHex(f.PortableDigest)
		if err != nil {
			return z, err
		}
		hdr.Folds = append(hdr.Folds, CatBoostOOFFold{
			SourceTrainBegin: f.SourceTrainBegin, SourceTrainEnd: f.SourceTrainEnd,
			SourceValBegin: f.SourceValBegin, SourceValEnd: f.SourceValEnd,
			OutputBegin: f.OutputBegin, OutputEnd: f.OutputEnd, SelectedN: f.SelectedN, PortableDigest: pcd,
		})
	}
	return hdr, nil
}
