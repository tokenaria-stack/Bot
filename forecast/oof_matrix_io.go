package forecast

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type oofHeaderJSON struct {
	Kind                              string         `json:"kind"`
	FormatVersion                     string         `json:"format_version"`
	AtUnit                            string         `json:"at_unit"`
	Market                            tapeMarketJSON `json:"market"`
	FeatureIDs                        []FeatureID    `json:"feature_ids"`
	FeaturePlanDigest                 string         `json:"feature_plan_digest"`
	FeatureTapeSourceRangeDigest      string         `json:"feature_tape_source_range_digest"`
	FeatureTapeContentDigest          string         `json:"feature_tape_content_digest"`
	LabelSetContentDigest             string         `json:"label_set_content_digest"`
	TargetDigest                      string         `json:"target_digest"`
	LabelSourceRangeDigest            string         `json:"label_source_range_digest"`
	FinerSourceDigest                 string         `json:"finer_source_digest"`
	FinerWindowCount                  int            `json:"finer_window_count"`
	LabelFeatureTapePlanDigest        string         `json:"label_feature_tape_plan_digest"`
	LabelFeatureTapeSourceRangeDigest string         `json:"label_feature_tape_source_range_digest"`
	LabelFeatureTapeContentDigest     string         `json:"label_feature_tape_content_digest"`
	LabelLogicVersion                 string         `json:"label_logic_version"`
	ValidationPlanDigest              string         `json:"validation_plan_digest"`
	ValidationLogicVersion            string         `json:"validation_logic_version"`
	ValidationPlan                    oofValPlanJSON `json:"validation_plan"`
	DevelopmentExclusiveEndAt         int64          `json:"development_exclusive_end_at"`
	DevelopmentRowCount               int            `json:"development_row_count"`
	HoldoutStartAt                    int64          `json:"holdout_start_at"`
	Folds                             []oofFoldJSON  `json:"folds"`
}

type oofValPlanJSON struct {
	Logic              string `json:"logic"`
	Timeframe          string `json:"timeframe"`
	HoldoutStartAt     int64  `json:"holdout_start_at"`
	ValidationSpanBars int    `json:"validation_span_bars"`
	FoldCount          int    `json:"fold_count"`
	TargetH            int    `json:"target_h"`
	ExtraGapBars       int    `json:"extra_gap_bars"`
	MinTrainRows       int    `json:"min_train_rows"`
}

type oofFoldJSON struct {
	TrainBegin                int   `json:"train_begin"`
	TrainEnd                  int   `json:"train_end"`
	ValidationBegin           int   `json:"validation_begin"`
	ValidationEnd             int   `json:"validation_end"`
	TrainFirstAt              int64 `json:"train_first_at"`
	TrainLastAt               int64 `json:"train_last_at"`
	ValidationBoundaryStartAt int64 `json:"validation_boundary_start_at"`
	ValidationBoundaryEndAt   int64 `json:"validation_boundary_end_at"`
	ValidationFirstActualAt   int64 `json:"validation_first_actual_at"`
	ValidationLastActualAt    int64 `json:"validation_last_actual_at"`
}

type oofRowJSON struct {
	Kind     string    `json:"kind"`
	At       int64     `json:"at"`
	Features []float64 `json:"features"`
	Outcome  string    `json:"outcome"`
}

type oofFooterJSON struct {
	Kind          string `json:"kind"`
	RowCount      int    `json:"row_count"`
	FirstAt       int64  `json:"first_at"`
	LastAt        int64  `json:"last_at"`
	ContentDigest string `json:"content_digest"`
}

func writeOOFMatrix(finalPath string, hdr OOFHeader, rows []OOFRow, want Digest) (OOFFooter, error) {
	var z OOFFooter
	if err := validateOOFHeader(hdr); err != nil {
		return z, err
	}
	if err := tapeDirOk(finalPath); err != nil {
		return z, err
	}
	if _, err := os.Stat(finalPath); err == nil {
		return z, fmt.Errorf("forecast: refuse overwrite of existing oof-matrix %s", finalPath)
	} else if !os.IsNotExist(err) {
		return z, err
	}
	tmpPath := finalPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return z, err
	}
	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	abort := func() {
		_ = f.Close()
		_ = os.Remove(tmpPath)
	}
	if err := enc.Encode(headerToJSON(hdr)); err != nil {
		abort()
		return z, err
	}
	for i := range rows {
		if err := enc.Encode(oofRowJSON{
			Kind:     oofKindRow,
			At:       rows[i].At,
			Features: append([]float64(nil), rows[i].Features...),
			Outcome:  string(rows[i].Outcome),
		}); err != nil {
			abort()
			return z, err
		}
	}
	ft := OOFFooter{RowCount: len(rows), ContentDigest: want}
	if len(rows) > 0 {
		ft.FirstAt = rows[0].At
		ft.LastAt = rows[len(rows)-1].At
	}
	if err := enc.Encode(oofFooterJSON{
		Kind:          oofKindFooter,
		RowCount:      ft.RowCount,
		FirstAt:       ft.FirstAt,
		LastAt:        ft.LastAt,
		ContentDigest: want.String(),
	}); err != nil {
		abort()
		return z, err
	}
	if err := bw.Flush(); err != nil {
		abort()
		return z, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return z, err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return z, err
	}
	return ft, nil
}

func headerToJSON(hdr OOFHeader) oofHeaderJSON {
	folds := make([]oofFoldJSON, len(hdr.Folds))
	for i, f := range hdr.Folds {
		folds[i] = oofFoldJSON{
			TrainBegin:                f.TrainBegin,
			TrainEnd:                  f.TrainEnd,
			ValidationBegin:           f.ValBegin,
			ValidationEnd:             f.ValEnd,
			TrainFirstAt:              f.TrainFirstAt,
			TrainLastAt:               f.TrainLastAt,
			ValidationBoundaryStartAt: f.ValBoundaryStartAt,
			ValidationBoundaryEndAt:   f.ValBoundaryEndAt,
			ValidationFirstActualAt:   f.ValFirstAt,
			ValidationLastActualAt:    f.ValLastAt,
		}
	}
	return oofHeaderJSON{
		Kind:                              oofKindHeader,
		FormatVersion:                     hdr.FormatVersion,
		AtUnit:                            hdr.AtUnit,
		Market:                            marketJSON(hdr.Market),
		FeatureIDs:                        append([]FeatureID(nil), hdr.FeatureIDs...),
		FeaturePlanDigest:                 hdr.PlanDigest.String(),
		FeatureTapeSourceRangeDigest:      hdr.TapeSource.String(),
		FeatureTapeContentDigest:          hdr.TapeContent.String(),
		LabelSetContentDigest:             hdr.LabelContent.String(),
		TargetDigest:                      hdr.TargetDigest.String(),
		LabelSourceRangeDigest:            hdr.LabelSource.String(),
		FinerSourceDigest:                 hdr.FinerSource.String(),
		FinerWindowCount:                  hdr.FinerWindows,
		LabelFeatureTapePlanDigest:        hdr.LabelTapePlan.String(),
		LabelFeatureTapeSourceRangeDigest: hdr.LabelTapeSrc.String(),
		LabelFeatureTapeContentDigest:     hdr.LabelTapeCnt.String(),
		LabelLogicVersion:                 string(hdr.LabelLogic),
		ValidationPlanDigest:              hdr.ValPlan.Digest.String(),
		ValidationLogicVersion:            string(hdr.ValPlanRules.Logic),
		ValidationPlan: oofValPlanJSON{
			Logic:              string(hdr.ValPlanRules.Logic),
			Timeframe:          hdr.ValPlanRules.Timeframe,
			HoldoutStartAt:     hdr.ValPlanRules.HoldoutStartAt,
			ValidationSpanBars: hdr.ValPlanRules.ValidationSpanBars,
			FoldCount:          hdr.ValPlanRules.FoldCount,
			TargetH:            hdr.ValPlanRules.TargetH,
			ExtraGapBars:       hdr.ValPlanRules.ExtraGapBars,
			MinTrainRows:       hdr.ValPlanRules.MinTrainRows,
		},
		DevelopmentExclusiveEndAt: hdr.DevExclusive,
		DevelopmentRowCount:       hdr.DevRowCount,
		HoldoutStartAt:            hdr.HoldoutStart,
		Folds:                     folds,
	}
}

func validateOOFHeader(hdr OOFHeader) error {
	if hdr.FormatVersion != OOFMatrixFormatV1 {
		return fmt.Errorf("forecast: unknown oof-matrix format %q", hdr.FormatVersion)
	}
	if hdr.AtUnit != OOFAtUnitUnixMs {
		return fmt.Errorf("forecast: oof-matrix AtUnit must be %q", OOFAtUnitUnixMs)
	}
	if err := hdr.Market.Validate(); err != nil {
		return err
	}
	if len(hdr.FeatureIDs) == 0 {
		return fmt.Errorf("forecast: oof-matrix FeatureIDs required")
	}
	if hdr.DevRowCount <= 0 {
		return fmt.Errorf("forecast: oof-matrix DevelopmentRowCount must be > 0")
	}
	id, err := hdr.ValPlanRules.Identity()
	if err != nil {
		return err
	}
	if id.Digest != hdr.ValPlan.Digest {
		return fmt.Errorf("forecast: oof-matrix ValidationPlanDigest does not match rules")
	}
	return nil
}

// ReadOOFMatrix decodes one oof-matrix-v1 file and verifies ContentDigest.
func ReadOOFMatrix(path string) (OOFHeader, []OOFRow, OOFFooter, error) {
	f, err := os.Open(path)
	if err != nil {
		return OOFHeader{}, nil, OOFFooter{}, err
	}
	defer f.Close()
	return decodeOOFMatrix(f)
}

func decodeOOFMatrix(r io.Reader) (OOFHeader, []OOFRow, OOFFooter, error) {
	br := bufio.NewReader(r)
	line, err := readJSONLLine(br)
	if err != nil {
		return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix missing header: %w", err)
	}
	var kind struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(line, &kind); err != nil {
		return OOFHeader{}, nil, OOFFooter{}, err
	}
	if kind.Kind != oofKindHeader {
		return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix first record must be header, got %q", kind.Kind)
	}
	var hj oofHeaderJSON
	if err := json.Unmarshal(line, &hj); err != nil {
		return OOFHeader{}, nil, OOFFooter{}, err
	}
	hdr, err := headerFromJSON(hj)
	if err != nil {
		return OOFHeader{}, nil, OOFFooter{}, err
	}
	if err := validateOOFHeader(hdr); err != nil {
		return OOFHeader{}, nil, OOFFooter{}, err
	}
	var rows []OOFRow
	var lastAt int64
	sawFooter := false
	var footer OOFFooter
	width := len(hdr.FeatureIDs)
	for {
		line, err = readJSONLLine(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return OOFHeader{}, nil, OOFFooter{}, err
		}
		if err := json.Unmarshal(line, &kind); err != nil {
			return OOFHeader{}, nil, OOFFooter{}, err
		}
		switch kind.Kind {
		case oofKindHeader:
			return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix duplicate header")
		case oofKindRow:
			if sawFooter {
				return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix row after footer")
			}
			var rj oofRowJSON
			if err := json.Unmarshal(line, &rj); err != nil {
				return OOFHeader{}, nil, OOFFooter{}, err
			}
			row := OOFRow{At: rj.At, Features: append([]float64(nil), rj.Features...), Outcome: TargetOutcome(rj.Outcome)}
			if len(rows) > 0 && row.At <= lastAt {
				return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix At must be strictly increasing")
			}
			rr := ResearchRow{At: row.At, Features: row.Features, Outcome: row.Outcome}
			if err := validateOOFExportRow(width, rr); err != nil {
				return OOFHeader{}, nil, OOFFooter{}, err
			}
			if row.At >= hdr.DevExclusive {
				return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix row At %d is not < DevelopmentExclusiveEndAt", row.At)
			}
			rows = append(rows, row)
			lastAt = row.At
		case oofKindFooter:
			if sawFooter {
				return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix duplicate footer")
			}
			var fj oofFooterJSON
			if err := json.Unmarshal(line, &fj); err != nil {
				return OOFHeader{}, nil, OOFFooter{}, err
			}
			cd, err := ParseDigestHex(fj.ContentDigest)
			if err != nil {
				return OOFHeader{}, nil, OOFFooter{}, err
			}
			footer = OOFFooter{RowCount: fj.RowCount, FirstAt: fj.FirstAt, LastAt: fj.LastAt, ContentDigest: cd}
			sawFooter = true
		default:
			return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix unknown kind %q", kind.Kind)
		}
	}
	if !sawFooter {
		return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix missing footer")
	}
	if footer.RowCount != len(rows) || hdr.DevRowCount != len(rows) {
		return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix row count mismatch")
	}
	if len(rows) > 0 && (footer.FirstAt != rows[0].At || footer.LastAt != rows[len(rows)-1].At) {
		return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix first/last At mismatch")
	}
	if err := validateOOFFolds(CompiledValidationPlan{
		DevelopmentEndIndex: hdr.DevRowCount,
		Folds:               hdr.Folds,
	}, hdr.DevRowCount); err != nil {
		return OOFHeader{}, nil, OOFFooter{}, err
	}
	got := hashOOFMatrix(hdr, rows)
	if got != footer.ContentDigest {
		return OOFHeader{}, nil, OOFFooter{}, fmt.Errorf("forecast: oof-matrix ContentDigest mismatch")
	}
	return hdr, rows, footer, nil
}

func headerFromJSON(hj oofHeaderJSON) (OOFHeader, error) {
	var z OOFHeader
	parse := func(s string) (Digest, error) {
		if s == "" {
			return Digest{}, nil
		}
		return ParseDigestHex(s)
	}
	plan, err := ParseDigestHex(hj.FeaturePlanDigest)
	if err != nil {
		return z, err
	}
	tapeSrc, err := ParseDigestHex(hj.FeatureTapeSourceRangeDigest)
	if err != nil {
		return z, err
	}
	tapeCnt, err := ParseDigestHex(hj.FeatureTapeContentDigest)
	if err != nil {
		return z, err
	}
	labCnt, err := ParseDigestHex(hj.LabelSetContentDigest)
	if err != nil {
		return z, err
	}
	target, err := ParseDigestHex(hj.TargetDigest)
	if err != nil {
		return z, err
	}
	labSrc, err := parse(hj.LabelSourceRangeDigest)
	if err != nil {
		return z, err
	}
	finer, err := parse(hj.FinerSourceDigest)
	if err != nil {
		return z, err
	}
	ltp, err := ParseDigestHex(hj.LabelFeatureTapePlanDigest)
	if err != nil {
		return z, err
	}
	lts, err := ParseDigestHex(hj.LabelFeatureTapeSourceRangeDigest)
	if err != nil {
		return z, err
	}
	ltc, err := ParseDigestHex(hj.LabelFeatureTapeContentDigest)
	if err != nil {
		return z, err
	}
	vpd, err := ParseDigestHex(hj.ValidationPlanDigest)
	if err != nil {
		return z, err
	}
	rules := ValidationPlan{
		Logic:              LogicVersion(hj.ValidationPlan.Logic),
		Timeframe:          hj.ValidationPlan.Timeframe,
		HoldoutStartAt:     hj.ValidationPlan.HoldoutStartAt,
		ValidationSpanBars: hj.ValidationPlan.ValidationSpanBars,
		FoldCount:          hj.ValidationPlan.FoldCount,
		TargetH:            hj.ValidationPlan.TargetH,
		ExtraGapBars:       hj.ValidationPlan.ExtraGapBars,
		MinTrainRows:       hj.ValidationPlan.MinTrainRows,
	}
	id, err := rules.Identity()
	if err != nil {
		return z, err
	}
	if id.Digest != vpd {
		return z, fmt.Errorf("forecast: oof-matrix ValidationPlanDigest does not match encoded rules")
	}
	folds := make([]CompiledFold, len(hj.Folds))
	for i, f := range hj.Folds {
		folds[i] = CompiledFold{
			TrainBegin:         f.TrainBegin,
			TrainEnd:           f.TrainEnd,
			ValBegin:           f.ValidationBegin,
			ValEnd:             f.ValidationEnd,
			ValBoundaryStartAt: f.ValidationBoundaryStartAt,
			ValBoundaryEndAt:   f.ValidationBoundaryEndAt,
			TrainFirstAt:       f.TrainFirstAt,
			TrainLastAt:        f.TrainLastAt,
			ValFirstAt:         f.ValidationFirstActualAt,
			ValLastAt:          f.ValidationLastActualAt,
		}
	}
	return OOFHeader{
		FormatVersion: hj.FormatVersion,
		AtUnit:        hj.AtUnit,
		Market:        marketFromJSON(hj.Market),
		FeatureIDs:    append([]FeatureID(nil), hj.FeatureIDs...),
		PlanDigest:    plan,
		TapeSource:    tapeSrc,
		TapeContent:   tapeCnt,
		LabelContent:  labCnt,
		TargetDigest:  target,
		LabelSource:   labSrc,
		FinerSource:   finer,
		FinerWindows:  hj.FinerWindowCount,
		LabelTapePlan: ltp,
		LabelTapeSrc:  lts,
		LabelTapeCnt:  ltc,
		LabelLogic:    LogicVersion(hj.LabelLogicVersion),
		ValPlan:       id,
		ValPlanRules:  rules,
		DevExclusive:  hj.DevelopmentExclusiveEndAt,
		DevRowCount:   hj.DevelopmentRowCount,
		HoldoutStart:  hj.HoldoutStartAt,
		Folds:         folds,
	}, nil
}
