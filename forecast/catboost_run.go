package forecast

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type CatBoostRunResult struct {
	SpecDigest     Digest
	FitPlanDigest  Digest
	MatrixDigest   Digest
	SelectedN      []int
	InnerLoss      []float64
	PortableDigest []Digest
	LogitsDigest   Digest
	LogitsRows     int
	Matched        bool
}

func catBoostPythonParams(spec CatBoostSpec1) map[string]any {
	return map[string]any{
		"loss_function": spec.Loss, "eval_metric": spec.Loss, "classes_count": spec.ClassCount,
		"nan_mode": spec.MissingValues, "auto_class_weights": spec.AutoClassWeights,
		"task_type": spec.TaskType, "grow_policy": spec.GrowPolicy, "boosting_type": spec.BoostingType,
		"bootstrap_type": spec.BootstrapType, "bagging_temperature": spec.BaggingTemperature,
		"border_count": spec.BorderCount, "feature_border_type": spec.FeatureBorderType,
		"leaf_estimation_method":       spec.LeafEstimationMethod,
		"leaf_estimation_iterations":   spec.LeafEstimationIterations,
		"leaf_estimation_backtracking": spec.LeafEstimationBacktracking,
		"random_strength":              spec.RandomStrength, "rsm": spec.RSM,
		"sampling_frequency": spec.SamplingFrequency, "score_function": spec.ScoreFunction,
		"model_shrink_rate": spec.ModelShrinkRate, "model_shrink_mode": spec.ModelShrinkMode,
		"model_size_reg": spec.ModelSizeReg, "min_data_in_leaf": spec.MinDataInLeaf,
		"posterior_sampling": spec.PosteriorSampling, "boost_from_average": spec.BoostFromAverage,
		"use_best_model": false, "random_seed": spec.RandomSeed, "thread_count": spec.ThreadCount,
		"depth": spec.Depth, "learning_rate": spec.LearningRate, "l2_leaf_reg": spec.L2LeafReg,
		"penalties_coefficient": spec.PenaltiesCoefficient, "random_score_type": spec.RandomScoreType,
		"best_model_min_trees": spec.BestModelMinTrees, "eval_fraction": spec.EvalFraction,
		"sparse_features_conflict_fraction": spec.SparseFeaturesConflictFraction,
		"max_leaves":                        1 << spec.Depth,
		"allow_writing_files":               false, "verbose": false,
	}
}

func catBoostExpectAllParams(spec CatBoostSpec1, iterations int) map[string]any {
	return map[string]any{
		"loss_function": spec.Loss, "eval_metric": spec.Loss, "classes_count": spec.ClassCount,
		"nan_mode": spec.MissingValues, "auto_class_weights": spec.AutoClassWeights,
		"task_type": spec.TaskType, "grow_policy": spec.GrowPolicy, "boosting_type": spec.BoostingType,
		"bootstrap_type": spec.BootstrapType, "bagging_temperature": spec.BaggingTemperature,
		"border_count": spec.BorderCount, "feature_border_type": spec.FeatureBorderType,
		"leaf_estimation_method": spec.LeafEstimationMethod, "leaf_estimation_iterations": spec.LeafEstimationIterations,
		"leaf_estimation_backtracking": spec.LeafEstimationBacktracking, "random_strength": spec.RandomStrength,
		"rsm": spec.RSM, "sampling_frequency": spec.SamplingFrequency, "score_function": spec.ScoreFunction,
		"model_shrink_rate": spec.ModelShrinkRate, "model_shrink_mode": spec.ModelShrinkMode,
		"model_size_reg": spec.ModelSizeReg, "min_data_in_leaf": spec.MinDataInLeaf,
		"posterior_sampling": spec.PosteriorSampling, "boost_from_average": spec.BoostFromAverage,
		"use_best_model": false, "random_seed": spec.RandomSeed, "depth": spec.Depth,
		"l2_leaf_reg": spec.L2LeafReg, "penalties_coefficient": spec.PenaltiesCoefficient,
		"random_score_type": spec.RandomScoreType, "best_model_min_trees": spec.BestModelMinTrees,
		"eval_fraction": spec.EvalFraction, "force_unit_auto_pair_weights": spec.ForceUnitAutoPairWeights,
		"sparse_features_conflict_fraction": spec.SparseFeaturesConflictFraction,
		"max_leaves":                        1 << spec.Depth, "iterations": iterations,
		"bayesian_matrix_reg": spec.BayesianMatrixReg, "learning_rate": spec.LearningRate,
	}
}

type catBoostFitReceipt struct {
	MatrixPath      string         `json:"matrix_path"`
	ExpectMatrix    string         `json:"expect_matrix"`
	FeatureIDs      []string       `json:"feature_ids"`
	RowIndexes      []int          `json:"row_indexes"`
	Y               []int          `json:"y"`
	Iterations      int            `json:"iterations"`
	CatBoostParams  map[string]any `json:"catboost_params"`
	ExpectAllParams map[string]any `json:"expect_all_params"`
	OutJSON         string         `json:"out_json"`
	OutCBM          string         `json:"out_cbm"`
	OutWitness      string         `json:"out_witness"`
}

func writeFitReceipt(path string, rec catBoostFitReceipt) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func pythonFit(python, repo, planPath string) error {
	cmd := exec.Command(python, "-m", "research.catboost1", "fit", planPath)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PYTHONHASHSEED=0")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("forecast: catboost python fit: %w", err)
	}
	return nil
}

func sliceIndexes(begin, end int, rows []OOFRow) (idx []int, y []int, err error) {
	if begin < 0 || end > len(rows) || begin >= end {
		return nil, nil, fmt.Errorf("forecast: FITPLAN_INVALID slice")
	}
	idx = make([]int, 0, end-begin)
	y = make([]int, 0, end-begin)
	for i := begin; i < end; i++ {
		c, err := EncodeOOFClass(rows[i].Outcome)
		if err != nil {
			return nil, nil, err
		}
		idx = append(idx, i)
		y = append(y, c)
	}
	return idx, y, nil
}

func featureIDStrings(ids []FeatureID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func convertVendorFile(path string, ids []FeatureID, spec CatBoostSpec1) (PortableCatBoost, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PortableCatBoost{}, err
	}
	return ConvertVendorCatBoostJSON(raw, ids, spec.ClassCount, spec.MaxIterations, spec.Depth)
}

func matrixRowsXY(rows []OOFRow, begin, end int) (xs [][]float64, ys []int, err error) {
	idx, y, err := sliceIndexes(begin, end, rows)
	if err != nil {
		return nil, nil, err
	}
	xs = make([][]float64, len(idx))
	for i, ri := range idx {
		xs[i] = rows[ri].Features
	}
	return xs, y, nil
}

// RunCatBoostBrain1 executes Gates A–C for one matrix+spec. Official logits are Go(portable).
func RunCatBoostBrain1(matrixPath, expectHex, python, repo, workDir, logitsPath string, spec CatBoostSpec1) (CatBoostRunResult, error) {
	var z CatBoostRunResult
	if err := spec.Validate(); err != nil {
		return z, err
	}
	sid, err := spec.Identity()
	if err != nil {
		return z, err
	}
	expect, err := ParseDigestHex(expectHex)
	if err != nil {
		return z, err
	}
	hdr, rows, ft, err := ReadOOFMatrix(matrixPath)
	if err != nil {
		return z, err
	}
	if ft.ContentDigest != expect {
		return z, fmt.Errorf("forecast: MATRIX_IDENTITY_MISMATCH")
	}
	plan, err := ResolveCatBoostFitPlan(hdr, rows, ft.ContentDigest, spec)
	if err != nil {
		return z, err
	}
	planD := plan.Digest()
	z.SpecDigest = sid.Digest
	z.FitPlanDigest = planD
	z.MatrixDigest = ft.ContentDigest
	matched, err := tryMatchCatBoostOfficial(&z, logitsPath, ft.ContentDigest, sid.Digest, planD)
	if err != nil {
		return z, err
	}
	if matched {
		return z, nil
	}

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return z, err
	}
	ids := hdr.FeatureIDs
	params := catBoostPythonParams(spec)
	selected := make([]int, len(plan.Folds))
	innerLoss := make([]float64, len(plan.Folds))
	portables := make([]PortableCatBoost, len(plan.Folds))
	portDig := make([]Digest, len(plan.Folds))

	for i, fold := range plan.Folds {
		foldDir := filepath.Join(workDir, fmt.Sprintf("fold%d", i))
		if err := os.MkdirAll(foldDir, 0o755); err != nil {
			return z, err
		}
		innerIdx, innerY, err := sliceIndexes(fold.InnerTrainBegin, fold.InnerTrainEnd, rows)
		if err != nil {
			return z, err
		}
		innerJSON := filepath.Join(foldDir, "inner.json")
		innerPlan := filepath.Join(foldDir, "inner_plan.json")
		if err := writeFitReceipt(innerPlan, catBoostFitReceipt{
			MatrixPath: matrixPath, ExpectMatrix: expectHex, FeatureIDs: featureIDStrings(ids),
			RowIndexes: innerIdx, Y: innerY, Iterations: spec.MaxIterations, CatBoostParams: params,
			ExpectAllParams: catBoostExpectAllParams(spec, spec.MaxIterations),
			OutJSON:         innerJSON, OutCBM: filepath.Join(foldDir, "inner.cbm"), OutWitness: filepath.Join(foldDir, "inner_witness.json"),
		}); err != nil {
			return z, err
		}
		if err := pythonFit(python, repo, innerPlan); err != nil {
			return z, err
		}
		innerM, err := convertVendorFile(innerJSON, ids, spec)
		if err != nil {
			return z, err
		}
		if err := innerM.Validate(ids, spec.ClassCount, spec.MaxIterations, spec.Depth); err != nil {
			return z, err
		}
		if len(innerM.Trees) != spec.MaxIterations {
			return z, fmt.Errorf("forecast: inner tree count %d != MaxIterations", len(innerM.Trees))
		}
		valX, valY, err := matrixRowsXY(rows, fold.InnerValBegin, fold.InnerValEnd)
		if err != nil {
			return z, err
		}
		curve, err := PrefixMeanLogLoss(innerM, valX, valY)
		if err != nil {
			return z, err
		}
		n, err := SelectTreeCount(curve, spec.MaxIterations)
		if err != nil {
			return z, err
		}
		selected[i] = n
		innerLoss[i] = curve[n-1]
		outerIdx, outerY, err := sliceIndexes(fold.OuterTrainBegin, fold.OuterTrainEnd, rows)
		if err != nil {
			return z, err
		}
		outerJSON := filepath.Join(foldDir, "outer.json")
		outerPlan := filepath.Join(foldDir, "outer_plan.json")
		if err := writeFitReceipt(outerPlan, catBoostFitReceipt{
			MatrixPath: matrixPath, ExpectMatrix: expectHex, FeatureIDs: featureIDStrings(ids),
			RowIndexes: outerIdx, Y: outerY, Iterations: n, CatBoostParams: params,
			ExpectAllParams: catBoostExpectAllParams(spec, n),
			OutJSON:         outerJSON, OutCBM: filepath.Join(foldDir, "outer.cbm"), OutWitness: filepath.Join(foldDir, "outer_witness.json"),
		}); err != nil {
			return z, err
		}
		if err := pythonFit(python, repo, outerPlan); err != nil {
			return z, err
		}
		outerM, err := convertVendorFile(outerJSON, ids, spec)
		if err != nil {
			return z, err
		}
		if err := outerM.Validate(ids, spec.ClassCount, spec.MaxIterations, spec.Depth); err != nil {
			return z, err
		}
		if len(outerM.Trees) != n {
			return z, fmt.Errorf("forecast: outer tree count %d != N %d", len(outerM.Trees), n)
		}
		portables[i] = outerM
		portDig[i] = outerM.ContentDigest()
	}

	var outRows []CatBoostOOFRow
	var outFolds []CatBoostOOFFold
	cursor := 0
	for i, fold := range plan.Folds {
		nval := fold.OuterValEnd - fold.OuterValBegin
		outFolds = append(outFolds, CatBoostOOFFold{
			SourceTrainBegin: fold.OuterTrainBegin, SourceTrainEnd: fold.OuterTrainEnd,
			SourceValBegin: fold.OuterValBegin, SourceValEnd: fold.OuterValEnd,
			OutputBegin: cursor, OutputEnd: cursor + nval, SelectedN: selected[i], PortableDigest: portDig[i],
		})
		for ri := fold.OuterValBegin; ri < fold.OuterValEnd; ri++ {
			zrow, err := portables[i].Logits(rows[ri].Features)
			if err != nil {
				return z, err
			}
			outRows = append(outRows, CatBoostOOFRow{At: rows[ri].At, Outcome: rows[ri].Outcome, Logits: zrow})
		}
		cursor += nval
	}
	for i := 1; i < len(outRows); i++ {
		if outRows[i].At <= outRows[i-1].At {
			return z, fmt.Errorf("forecast: OOF logits At not increasing")
		}
	}
	oh := CatBoostOOFHeader{
		FormatVersion: CatBoostOOFLogitsFormatV1, AtUnit: OOFAtUnitUnixMs, Market: hdr.Market,
		FeatureIDs: append([]FeatureID(nil), hdr.FeatureIDs...), MatrixDigest: ft.ContentDigest,
		SpecDigest: sid.Digest, FitPlanDigest: planD, ClassOrder: OOFClassOrder, Folds: outFolds,
	}
	if err := publishCatBoostOfficial(logitsPath, oh, outRows, portables, selected, innerLoss, sid.Digest, planD, ft.ContentDigest); err != nil {
		return z, err
	}
	_, rowsOut, ftL, err := ReadCatBoostOOFLogits(logitsPath)
	if err != nil {
		return z, err
	}
	_ = rowsOut
	z.SelectedN = selected
	z.InnerLoss = innerLoss
	z.PortableDigest = portDig
	z.LogitsDigest = ftL.ContentDigest
	z.LogitsRows = ftL.RowCount
	return z, nil
}

func officialCatBoostPaths(logitsPath string, nFolds int) []string {
	out := []string{logitsPath, CatBoostManifestPath(logitsPath)}
	for i := 0; i < nFolds; i++ {
		out = append(out, CatBoostPortablePath(logitsPath, i))
	}
	return out
}

func anyOfficialCatBoostPresent(logitsPath string, nFolds int) (bool, error) {
	for _, p := range officialCatBoostPaths(logitsPath, nFolds) {
		_, err := os.Stat(p)
		if err == nil {
			return true, nil
		}
		if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

func tryMatchCatBoostOfficial(z *CatBoostRunResult, logitsPath string, matrix, spec, plan Digest) (bool, error) {
	present, err := anyOfficialCatBoostPresent(logitsPath, 4)
	if err != nil {
		return false, err
	}
	if !present {
		return false, nil
	}
	gotH, _, gotF, err := ReadCatBoostOOFLogits(logitsPath)
	if err != nil {
		return false, fmt.Errorf("forecast: ATOMIC_PUBLICATION_FAILED incomplete or corrupt oof-logits: %w", err)
	}
	if gotH.MatrixDigest != matrix || gotH.SpecDigest != spec || gotH.FitPlanDigest != plan || gotH.FormatVersion != CatBoostOOFLogitsFormatV1 {
		return false, fmt.Errorf("forecast: refuse overwrite of different catboost oof-logits %s", logitsPath)
	}
	man, err := ReadCatBoostResultManifest(CatBoostManifestPath(logitsPath))
	if err != nil {
		return false, fmt.Errorf("forecast: ATOMIC_PUBLICATION_FAILED incomplete result manifest: %w", err)
	}
	if man.MatrixDigest != matrix || man.SpecDigest != spec || man.FitPlanDigest != plan || man.LogitsDigest != gotF.ContentDigest {
		return false, fmt.Errorf("forecast: refuse overwrite of different catboost result %s", logitsPath)
	}
	if len(man.Folds) != len(gotH.Folds) {
		return false, fmt.Errorf("forecast: ATOMIC_PUBLICATION_FAILED fold count")
	}
	z.LogitsDigest = gotF.ContentDigest
	z.LogitsRows = gotF.RowCount
	z.Matched = true
	z.SelectedN = make([]int, len(gotH.Folds))
	z.PortableDigest = make([]Digest, len(gotH.Folds))
	z.InnerLoss = make([]float64, len(gotH.Folds))
	for i, f := range gotH.Folds {
		pcb, err := ReadPortableCatBoost(CatBoostPortablePath(logitsPath, i))
		if err != nil {
			return false, fmt.Errorf("forecast: ATOMIC_PUBLICATION_FAILED incomplete portable fold %d: %w", i, err)
		}
		if pcb.ContentDigest() != f.PortableDigest || pcb.ContentDigest() != man.Folds[i].PortableDigest {
			return false, fmt.Errorf("forecast: refuse portable digest mismatch fold %d", i)
		}
		if len(pcb.Trees) != f.SelectedN || f.SelectedN != man.Folds[i].SelectedN {
			return false, fmt.Errorf("forecast: refuse selected N mismatch fold %d", i)
		}
		z.SelectedN[i] = f.SelectedN
		z.PortableDigest[i] = f.PortableDigest
		z.InnerLoss[i] = man.Folds[i].InnerGoLogLoss
	}
	return true, nil
}

func publishCatBoostOfficial(logitsPath string, hdr CatBoostOOFHeader, rows []CatBoostOOFRow, models []PortableCatBoost, selected []int, innerLoss []float64, spec, plan, matrix Digest) error {
	if len(models) != len(selected) || len(models) != len(innerLoss) || len(models) != len(hdr.Folds) {
		return fmt.Errorf("forecast: ATOMIC_PUBLICATION_FAILED fold alignment")
	}
	present, err := anyOfficialCatBoostPresent(logitsPath, len(models))
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("forecast: ATOMIC_PUBLICATION_FAILED official slot already occupied")
	}
	staging := logitsPath + ".publish.tmp"
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	stageLogits := filepath.Join(staging, filepath.Base(logitsPath))
	ftL, err := WriteCatBoostOOFLogits(stageLogits, hdr, rows)
	if err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	man := CatBoostResultManifest{
		Format: CatBoostResultFormatV1, MatrixDigest: matrix, SpecDigest: spec, FitPlanDigest: plan,
		LogitsDigest: ftL.ContentDigest, ClassOrder: OOFClassOrder,
	}
	for i := range models {
		man.Folds = append(man.Folds, CatBoostResultFold{
			SelectedN: selected[i], PortableDigest: models[i].ContentDigest(), InnerGoLogLoss: innerLoss[i],
		})
		if err := WritePortableCatBoost(filepath.Join(staging, filepath.Base(CatBoostPortablePath(logitsPath, i))), models[i]); err != nil {
			_ = os.RemoveAll(staging)
			return err
		}
	}
	if err := WriteCatBoostResultManifest(filepath.Join(staging, filepath.Base(CatBoostManifestPath(logitsPath))), man); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	commit := []struct{ from, to string }{
		{stageLogits, logitsPath},
		{filepath.Join(staging, filepath.Base(CatBoostManifestPath(logitsPath))), CatBoostManifestPath(logitsPath)},
	}
	for i := range models {
		commit = append(commit, struct{ from, to string }{
			filepath.Join(staging, filepath.Base(CatBoostPortablePath(logitsPath, i))), CatBoostPortablePath(logitsPath, i),
		})
	}
	for _, c := range commit {
		if err := os.Rename(c.from, c.to); err != nil {
			for _, d := range commit {
				_ = os.Remove(d.to)
			}
			_ = os.RemoveAll(staging)
			return fmt.Errorf("forecast: ATOMIC_PUBLICATION_FAILED: %w", err)
		}
	}
	_ = os.RemoveAll(staging)
	return nil
}
