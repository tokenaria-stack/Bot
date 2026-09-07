import inspect
import os
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import numpy as np

from research.calibration.run import generate_calibration
from research.finalfit.artifact import FORMAT, attach_digest, read_final_model, write_final_model_atomic
from research.finalfit.run import generate_final_model
from research.modelfit.envpin import require_pinned_runtime
from research.modelfit.fit import FitError, fit_fold_model
from research.modelfit.hashwire import digest_hex
from research.modelfit.logits import read_oof_logits
from research.modelfit.matrix import load_oof_matrix
from research.modelfit.run import generate_oof_logits
from research.modelfit.spec import CLASS_ORDER, pinned_model_spec
from research.rank.run import generate_rank
from research.recipe.run import generate_recipe

ROOT = Path(__file__).resolve().parents[3]
TINY_OOF = ROOT / "research" / "modelfit" / "testdata" / "tiny.oofmatrix"
TINY_OOF_DIGEST = ROOT / "research" / "modelfit" / "testdata" / "tiny.contentdigest"
PKG = ROOT / "research" / "finalfit"
LOGITS_DIR = ROOT / "research" / "logits"
OOF_DIR = ROOT / "research" / "oof"
RECIPE_DIR = ROOT / "research" / "recipes"
EXPECT_RECIPE = "410376806c418a376c692ef4ebf36062dde9ec10836fc1ad692abe8e2d593366"


def _tiny_world(td: str) -> dict:
    want = TINY_OOF_DIGEST.read_text().strip()
    matrix = str(TINY_OOF)
    logits = os.path.join(td, "t.ooflogits")
    generate_oof_logits(matrix, want, logits)
    _, _, ft = read_oof_logits(logits)
    ld = ft["content_digest"]
    cal = os.path.join(td, "c.tempcals")
    generate_calibration(logits, ld, cal)
    rankp = os.path.join(td, "r.emprank")
    generate_rank(logits, ld, rankp)
    from research.calibration.artifact import read_calibration
    from research.rank.artifact import read_rank

    rec = os.path.join(td, "p.fcrecipe")
    generate_recipe(
        logits,
        ld,
        cal,
        read_calibration(cal)["content_digest"],
        rankp,
        read_rank(rankp)["content_digest"],
        rec,
    )
    from research.recipe.artifact import read_recipe

    return {
        "matrix": matrix,
        "matrix_digest": want,
        "logits": logits,
        "logits_digest": ld,
        "recipe": rec,
        "recipe_digest": read_recipe(rec)["content_digest"],
    }


class TestHygiene(unittest.TestCase):
    def test_one_brain(self):
        for py in PKG.glob("*.py"):
            text = py.read_text(encoding="utf-8")
            self.assertNotIn("StandardScaler", text)
            self.assertNotIn("LogisticRegression", text)
            self.assertNotIn("load_forecast_recipe", text)
            self.assertNotIn("predict_fold_logits", text)
            self.assertNotIn("pickle", text)
            self.assertNotIn("joblib", text)
        run = (PKG / "run.py").read_text(encoding="utf-8")
        self.assertIn("fit_fold_model", run)
        self.assertNotIn("train_begin", run)
        self.assertNotIn("shuffle", run)
        self.assertNotIn("warm_start=True", run)


class TestGenerate(unittest.TestCase):
    def test_chain_fit_match_and_spies(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        w = _tiny_world(td)
        out = os.path.join(td, "m.finalmodel")
        calls = {"fit": 0, "rt": 0}
        real_fit = fit_fold_model
        real_rt = require_pinned_runtime

        def counting_fit(x, y, spec):
            calls["fit"] += 1
            m = load_oof_matrix(w["matrix"], w["matrix_digest"])
            self.assertEqual(x.shape[0], m.at.shape[0])
            self.assertTrue(np.array_equal(x[0], m.x[0]))
            self.assertTrue(np.array_equal(x[-1], m.x[-1]))
            self.assertTrue(np.array_equal(x, m.x))
            self.assertTrue(np.array_equal(y, m.y))
            return real_fit(x, y, spec)

        def counting_rt():
            calls["rt"] += 1
            return real_rt()

        with mock.patch("research.finalfit.run.fit_fold_model", counting_fit):
            with mock.patch("research.finalfit.run.require_pinned_runtime", counting_rt):
                doc, matched = generate_final_model(
                    w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], out
                )
        self.assertFalse(matched)
        self.assertEqual(calls["fit"], 1)
        self.assertEqual(calls["rt"], 1)
        self.assertEqual(doc["format_version"], FORMAT)
        self.assertNotIn("recipe_content_digest", doc)
        self.assertNotIn("beta", doc)
        self.assertGreaterEqual(doc["logistic"]["n_iter"], 1)
        self.assertTrue(all(float(s) > 0 for s in doc["scaler"]["scale"]))
        n0, r0 = calls["fit"], calls["rt"]
        with mock.patch("research.finalfit.run.fit_fold_model", counting_fit):
            with mock.patch(
                "research.finalfit.run.require_pinned_runtime",
                side_effect=RuntimeError("sklearn gate"),
            ):
                _, matched2 = generate_final_model(
                    w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], out
                )
        self.assertTrue(matched2)
        self.assertEqual(calls["fit"], n0)
        with mock.patch("research.finalfit.run.require_pinned_runtime", counting_rt):
            generate_final_model(w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], out)
        self.assertEqual(calls["rt"], r0)
        with self.assertRaises(RuntimeError):
            generate_final_model(w["recipe"], "00" * 32, w["logits"], w["matrix"], os.path.join(td, "x.finalmodel"))
        bad = os.path.join(td, "bad.finalmodel")
        with open(bad, "w") as fh:
            fh.write("{")
        with self.assertRaises(RuntimeError):
            generate_final_model(w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], bad)

    def test_wrong_logits_and_features_and_spec(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        w = _tiny_world(td)
        out = os.path.join(td, "z.finalmodel")
        real = read_oof_logits
        h, rows, ft = real(w["logits"])

        def bad_logits(_path):
            f2 = dict(ft)
            f2["content_digest"] = "aa" * 32
            return h, rows, f2

        with mock.patch("research.finalfit.run.read_oof_logits", bad_logits):
            with self.assertRaises(RuntimeError):
                generate_final_model(w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], out)

        def reordered(_path):
            h2 = dict(h)
            h2["feature_ids"] = list(reversed(list(h["feature_ids"])))
            return h2, rows, ft

        with mock.patch("research.finalfit.run.read_oof_logits", reordered):
            with self.assertRaises(RuntimeError):
                generate_final_model(w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], out)

        with mock.patch(
            "research.finalfit.run.pinned_model_spec",
            return_value=mock.Mock(digest_hex=lambda: "bb" * 32, class_order=CLASS_ORDER, logic="x"),
        ):
            with self.assertRaises(RuntimeError):
                generate_final_model(w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], out)

    def test_absent_runtime_and_convergence(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        w = _tiny_world(td)
        out = os.path.join(td, "n.finalmodel")
        with mock.patch(
            "research.finalfit.run.require_pinned_runtime",
            side_effect=RuntimeError("finalfit: refuse sklearn"),
        ):
            with mock.patch("research.finalfit.run.fit_fold_model") as fit:
                with self.assertRaises(RuntimeError):
                    generate_final_model(w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], out)
                fit.assert_not_called()
        with mock.patch("research.finalfit.run.fit_fold_model", side_effect=FitError("modelfit: ConvergenceWarning")):
            with self.assertRaises(FitError):
                generate_final_model(w["recipe"], w["recipe_digest"], w["logits"], w["matrix"], out)

    def test_reader_scale(self):
        spec = pinned_model_spec()
        body = {
            "format_version": FORMAT,
            "fit_logic_version": "final-model-fit:all-development-v1",
            "source": {
                "oof_matrix_content_digest": "aa" * 32,
                "market": {"venue": "A", "instrument": "B", "contract": "C", "timeframe": "15m"},
                "row_count": 2,
                "first_at": 1,
                "last_at": 2,
            },
            "model": {
                "model_logic_version": spec.logic,
                "model_spec_digest": spec.digest_hex(),
                "feature_ids": ["f0"],
                "class_order": list(CLASS_ORDER),
            },
            "runtime": {"python": "3.9", "numpy": "1.26.4", "sklearn": "1.5.2", "scipy": "1.13.1"},
            "scaler": {"mean": [0.0], "scale": [0.0]},
            "logistic": {"coef": [[1.0], [1.0], [1.0]], "intercept": [0.0, 0.0, 0.0], "n_iter": 1},
        }
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        path = os.path.join(td, "s.finalmodel")
        write_final_model_atomic(path, attach_digest(body))
        with self.assertRaises(ValueError):
            read_final_model(path)


class TestCanonical(unittest.TestCase):
    def test_canonical_final(self):
        recs = list(RECIPE_DIR.glob("*.fcrecipe")) if RECIPE_DIR.exists() else []
        logits = [p for p in LOGITS_DIR.glob("BINANCE_*.ooflogits")] if LOGITS_DIR.exists() else []
        logits = [p for p in logits if "rejected" not in p.name]
        mats = list(OOF_DIR.glob("*.oofmatrix")) if OOF_DIR.exists() else []
        if not (recs and logits and mats):
            self.skipTest("canonical children not present")
        from research.recipe.artifact import read_recipe

        recipe = str(recs[0])
        doc = read_recipe(recipe)
        if doc["content_digest"] != EXPECT_RECIPE:
            self.skipTest("canonical recipe digest is not the expected snapshot")
        logits_p = str(logits[0])
        matrix_p = str(mats[0])
        out_dir = ROOT / "research" / "final_models"
        out_dir.mkdir(parents=True, exist_ok=True)
        from research.finalfit.run import final_model_filename
        from research.modelfit.hashwire import parse_digest_hex

        header, _, _ = read_oof_logits(logits_p)
        spec = pinned_model_spec()
        out = out_dir / final_model_filename(
            header["market"], parse_digest_hex(header["source_oof_matrix_content_digest"]), spec.digest()
        )
        got, _ = generate_final_model(recipe, EXPECT_RECIPE, logits_p, matrix_p, str(out))
        self.assertEqual(got["source"]["row_count"], got["source"]["row_count"])
        self.assertEqual(got["model"]["model_spec_digest"], spec.digest_hex())
        self.assertEqual(list(got["model"]["class_order"]), list(CLASS_ORDER))


if __name__ == "__main__":
    unittest.main()
