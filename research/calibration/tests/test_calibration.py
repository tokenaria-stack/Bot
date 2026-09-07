import inspect
import os
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import numpy as np
from scipy.optimize import OptimizeResult

from research.calibration.env import require_calibration_runtime
from research.calibration.fit import CalibrationError, FIT_PARAMS, fit_temperature
from research.calibration.objective import nll_and_grad
from research.calibration.run import generate_calibration
from research.calibration.spec import CalibrationSpec, pinned_calibration_spec
from research.modelfit.run import generate_oof_logits

ROOT = Path(__file__).resolve().parents[3]
TINY_OOF = ROOT / "research" / "modelfit" / "testdata" / "tiny.oofmatrix"
TINY_OOF_DIGEST = ROOT / "research" / "modelfit" / "testdata" / "tiny.contentdigest"
LOGITS_DIR = ROOT / "research" / "logits"
EXPECT_LOGITS = "e347ea29dfd089615b99d8aab5754ece8ea453290c24bf20a8acfa1962d6ac18"


def _tiny_logits(td: str) -> tuple:
    want = TINY_OOF_DIGEST.read_text().strip()
    path = os.path.join(td, "t.ooflogits")
    generate_oof_logits(str(TINY_OOF), want, path)
    from research.modelfit.logits import read_oof_logits

    _, _, ft = read_oof_logits(path)
    return path, ft["content_digest"]


class TestAPI(unittest.TestCase):
    def test_fit_signature_no_x_folds(self):
        self.assertEqual(FIT_PARAMS, ("y", "logits", "spec"))
        self.assertEqual(tuple(inspect.signature(fit_temperature).parameters), ("y", "logits", "spec"))
        self.assertNotIn("x", FIT_PARAMS)
        self.assertNotIn("folds", FIT_PARAMS)
        self.assertNotIn("features", FIT_PARAMS)


class TestSpecIdentity(unittest.TestCase):
    def test_rules_digest(self):
        a = pinned_calibration_spec()
        self.assertEqual(a.digest_hex(), pinned_calibration_spec().digest_hex())
        other = CalibrationSpec(**{**a.__dict__, "max_iter": 50})
        self.assertNotEqual(a.digest_hex(), other.digest_hex())
        self.assertNotIn("source", a.payload())
        self.assertNotIn("beta", a.payload())
        self.assertNotIn("sklearn", a.payload())
        self.assertIn("ftol", a.payload())
        other_ftol = CalibrationSpec(**{**a.__dict__, "ftol": 1e-8})
        self.assertNotEqual(a.digest_hex(), other_ftol.digest_hex())


class TestObjective(unittest.TestCase):
    def setUp(self):
        rng = np.random.default_rng(1)
        n = 60
        self.z = rng.normal(size=(n, 3)).astype(np.float64)
        self.y = np.array([i % 3 for i in range(n)], dtype=np.int64)
        self.spec = pinned_calibration_spec()

    def test_beta_zero_finite(self):
        nll, g = nll_and_grad(0.0, self.y, self.z)
        self.assertTrue(np.isfinite(nll) and np.isfinite(g))

    def test_analytic_matches_finite_difference(self):
        b = 0.7
        _, g = nll_and_grad(b, self.y, self.z)
        eps = 1e-6
        nll_p, _ = nll_and_grad(b + eps, self.y, self.z)
        nll_m, _ = nll_and_grad(b - eps, self.y, self.z)
        fd = (nll_p - nll_m) / (2 * eps)
        self.assertAlmostEqual(g, fd, places=6)

    def test_shift_invariance(self):
        require_calibration_runtime()
        a = fit_temperature(self.y, self.z, self.spec)
        c = np.linspace(-2, 2, self.z.shape[0])[:, None]
        z2 = self.z + c
        b = fit_temperature(self.y, z2, self.spec)
        self.assertAlmostEqual(a.beta, b.beta, places=6)

    def test_missing_class(self):
        y = self.y.copy()
        y[y == 2] = 0
        with self.assertRaises(CalibrationError):
            fit_temperature(y, self.z, self.spec)

    def test_optimizer_failure(self):
        bad = OptimizeResult(
            success=False, message="nope", x=np.array([1.0]), fun=1.0, nit=1, jac=np.array([0.0])
        )
        with mock.patch("research.calibration.fit.minimize", return_value=bad):
            with self.assertRaises(CalibrationError):
                fit_temperature(self.y, self.z, self.spec)

    def test_illegal_beta(self):
        from research.calibration.fit import _validate_fit

        with self.assertRaises(CalibrationError):
            _validate_fit(float("nan"), 1.0, 0.0, 1)
        with self.assertRaises(CalibrationError):
            _validate_fit(-0.1, 1.0, 0.0, 1)
        with self.assertRaises(CalibrationError):
            _validate_fit(1.0, float("inf"), 0.0, 1)


class TestGenerate(unittest.TestCase):
    def test_match_refuse_and_sklearn_independence(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        logits_path, digest = _tiny_logits(td)
        out = os.path.join(td, "c.tempcals")
        calls = {"n": 0}
        real = fit_temperature

        def counting(y, logits, spec):
            calls["n"] += 1
            return real(y, logits, spec)

        with mock.patch("research.calibration.run.fit_temperature", counting):
            with mock.patch(
                "research.modelfit.envpin.require_pinned_runtime",
                side_effect=RuntimeError("sklearn gate"),
            ):
                doc, matched = generate_calibration(logits_path, digest, out)
        self.assertFalse(matched)
        self.assertGreater(calls["n"], 0)
        n0 = calls["n"]
        with mock.patch("research.calibration.run.fit_temperature", counting):
            _, matched2 = generate_calibration(logits_path, digest, out)
        self.assertTrue(matched2)
        self.assertEqual(calls["n"], n0)
        with self.assertRaises(RuntimeError):
            generate_calibration(logits_path, "00" * 32, os.path.join(td, "x.tempcals"))
        bad = os.path.join(td, "bad.tempcals")
        with open(bad, "w") as fh:
            fh.write("{")
        with self.assertRaises(RuntimeError):
            generate_calibration(logits_path, digest, bad)
        other = os.path.join(td, "other.tempcals")
        shutil.copy(out, other)
        with self.assertRaises(RuntimeError):
            generate_calibration(logits_path, "11" * 32, other)

    def test_runtime_mismatch(self):
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        with mock.patch(
            "research.calibration.run.require_calibration_runtime",
            side_effect=RuntimeError("calibration: refuse SciPy"),
        ):
            with self.assertRaises(RuntimeError):
                generate_calibration("x", "00" * 32, os.path.join(td, "z.tempcals"))

    def test_spec_digest_stable_across_sources(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        spec = pinned_calibration_spec()
        other_spec = CalibrationSpec(**{**spec.__dict__, "gtol": 1e-8})
        self.assertNotEqual(spec.digest_hex(), other_spec.digest_hex())


class TestCanonical(unittest.TestCase):
    def test_canonical_logits(self):
        cands = list(LOGITS_DIR.glob("BINANCE_*.ooflogits")) if LOGITS_DIR.exists() else []
        cands = [p for p in cands if "rejected" not in p.name]
        if not cands:
            self.skipTest("canonical oof-logits-v1 not present")
        path = str(cands[0])
        from research.modelfit.logits import read_oof_logits

        _, rows, ft = read_oof_logits(path)
        if ft["content_digest"] != EXPECT_LOGITS:
            self.skipTest("canonical logits digest is not the expected snapshot")
        self.assertEqual(len(rows), 70259)
        out_dir = ROOT / "research" / "calibrations"
        out_dir.mkdir(parents=True, exist_ok=True)
        spec = pinned_calibration_spec()
        from research.calibration.run import calibration_filename
        from research.modelfit.hashwire import parse_digest_hex

        out = out_dir / calibration_filename(parse_digest_hex(EXPECT_LOGITS), spec.digest())
        doc, _ = generate_calibration(path, EXPECT_LOGITS, str(out))
        self.assertEqual(doc["format_version"], "temperature-calibration-v1")
        self.assertEqual(doc["source"]["oof_logits_content_digest"], EXPECT_LOGITS)
        self.assertEqual(doc["source"]["source_row_count"], 70259)
        self.assertGreaterEqual(doc["fit"]["beta"], 0)
        self.assertTrue(np.isfinite(doc["fit"]["beta"]))
        self.assertTrue(doc["fit"]["success"])
        self.assertNotIn("sklearn", doc["runtime"])


if __name__ == "__main__":
    unittest.main()
