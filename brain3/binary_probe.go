package brain3

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"trading_bot/ml"
)

type BinaryOOFRow struct {
	At        int64
	Fold      int
	YBinary   int
	RawMargin float64
	PTP       float64
}

type BinaryFoldResult struct {
	Index                     int
	ResTrainN, ResValN        int
	TrainTP, TrainStop        int
	ValTP, ValStop            int
	InnerTrainN, InnerValN    int
	SelectedN                 int
	InnerMinLoss              float64
	OOFLoss, PriorLoss, Delta float64
}

type BinaryProbeResult struct {
	SpecDigest     string
	OOFDigest      string
	Folds          []BinaryFoldResult
	PooledCB       float64
	PooledPrior    float64
	PooledDelta    float64
	OOFRows        int
	MaxOOFAt       int64
	HoldoutStartAt int64
	PythonVer      string
	SelectedN      []int
	ParityOK       bool
	Rows           []BinaryOOFRow
	Text           string
}

func fitVendorBinary(python, repo, work, tag string, xs [][]float64, ys []int, names []string, iterations int, params map[string]any) (ml.Model, string, error) {
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
	if err := writeJSON(plan, map[string]any{
		"x_path": xPath, "y_path": yPath, "iterations": iterations,
		"catboost_params": params, "feature_names": names,
		"out_json": outJSON, "out_witness": wit,
	}); err != nil {
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
	m, err := ml.ConvertVendor(raw, names, 2, Spec1MaxIterations, Spec1Depth)
	if err != nil {
		return ml.Model{}, "", err
	}
	if err := m.Validate(names, 2, Spec1MaxIterations, Spec1Depth); err != nil {
		return ml.Model{}, "", err
	}
	if len(m.Trees) != iterations {
		return ml.Model{}, "", fmt.Errorf("brain3: tree count %d != %d", len(m.Trees), iterations)
	}
	return m, outJSON, nil
}

func checkPortableBinary(python, repo, work, tag, modelJSON string, xs [][]float64, ys []int, names []string, goModel ml.Model) error {
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
		"x_path": xPath, "y_path": yPath, "model_json": modelJSON, "out_pred": predPath, "feature_names": names,
	}); err != nil {
		return err
	}
	if err := runPython(python, repo, filepath.Join(repo, "research", "brain3", "fit.py"), "predict", plan); err != nil {
		return err
	}
	raw, err := os.ReadFile(predPath)
	if err != nil {
		return err
	}
	var py struct {
		Raw []float64 `json:"raw"`
		P1  []float64 `json:"p1"`
	}
	if err := json.Unmarshal(raw, &py); err != nil {
		return err
	}
	if len(py.Raw) != n || len(py.P1) != n {
		return fmt.Errorf("brain3: PORTABLE_MODEL_MISMATCH binary pred n")
	}
	for i := 0; i < n; i++ {
		z, err := goModel.Margin(xs[i])
		if err != nil {
			return err
		}
		p, err := goModel.ProbTP(xs[i])
		if err != nil {
			return err
		}
		if math.Abs(z-py.Raw[i]) > researchLogitTol {
			return fmt.Errorf("brain3: PORTABLE_MODEL_MISMATCH margin row %d go=%v py=%v", i, z, py.Raw[i])
		}
		if math.Abs(p-py.P1[i]) > researchLogitTol {
			return fmt.Errorf("brain3: PORTABLE_MODEL_MISMATCH p1 row %d go=%v py=%v", i, p, py.P1[i])
		}
	}
	return nil
}

func meanBinaryLossRows(m ml.Model, xs [][]float64, ys []int) (float64, error) {
	var s float64
	for i, x := range xs {
		p, err := m.ProbTP(x)
		if err != nil {
			return 0, err
		}
		v, err := ml.BinaryLogLoss(p, ys[i])
		if err != nil {
			return 0, err
		}
		s += v
	}
	if len(ys) == 0 {
		return 0, fmt.Errorf("brain3: empty binary loss")
	}
	return s / float64(len(ys)), nil
}

func priorQ(tp, stop int) (float64, error) {
	d := tp + stop
	if d == 0 {
		return 0, fmt.Errorf("brain3: no resolved train")
	}
	p := float64(tp) / float64(d)
	if p <= 0 || p >= 1 {
		return 0, fmt.Errorf("brain3: prior_q=%v", p)
	}
	return p, nil
}

func RunBinaryProbe(ds Dataset, val Validation, spec TPStopBinaryProbeSpec1, geoms []BinaryFoldGeom, python, repo, work string) (BinaryProbeResult, error) {
	var z BinaryProbeResult
	if err := spec.validate(); err != nil {
		return z, err
	}
	specHex, err := spec.DigestHex(ds, val)
	if err != nil {
		return z, err
	}
	z.SpecDigest = specHex
	z.HoldoutStartAt = val.HoldoutStartAt
	z.PythonVer = pythonVersion(python)
	params := ResearchBinaryCatBoostParams(spec)
	names := ds.FeatureNames
	var oof []BinaryOOFRow
	var wCB, wPr float64
	var wN int
	selected := make([]int, len(geoms))
	for i, g := range geoms {
		innerX, innerY := resolvedXY(g.Train[g.Split.InnerTrainBegin:g.Split.InnerTrainEnd])
		valX, valY := resolvedXY(g.Train[g.Split.InnerValBegin:g.Split.InnerValEnd])
		innerM, innerJSON, err := fitVendorBinary(python, repo, work, fmt.Sprintf("bin_fold%d_inner", i), innerX, innerY, names, spec.MaxIterations, params)
		if err != nil {
			return z, err
		}
		if err := checkPortableBinary(python, repo, work, fmt.Sprintf("bin_fold%d_inner", i), innerJSON, valX, valY, names, innerM); err != nil {
			return z, err
		}
		z.ParityOK = true
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
		outerX, outerY := resolvedXY(g.Train)
		outerM, outerJSON, err := fitVendorBinary(python, repo, work, fmt.Sprintf("bin_fold%d_outer", i), outerX, outerY, names, nStar, params)
		if err != nil {
			return z, err
		}
		if err := checkPortableBinary(python, repo, work, fmt.Sprintf("bin_fold%d_outer", i), outerJSON, outerX, outerY, names, outerM); err != nil {
			return z, err
		}
		oofX, oofY := resolvedXY(g.Val)
		cbLoss, err := meanBinaryLossRows(outerM, oofX, oofY)
		if err != nil {
			return z, err
		}
		pq, err := priorQ(g.TrainTP, g.TrainStop)
		if err != nil {
			return z, err
		}
		var pLoss float64
		for _, y := range oofY {
			v, err := ml.BinaryLogLoss(pq, y)
			if err != nil {
				return z, err
			}
			pLoss += v
		}
		pLoss /= float64(len(oofY))
		fr := BinaryFoldResult{
			Index: i, ResTrainN: g.ResTrainN, ResValN: g.ResValN,
			TrainTP: g.TrainTP, TrainStop: g.TrainStop, ValTP: g.ValTP, ValStop: g.ValStop,
			InnerTrainN: g.InnerTrainN, InnerValN: g.InnerValN,
			SelectedN: nStar, InnerMinLoss: curve[nStar-1],
			OOFLoss: cbLoss, PriorLoss: pLoss, Delta: pLoss - cbLoss,
		}
		z.Folds = append(z.Folds, fr)
		for j := range oofX {
			at := g.Val[j].At
			if at >= val.HoldoutStartAt {
				return z, fmt.Errorf("brain3: holdout At in binary OOF")
			}
			mg, err := outerM.Margin(oofX[j])
			if err != nil {
				return z, err
			}
			p, err := outerM.ProbTP(oofX[j])
			if err != nil {
				return z, err
			}
			oof = append(oof, BinaryOOFRow{At: at, Fold: i, YBinary: oofY[j], RawMargin: mg, PTP: p})
			if at > z.MaxOOFAt {
				z.MaxOOFAt = at
			}
		}
		wCB += cbLoss * float64(len(oofY))
		wPr += pLoss * float64(len(oofY))
		wN += len(oofY)
	}
	want := 0
	for _, g := range geoms {
		want += g.ResValN
	}
	if wN != want {
		return z, fmt.Errorf("brain3: binary OOF n=%d != resolved val %d", wN, want)
	}
	seen := map[int64]struct{}{}
	for i, r := range oof {
		if _, ok := seen[r.At]; ok {
			return z, fmt.Errorf("brain3: duplicate binary OOF At")
		}
		seen[r.At] = struct{}{}
		if i > 0 && r.At <= oof[i-1].At {
			return z, fmt.Errorf("brain3: binary OOF At not increasing")
		}
	}
	z.Rows = oof
	z.OOFRows = len(oof)
	z.PooledCB = wCB / float64(wN)
	z.PooledPrior = wPr / float64(wN)
	z.PooledDelta = z.PooledPrior - z.PooledCB
	z.SelectedN = selected
	hdr, _ := json.Marshal(struct {
		Format, Spec string
		Schema       []string
	}{BinaryOOFFormatV1, specHex, BinaryClassSchema()})
	h := sha256.New()
	h.Write(hdr)
	var buf [8]byte
	for _, r := range oof {
		binary.LittleEndian.PutUint64(buf[:], uint64(r.At))
		h.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], uint64(r.Fold))
		h.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], uint64(r.YBinary))
		h.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(r.RawMargin))
		h.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(r.PTP))
		h.Write(buf[:])
	}
	z.OOFDigest = hex.EncodeToString(h.Sum(nil))
	z.Text = FormatBinaryProbe(z, geoms)
	return z, nil
}

func FormatBinaryProbe(z BinaryProbeResult, geoms []BinaryFoldGeom) string {
	var b strings.Builder
	b.WriteString("E. TP-STOP-BINARY-PROBE-1 (resolved-only; not a strategy)\n")
	b.WriteString(fmt.Sprintf("spec=%s oof=%s python=%s parity_ok=%v\n", z.SpecDigest, z.OOFDigest, z.PythonVer, z.ParityOK))
	b.WriteString(fmt.Sprintf("resolved_oof_n=%d max_at=%d holdout=%d selected_N=%v\n", z.OOFRows, z.MaxOOFAt, z.HoldoutStartAt, z.SelectedN))
	for _, f := range z.Folds {
		b.WriteString(fmt.Sprintf("fold %d  res_train/val=%d/%d TP/STOP train=%d/%d val=%d/%d inner=%d/%d N*=%d inner_ll=%.6f oof=%.6f prior=%.6f delta=%.6f\n",
			f.Index, f.ResTrainN, f.ResValN, f.TrainTP, f.TrainStop, f.ValTP, f.ValStop, f.InnerTrainN, f.InnerValN,
			f.SelectedN, f.InnerMinLoss, f.OOFLoss, f.PriorLoss, f.Delta))
	}
	b.WriteString(fmt.Sprintf("pooled oof=%.6f prior=%.6f delta=%.6f\n", z.PooledCB, z.PooledPrior, z.PooledDelta))
	return b.String()
}

func describeFork(probe BinaryProbeResult) (string, string) {
	pos := 0
	for _, f := range probe.Folds {
		if f.Delta > 0 {
			pos++
		}
	}
	switch {
	case probe.PooledDelta > 0 && pos >= 3:
		return "OBJECTIVE_GAP_SUPPORTED", "Resolved-only binary CatBoost beats the causal resolved prior on pooled OOF and in >=3/4 future folds. The 66 facts contain TP-vs-STOP information that the 3-class readout did not extract."
	case probe.PooledDelta <= 0 && pos <= 1:
		return "FACT_GAP_SUPPORTED", "The locked binary probe does not beat the causal resolved prior in a useful/stable way. Combined with flat multiclass q, this is a practical fact gap for this CatBoost probe — not a proof that no information exists anywhere."
	case pos == 2 || (probe.PooledDelta > 0 && pos < 3) || (probe.PooledDelta <= 0 && pos >= 2):
		return "REGIME_SENSITIVE_TP_STOP_INFORMATION", "Binary OOF vs prior is mixed across 2022–2025. Inspect regime/fact context before changing architecture."
	default:
		return "INCONCLUSIVE", "Evidence does not support a stronger label."
	}
}
