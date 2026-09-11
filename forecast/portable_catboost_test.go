package forecast

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func parseF64Bits(t *testing.T, s string) float64 {
	t.Helper()
	u, err := parseHexU64(s)
	if err != nil {
		t.Fatalf("bits %s: %v", s, err)
	}
	return math.Float64frombits(u)
}

func parseHexU64(s string) (uint64, error) {
	if len(s) != 16 {
		return 0, os.ErrInvalid
	}
	var u uint64
	for i := 0; i < 16; i++ {
		c := s[i]
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		default:
			return 0, os.ErrInvalid
		}
		u = u<<4 | uint64(v)
	}
	return u, nil
}

func TestConvertPortableCatBoost_TinyParity(t *testing.T) {
	t.Parallel()
	root := filepath.Join("testdata", "catboost")
	for _, name := range []string{"tiny_multi", "tiny_depth1"} {
		raw, err := os.ReadFile(filepath.Join(root, name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var exp struct {
			FeatureIDs []string              `json:"feature_ids"`
			X          [][]float64           `json:"X"`
			FullBits   [][]string            `json:"full_bits"`
			PrefixBits map[string][][]string `json:"prefix_bits"`
			TreeCount  int                   `json:"tree_count"`
			Classes    []int                 `json:"classes"`
		}
		if err := json.Unmarshal(mustRead(t, filepath.Join(root, name+".expect.json")), &exp); err != nil {
			t.Fatal(err)
		}
		if got := exp.Classes; len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
			t.Fatal(got)
		}
		ids := make([]FeatureID, len(exp.FeatureIDs))
		for i, s := range exp.FeatureIDs {
			ids[i] = FeatureID(s)
		}
		m, err := ConvertVendorCatBoostJSON(raw, ids, 3, 16, 8)
		if err != nil {
			t.Fatal(name, err)
		}
		if err := m.Validate(ids, 3, 16, 8); err != nil {
			t.Fatal(err)
		}
		d1 := m.ContentDigest()
		m2, err := ConvertVendorCatBoostJSON(raw, ids, 3, 16, 8)
		if err != nil {
			t.Fatal(err)
		}
		if d1 != m2.ContentDigest() {
			t.Fatal("portable digest")
		}
		for i, x := range exp.X {
			z, err := m.Logits(x)
			if err != nil {
				t.Fatal(err)
			}
			for c := 0; c < 3; c++ {
				want := parseF64Bits(t, exp.FullBits[i][c])
				if math.Float64bits(z[c]) != math.Float64bits(want) {
					t.Fatalf("%s row %d class %d go=%x want=%x gof=%v wantf=%v", name, i, c, math.Float64bits(z[c]), math.Float64bits(want), z[c], want)
				}
			}
		}
		for ns, rows := range exp.PrefixBits {
			var n int
			if _, err := parseInt(&n, ns); err != nil {
				t.Fatal(err)
			}
			for i, x := range exp.X {
				z, err := m.PrefixLogits(x, n)
				if err != nil {
					t.Fatal(err)
				}
				for c := 0; c < 3; c++ {
					want := parseF64Bits(t, rows[i][c])
					if math.Float64bits(z[c]) != math.Float64bits(want) {
						t.Fatalf("prefix n=%d row %d class %d", n, i, c)
					}
				}
			}
		}
	}
}

func parseInt(dst *int, s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(c-'0')
	}
	*dst = n
	return n, nil
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestConvertPortableCatBoost_PrefixFitLaw(t *testing.T) {
	t.Parallel()
	root := filepath.Join("testdata", "catboost")
	var exp struct {
		FeatureIDs []string    `json:"feature_ids"`
		X          [][]float64 `json:"X"`
		MaxBits    [][]string  `json:"prefix3_from_max_bits"`
		NBits      [][]string  `json:"fitted3_bits"`
	}
	if err := json.Unmarshal(mustRead(t, filepath.Join(root, "prefix_fit.expect.json")), &exp); err != nil {
		t.Fatal(err)
	}
	ids := []FeatureID{FeatureID(exp.FeatureIDs[0]), FeatureID(exp.FeatureIDs[1])}
	maxM, err := ConvertVendorCatBoostJSON(mustRead(t, filepath.Join(root, "prefix_max.json")), ids, 3, 16, 8)
	if err != nil {
		t.Fatal(err)
	}
	nM, err := ConvertVendorCatBoostJSON(mustRead(t, filepath.Join(root, "prefix_n.json")), ids, 3, 16, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(nM.Trees) != 3 {
		t.Fatal(len(nM.Trees))
	}
	for i, x := range exp.X {
		a, err := maxM.PrefixLogits(x, 3)
		if err != nil {
			t.Fatal(err)
		}
		b, err := nM.Logits(x)
		if err != nil {
			t.Fatal(err)
		}
		for c := 0; c < 3; c++ {
			if math.Float64bits(a[c]) != math.Float64bits(b[c]) {
				t.Fatalf("PREFIX_FIT_LAW_FAILED row %d class %d", i, c)
			}
			want := parseF64Bits(t, exp.MaxBits[i][c])
			if math.Float64bits(a[c]) != math.Float64bits(want) {
				t.Fatalf("prefix witness row %d", i)
			}
		}
	}
}

func TestConvertPortableCatBoost_Hostile(t *testing.T) {
	t.Parallel()
	raw := mustRead(t, filepath.Join("testdata", "catboost", "tiny_depth1.json"))
	ids := []FeatureID{"f0"}
	if _, err := ConvertVendorCatBoostJSON(raw, []FeatureID{"nope", "x"}, 3, 16, 8); err == nil {
		t.Fatal("width mismatch")
	}
	m, err := ConvertVendorCatBoostJSON(raw, ids, 3, 16, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate([]FeatureID{"other"}, 3, 16, 8); err == nil {
		t.Fatal("feature order")
	}
	if _, err := ConvertVendorCatBoostJSON(raw, ids, 2, 16, 8); err == nil {
		t.Fatal("class count")
	}
	if _, err := ConvertVendorCatBoostJSON([]byte(`{"oblivious_trees":[]}`), ids, 3, 16, 8); err == nil {
		t.Fatal("malformed")
	}
}

func TestCatboostFloatGt_Border(t *testing.T) {
	t.Parallel()
	b := 1.5
	if catboostFloatGt(1.5, b) {
		t.Fatal("equal border must go left")
	}
	if catboostFloatGt(math.Nextafter(1.5, -1), b) {
		t.Fatal("-ulp")
	}
	if !catboostFloatGt(2.0, b) {
		t.Fatal("2.0")
	}
	if catboostFloatGt(0.0, 0.0) || catboostFloatGt(math.Copysign(0, -1), 0.0) {
		t.Fatal("signed zero must not be > +0")
	}
	b32 := float64(float32(0.8))
	if catboostFloatGt(0.8, b32) {
		t.Fatal("0.8 vs f32 0.8")
	}
}

func TestSelectTreeCount_TieAndCap(t *testing.T) {
	t.Parallel()
	n, err := SelectTreeCount([]float64{0.4, 0.2, 0.2, 0.3}, 8)
	if err != nil || n != 2 {
		t.Fatalf("tie smallest N got %d %v", n, err)
	}
	if _, err := SelectTreeCount([]float64{0.3, 0.2, 0.1}, 3); err == nil {
		t.Fatal("cap")
	}
}

func TestMulticlassLogLoss_Fixture(t *testing.T) {
	t.Parallel()
	z := [3]float64{0, 0, 0}
	l, err := MulticlassLogLoss(z, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := math.Log(3)
	if math.Abs(l-want) > 1e-15 {
		t.Fatal(l, want)
	}
}
