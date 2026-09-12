package forecast

import (
	"fmt"
	"math"
)

const (
	CatBoostLogicV1        LogicVersion = "catboost:multiclass-v1"
	CatBoostFamily                      = "CATBOOST"
	CatBoostLossMultiClass              = "MultiClass"
	CatBoostTaskCPU                     = "CPU"
	CatBoostGrowSymmetric               = "SymmetricTree"
	CatBoostNanForbidden                = "Forbidden"
	CatBoostSelLogLoss                  = "go_multiclass_logloss"
	CatBoostTieSmallestN                = "smallest_tree_count"
	CatBoostCapRefuse                   = "ITERATION_CAP_REACHED"

	CatBoostSpecDepth         = 6
	CatBoostSpecLearningRate  = 0.03
	CatBoostSpecL2LeafReg     = 3.0
	CatBoostSpecMaxIterations = 1000
	CatBoostSpecInnerSpanBars = 17568
	CatBoostSpecMinInnerTrain = 35040
	CatBoostSpecRandomSeed    = 0
	CatBoostSpecThreadCount   = 1
	CatBoostSpecBorderCount   = 254
	CatBoostSpecClassCount    = 3
	CatBoostSpecExtraGapBars  = 0
)

// CatBoostSpec1 is the frozen Brain-V2 CatBoost run contract (cecc5a3).
// Matrix digest is a run pin, not this identity.
//
// Field layout is frozen. Do not split this struct for prettier layers:
// Identity hashes catBoostSpec1Payload of these fields. MODEL-IDENTITY-LAYERS-1
// is the later split (ModelSpec / ExecutionProfile / RunWitness) when a second
// model family or trainer environment exists. Comments below are that map.
type CatBoostSpec1 struct {
	// Hypothesis: class geometry and loss.
	Logic            LogicVersion
	Family           string
	Loss             string
	ClassCount       int
	ClassOrder       [3]TargetOutcome
	NumericOnly      bool
	MissingValues    string
	StandardScaler   string
	ClassWeights     string
	AutoClassWeights string
	// Hypothesis: tree-growing choices.
	TaskType                   string
	GrowPolicy                 string
	BoostingType               string
	BootstrapType              string
	BaggingTemperature         float64
	BorderCount                int
	FeatureBorderType          string
	LeafEstimationMethod       string
	LeafEstimationIterations   int
	LeafEstimationBacktracking string
	RandomStrength             float64
	RSM                        float64
	SamplingFrequency          string
	ScoreFunction              string
	ModelShrinkRate            float64
	ModelShrinkMode            string
	ModelSizeReg               float64
	MinDataInLeaf              int
	PosteriorSampling          bool
	BoostFromAverage           bool
	UseBestModel               bool
	RandomScoreType            string
	// Execution / vendor-resolved pins mixed into this chapter's identity
	// because they can change trees. Not a scientific hypothesis.
	BayesianMatrixReg              float64
	PenaltiesCoefficient           float64
	BestModelMinTrees              int
	EvalFraction                   float64
	ForceUnitAutoPairWeights       bool
	SparseFeaturesConflictFraction float64
	// Hypothesis: size / rate / seed.
	Depth         int
	LearningRate  float64
	L2LeafReg     float64
	MaxIterations int
	RandomSeed    int
	// Execution: bit-reproducibility, not an ML hypothesis.
	ThreadCount int
	// Causal inner-tail / Go selection law (not CatBoost vendor knobs).
	InnerSpanBars            int
	MinInnerTrainRows        int
	ExtraGapBars             int
	IterationSelectionMetric string
	TieRule                  string
	IterationCapLaw          string
}

type catBoostSpec1Payload struct {
	Logic                          LogicVersion
	Family                         string
	Loss                           string
	ClassCount                     int
	ClassOrder                     [3]TargetOutcome
	NumericOnly                    bool
	MissingValues                  string
	StandardScaler                 string
	ClassWeights                   string
	AutoClassWeights               string
	TaskType                       string
	GrowPolicy                     string
	BoostingType                   string
	BootstrapType                  string
	BaggingTemperature             float64
	BorderCount                    int
	FeatureBorderType              string
	LeafEstimationMethod           string
	LeafEstimationIterations       int
	LeafEstimationBacktracking     string
	RandomStrength                 float64
	RSM                            float64
	SamplingFrequency              string
	ScoreFunction                  string
	ModelShrinkRate                float64
	ModelShrinkMode                string
	ModelSizeReg                   float64
	MinDataInLeaf                  int
	PosteriorSampling              bool
	BoostFromAverage               bool
	UseBestModel                   bool
	RandomScoreType                string
	BayesianMatrixReg              float64
	PenaltiesCoefficient           float64
	BestModelMinTrees              int
	EvalFraction                   float64
	ForceUnitAutoPairWeights       bool
	SparseFeaturesConflictFraction float64
	Depth                          int
	LearningRate                   float64
	L2LeafReg                      float64
	MaxIterations                  int
	RandomSeed                     int
	ThreadCount                    int
	InnerSpanBars                  int
	MinInnerTrainRows              int
	ExtraGapBars                   int
	IterationSelectionMetric       string
	TieRule                        string
	IterationCapLaw                string
}

// PinnedCatBoostSpec1 is the boring V1 CatBoost hypothesis. Not tuned from OOF metrics.
func PinnedCatBoostSpec1() CatBoostSpec1 {
	return CatBoostSpec1{
		Logic: CatBoostLogicV1, Family: CatBoostFamily, Loss: CatBoostLossMultiClass,
		ClassCount: CatBoostSpecClassCount, ClassOrder: OOFClassOrder,
		NumericOnly: true, MissingValues: CatBoostNanForbidden, StandardScaler: "NONE",
		ClassWeights: "NONE", AutoClassWeights: "None",
		TaskType: CatBoostTaskCPU, GrowPolicy: CatBoostGrowSymmetric,
		BoostingType: "Plain", BootstrapType: "Bayesian", BaggingTemperature: 1,
		BorderCount: CatBoostSpecBorderCount, FeatureBorderType: "GreedyLogSum",
		LeafEstimationMethod: "Newton", LeafEstimationIterations: 1,
		LeafEstimationBacktracking: "AnyImprovement",
		RandomStrength:             1, RSM: 1, SamplingFrequency: "PerTree", ScoreFunction: "Cosine",
		ModelShrinkRate: 0, ModelShrinkMode: "Constant", ModelSizeReg: 0.5, MinDataInLeaf: 1,
		PosteriorSampling: false, BoostFromAverage: false, UseBestModel: false,
		RandomScoreType: "NormalWithModelSizeDecrease", BayesianMatrixReg: 0.1, PenaltiesCoefficient: 1,
		BestModelMinTrees: 1, EvalFraction: 0, ForceUnitAutoPairWeights: false, SparseFeaturesConflictFraction: 0,
		Depth: CatBoostSpecDepth, LearningRate: CatBoostSpecLearningRate, L2LeafReg: CatBoostSpecL2LeafReg,
		MaxIterations: CatBoostSpecMaxIterations, RandomSeed: CatBoostSpecRandomSeed, ThreadCount: CatBoostSpecThreadCount,
		InnerSpanBars: CatBoostSpecInnerSpanBars, MinInnerTrainRows: CatBoostSpecMinInnerTrain,
		ExtraGapBars:             CatBoostSpecExtraGapBars,
		IterationSelectionMetric: CatBoostSelLogLoss, TieRule: CatBoostTieSmallestN, IterationCapLaw: CatBoostCapRefuse,
	}
}

func (s CatBoostSpec1) Validate() error {
	if s.Logic != CatBoostLogicV1 || s.Family != CatBoostFamily || s.Loss != CatBoostLossMultiClass {
		return fmt.Errorf("forecast: CatBoostSpec1 identity fields")
	}
	if s.ClassCount != 3 || s.ClassOrder != OOFClassOrder {
		return fmt.Errorf("forecast: CLASS_ORDER_MISMATCH")
	}
	if !s.NumericOnly || s.MissingValues != CatBoostNanForbidden || s.StandardScaler != "NONE" {
		return fmt.Errorf("forecast: CatBoostSpec1 numeric/finite contract")
	}
	if s.ClassWeights != "NONE" || s.UseBestModel || s.TaskType != CatBoostTaskCPU || s.GrowPolicy != CatBoostGrowSymmetric {
		return fmt.Errorf("forecast: CatBoostSpec1 forbidden trainer knobs")
	}
	if s.ThreadCount != 1 || s.InnerSpanBars <= 0 || s.MinInnerTrainRows <= 0 {
		return fmt.Errorf("forecast: CatBoostSpec1 inner-tail/thread pins")
	}
	if s.EvalFraction != 0 || s.ForceUnitAutoPairWeights || s.SparseFeaturesConflictFraction != 0 || s.BestModelMinTrees != 1 {
		return fmt.Errorf("forecast: CatBoostSpec1 eval-split/pair pins")
	}
	if s.MaxIterations <= 1 || s.Depth <= 0 || s.LearningRate <= 0 || math.IsNaN(s.LearningRate) {
		return fmt.Errorf("forecast: CatBoostSpec1 tree pins")
	}
	if s.IterationSelectionMetric != CatBoostSelLogLoss || s.TieRule != CatBoostTieSmallestN || s.IterationCapLaw != CatBoostCapRefuse {
		return fmt.Errorf("forecast: CatBoostSpec1 selection law")
	}
	return nil
}

func (s CatBoostSpec1) Identity() (Identity, error) {
	if err := s.Validate(); err != nil {
		return Identity{}, err
	}
	return NewIdentity("catboost-spec1", catBoostSpec1Payload(s), s.Logic)
}

// EncodeOOFClass maps symbolic outcomes onto CatBoost integer classes.
func EncodeOOFClass(o TargetOutcome) (int, error) {
	switch o {
	case OutcomeUpFirst:
		return 0, nil
	case OutcomeDownFirst:
		return 1, nil
	case OutcomeTimeout:
		return 2, nil
	default:
		return -1, fmt.Errorf("forecast: CLASS_ORDER_MISMATCH outcome %q", o)
	}
}

func DecodeOOFClass(code int) (TargetOutcome, error) {
	if code < 0 || code >= len(OOFClassOrder) {
		return "", fmt.Errorf("forecast: CLASS_ORDER_MISMATCH code %d", code)
	}
	return OOFClassOrder[code], nil
}
