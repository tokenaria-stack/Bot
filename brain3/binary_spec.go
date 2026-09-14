package brain3

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	BinaryProbeLossLogloss     = "Logloss"
	BinaryProbeSelectionMetric = "binary_logloss"
	BinaryOOFFormatV1          = "brain3-binary-oof-v1"
)

func BinaryClassSchema() []string {
	return []string{ClassStopFirst, ClassTPFirst}
}

// TPStopBinaryProbeSpec1 is one locked diagnostic hypothesis. Declared before the audit.
type TPStopBinaryProbeSpec1 struct {
	Loss            string
	Depth           int
	LearningRate    float64
	L2LeafReg       float64
	MaxIterations   int
	InnerSpanBars   int
	MinInnerTrain   int
	HorizonBars     int
	ExtraGapBars    int
	Timeframe       string
	SelectionMetric string
	TieLaw          string
	CapLaw          string
	Population      string
}

func BinaryProbeSpec1() TPStopBinaryProbeSpec1 {
	return TPStopBinaryProbeSpec1{
		Loss: BinaryProbeLossLogloss, Depth: Spec1Depth, LearningRate: Spec1LearningRate,
		L2LeafReg: Spec1L2LeafReg, MaxIterations: Spec1MaxIterations,
		InnerSpanBars: Spec1InnerSpanBars, MinInnerTrain: Spec1MinInnerTrain,
		HorizonBars: Spec1HorizonBars, ExtraGapBars: 0, Timeframe: Spec1Timeframe,
		SelectionMetric: BinaryProbeSelectionMetric, TieLaw: MetaLabelTieSmallest, CapLaw: MetaLabelCapRefuse,
		Population: "resolved_only",
	}
}

func (s TPStopBinaryProbeSpec1) validate() error {
	if s.Loss != BinaryProbeLossLogloss || s.Depth != Spec1Depth || s.LearningRate != Spec1LearningRate || s.L2LeafReg != Spec1L2LeafReg {
		return fmt.Errorf("brain3: TPStopBinaryProbeSpec1 numeric mismatch")
	}
	if s.MaxIterations != Spec1MaxIterations || s.InnerSpanBars != Spec1InnerSpanBars || s.MinInnerTrain != Spec1MinInnerTrain {
		return fmt.Errorf("brain3: TPStopBinaryProbeSpec1 inner/cap mismatch")
	}
	if s.HorizonBars != Spec1HorizonBars || s.Timeframe != Spec1Timeframe || s.ExtraGapBars != 0 {
		return fmt.Errorf("brain3: TPStopBinaryProbeSpec1 embargo mismatch")
	}
	if s.SelectionMetric != BinaryProbeSelectionMetric || s.TieLaw != MetaLabelTieSmallest || s.CapLaw != MetaLabelCapRefuse {
		return fmt.Errorf("brain3: TPStopBinaryProbeSpec1 selection mismatch")
	}
	if s.Population != "resolved_only" {
		return fmt.Errorf("brain3: TPStopBinaryProbeSpec1 population")
	}
	return nil
}

type binarySpecIdentity struct {
	DatasetContent  string
	ValidationPlan  string
	DataSplit       string
	FeatureNames    []string
	Width           int
	ClassSchema     []string
	Population      string
	Loss            string
	Depth           int
	LearningRate    float64
	L2LeafReg       float64
	MaxIterations   int
	InnerSpanBars   int
	MinInnerTrain   int
	HorizonBars     int
	ExtraGapBars    int
	Timeframe       string
	SelectionMetric string
	TieLaw          string
	CapLaw          string
}

func (s TPStopBinaryProbeSpec1) DigestHex(ds Dataset, val Validation) (string, error) {
	if err := s.validate(); err != nil {
		return "", err
	}
	b, err := json.Marshal(binarySpecIdentity{
		DatasetContent: ds.ContentHex, ValidationPlan: val.PlanHex, DataSplit: val.SplitHex,
		FeatureNames: ds.FeatureNames, Width: ds.Width, ClassSchema: BinaryClassSchema(),
		Population: s.Population, Loss: s.Loss, Depth: s.Depth, LearningRate: s.LearningRate,
		L2LeafReg: s.L2LeafReg, MaxIterations: s.MaxIterations, InnerSpanBars: s.InnerSpanBars,
		MinInnerTrain: s.MinInnerTrain, HorizonBars: s.HorizonBars, ExtraGapBars: s.ExtraGapBars,
		Timeframe: s.Timeframe, SelectionMetric: s.SelectionMetric, TieLaw: s.TieLaw, CapLaw: s.CapLaw,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func ResearchBinaryCatBoostParams(s TPStopBinaryProbeSpec1) map[string]any {
	return map[string]any{
		"loss_function": s.Loss, "eval_metric": s.Loss,
		"depth": s.Depth, "learning_rate": s.LearningRate, "l2_leaf_reg": s.L2LeafReg,
		"random_seed": 0, "thread_count": 1, "task_type": "CPU",
		"use_best_model": false, "allow_writing_files": false, "verbose": false,
		"nan_mode": "Forbidden",
	}
}
