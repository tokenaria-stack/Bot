package brain3

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"trading_bot/ml"
)

const researchLogitTol = 1e-4

type OOFRow struct {
	At    int64      `json:"at"`
	Fold  int        `json:"fold"`
	Y     int        `json:"y"`
	Logit [3]float64 `json:"logits"`
}

type FoldResult struct {
	Index                 int
	OuterTrainN           int
	OuterValN             int
	InnerTrainN           int
	InnerValN             int
	SelectedN             int
	InnerMinLoss          float64
	OuterCBLoss           float64
	OuterPriorLoss        float64
	Delta                 float64
	ValTP, ValStop, ValTO int
}

type RunResult struct {
	SpecDigest     string
	OOFDigest      string
	Folds          []FoldResult
	PooledCB       float64
	PooledPrior    float64
	OOFRows        int
	MaxOOFAt       int64
	HoldoutStartAt int64
	CatBoostVer    string
	PythonVer      string
	SelectedN      []int
	Text           string
	Rows           []OOFRow
}

func sliceXY(ds Dataset, begin, end int) (xs [][]float64, ys []int) {
	n := end - begin
	xs = make([][]float64, n)
	ys = make([]int, n)
	for i := 0; i < n; i++ {
		xs[i] = ds.X[begin+i]
		ys[i] = ds.Y[begin+i]
	}
	return xs, ys
}

func classPrior(ys []int) [3]float64 {
	var c [3]float64
	for _, y := range ys {
		if y >= 0 && y < 3 {
			c[y]++
		}
	}
	n := float64(len(ys))
	if n == 0 {
		return c
	}
	return [3]float64{c[0] / n, c[1] / n, c[2] / n}
}

func priorLoss(ys []int, p [3]float64) (float64, error) {
	if len(ys) == 0 {
		return 0, fmt.Errorf("brain3: empty prior population")
	}
	var s float64
	for _, y := range ys {
		if y < 0 || y > 2 || p[y] <= 0 {
			return 0, fmt.Errorf("brain3: prior P[%d]=%v", y, p[y])
		}
		s += -math.Log(p[y])
	}
	return s / float64(len(ys)), nil
}

func meanLogLoss(model ml.Model, xs [][]float64, ys []int) (float64, error) {
	var s float64
	for i, x := range xs {
		z, err := model.Logits(x)
		if err != nil {
			return 0, err
		}
		v, err := ml.MulticlassLogLoss(z, ys[i])
		if err != nil {
			return 0, err
		}
		s += v
	}
	return s / float64(len(xs)), nil
}

func writeJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func pythonCmd() string {
	return defaultPython(".")
}

func defaultPython(repo string) string {
	if p := os.Getenv("PYTHON"); p != "" {
		return p
	}
	cand := filepath.Join(repo, "research", "brain3", ".venv", "bin", "python")
	if st, err := os.Stat(cand); err == nil && !st.IsDir() {
		return cand
	}
	cand = filepath.Join(repo, "research", "modelfit", ".venv", "bin", "python")
	if st, err := os.Stat(cand); err == nil && !st.IsDir() {
		return cand
	}
	return "python3"
}

func runPython(python, repo, script, verb, plan string) error {
	cmd := exec.Command(python, script, verb, plan)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PYTHONHASHSEED=0")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brain3: python %s: %w", verb, err)
	}
	return nil
}

func fitVendor(python, repo, work, tag string, xs [][]float64, ys []int, names []string, iterations int, params map[string]any) (ml.Model, string, error) {
	dir := filepath.Join(work, tag)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ml.Model{}, "", err
	}
	xPath := filepath.Join(dir, "x.json")
	yPath := filepath.Join(dir, "y.json")
	outJSON := filepath.Join(dir, "model.json")
	wit := filepath.Join(dir, "witness.json")
	plan := filepath.Join(dir, "plan.json")
	if err := writeJSON(xPath, xs); err != nil {
		return ml.Model{}, "", err
	}
	if err := writeJSON(yPath, ys); err != nil {
		return ml.Model{}, "", err
	}
	rec := map[string]any{
		"x_path": xPath, "y_path": yPath, "iterations": iterations,
		"catboost_params": params, "feature_names": names,
		"out_json": outJSON, "out_witness": wit,
	}
	if err := writeJSON(plan, rec); err != nil {
		return ml.Model{}, "", err
	}
	script := filepath.Join(repo, "research", "brain3", "fit.py")
	if err := runPython(python, repo, script, "fit", plan); err != nil {
		return ml.Model{}, "", err
	}
	raw, err := os.ReadFile(outJSON)
	if err != nil {
		return ml.Model{}, "", err
	}
	m, err := ml.ConvertVendor(raw, names, 3, Spec1MaxIterations, Spec1Depth)
	if err != nil {
		return ml.Model{}, "", err
	}
	if err := m.Validate(names, 3, Spec1MaxIterations, Spec1Depth); err != nil {
		return ml.Model{}, "", err
	}
	if len(m.Trees) != iterations {
		return ml.Model{}, "", fmt.Errorf("brain3: tree count %d != %d", len(m.Trees), iterations)
	}
	return m, wit, nil
}

func checkPortable(python, repo, work, tag string, modelJSON string, xs [][]float64, ys []int, names []string, goModel ml.Model) error {
	n := len(xs)
	if n > 32 {
		n = 32
	}
	dir := filepath.Join(work, tag+"_chk")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	xPath := filepath.Join(dir, "x.json")
	yPath := filepath.Join(dir, "y.json")
	predPath := filepath.Join(dir, "pred.json")
	plan := filepath.Join(dir, "plan.json")
	if err := writeJSON(xPath, xs[:n]); err != nil {
		return err
	}
	if err := writeJSON(yPath, ys[:n]); err != nil {
		return err
	}
	if err := writeJSON(plan, map[string]any{
		"x_path": xPath, "y_path": yPath, "model_json": modelJSON, "out_pred": predPath,
		"feature_names": names,
	}); err != nil {
		return err
	}
	script := filepath.Join(repo, "research", "brain3", "fit.py")
	if err := runPython(python, repo, script, "predict", plan); err != nil {
		return err
	}
	var py [][]float64
	raw, err := os.ReadFile(predPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &py); err != nil {
		return err
	}
	if len(py) != n {
		return fmt.Errorf("brain3: PORTABLE_MODEL_MISMATCH pred n")
	}
	for i := 0; i < n; i++ {
		z, err := goModel.Logits(xs[i])
		if err != nil {
			return err
		}
		if len(py[i]) != 3 {
			return fmt.Errorf("brain3: PORTABLE_MODEL_MISMATCH width")
		}
		for c := 0; c < 3; c++ {
			if math.Abs(z[c]-py[i][c]) > researchLogitTol {
				return fmt.Errorf("brain3: PORTABLE_MODEL_MISMATCH row %d class %d go=%v py=%v", i, c, z[c], py[i][c])
			}
		}
	}
	return nil
}

func hashOOF(header []byte, rows []OOFRow) string {
	h := sha256.New()
	h.Write(header)
	var buf [8]byte
	for _, r := range rows {
		binary.LittleEndian.PutUint64(buf[:], uint64(r.At))
		h.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], uint64(r.Fold))
		h.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], uint64(r.Y))
		h.Write(buf[:])
		for c := 0; c < 3; c++ {
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(r.Logit[c]))
			h.Write(buf[:])
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func pythonVersion(python string) string {
	cmd := exec.Command(python, "-c", "import sys,catboost; print(sys.version.split()[0]+' catboost='+catboost.__version__)")
	b, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

// RunMetalabel1 fits Spec1 causally and returns official portable OOF logits.
func RunMetalabel1(ds Dataset, val Validation, spec MetaLabelCatBoostSpec1, python, repo, work string) (RunResult, error) {
	var z RunResult
	if err := spec.validate(); err != nil {
		return z, err
	}
	census, err := ResolveOuterFolds(ds, val)
	if err != nil {
		return z, err
	}
	inner, err := PreflightInnerGeometry(ds, census, spec)
	if err != nil {
		return z, err
	}
	specHex, err := spec.DigestHex(ds, val)
	if err != nil {
		return z, err
	}
	z.SpecDigest = specHex
	z.HoldoutStartAt = val.HoldoutStartAt
	z.PythonVer = pythonVersion(python)
	params := ResearchCatBoostParams(spec)
	names := ds.FeatureNames
	var oof []OOFRow
	var foldRes []FoldResult
	var pooledCB, pooledPrior float64
	var pooledN int
	selected := make([]int, len(inner))
	for i, g := range inner {
		f := census.Folds[i]
		innerX, innerY := sliceXY(ds, f.TrainBegin+g.Split.InnerTrainBegin, f.TrainBegin+g.Split.InnerTrainEnd)
		valX, valY := sliceXY(ds, f.TrainBegin+g.Split.InnerValBegin, f.TrainBegin+g.Split.InnerValEnd)
		innerM, _, err := fitVendor(python, repo, work, fmt.Sprintf("fold%d_inner", i), innerX, innerY, names, spec.MaxIterations, params)
		if err != nil {
			return z, err
		}
		modelJSON := filepath.Join(work, fmt.Sprintf("fold%d_inner", i), "model.json")
		if err := checkPortable(python, repo, work, fmt.Sprintf("fold%d_inner", i), modelJSON, valX, valY, names, innerM); err != nil {
			return z, err
		}
		curve, err := ml.PrefixMeanLogLoss(innerM, valX, valY)
		if err != nil {
			return z, err
		}
		nStar, err := ml.SelectTreeCount(curve, spec.MaxIterations)
		if err != nil {
			if strings.Contains(err.Error(), "ITERATION_CAP_REACHED") {
				return z, fmt.Errorf("brain3: ITERATION_CAP_REACHED fold %d", i)
			}
			return z, err
		}
		selected[i] = nStar
		outerX, outerY := sliceXY(ds, f.TrainBegin, f.TrainEnd)
		outerM, _, err := fitVendor(python, repo, work, fmt.Sprintf("fold%d_outer", i), outerX, outerY, names, nStar, params)
		if err != nil {
			return z, err
		}
		outerJSON := filepath.Join(work, fmt.Sprintf("fold%d_outer", i), "model.json")
		if err := checkPortable(python, repo, work, fmt.Sprintf("fold%d_outer", i), outerJSON, outerX, outerY, names, outerM); err != nil {
			return z, err
		}
		oofX, oofY := sliceXY(ds, f.ValBegin, f.ValEnd)
		cbLoss, err := meanLogLoss(outerM, oofX, oofY)
		if err != nil {
			return z, err
		}
		pr := classPrior(outerY)
		pLoss, err := priorLoss(oofY, pr)
		if err != nil {
			return z, err
		}
		tp, st, to, err := countClasses(ds.Y, f.ValBegin, f.ValEnd)
		if err != nil {
			return z, err
		}
		fr := FoldResult{
			Index: i, OuterTrainN: f.TrainN, OuterValN: f.ValN,
			InnerTrainN: g.InnerTrainN, InnerValN: g.InnerValN,
			SelectedN: nStar, InnerMinLoss: curve[nStar-1],
			OuterCBLoss: cbLoss, OuterPriorLoss: pLoss, Delta: pLoss - cbLoss,
			ValTP: tp, ValStop: st, ValTO: to,
		}
		foldRes = append(foldRes, fr)
		for j := 0; j < len(oofX); j++ {
			at := ds.At[f.ValBegin+j]
			if at >= val.HoldoutStartAt {
				return z, fmt.Errorf("brain3: holdout At in OOF %d", at)
			}
			zlog, err := outerM.Logits(oofX[j])
			if err != nil {
				return z, err
			}
			oof = append(oof, OOFRow{At: at, Fold: i, Y: oofY[j], Logit: zlog})
			if at > z.MaxOOFAt {
				z.MaxOOFAt = at
			}
		}
		pooledCB += cbLoss * float64(len(oofY))
		pooledPrior += pLoss * float64(len(oofY))
		pooledN += len(oofY)
	}
	if pooledN != census.ValTotal {
		return z, fmt.Errorf("brain3: OOF n=%d != frozen val %d", pooledN, census.ValTotal)
	}
	want := map[int64]int{}
	for i, f := range census.Folds {
		for j := f.ValBegin; j < f.ValEnd; j++ {
			want[ds.At[j]] = i
		}
	}
	seen := map[int64]struct{}{}
	for i, r := range oof {
		fold, ok := want[r.At]
		if !ok {
			return z, fmt.Errorf("brain3: extra OOF At %d", r.At)
		}
		if fold != r.Fold {
			return z, fmt.Errorf("brain3: OOF At %d fold %d != frozen %d", r.At, r.Fold, fold)
		}
		if _, ok := seen[r.At]; ok {
			return z, fmt.Errorf("brain3: duplicate OOF At %d", r.At)
		}
		seen[r.At] = struct{}{}
		if i > 0 && r.At <= oof[i-1].At {
			return z, fmt.Errorf("brain3: OOF At not increasing")
		}
	}
	if len(seen) != len(want) {
		return z, fmt.Errorf("brain3: OOF At set %d != frozen validation At set %d", len(seen), len(want))
	}
	hdr, _ := json.Marshal(struct {
		Format, Spec, Dataset, Plan, Split string
		Schema                             []string
		Width                              int
	}{OOFFormatV1, specHex, ds.ContentHex, val.PlanHex, val.SplitHex, ds.ClassSchema, ds.Width})
	z.Folds = foldRes
	z.Rows = oof
	z.OOFRows = len(oof)
	z.OOFDigest = hashOOF(hdr, oof)
	z.PooledCB = pooledCB / float64(pooledN)
	z.PooledPrior = pooledPrior / float64(pooledN)
	z.SelectedN = selected
	z.CatBoostVer = z.PythonVer
	z.Text = FormatMetalabelReport(z, ds, val, specHex, inner)
	return z, nil
}

func FormatMetalabelReport(z RunResult, ds Dataset, val Validation, specHex string, inner []InnerGeometry) string {
	var b strings.Builder
	b.WriteString("BRAIN3-METALABEL-1 (one archive OOF; no strategy)\n")
	b.WriteString(fmt.Sprintf("spec=%s\ndataset=%s\nvalidation=%s\nsplit=%s\noof_digest=%s\n",
		specHex, ds.ContentHex, val.PlanHex, val.SplitHex, z.OOFDigest))
	b.WriteString(fmt.Sprintf("width=%d class_schema=%v oof_rows=%d max_oof_at=%d holdout=%d\n",
		ds.Width, ds.ClassSchema, z.OOFRows, z.MaxOOFAt, z.HoldoutStartAt))
	b.WriteString(fmt.Sprintf("python=%s\n", z.PythonVer))
	b.WriteString(FormatInnerPreflight(inner))
	for _, f := range z.Folds {
		b.WriteString(fmt.Sprintf("fold %d  N*=%d  inner_logloss=%.6f  oof_cb=%.6f  oof_prior=%.6f  delta=%.6f  val TP/STOP/TO=%d/%d/%d\n",
			f.Index, f.SelectedN, f.InnerMinLoss, f.OuterCBLoss, f.OuterPriorLoss, f.Delta, f.ValTP, f.ValStop, f.ValTO))
	}
	b.WriteString(fmt.Sprintf("pooled oof_cb=%.6f  pooled_prior=%.6f  delta=%.6f\n", z.PooledCB, z.PooledPrior, z.PooledPrior-z.PooledCB))
	b.WriteString("no 2026 / no coverage curves / no CatBoost retune\n")
	return b.String()
}

func WriteOOF(path string, z RunResult, ds Dataset, val Validation) error {
	type hdr struct {
		Kind         string   `json:"kind"`
		Format       string   `json:"format"`
		ClassSchema  []string `json:"class_schema"`
		Dataset      string   `json:"dataset_digest"`
		Validation   string   `json:"validation_plan_digest"`
		Split        string   `json:"data_split_digest"`
		Spec         string   `json:"model_spec_digest"`
		Content      string   `json:"content_digest"`
		Width        int      `json:"width"`
		Rows         int      `json:"row_count"`
		HoldoutStart int64    `json:"holdout_start_at"`
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	h := hdr{
		Kind: "header", Format: OOFFormatV1, ClassSchema: ds.ClassSchema,
		Dataset: ds.ContentHex, Validation: val.PlanHex, Split: val.SplitHex,
		Spec: z.SpecDigest, Content: z.OOFDigest, Width: ds.Width, Rows: z.OOFRows,
		HoldoutStart: val.HoldoutStartAt,
	}
	if err := enc.Encode(h); err != nil {
		return err
	}
	for _, r := range z.Rows {
		if err := enc.Encode(map[string]any{
			"kind": "row", "at": r.At, "fold": r.Fold, "y": r.Y,
			"logit0": r.Logit[0], "logit1": r.Logit[1], "logit2": r.Logit[2],
		}); err != nil {
			return err
		}
	}
	return enc.Encode(map[string]any{"kind": "footer", "row_count": z.OOFRows, "content_digest": z.OOFDigest})
}
