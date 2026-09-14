package brain3

import (
	"fmt"
	"strings"
)

// FoldRows is exact dataset membership for one compiled outer fold.
type FoldRows struct {
	Index                     int
	TrainN, ValN              int
	TrainFirstAt, TrainLastAt int64
	ValFirstAt, ValLastAt     int64
	TrainBegin, TrainEnd      int
	ValBegin, ValEnd          int
}

// Census is the foundation preflight report. No model metrics.
type Census struct {
	Rows            int
	Width           int
	ClassSchema     []string
	MinAt, MaxAt    int64
	HoldoutStartAt  int64
	HoldoutRowsSeen int
	MissingJoins    int
	DuplicateValAt  int
	Folds           []FoldRows
	ValTotal        int
	TrainOnlyPrefix int
	Text            string
}

// ResolveOuterFolds maps frozen compiled index ranges onto the dataset by exact At.
func ResolveOuterFolds(ds Dataset, val Validation) (Census, error) {
	var z Census
	z.Rows = len(ds.At)
	z.Width = ds.Width
	z.ClassSchema = append([]string(nil), ds.ClassSchema...)
	z.MinAt = ds.MinAt
	z.MaxAt = ds.MaxAt
	z.HoldoutStartAt = val.HoldoutStartAt
	if val.HoldoutStartAt <= 0 {
		return z, fmt.Errorf("brain3: holdout wall required")
	}
	if ds.MaxAt >= val.HoldoutStartAt {
		return z, fmt.Errorf("brain3: MaxAt %d is not < holdout wall %d", ds.MaxAt, val.HoldoutStartAt)
	}
	for _, at := range ds.At {
		if at >= val.HoldoutStartAt {
			z.HoldoutRowsSeen++
		}
	}
	if z.HoldoutRowsSeen != 0 {
		return z, fmt.Errorf("brain3: holdout At entered the dataset n=%d", z.HoldoutRowsSeen)
	}
	if val.HoldoutBeginIndex != 0 && val.HoldoutBeginIndex != len(ds.At) {
		return z, fmt.Errorf("brain3: HoldoutBeginIndex %d != dataset rows %d", val.HoldoutBeginIndex, len(ds.At))
	}
	seenVal := map[int64]int{}
	inVal := make([]bool, len(ds.At))
	z.Folds = make([]FoldRows, len(val.Folds))
	for i, cf := range val.Folds {
		if cf.TrainEnd > len(ds.At) || cf.ValEnd > len(ds.At) {
			z.MissingJoins++
			return z, fmt.Errorf("brain3: fold %d index past dataset", i)
		}
		if ds.At[cf.TrainBegin] != cf.TrainFirstAt || ds.At[cf.TrainEnd-1] != cf.TrainLastAt {
			return z, fmt.Errorf("brain3: fold %d train At mismatch compiled vs dataset", i)
		}
		if ds.At[cf.ValBegin] != cf.ValFirstAt || ds.At[cf.ValEnd-1] != cf.ValLastAt {
			return z, fmt.Errorf("brain3: fold %d val At mismatch compiled vs dataset", i)
		}
		for j := cf.TrainBegin; j < cf.TrainEnd; j++ {
			if ds.At[j] >= val.HoldoutStartAt {
				return z, fmt.Errorf("brain3: fold %d train holdout At %d", i, ds.At[j])
			}
		}
		for j := cf.ValBegin; j < cf.ValEnd; j++ {
			at := ds.At[j]
			if at >= val.HoldoutStartAt {
				return z, fmt.Errorf("brain3: fold %d val holdout At %d", i, at)
			}
			if prev, ok := seenVal[at]; ok {
				z.DuplicateValAt++
				return z, fmt.Errorf("brain3: validation At %d in folds %d and %d", at, prev, i)
			}
			seenVal[at] = i
			inVal[j] = true
		}
		z.Folds[i] = FoldRows{
			Index: i, TrainN: cf.TrainEnd - cf.TrainBegin, ValN: cf.ValEnd - cf.ValBegin,
			TrainFirstAt: cf.TrainFirstAt, TrainLastAt: cf.TrainLastAt,
			ValFirstAt: cf.ValFirstAt, ValLastAt: cf.ValLastAt,
			TrainBegin: cf.TrainBegin, TrainEnd: cf.TrainEnd, ValBegin: cf.ValBegin, ValEnd: cf.ValEnd,
		}
		z.ValTotal += z.Folds[i].ValN
	}
	for _, ok := range inVal {
		if !ok {
			z.TrainOnlyPrefix++
		}
	}
	z.Text = FormatCensus(z)
	return z, nil
}

func FormatCensus(z Census) string {
	var b strings.Builder
	b.WriteString("BRAIN3-FOUNDATION-1 (no CatBoost fit)\n")
	b.WriteString(fmt.Sprintf("rows=%d width=%d class_schema=%v\n", z.Rows, z.Width, z.ClassSchema))
	b.WriteString(fmt.Sprintf("min_at=%d max_at=%d holdout_wall=%d\n", z.MinAt, z.MaxAt, z.HoldoutStartAt))
	b.WriteString(fmt.Sprintf("holdout_rows_seen=%d missing_joins=%d duplicate_val_at=%d\n", z.HoldoutRowsSeen, z.MissingJoins, z.DuplicateValAt))
	for _, f := range z.Folds {
		b.WriteString(fmt.Sprintf("fold %d  train_n=%d  val_n=%d  train=[%d,%d] val=[%d,%d]\n",
			f.Index, f.TrainN, f.ValN, f.TrainFirstAt, f.TrainLastAt, f.ValFirstAt, f.ValLastAt))
	}
	b.WriteString(fmt.Sprintf("validation_total=%d  train_only_prefix=%d\n", z.ValTotal, z.TrainOnlyPrefix))
	b.WriteString("MaxAt < holdout wall: hard assertion passed\n")
	b.WriteString("no model trained\n")
	return b.String()
}
