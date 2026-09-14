package ml

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
)

const (
	PortableCatBoostV1 = "portable-catboost-v1"
	// Execution safety limits — not scientific feature-count assumptions.
	MaxWidthSafety = 4096
	MaxTreesSafety = 4096
	MaxDepthSafety = 16
	portableMaxCls = 3
)

// Model is the one Go-owned SymmetricTree numeric MultiClass evaluator.
type Model struct {
	Format     string
	Names      []string
	ClassCount int
	Scale      float64
	Bias       [3]float64
	Trees      []Tree
}

type Tree struct {
	Splits []Split
	Leaves []float64
}

type Split struct {
	Feature int
	Border  float64
}

type Digest [sha256.Size]byte

func (d Digest) Bytes() [sha256.Size]byte { return d }

type vendorCatBoostJSON struct {
	FeaturesInfo struct {
		FloatFeatures []struct {
			Borders           []float64 `json:"borders"`
			HasNans           bool      `json:"has_nans"`
			NanValueTreatment string    `json:"nan_value_treatment"`
		} `json:"float_features"`
		CategoricalFeatures json.RawMessage `json:"categorical_features"`
	} `json:"features_info"`
	ModelInfo struct {
		ClassParams struct {
			ClassNames   []json.RawMessage `json:"class_names"`
			ClassesCount int               `json:"classes_count"`
		} `json:"class_params"`
	} `json:"model_info"`
	ObliviousTrees []struct {
		Splits []struct {
			Border            float64 `json:"border"`
			FloatFeatureIndex int     `json:"float_feature_index"`
			SplitType         string  `json:"split_type"`
		} `json:"splits"`
		LeafValues []float64 `json:"leaf_values"`
	} `json:"oblivious_trees"`
	ScaleAndBias json.RawMessage `json:"scale_and_bias"`
}

// ConvertVendor converts CatBoost vendor JSON. inputWidth is len(featureNames).
func ConvertVendor(raw []byte, featureNames []string, classCount, maxTrees, maxDepth int) (Model, error) {
	var z Model
	inputWidth := len(featureNames)
	if inputWidth == 0 || inputWidth > MaxWidthSafety {
		return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL feature width")
	}
	if classCount != portableMaxCls {
		return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL class count")
	}
	if maxTrees <= 0 || maxTrees > MaxTreesSafety || maxDepth <= 0 || maxDepth > MaxDepthSafety {
		return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL bounds")
	}
	var doc vendorCatBoostJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL json: %w", err)
	}
	if len(doc.FeaturesInfo.CategoricalFeatures) > 0 && string(doc.FeaturesInfo.CategoricalFeatures) != "null" && string(doc.FeaturesInfo.CategoricalFeatures) != "[]" {
		return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL categorical features")
	}
	if len(doc.FeaturesInfo.FloatFeatures) != inputWidth {
		return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL float_features %d != %d", len(doc.FeaturesInfo.FloatFeatures), inputWidth)
	}
	for _, ff := range doc.FeaturesInfo.FloatFeatures {
		if ff.HasNans {
			return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL has_nans")
		}
		for _, b := range ff.Borders {
			if math.IsNaN(b) || math.IsInf(b, 0) {
				return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL nonfinite border table")
			}
		}
	}
	if doc.ModelInfo.ClassParams.ClassesCount != 0 && doc.ModelInfo.ClassParams.ClassesCount != classCount {
		return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL classes_count")
	}
	if len(doc.ObliviousTrees) > maxTrees {
		return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL tree count %d > %d", len(doc.ObliviousTrees), maxTrees)
	}
	scale, bias, err := parseScaleBias(doc.ScaleAndBias, classCount)
	if err != nil {
		return z, err
	}
	trees := make([]Tree, 0, len(doc.ObliviousTrees))
	for ti, tr := range doc.ObliviousTrees {
		d := len(tr.Splits)
		if d == 0 || d > maxDepth {
			return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL tree %d depth %d", ti, d)
		}
		nleaf := 1 << d
		if len(tr.LeafValues) != nleaf*classCount {
			return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL tree %d leaf size %d", ti, len(tr.LeafValues))
		}
		splits := make([]Split, d)
		for si, sp := range tr.Splits {
			if sp.SplitType != "FloatFeature" {
				return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL tree %d split type %q", ti, sp.SplitType)
			}
			if sp.FloatFeatureIndex < 0 || sp.FloatFeatureIndex >= inputWidth {
				return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL feature index")
			}
			if math.IsNaN(sp.Border) || math.IsInf(sp.Border, 0) {
				return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL nonfinite split border")
			}
			splits[si] = Split{Feature: sp.FloatFeatureIndex, Border: sp.Border}
		}
		leaves := append([]float64(nil), tr.LeafValues...)
		for _, v := range leaves {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return z, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL nonfinite leaf")
			}
		}
		trees = append(trees, Tree{Splits: splits, Leaves: leaves})
	}
	return Model{
		Format:     PortableCatBoostV1,
		Names:      append([]string(nil), featureNames...),
		ClassCount: classCount,
		Scale:      scale,
		Bias:       bias,
		Trees:      trees,
	}, nil
}

func parseScaleBias(raw json.RawMessage, classCount int) (float64, [3]float64, error) {
	var bias [3]float64
	if len(raw) == 0 || string(raw) == "null" {
		return 1, bias, nil
	}
	var pair []json.RawMessage
	if err := json.Unmarshal(raw, &pair); err != nil {
		return 0, bias, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL scale_and_bias")
	}
	if len(pair) != 2 {
		return 0, bias, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL scale_and_bias len")
	}
	var scale float64
	if err := json.Unmarshal(pair[0], &scale); err != nil {
		return 0, bias, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL scale")
	}
	if math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0, bias, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL nonfinite scale")
	}
	var b []float64
	if err := json.Unmarshal(pair[1], &b); err != nil {
		var s float64
		if err2 := json.Unmarshal(pair[1], &s); err2 != nil {
			return 0, bias, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL bias")
		}
		b = []float64{s, s, s}
	}
	if len(b) != classCount {
		return 0, bias, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL bias len")
	}
	for i := 0; i < classCount; i++ {
		if math.IsNaN(b[i]) || math.IsInf(b[i], 0) {
			return 0, bias, fmt.Errorf("ml: UNSUPPORTED_VENDOR_MODEL nonfinite bias")
		}
		bias[i] = b[i]
	}
	return scale, bias, nil
}

func (m Model) Validate(expectNames []string, classCount, maxTrees, maxDepth int) error {
	if m.Format != PortableCatBoostV1 {
		return fmt.Errorf("ml: portable format %q", m.Format)
	}
	if m.ClassCount != classCount || classCount != 3 {
		return fmt.Errorf("ml: CLASS_ORDER_MISMATCH portable classes")
	}
	if len(m.Names) != len(expectNames) {
		return fmt.Errorf("ml: portable feature width")
	}
	for i := range expectNames {
		if m.Names[i] != expectNames[i] {
			return fmt.Errorf("ml: portable feature order mismatch at %d", i)
		}
	}
	w := len(m.Names)
	if w == 0 || w > MaxWidthSafety {
		return fmt.Errorf("ml: portable feature width")
	}
	if len(m.Trees) > maxTrees {
		return fmt.Errorf("ml: portable tree count")
	}
	if math.IsNaN(m.Scale) || math.IsInf(m.Scale, 0) {
		return fmt.Errorf("ml: portable nonfinite scale")
	}
	for _, b := range m.Bias {
		if math.IsNaN(b) || math.IsInf(b, 0) {
			return fmt.Errorf("ml: portable nonfinite bias")
		}
	}
	for _, tr := range m.Trees {
		if len(tr.Splits) > maxDepth || len(tr.Splits) == 0 {
			return fmt.Errorf("ml: portable depth")
		}
		if len(tr.Leaves) != (1<<len(tr.Splits))*classCount {
			return fmt.Errorf("ml: portable leaf size")
		}
		for _, sp := range tr.Splits {
			if sp.Feature < 0 || sp.Feature >= w {
				return fmt.Errorf("ml: portable feature index")
			}
		}
	}
	return nil
}

func (m Model) ContentDigest() Digest {
	h := sha256.New()
	hashPutString(h, "PC1C")
	hashPutString(h, m.Format)
	hashPutU32(h, uint32(len(m.Names)))
	for _, id := range m.Names {
		hashPutString(h, id)
	}
	hashPutU32(h, uint32(m.ClassCount))
	hashPutF64(h, m.Scale)
	for i := 0; i < 3; i++ {
		hashPutF64(h, m.Bias[i])
	}
	hashPutU32(h, uint32(len(m.Trees)))
	for _, tr := range m.Trees {
		hashPutU32(h, uint32(len(tr.Splits)))
		for _, sp := range tr.Splits {
			hashPutU32(h, uint32(sp.Feature))
			hashPutF64(h, sp.Border)
		}
		hashPutU32(h, uint32(len(tr.Leaves)))
		for _, v := range tr.Leaves {
			hashPutF64(h, v)
		}
	}
	var d Digest
	copy(d[:], h.Sum(nil))
	return d
}

// FloatGt is the frozen CatBoost 1.2.7 numeric split: float32(x) > float32(border).
func FloatGt(x, border float64) bool {
	return float32(x) > float32(border)
}

func (m Model) logitsPrefix(x []float64, nTrees int) ([3]float64, error) {
	var z [3]float64
	if len(x) != len(m.Names) {
		return z, fmt.Errorf("ml: portable eval width")
	}
	if nTrees < 0 || nTrees > len(m.Trees) {
		return z, fmt.Errorf("ml: portable prefix")
	}
	for i := 0; i < 3; i++ {
		z[i] = m.Bias[i]
	}
	for ti := 0; ti < nTrees; ti++ {
		tr := m.Trees[ti]
		leaf := 0
		for i, sp := range tr.Splits {
			if FloatGt(x[sp.Feature], sp.Border) {
				leaf |= 1 << i
			}
		}
		off := leaf * m.ClassCount
		for c := 0; c < 3; c++ {
			z[c] += m.Scale * tr.Leaves[off+c]
		}
	}
	for c := 0; c < 3; c++ {
		if math.IsNaN(z[c]) || math.IsInf(z[c], 0) {
			return z, fmt.Errorf("ml: NONFINITE_LOGITS")
		}
	}
	return z, nil
}

func (m Model) Logits(x []float64) ([3]float64, error) {
	return m.logitsPrefix(x, len(m.Trees))
}

func (m Model) PrefixLogits(x []float64, n int) ([3]float64, error) {
	return m.logitsPrefix(x, n)
}

func logSumExp3(z [3]float64) float64 {
	m := z[0]
	if z[1] > m {
		m = z[1]
	}
	if z[2] > m {
		m = z[2]
	}
	s := math.Exp(z[0]-m) + math.Exp(z[1]-m) + math.Exp(z[2]-m)
	return m + math.Log(s)
}

func MulticlassLogLoss(z [3]float64, y int) (float64, error) {
	if y < 0 || y > 2 {
		return 0, fmt.Errorf("ml: CLASS_ORDER_MISMATCH y=%d", y)
	}
	lse := logSumExp3(z)
	v := lse - z[y]
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("ml: NONFINITE_LOGITS logloss")
	}
	return v, nil
}

// Softmax3 is a numerically stable 3-class softmax.
func Softmax3(z [3]float64) [3]float64 {
	lse := logSumExp3(z)
	return [3]float64{math.Exp(z[0] - lse), math.Exp(z[1] - lse), math.Exp(z[2] - lse)}
}

func PrefixMeanLogLoss(m Model, xs [][]float64, ys []int) ([]float64, error) {
	t := len(m.Trees)
	if t == 0 {
		return nil, fmt.Errorf("ml: empty portable forest")
	}
	if len(xs) == 0 || len(xs) != len(ys) {
		return nil, fmt.Errorf("ml: prefix loss population")
	}
	sum := make([]float64, t)
	for i, x := range xs {
		if len(x) != len(m.Names) {
			return nil, fmt.Errorf("ml: prefix width")
		}
		var z [3]float64
		z[0], z[1], z[2] = m.Bias[0], m.Bias[1], m.Bias[2]
		for ti := 0; ti < t; ti++ {
			tr := m.Trees[ti]
			leaf := 0
			for si, sp := range tr.Splits {
				if FloatGt(x[sp.Feature], sp.Border) {
					leaf |= 1 << si
				}
			}
			off := leaf * m.ClassCount
			for c := 0; c < 3; c++ {
				z[c] += m.Scale * tr.Leaves[off+c]
			}
			row, err := MulticlassLogLoss(z, ys[i])
			if err != nil {
				return nil, err
			}
			sum[ti] += row
		}
	}
	n := float64(len(xs))
	out := make([]float64, t)
	for i := range sum {
		out[i] = sum[i] / n
		if math.IsNaN(out[i]) || math.IsInf(out[i], 0) {
			return nil, fmt.Errorf("ml: NONFINITE_LOGITS mean loss")
		}
	}
	return out, nil
}

func SelectTreeCount(loss []float64, maxIterations int) (int, error) {
	if len(loss) == 0 || len(loss) > maxIterations {
		return 0, fmt.Errorf("ml: selection curve")
	}
	bestI := 0
	best := loss[0]
	for i := 1; i < len(loss); i++ {
		if loss[i] < best {
			best = loss[i]
			bestI = i
		}
	}
	n := bestI + 1
	if n == maxIterations && len(loss) == maxIterations {
		return 0, fmt.Errorf("ml: ITERATION_CAP_REACHED")
	}
	return n, nil
}
