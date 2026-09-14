package forecast

import (
	"fmt"

	"trading_bot/ml"
)

const PortableCatBoostV1 = ml.PortableCatBoostV1

// PortableCatBoost is the Brain V2 JSON/file shape. Evaluation is ml.Model.
type PortableCatBoost struct {
	Format     string
	FeatureIDs []FeatureID
	ClassCount int
	Scale      float64
	Bias       [3]float64
	Trees      []PortableObliviousTree
}

type PortableObliviousTree struct {
	Splits []PortableFloatSplit
	Leaves []float64
}

type PortableFloatSplit struct {
	Feature int
	Border  float64
}

func (m PortableCatBoost) toML() ml.Model {
	names := make([]string, len(m.FeatureIDs))
	for i, id := range m.FeatureIDs {
		names[i] = string(id)
	}
	trees := make([]ml.Tree, len(m.Trees))
	for i, tr := range m.Trees {
		sp := make([]ml.Split, len(tr.Splits))
		for j, s := range tr.Splits {
			sp[j] = ml.Split{Feature: s.Feature, Border: s.Border}
		}
		trees[i] = ml.Tree{Splits: sp, Leaves: append([]float64(nil), tr.Leaves...)}
	}
	return ml.Model{
		Format: m.Format, Names: names, ClassCount: m.ClassCount,
		Scale: m.Scale, Bias: m.Bias, Trees: trees,
	}
}

func portableFromML(m ml.Model) PortableCatBoost {
	ids := make([]FeatureID, len(m.Names))
	for i, n := range m.Names {
		ids[i] = FeatureID(n)
	}
	trees := make([]PortableObliviousTree, len(m.Trees))
	for i, tr := range m.Trees {
		sp := make([]PortableFloatSplit, len(tr.Splits))
		for j, s := range tr.Splits {
			sp[j] = PortableFloatSplit{Feature: s.Feature, Border: s.Border}
		}
		trees[i] = PortableObliviousTree{Splits: sp, Leaves: append([]float64(nil), tr.Leaves...)}
	}
	return PortableCatBoost{
		Format: m.Format, FeatureIDs: ids, ClassCount: m.ClassCount,
		Scale: m.Scale, Bias: m.Bias, Trees: trees,
	}
}

func ConvertVendorCatBoostJSON(raw []byte, featureIDs []FeatureID, classCount, maxTrees, maxDepth int) (PortableCatBoost, error) {
	names := make([]string, len(featureIDs))
	for i, id := range featureIDs {
		names[i] = string(id)
	}
	m, err := ml.ConvertVendor(raw, names, classCount, maxTrees, maxDepth)
	if err != nil {
		return PortableCatBoost{}, fmt.Errorf("forecast: %v", err)
	}
	return portableFromML(m), nil
}

func (m PortableCatBoost) Validate(expectIDs []FeatureID, classCount, maxTrees, maxDepth int) error {
	names := make([]string, len(expectIDs))
	for i, id := range expectIDs {
		names[i] = string(id)
	}
	if err := m.toML().Validate(names, classCount, maxTrees, maxDepth); err != nil {
		return fmt.Errorf("forecast: %v", err)
	}
	return nil
}

func (m PortableCatBoost) ContentDigest() Digest {
	return Digest(m.toML().ContentDigest())
}

func catboostFloatGt(x, border float64) bool {
	return ml.FloatGt(x, border)
}

func (m PortableCatBoost) Logits(x []float64) ([3]float64, error) {
	return m.toML().Logits(x)
}

func (m PortableCatBoost) PrefixLogits(x []float64, n int) ([3]float64, error) {
	return m.toML().PrefixLogits(x, n)
}

func MulticlassLogLoss(z [3]float64, y int) (float64, error) {
	v, err := ml.MulticlassLogLoss(z, y)
	if err != nil {
		return 0, fmt.Errorf("forecast: %v", err)
	}
	return v, nil
}

func PrefixMeanLogLoss(m PortableCatBoost, xs [][]float64, ys []int) ([]float64, error) {
	v, err := ml.PrefixMeanLogLoss(m.toML(), xs, ys)
	if err != nil {
		return nil, fmt.Errorf("forecast: %v", err)
	}
	return v, nil
}

func SelectTreeCount(loss []float64, maxIterations int) (int, error) {
	n, err := ml.SelectTreeCount(loss, maxIterations)
	if err != nil {
		return 0, fmt.Errorf("forecast: %v", err)
	}
	return n, nil
}
