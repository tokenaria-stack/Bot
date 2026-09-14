package brain3

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	MetaLabelSpec1Logic       = "metalabel-catboost-spec1"
	MetaLabelLossMultiClass   = "MultiClass"
	MetaLabelSelectionLogLoss = "multiclass_logloss"
	MetaLabelTieSmallest      = "smallest_tree_count"
	MetaLabelCapRefuse        = "ITERATION_CAP_REACHED"
	Spec1InnerSpanBars        = 35040
	Spec1MinInnerTrain        = 1000
	Spec1HorizonBars          = 72
	Spec1MaxIterations        = 1000
	Spec1Depth                = 6
	Spec1LearningRate         = 0.03
	Spec1L2LeafReg            = 3.0
	Spec1Timeframe            = "15m"
	OOFFormatV1               = "brain3-oof-logits-v1"
)

// MetaLabelCatBoostSpec1 is one disposable hypothesis, not Brain 3 itself.
type MetaLabelCatBoostSpec1 struct {
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

func Spec1() MetaLabelCatBoostSpec1 {
	return MetaLabelCatBoostSpec1{
		Loss: MetaLabelLossMultiClass, Depth: Spec1Depth, LearningRate: Spec1LearningRate,
		L2LeafReg: Spec1L2LeafReg, MaxIterations: Spec1MaxIterations,
		InnerSpanBars: Spec1InnerSpanBars, MinInnerTrain: Spec1MinInnerTrain,
		HorizonBars: Spec1HorizonBars, ExtraGapBars: 0, Timeframe: Spec1Timeframe,
		SelectionMetric: MetaLabelSelectionLogLoss, TieLaw: MetaLabelTieSmallest, CapLaw: MetaLabelCapRefuse,
	}
}

func (s MetaLabelCatBoostSpec1) validate() error {
	if s.Loss != MetaLabelLossMultiClass || s.Depth != Spec1Depth || s.LearningRate != Spec1LearningRate || s.L2LeafReg != Spec1L2LeafReg {
		return fmt.Errorf("brain3: MetaLabelCatBoostSpec1 numeric hypothesis mismatch")
	}
	if s.MaxIterations != Spec1MaxIterations || s.InnerSpanBars != Spec1InnerSpanBars || s.MinInnerTrain != Spec1MinInnerTrain {
		return fmt.Errorf("brain3: MetaLabelCatBoostSpec1 inner/cap mismatch")
	}
	if s.HorizonBars != Spec1HorizonBars || s.Timeframe != Spec1Timeframe || s.ExtraGapBars != 0 {
		return fmt.Errorf("brain3: MetaLabelCatBoostSpec1 embargo mismatch")
	}
	if s.SelectionMetric != MetaLabelSelectionLogLoss || s.TieLaw != MetaLabelTieSmallest || s.CapLaw != MetaLabelCapRefuse {
		return fmt.Errorf("brain3: MetaLabelCatBoostSpec1 selection mismatch")
	}
	return nil
}

type specIdentityPayload struct {
	DatasetContent  string
	ValidationPlan  string
	DataSplit       string
	FeatureNames    []string
	Width           int
	ClassSchema     []string
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

func (s MetaLabelCatBoostSpec1) DigestHex(ds Dataset, val Validation) (string, error) {
	if err := s.validate(); err != nil {
		return "", err
	}
	b, err := json.Marshal(specIdentityPayload{
		DatasetContent: ds.ContentHex, ValidationPlan: val.PlanHex, DataSplit: val.SplitHex,
		FeatureNames: ds.FeatureNames, Width: ds.Width, ClassSchema: ds.ClassSchema,
		Loss: s.Loss, Depth: s.Depth, LearningRate: s.LearningRate, L2LeafReg: s.L2LeafReg,
		MaxIterations: s.MaxIterations, InnerSpanBars: s.InnerSpanBars, MinInnerTrain: s.MinInnerTrain,
		HorizonBars: s.HorizonBars, ExtraGapBars: s.ExtraGapBars, Timeframe: s.Timeframe,
		SelectionMetric: s.SelectionMetric, TieLaw: s.TieLaw, CapLaw: s.CapLaw,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func ResearchCatBoostParams(s MetaLabelCatBoostSpec1) map[string]any {
	return map[string]any{
		"loss_function": s.Loss, "eval_metric": s.Loss, "classes_count": 3,
		"depth": s.Depth, "learning_rate": s.LearningRate, "l2_leaf_reg": s.L2LeafReg,
		"random_seed": 0, "thread_count": 1, "task_type": "CPU",
		"use_best_model": false, "allow_writing_files": false, "verbose": false,
		"nan_mode": "Forbidden",
	}
}
