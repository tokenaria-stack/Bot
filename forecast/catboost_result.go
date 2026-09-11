package forecast

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const CatBoostResultFormatV1 = "catboost-result-v1"

// CatBoostResultManifest binds one CatBoostSpec1 run against one certified matrix.
type CatBoostResultManifest struct {
	Format        string
	MatrixDigest  Digest
	SpecDigest    Digest
	FitPlanDigest Digest
	LogitsDigest  Digest
	ClassOrder    [3]TargetOutcome
	Folds         []CatBoostResultFold
}

type CatBoostResultFold struct {
	SelectedN      int
	PortableDigest Digest
	InnerGoLogLoss float64
}

func (m CatBoostResultManifest) Digest() Digest {
	h := sha256.New()
	hashPutString(h, "CB1R")
	hashPutString(h, m.Format)
	hashPutDigest(h, m.MatrixDigest)
	hashPutDigest(h, m.SpecDigest)
	hashPutDigest(h, m.FitPlanDigest)
	hashPutDigest(h, m.LogitsDigest)
	hashPutU32(h, 3)
	for i := 0; i < 3; i++ {
		hashPutString(h, string(m.ClassOrder[i]))
	}
	hashPutU32(h, uint32(len(m.Folds)))
	for _, f := range m.Folds {
		hashPutU32(h, uint32(f.SelectedN))
		hashPutDigest(h, f.PortableDigest)
		hashPutF64(h, f.InnerGoLogLoss)
	}
	var d Digest
	copy(d[:], h.Sum(nil))
	return d
}

type catBoostResultJSON struct {
	Format        string                   `json:"format_version"`
	MatrixDigest  string                   `json:"source_oof_matrix_content_digest"`
	SpecDigest    string                   `json:"catboost_spec_digest"`
	FitPlanDigest string                   `json:"catboost_fitplan_digest"`
	LogitsDigest  string                   `json:"oof_logits_content_digest"`
	ClassOrder    []TargetOutcome          `json:"class_order"`
	Folds         []catBoostResultFoldJSON `json:"folds"`
	ContentDigest string                   `json:"content_digest"`
}

type catBoostResultFoldJSON struct {
	SelectedN      int     `json:"selected_tree_count"`
	PortableDigest string  `json:"portable_model_content_digest"`
	InnerGoLogLoss float64 `json:"inner_go_logloss"`
}

func catBoostOfficialBase(logitsPath string) string {
	return strings.TrimSuffix(logitsPath, filepath.Ext(logitsPath))
}

func CatBoostManifestPath(logitsPath string) string {
	return catBoostOfficialBase(logitsPath) + ".catboostresult.json"
}

func CatBoostPortablePath(logitsPath string, fold int) string {
	return fmt.Sprintf("%s.fold%d.pcb.json", catBoostOfficialBase(logitsPath), fold)
}

func WriteCatBoostResultManifest(path string, m CatBoostResultManifest) error {
	if m.Format != CatBoostResultFormatV1 {
		return fmt.Errorf("forecast: catboost result format")
	}
	if m.ClassOrder != OOFClassOrder {
		return fmt.Errorf("forecast: CLASS_ORDER_MISMATCH")
	}
	want := m.Digest()
	if _, err := os.Stat(path); err == nil {
		got, err := ReadCatBoostResultManifest(path)
		if err != nil {
			return fmt.Errorf("forecast: refuse existing catboost result %s: %w", path, err)
		}
		if got.Digest() == want && got.MatrixDigest == m.MatrixDigest && got.SpecDigest == m.SpecDigest && got.FitPlanDigest == m.FitPlanDigest && got.LogitsDigest == m.LogitsDigest {
			return errMatchExisting
		}
		return fmt.Errorf("forecast: refuse overwrite of different catboost result %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := tapeDirOk(path); err != nil {
		return err
	}
	body := catBoostResultJSON{
		Format: m.Format, MatrixDigest: m.MatrixDigest.String(), SpecDigest: m.SpecDigest.String(),
		FitPlanDigest: m.FitPlanDigest.String(), LogitsDigest: m.LogitsDigest.String(),
		ClassOrder: m.ClassOrder[:], ContentDigest: want.String(),
	}
	for _, f := range m.Folds {
		body.Folds = append(body.Folds, catBoostResultFoldJSON{
			SelectedN: f.SelectedN, PortableDigest: f.PortableDigest.String(), InnerGoLogLoss: f.InnerGoLogLoss,
		})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ReadCatBoostResultManifest(path string) (CatBoostResultManifest, error) {
	var z CatBoostResultManifest
	raw, err := os.ReadFile(path)
	if err != nil {
		return z, err
	}
	var body catBoostResultJSON
	if err := json.Unmarshal(raw, &body); err != nil {
		return z, err
	}
	if body.Format != CatBoostResultFormatV1 {
		return z, fmt.Errorf("forecast: catboost result format")
	}
	md, err := ParseDigestHex(body.MatrixDigest)
	if err != nil {
		return z, err
	}
	sd, err := ParseDigestHex(body.SpecDigest)
	if err != nil {
		return z, err
	}
	pd, err := ParseDigestHex(body.FitPlanDigest)
	if err != nil {
		return z, err
	}
	ld, err := ParseDigestHex(body.LogitsDigest)
	if err != nil {
		return z, err
	}
	if len(body.ClassOrder) != 3 {
		return z, fmt.Errorf("forecast: CLASS_ORDER_MISMATCH")
	}
	var order [3]TargetOutcome
	copy(order[:], body.ClassOrder)
	if order != OOFClassOrder {
		return z, fmt.Errorf("forecast: CLASS_ORDER_MISMATCH")
	}
	z = CatBoostResultManifest{
		Format: body.Format, MatrixDigest: md, SpecDigest: sd, FitPlanDigest: pd, LogitsDigest: ld, ClassOrder: order,
	}
	for _, f := range body.Folds {
		pcd, err := ParseDigestHex(f.PortableDigest)
		if err != nil {
			return CatBoostResultManifest{}, err
		}
		z.Folds = append(z.Folds, CatBoostResultFold{SelectedN: f.SelectedN, PortableDigest: pcd, InnerGoLogLoss: f.InnerGoLogLoss})
	}
	cd, err := ParseDigestHex(body.ContentDigest)
	if err != nil {
		return CatBoostResultManifest{}, err
	}
	if z.Digest() != cd {
		return CatBoostResultManifest{}, fmt.Errorf("forecast: catboost result ContentDigest mismatch")
	}
	return z, nil
}

type portableFileJSON struct {
	Format        string                 `json:"format"`
	FeatureIDs    []FeatureID            `json:"feature_ids"`
	ClassCount    int                    `json:"class_count"`
	Scale         float64                `json:"scale"`
	Bias          [3]float64             `json:"bias"`
	Trees         []portableTreeFileJSON `json:"trees"`
	ContentDigest string                 `json:"content_digest"`
}

type portableTreeFileJSON struct {
	Splits []portableSplitFileJSON `json:"splits"`
	Leaves []float64               `json:"leaves"`
}

type portableSplitFileJSON struct {
	Feature int     `json:"feature"`
	Border  float64 `json:"border"`
}

func WritePortableCatBoost(path string, m PortableCatBoost) error {
	if err := tapeDirOk(path); err != nil {
		return err
	}
	body := portableFileJSON{
		Format: m.Format, FeatureIDs: m.FeatureIDs, ClassCount: m.ClassCount, Scale: m.Scale, Bias: m.Bias,
		ContentDigest: m.ContentDigest().String(),
	}
	for _, tr := range m.Trees {
		tj := portableTreeFileJSON{Leaves: append([]float64(nil), tr.Leaves...)}
		for _, sp := range tr.Splits {
			tj.Splits = append(tj.Splits, portableSplitFileJSON{Feature: sp.Feature, Border: sp.Border})
		}
		body.Trees = append(body.Trees, tj)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ReadPortableCatBoost(path string) (PortableCatBoost, error) {
	var z PortableCatBoost
	raw, err := os.ReadFile(path)
	if err != nil {
		return z, err
	}
	var body portableFileJSON
	if err := json.Unmarshal(raw, &body); err != nil {
		return z, err
	}
	z = PortableCatBoost{Format: body.Format, FeatureIDs: body.FeatureIDs, ClassCount: body.ClassCount, Scale: body.Scale, Bias: body.Bias}
	for _, tr := range body.Trees {
		ot := PortableObliviousTree{Leaves: append([]float64(nil), tr.Leaves...)}
		for _, sp := range tr.Splits {
			ot.Splits = append(ot.Splits, PortableFloatSplit{Feature: sp.Feature, Border: sp.Border})
		}
		z.Trees = append(z.Trees, ot)
	}
	want, err := ParseDigestHex(body.ContentDigest)
	if err != nil {
		return PortableCatBoost{}, err
	}
	if z.ContentDigest() != want {
		return PortableCatBoost{}, fmt.Errorf("forecast: portable ContentDigest mismatch")
	}
	return z, nil
}
