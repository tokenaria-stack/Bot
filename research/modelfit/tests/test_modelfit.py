import inspect
import os
import shutil
import tempfile
import unittest
import warnings
from pathlib import Path
from unittest import mock

import numpy as np
from sklearn.exceptions import ConvergenceWarning
from sklearn.linear_model import LogisticRegression

from research.modelfit.envpin import require_pinned_runtime
from research.modelfit.fit import FitError, fit_fold_model, predict_fold_logits
from research.modelfit.matrix import hash_oof_matrix_file, load_oof_matrix
from research.modelfit.run import FIT_PARAMS, PREDICT_PARAMS, assignment_expected, generate_oof_logits
from research.modelfit.spec import ModelSpec, pinned_model_spec

ROOT = Path(__file__).resolve().parents[3]
TINY = ROOT / "research" / "modelfit" / "testdata" / "tiny.oofmatrix"
TINY_DIGEST = ROOT / "research" / "modelfit" / "testdata" / "tiny.contentdigest"
CANON_DIR = ROOT / "research" / "oof"


class TestSignatures(unittest.TestCase):
    def test_fit_has_no_validation_args(self):
        self.assertEqual(FIT_PARAMS, ("x_train", "y_train", "spec"))
        self.assertEqual(tuple(inspect.signature(fit_fold_model).parameters), ("x_train", "y_train", "spec"))
        self.assertNotIn("x_val", inspect.signature(fit_fold_model).parameters)
        self.assertNotIn("y_val", inspect.signature(fit_fold_model).parameters)

    def test_predict_has_no_y_val(self):
        self.assertEqual(PREDICT_PARAMS, ("fitted", "x_val"))
        self.assertNotIn("y_val", inspect.signature(predict_fold_logits).parameters)


class TestModelSpecIdentity(unittest.TestCase):
    def test_stable_and_sensitive(self):
        a = pinned_model_spec()
        b = pinned_model_spec()
        self.assertEqual(a.digest_hex(), b.digest_hex())
        other = ModelSpec(**{**a.__dict__, "C": 10.0})
        self.assertNotEqual(a.digest_hex(), other.digest_hex())
        # rules only: digest payload has no matrix field and no runtime versions
        self.assertNotIn("matrix", a.payload())
        self.assertNotIn("fold", str(a.payload()).lower())
        self.assertNotIn("scipy", a.payload())
        self.assertNotIn("numpy", a.payload())
        self.assertEqual(
            a.digest_hex(),
            "66a9ac20dfb335f664180f5a1d63c5d81e3f2b6e782a7cf936cd355c603e8794",
        )


class TestMatrixGolden(unittest.TestCase):
    def test_go_python_digest(self):
        if not TINY.exists() or not TINY_DIGEST.exists():
            self.skipTest("tiny oof-matrix golden not generated")
        want = TINY_DIGEST.read_text().strip().lower()
        got = hash_oof_matrix_file(str(TINY))
        self.assertEqual(got, want)
        load_oof_matrix(str(TINY), want)


class TestFitLaws(unittest.TestCase):
    def setUp(self):
        require_pinned_runtime()
        rng = np.random.default_rng(0)
        n = 90
        self.x = rng.normal(size=(n, 4)).astype(np.float64)
        self.y = np.array([i % 3 for i in range(n)], dtype=np.int8)
        self.spec = pinned_model_spec()
        self.train_end = 60

    def test_mutation_after_train_end_does_not_change_fit(self):
        x1 = self.x.copy()
        a = fit_fold_model(x1[: self.train_end], self.y[: self.train_end], self.spec)
        x2 = self.x.copy()
        x2[self.train_end :] += 1000.0
        b = fit_fold_model(x2[: self.train_end], self.y[: self.train_end], self.spec)
        np.testing.assert_array_equal(a.mean, b.mean)
        np.testing.assert_array_equal(a.scale, b.scale)
        np.testing.assert_array_equal(a.coef, b.coef)
        np.testing.assert_array_equal(a.intercept, b.intercept)
        logits_a = predict_fold_logits(a, x1[self.train_end :])
        logits_b = predict_fold_logits(b, x2[self.train_end :])
        self.assertFalse(np.allclose(logits_a, logits_b))

    def test_fresh_estimators(self):
        a = fit_fold_model(self.x[:60], self.y[:60], self.spec)
        b = fit_fold_model(self.x[:70], self.y[:70], self.spec)
        self.assertIsNot(a.scaler, b.scaler)
        self.assertIsNot(a.model, b.model)

    def test_missing_class_refuses(self):
        y = self.y.copy()
        y[y == 2] = 0
        with self.assertRaises(FitError):
            fit_fold_model(self.x[:60], y[:60], self.spec)

    def test_convergence_warning_refuses(self):
        def boom(self_est, *args, **kwargs):
            warnings.warn("nope", ConvergenceWarning)
            return self_est

        with mock.patch.object(LogisticRegression, "fit", boom):
            with self.assertRaises(FitError):
                fit_fold_model(self.x[:60], self.y[:60], self.spec)

    def test_nonfinite_logits_refuse(self):
        fitted = fit_fold_model(self.x[:60], self.y[:60], self.spec)

        def inf_decision(x):
            return np.full((x.shape[0], 3), np.inf)

        fitted.model.decision_function = inf_decision
        with self.assertRaises(FitError):
            predict_fold_logits(fitted, self.x[60:70])

    def test_logit_shape(self):
        fitted = fit_fold_model(self.x[:60], self.y[:60], self.spec)
        logits = predict_fold_logits(fitted, self.x[60:75])
        self.assertEqual(logits.shape, (15, 3))


class TestAssignmentAndGenerate(unittest.TestCase):
    def test_assignment_mask_helpers(self):
        class F:
            def __init__(self, a, b):
                self.val_begin, self.val_end = a, b

        n = 20
        exp = assignment_expected(n, [F(5, 10), F(12, 15)])
        self.assertEqual(int(exp.sum()), 8)
        self.assertFalse(exp[4] or exp[10] or exp[11] or exp[15])

    def test_generate_match_and_refuse(self):
        if not TINY.exists():
            self.skipTest("tiny golden missing")
        want = TINY_DIGEST.read_text().strip()
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        out = os.path.join(td, "a.ooflogits")
        calls = {"n": 0}
        real = fit_fold_model

        def counting(x_train, y_train, spec):
            calls["n"] += 1
            return real(x_train, y_train, spec)

        with mock.patch("research.modelfit.run.fit_fold_model", counting):
            h, f, matched = generate_oof_logits(str(TINY), want, out)
        self.assertFalse(matched)
        self.assertGreater(calls["n"], 0)
        self.assertEqual(h["format_version"], "oof-logits-v1")
        n0 = calls["n"]
        with mock.patch("research.modelfit.run.fit_fold_model", counting):
            _, _, matched2 = generate_oof_logits(str(TINY), want, out)
        self.assertTrue(matched2)
        self.assertEqual(calls["n"], n0)
        other = os.path.join(td, "b.ooflogits")
        shutil.copy(out, other)
        with self.assertRaises(RuntimeError):
            generate_oof_logits(str(TINY), "00" * 32, other)
        bad = os.path.join(td, "bad.ooflogits")
        with open(bad, "w") as fh:
            fh.write("not-json\n")
        with self.assertRaises(RuntimeError):
            generate_oof_logits(str(TINY), want, bad)

    def test_environment_mismatch_before_fit(self):
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        with mock.patch("research.modelfit.run.require_pinned_runtime", side_effect=RuntimeError("modelfit: refuse NumPy")):
            with self.assertRaises(RuntimeError):
                generate_oof_logits("x", "00" * 32, os.path.join(td, "z.ooflogits"))


class TestCanonical(unittest.TestCase):
    def test_canonical_matrix_fit(self):
        expect = "ead8b06c8f82fe041b1f53376f8a062eeae52e5a1a85a164c9a69a368c9c0ed8"
        cands = list(CANON_DIR.glob("*.oofmatrix")) if CANON_DIR.exists() else []
        if not cands:
            self.skipTest("canonical oof-matrix-v1 not present")
        path = str(cands[0])
        from research.modelfit.hashwire import digest_hex
        from research.modelfit.matrix import hash_oof_matrix_file

        if hash_oof_matrix_file(path) != expect:
            self.skipTest("canonical file digest is not the expected snapshot")
        out_dir = ROOT / "research" / "logits"
        out_dir.mkdir(parents=True, exist_ok=True)
        spec = pinned_model_spec()
        from research.modelfit.matrix import load_oof_matrix
        from research.modelfit.run import logits_filename

        m = load_oof_matrix(path, expect)
        self.assertEqual(m.at.shape[0], 221305)
        self.assertEqual(m.feature_ids, ["rsx_value", "rsx_signal", "tv_bull_present", "tv_bull_age"])
        out = out_dir / logits_filename(m.header["market"], m.content_digest, spec.digest())
        h, f, _ = generate_oof_logits(path, expect, str(out))
        self.assertEqual(h["runtime"]["scipy"], "1.13.1")
        self.assertEqual(h["runtime"]["sklearn"], "1.5.2")
        self.assertEqual(h["model_spec_digest"], spec.digest_hex())
        self.assertEqual(f["row_count"], 70259)
        self.assertEqual(len(h["folds"]), 4)
        want_n = [17564, 17561, 17568, 17566]
        for i, fj in enumerate(h["folds"]):
            self.assertEqual(fj["output_end"] - fj["output_begin"], want_n[i])
            self.assertEqual(fj["source_validation_end"] - fj["source_validation_begin"], want_n[i])
        n = m.at.shape[0]
        exp = assignment_expected(n, m.folds)
        self.assertEqual(int(exp.sum()), 70259)


if __name__ == "__main__":
    unittest.main()
