package brain3

import (
	"encoding/json"
	"fmt"
	"math"
)

// Dataset is a certified rectangular matrix. Brain 3 does not recompute X/Y/At.
type Dataset struct {
	Format       string
	Width        int
	FeatureNames []string
	ClassSchema  []string
	At           []int64
	X            [][]float64
	Y            []int
	YLabel       []string
	MinAt        int64
	MaxAt        int64
	ContentHex   string
	SplitHex     string
	PlanHex      string
}

type datasetFile struct {
	Format               string           `json:"format"`
	Width                int              `json:"width"`
	FeatureNames         []string         `json:"feature_names"`
	Rows                 []datasetRowFile `json:"rows"`
	MinCandidateAt       int64            `json:"min_candidate_at"`
	MaxCandidateAt       int64            `json:"max_candidate_at"`
	HoldoutStartAt       int64            `json:"holdout_start_at"`
	ContentDigest        string           `json:"content_digest"`
	DataSplitDigest      string           `json:"data_split_digest"`
	ValidationPlanDigest string           `json:"validation_plan_digest"`
}

type datasetRowFile struct {
	At int64     `json:"at"`
	Y  string    `json:"y"`
	X  []float64 `json:"x"`
}

func classIndex(schema []string, label string) (int, error) {
	for i, s := range schema {
		if s == label {
			return i, nil
		}
	}
	return -1, fmt.Errorf("brain3: class %q is not in ClassSchema", label)
}

// LoadDataset parses a certified setup-dataset JSON. schema is identity, not inferred.
func LoadDataset(raw []byte, schema []string) (Dataset, error) {
	var z Dataset
	if len(schema) == 0 {
		return z, fmt.Errorf("brain3: ClassSchema is required")
	}
	var f datasetFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return z, fmt.Errorf("brain3: dataset json: %w", err)
	}
	if f.Format == "" || f.Width <= 0 || len(f.FeatureNames) != f.Width {
		return z, fmt.Errorf("brain3: dataset width/names")
	}
	n := len(f.Rows)
	if n == 0 {
		return z, fmt.Errorf("brain3: empty dataset")
	}
	z = Dataset{
		Format: f.Format, Width: f.Width, FeatureNames: append([]string(nil), f.FeatureNames...),
		ClassSchema: append([]string(nil), schema...),
		At:          make([]int64, n), X: make([][]float64, n), Y: make([]int, n), YLabel: make([]string, n),
		MinAt: f.MinCandidateAt, MaxAt: f.MaxCandidateAt,
		ContentHex: f.ContentDigest, SplitHex: f.DataSplitDigest, PlanHex: f.ValidationPlanDigest,
	}
	seen := make(map[int64]struct{}, n)
	for i, row := range f.Rows {
		if row.At <= 0 {
			return Dataset{}, fmt.Errorf("brain3: row %d missing At", i)
		}
		if _, ok := seen[row.At]; ok {
			return Dataset{}, fmt.Errorf("brain3: duplicate At %d", row.At)
		}
		seen[row.At] = struct{}{}
		if i > 0 && row.At <= z.At[i-1] {
			return Dataset{}, fmt.Errorf("brain3: At not strictly increasing")
		}
		if len(row.X) != f.Width {
			return Dataset{}, fmt.Errorf("brain3: row %d width %d != %d", i, len(row.X), f.Width)
		}
		for j, v := range row.X {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return Dataset{}, fmt.Errorf("brain3: row %d X[%d] not finite", i, j)
			}
		}
		yi, err := classIndex(schema, row.Y)
		if err != nil {
			return Dataset{}, err
		}
		if yi < 0 || yi >= len(schema) {
			return Dataset{}, fmt.Errorf("brain3: y out of schema")
		}
		z.At[i] = row.At
		z.X[i] = append([]float64(nil), row.X...)
		z.Y[i] = yi
		z.YLabel[i] = row.Y
	}
	if f.MinCandidateAt != 0 && f.MinCandidateAt != z.At[0] {
		return Dataset{}, fmt.Errorf("brain3: min_at mismatch")
	}
	if f.MaxCandidateAt != 0 && f.MaxCandidateAt != z.At[n-1] {
		return Dataset{}, fmt.Errorf("brain3: max_at mismatch")
	}
	z.MinAt = z.At[0]
	z.MaxAt = z.At[n-1]
	return z, nil
}
