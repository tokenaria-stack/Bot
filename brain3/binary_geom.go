package brain3

import (
	"fmt"

	"trading_bot/data"
	"trading_bot/ml"
)

type ResolvedRow struct {
	Index int
	At    int64
	X     []float64
	YBin  int // 0 STOP_FIRST, 1 TP_FIRST
}

func toBinaryY(datasetY int) (int, bool) {
	switch datasetY {
	case 0:
		return 1, true
	case 1:
		return 0, true
	default:
		return 0, false
	}
}

func filterResolved(ds Dataset, begin, end int, holdout int64) ([]ResolvedRow, error) {
	var out []ResolvedRow
	for i := begin; i < end; i++ {
		yb, ok := toBinaryY(ds.Y[i])
		if !ok {
			continue
		}
		if holdout > 0 && ds.At[i] >= holdout {
			return nil, fmt.Errorf("brain3: holdout At in resolved filter")
		}
		out = append(out, ResolvedRow{Index: i, At: ds.At[i], X: ds.X[i], YBin: yb})
	}
	return out, nil
}

func resolvedCensus(rows []ResolvedRow) (tp, stop int) {
	for _, r := range rows {
		if r.YBin == 1 {
			tp++
		} else {
			stop++
		}
	}
	return tp, stop
}

func resolvedAt(rows []ResolvedRow) []int64 {
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.At
	}
	return out
}

func resolvedXY(rows []ResolvedRow) (xs [][]float64, ys []int) {
	xs = make([][]float64, len(rows))
	ys = make([]int, len(rows))
	for i, r := range rows {
		xs[i] = r.X
		ys[i] = r.YBin
	}
	return xs, ys
}

type BinaryFoldGeom struct {
	OrigTrainN, OrigValN             int
	ResTrainN, ResValN               int
	TrainTP, TrainStop               int
	ValTP, ValStop                   int
	InnerTrainN, InnerValN, EmbargoN int
	InnerTrainFirst, InnerTrainLast  int64
	InnerValFirst, InnerValLast      int64
	InnerTrainTP, InnerTrainStop     int
	InnerValTP, InnerValStop         int
	OuterTrainFirst, OuterTrainLast  int64
	OuterValFirst, OuterValLast      int64
	Split                            ml.CausalTailSplit
	Train                            []ResolvedRow
	Val                              []ResolvedRow
}

func PreflightBinaryGeometry(ds Dataset, census Census, spec TPStopBinaryProbeSpec1) ([]BinaryFoldGeom, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	out := make([]BinaryFoldGeom, len(census.Folds))
	for i, f := range census.Folds {
		train, err := filterResolved(ds, f.TrainBegin, f.TrainEnd, census.HoldoutStartAt)
		if err != nil {
			return nil, err
		}
		val, err := filterResolved(ds, f.ValBegin, f.ValEnd, census.HoldoutStartAt)
		if err != nil {
			return nil, err
		}
		if len(train) == 0 {
			return nil, fmt.Errorf("brain3: TP_STOP_BINARY_SPEC1_FAILED_TO_COMPILE fold %d empty resolved train", i)
		}
		for j := 1; j < len(train); j++ {
			if train[j].At <= train[j-1].At {
				return nil, fmt.Errorf("brain3: resolved train At not increasing")
			}
		}
		last := train[len(train)-1].At
		excl, err := data.NextBarOpen(last, spec.Timeframe)
		if err != nil {
			return nil, err
		}
		split, err := ml.SplitCausalTail(resolvedAt(train), spec.Timeframe, excl, spec.InnerSpanBars, spec.HorizonBars, spec.ExtraGapBars, spec.MinInnerTrain)
		if err != nil {
			return nil, fmt.Errorf("brain3: TP_STOP_BINARY_SPEC1_FAILED_TO_COMPILE fold %d: %w", i, err)
		}
		innerTr := train[split.InnerTrainBegin:split.InnerTrainEnd]
		innerVa := train[split.InnerValBegin:split.InnerValEnd]
		trTP, trST := resolvedCensus(train)
		vaTP, vaST := resolvedCensus(val)
		iTTP, iTST := resolvedCensus(innerTr)
		iVTP, iVST := resolvedCensus(innerVa)
		g := BinaryFoldGeom{
			OrigTrainN: f.TrainN, OrigValN: f.ValN,
			ResTrainN: len(train), ResValN: len(val),
			TrainTP: trTP, TrainStop: trST, ValTP: vaTP, ValStop: vaST,
			InnerTrainN: len(innerTr), InnerValN: len(innerVa),
			EmbargoN:        split.InnerValBegin - split.InnerTrainEnd,
			InnerTrainFirst: innerTr[0].At, InnerTrainLast: innerTr[len(innerTr)-1].At,
			InnerValFirst: innerVa[0].At, InnerValLast: innerVa[len(innerVa)-1].At,
			InnerTrainTP: iTTP, InnerTrainStop: iTST, InnerValTP: iVTP, InnerValStop: iVST,
			OuterTrainFirst: f.TrainFirstAt, OuterTrainLast: f.TrainLastAt,
			OuterValFirst: f.ValFirstAt, OuterValLast: f.ValLastAt,
			Split: split, Train: train, Val: val,
		}
		if g.InnerTrainLast >= g.InnerValFirst {
			return nil, fmt.Errorf("brain3: TP_STOP_BINARY_SPEC1_FAILED_TO_COMPILE fold %d inner overlap", i)
		}
		out[i] = g
	}
	return out, nil
}

func FormatBinaryPreflight(gs []BinaryFoldGeom) string {
	var b []byte
	b = append(b, "TP-STOP-BINARY-PROBE-1 geometry preflight (no fit)\n"...)
	b = append(b, "inner exclusive end = NextBarOpen(last RESOLVED outer-train At); SplitCausalTail contract.\n"...)
	for i, g := range gs {
		b = append(b, fmt.Sprintf("fold %d  orig_train/val=%d/%d  resolved_train/val=%d/%d  inner_train/val=%d/%d embargo=%d\n",
			i, g.OrigTrainN, g.OrigValN, g.ResTrainN, g.ResValN, g.InnerTrainN, g.InnerValN, g.EmbargoN)...)
		b = append(b, fmt.Sprintf("         resolved_train TP/STOP=%d/%d  resolved_val TP/STOP=%d/%d\n", g.TrainTP, g.TrainStop, g.ValTP, g.ValStop)...)
		b = append(b, fmt.Sprintf("         inner_train=[%d,%d] TP/STOP=%d/%d  inner_val=[%d,%d] TP/STOP=%d/%d\n",
			g.InnerTrainFirst, g.InnerTrainLast, g.InnerTrainTP, g.InnerTrainStop, g.InnerValFirst, g.InnerValLast, g.InnerValTP, g.InnerValStop)...)
	}
	return string(b)
}
