package brain3

import (
	"encoding/json"
	"fmt"
)

// Validation is compiled outer-fold membership from a certified validation artifact.
type Validation struct {
	HoldoutStartAt    int64
	HoldoutBeginIndex int
	PlanHex           string
	SplitHex          string
	Folds             []CompiledFold
}

// CompiledFold is the frozen index range into the DEV trainable At series.
type CompiledFold struct {
	TrainBegin, TrainEnd int
	ValBegin, ValEnd     int
	TrainFirstAt         int64
	TrainLastAt          int64
	ValFirstAt           int64
	ValLastAt            int64
}

type validationFile struct {
	DataSplitDigest      string `json:"data_split_digest"`
	ValidationPlanDigest string `json:"validation_plan_digest"`
	Compiled             struct {
		HoldoutStartAt    int64 `json:"HoldoutStartAt"`
		HoldoutBeginIndex int   `json:"HoldoutBeginIndex"`
		Folds             []struct {
			TrainBegin   int   `json:"TrainBegin"`
			TrainEnd     int   `json:"TrainEnd"`
			ValBegin     int   `json:"ValBegin"`
			ValEnd       int   `json:"ValEnd"`
			TrainFirstAt int64 `json:"TrainFirstAt"`
			TrainLastAt  int64 `json:"TrainLastAt"`
			ValFirstAt   int64 `json:"ValFirstAt"`
			ValLastAt    int64 `json:"ValLastAt"`
		} `json:"Folds"`
	} `json:"compiled"`
	DataSplit struct {
		HoldoutStartAt int64 `json:"HoldoutStartAt"`
	} `json:"data_split"`
}

func LoadValidation(raw []byte) (Validation, error) {
	var z Validation
	var f validationFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return z, fmt.Errorf("brain3: validation json: %w", err)
	}
	wall := f.Compiled.HoldoutStartAt
	if wall == 0 {
		wall = f.DataSplit.HoldoutStartAt
	}
	if wall <= 0 || len(f.Compiled.Folds) == 0 {
		return z, fmt.Errorf("brain3: validation compiled folds required")
	}
	z.HoldoutStartAt = wall
	z.HoldoutBeginIndex = f.Compiled.HoldoutBeginIndex
	z.PlanHex = f.ValidationPlanDigest
	z.SplitHex = f.DataSplitDigest
	z.Folds = make([]CompiledFold, len(f.Compiled.Folds))
	for i, cf := range f.Compiled.Folds {
		if cf.TrainEnd <= cf.TrainBegin || cf.ValEnd <= cf.ValBegin {
			return Validation{}, fmt.Errorf("brain3: empty fold %d", i)
		}
		if cf.TrainBegin < 0 || cf.ValBegin < 0 {
			return Validation{}, fmt.Errorf("brain3: negative fold %d index", i)
		}
		z.Folds[i] = CompiledFold{
			TrainBegin: cf.TrainBegin, TrainEnd: cf.TrainEnd,
			ValBegin: cf.ValBegin, ValEnd: cf.ValEnd,
			TrainFirstAt: cf.TrainFirstAt, TrainLastAt: cf.TrainLastAt,
			ValFirstAt: cf.ValFirstAt, ValLastAt: cf.ValLastAt,
		}
	}
	return z, nil
}
