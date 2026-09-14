package brain3

import (
	"fmt"

	"trading_bot/data"
	"trading_bot/ml"
)

type InnerGeometry struct {
	OuterTrainN, OuterValN                     int
	InnerTrainN, InnerValN                     int
	EmbargoN                                   int
	OuterTrainFirst, OuterTrainLast            int64
	OuterValFirst, OuterValLast                int64
	InnerTrainFirst, InnerTrainLast            int64
	InnerValFirst, InnerValLast                int64
	InnerTrainTP, InnerTrainStop, InnerTrainTO int
	InnerValTP, InnerValStop, InnerValTO       int
	Split                                      ml.CausalTailSplit
}

func countClasses(y []int, begin, end int) (tp, stop, timeout int, err error) {
	for i := begin; i < end; i++ {
		switch y[i] {
		case 0:
			tp++
		case 1:
			stop++
		case 2:
			timeout++
		default:
			return 0, 0, 0, fmt.Errorf("brain3: y=%d not in schema", y[i])
		}
	}
	return tp, stop, timeout, nil
}

func sliceAt(ds Dataset, begin, end int) []int64 {
	out := make([]int64, end-begin)
	copy(out, ds.At[begin:end])
	return out
}

// PreflightInnerGeometry compiles Spec1 inner tails inside frozen outer TRAIN only.
func PreflightInnerGeometry(ds Dataset, census Census, spec MetaLabelCatBoostSpec1) ([]InnerGeometry, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	out := make([]InnerGeometry, len(census.Folds))
	for i, f := range census.Folds {
		trainAt := sliceAt(ds, f.TrainBegin, f.TrainEnd)
		if len(trainAt) == 0 {
			return nil, fmt.Errorf("brain3: SPEC1_FAILED_TO_COMPILE fold %d empty outer train", i)
		}
		last := trainAt[len(trainAt)-1]
		excl, err := data.NextBarOpen(last, spec.Timeframe)
		if err != nil {
			return nil, err
		}
		split, err := ml.SplitCausalTail(trainAt, spec.Timeframe, excl, spec.InnerSpanBars, spec.HorizonBars, spec.ExtraGapBars, spec.MinInnerTrain)
		if err != nil {
			return nil, fmt.Errorf("brain3: SPEC1_FAILED_TO_COMPILE fold %d: %w", i, err)
		}
		base := f.TrainBegin
		g := InnerGeometry{
			OuterTrainN: f.TrainN, OuterValN: f.ValN,
			InnerTrainN:     split.InnerTrainEnd - split.InnerTrainBegin,
			InnerValN:       split.InnerValEnd - split.InnerValBegin,
			EmbargoN:        split.InnerValBegin - split.InnerTrainEnd,
			OuterTrainFirst: f.TrainFirstAt, OuterTrainLast: f.TrainLastAt,
			OuterValFirst: f.ValFirstAt, OuterValLast: f.ValLastAt,
			Split: split,
		}
		g.InnerTrainFirst = ds.At[base+split.InnerTrainBegin]
		g.InnerTrainLast = ds.At[base+split.InnerTrainEnd-1]
		g.InnerValFirst = ds.At[base+split.InnerValBegin]
		g.InnerValLast = ds.At[base+split.InnerValEnd-1]
		g.InnerTrainTP, g.InnerTrainStop, g.InnerTrainTO, err = countClasses(ds.Y, base+split.InnerTrainBegin, base+split.InnerTrainEnd)
		if err != nil {
			return nil, err
		}
		g.InnerValTP, g.InnerValStop, g.InnerValTO, err = countClasses(ds.Y, base+split.InnerValBegin, base+split.InnerValEnd)
		if err != nil {
			return nil, err
		}
		if g.InnerTrainLast >= g.InnerValFirst {
			return nil, fmt.Errorf("brain3: SPEC1_FAILED_TO_COMPILE fold %d inner overlap", i)
		}
		out[i] = g
	}
	return out, nil
}

func FormatInnerPreflight(gs []InnerGeometry) string {
	var b []byte
	b = append(b, "BRAIN3-METALABEL-1 inner geometry preflight (no fit)\n"...)
	for i, g := range gs {
		line := fmt.Sprintf("fold %d  outer_train_n=%d outer_val_n=%d  inner_train_n=%d inner_val_n=%d embargo_n=%d\n",
			i, g.OuterTrainN, g.OuterValN, g.InnerTrainN, g.InnerValN, g.EmbargoN)
		b = append(b, line...)
		b = append(b, fmt.Sprintf("         outer_train=[%d,%d] outer_val=[%d,%d]\n", g.OuterTrainFirst, g.OuterTrainLast, g.OuterValFirst, g.OuterValLast)...)
		b = append(b, fmt.Sprintf("         inner_train=[%d,%d] inner_val=[%d,%d]\n", g.InnerTrainFirst, g.InnerTrainLast, g.InnerValFirst, g.InnerValLast)...)
		b = append(b, fmt.Sprintf("         inner_train TP/STOP/TO=%d/%d/%d  inner_val TP/STOP/TO=%d/%d/%d\n",
			g.InnerTrainTP, g.InnerTrainStop, g.InnerTrainTO, g.InnerValTP, g.InnerValStop, g.InnerValTO)...)
	}
	return string(b)
}
