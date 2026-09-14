package brain3

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"trading_bot/ml"
)

func TestLoadDataset_SchemaAndY(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal(map[string]any{
		"format": "setup-dataset-v1", "width": 2, "feature_names": []string{"a", "b"},
		"min_candidate_at": 10, "max_candidate_at": 20,
		"rows": []map[string]any{
			{"at": 10, "y": "TP_FIRST", "x": []float64{1, 2}},
			{"at": 20, "y": "TIMEOUT", "x": []float64{3, 4}},
		},
	})
	ds, err := LoadDataset(raw, SetupClassSchema())
	if err != nil {
		t.Fatal(err)
	}
	if ds.Width != 2 || ds.Y[0] != 0 || ds.Y[1] != 2 {
		t.Fatalf("%+v", ds)
	}
	if _, err := LoadDataset(raw, []string{"NOPE"}); err == nil {
		t.Fatal("unknown label must fail")
	}
}

func TestResolveOuterFolds_ExactAt(t *testing.T) {
	t.Parallel()
	ats := []int64{100, 200, 300, 400, 500}
	x := make([][]float64, len(ats))
	y := make([]int, len(ats))
	for i := range ats {
		x[i] = []float64{float64(i), 0}
	}
	ds := Dataset{
		Width: 2, FeatureNames: []string{"a", "b"}, ClassSchema: SetupClassSchema(),
		At: ats, X: x, Y: y, MinAt: 100, MaxAt: 500,
	}
	val := Validation{
		HoldoutStartAt: 1000, HoldoutBeginIndex: 5,
		Folds: []CompiledFold{
			{TrainBegin: 0, TrainEnd: 2, ValBegin: 2, ValEnd: 3, TrainFirstAt: 100, TrainLastAt: 200, ValFirstAt: 300, ValLastAt: 300},
			{TrainBegin: 0, TrainEnd: 3, ValBegin: 3, ValEnd: 5, TrainFirstAt: 100, TrainLastAt: 300, ValFirstAt: 400, ValLastAt: 500},
		},
	}
	c, err := ResolveOuterFolds(ds, val)
	if err != nil {
		t.Fatal(err)
	}
	if c.Folds[0].ValN != 1 || c.Folds[1].ValN != 2 || c.ValTotal != 3 || c.TrainOnlyPrefix != 2 {
		t.Fatalf("%+v", c)
	}
	val.Folds[0].ValFirstAt = 999
	if _, err := ResolveOuterFolds(ds, val); err == nil || !strings.Contains(err.Error(), "val At mismatch") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveOuterFolds_RefusesHoldout(t *testing.T) {
	t.Parallel()
	ds := Dataset{
		Width: 1, FeatureNames: []string{"a"}, ClassSchema: SetupClassSchema(),
		At: []int64{10, 50}, X: [][]float64{{1}, {1}}, Y: []int{0, 0}, MinAt: 10, MaxAt: 50,
	}
	val := Validation{HoldoutStartAt: 40, HoldoutBeginIndex: 2, Folds: []CompiledFold{
		{TrainBegin: 0, TrainEnd: 1, ValBegin: 0, ValEnd: 1, TrainFirstAt: 10, TrainLastAt: 10, ValFirstAt: 10, ValLastAt: 10},
	}}
	if _, err := ResolveOuterFolds(ds, val); err == nil {
		t.Fatal("MaxAt >= wall")
	}
}

func TestPortable_Feature65ChangesLogits(t *testing.T) {
	t.Parallel()
	names := make([]string, 66)
	for i := range names {
		names[i] = "f"
	}
	names[65] = "s0_anchor_age_bars"
	m := ml.Model{
		Format: ml.PortableCatBoostV1, Names: names, ClassCount: 3, Scale: 1,
		Trees: []ml.Tree{{
			Splits: []ml.Split{{Feature: 65, Border: 0.5}},
			Leaves: []float64{0, 0, 0, 1, 2, 3},
		}},
	}
	if err := m.Validate(names, 3, 8, 8); err != nil {
		t.Fatal(err)
	}
	x0 := make([]float64, 66)
	x1 := make([]float64, 66)
	x1[65] = 1
	z0, err := m.Logits(x0)
	if err != nil {
		t.Fatal(err)
	}
	z1, err := m.Logits(x1)
	if err != nil {
		t.Fatal(err)
	}
	if z0 == z1 {
		t.Fatal("column 65 must change logits")
	}
	x64 := make([]float64, 66)
	x64[64] = 1
	z64, err := m.Logits(x64)
	if err != nil {
		t.Fatal(err)
	}
	if z64 != z0 {
		t.Fatal("column 64 must not affect a tree that only splits on 65")
	}
	if math.IsNaN(z1[0]) {
		t.Fatal("nan")
	}
}

func TestConvertVendor_Width66Split65(t *testing.T) {
	t.Parallel()
	names := make([]string, 66)
	feats := make([]map[string]any, 66)
	for i := range names {
		names[i] = "c"
		feats[i] = map[string]any{"borders": []float64{0.5}, "has_nans": false, "nan_value_treatment": "AsIs"}
	}
	doc := map[string]any{
		"features_info": map[string]any{"float_features": feats},
		"model_info":    map[string]any{"class_params": map[string]any{"classes_count": 3}},
		"oblivious_trees": []map[string]any{{
			"splits":      []map[string]any{{"border": 0.5, "float_feature_index": 65, "split_type": "FloatFeature"}},
			"leaf_values": []float64{0, 0, 0, 4, 5, 6},
		}},
		"scale_and_bias": []any{1.0, []float64{0, 0, 0}},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ml.ConvertVendor(raw, names, 3, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	x0 := make([]float64, 66)
	x1 := make([]float64, 66)
	x1[65] = 1
	a, _ := m.Logits(x0)
	b, _ := m.Logits(x1)
	if a == b {
		t.Fatal("converted tree must use feature 65")
	}
	bad := doc
	trees := []map[string]any{{
		"splits":      []map[string]any{{"border": 0.5, "float_feature_index": 66, "split_type": "FloatFeature"}},
		"leaf_values": []float64{0, 0, 0, 1, 2, 3},
	}}
	bad["oblivious_trees"] = trees
	rawBad, _ := json.Marshal(bad)
	if _, err := ml.ConvertVendor(rawBad, names, 3, 8, 8); err == nil {
		t.Fatal("index 66 must refuse")
	}
}
