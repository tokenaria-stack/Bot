import inspect
import json
import math
import os
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import numpy as np

from research.bundle.artifact import FORMAT, attach_digest, read_bundle, write_bundle_atomic
from research.bundle.laws import bundle_payload
from research.bundle.portable import BundleError, portable_raw_logits
from research.bundle.run import generate_bundle
from research.bundle.runtime import (
    BoundForecaster,
    FeatureProducerContract,
    LoadedForecastBundle,
    bind_feature_contract,
    forecast,
    load_forecast_bundle,
)
from research.calibration.run import generate_calibration
from research.finalfit.run import generate_final_model
from research.modelfit.fit import fit_fold_model, predict_fold_logits
from research.modelfit.logits import read_oof_logits
from research.modelfit.matrix import load_oof_matrix
from research.modelfit.run import generate_oof_logits
from research.modelfit.spec import CLASS_ORDER, pinned_model_spec
from research.rank.run import generate_rank
from research.recipe.artifact import read_recipe
from research.recipe.run import generate_recipe
from research.recipe.project import project_forecast

ROOT = Path(__file__).resolve().parents[3]
TINY_OOF = ROOT / "research" / "modelfit" / "testdata" / "tiny.oofmatrix"
TINY_OOF_DIGEST = ROOT / "research" / "modelfit" / "testdata" / "tiny.contentdigest"
PKG = ROOT / "research" / "bundle"
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
        logits, ld, cal, read_calibration(cal)["content_digest"], rankp, read_rank(rankp)["content_digest"], rec
    )
    fm = os.path.join(td, "m.finalmodel")
    generate_final_model(rec, read_recipe(rec)["content_digest"], logits, matrix, fm)
    from research.finalfit.artifact import read_final_model

    return {
        "matrix": matrix,
        "logits": logits,
        "cal": cal,
        "rank": rankp,
        "recipe": rec,
        "model": fm,
        "recipe_digest": read_recipe(rec)["content_digest"],
        "model_digest": read_final_model(fm)["content_digest"],
        "cal_digest": read_calibration(cal)["content_digest"],
        "rank_digest": read_rank(rankp)["content_digest"],
        "logits_digest": ld,
        "matrix_digest": want,
    }


class TestHygiene(unittest.TestCase):
    def test_one_brain(self):
        for py in PKG.glob("*.py"):
            text = py.read_text(encoding="utf-8")
            self.assertNotIn("StandardScaler", text)
            self.assertNotIn("LogisticRegression", text)
            self.assertNotIn("searchsorted", text)
            self.assertNotIn("calibrated_probabilities", text)
            self.assertNotIn("fit_fold_model", text)
        rt = (PKG / "runtime.py").read_text(encoding="utf-8")
        self.assertIn("project_forecast", rt)
        self.assertNotIn("def forecast", inspect.getsource(LoadedForecastBundle))


class TestPortableParity(unittest.TestCase):
    def test_sklearn_agreement(self):
        rng = np.random.default_rng(7)
        x = rng.normal(size=(40, 4)).astype(np.float64)
        y = np.array([i % 3 for i in range(40)], dtype=np.int8)
        spec = pinned_model_spec()
        fitted = fit_fold_model(x, y, spec)
        xt = rng.normal(size=(8, 4)).astype(np.float64)
        a = predict_fold_logits(fitted, xt)
        for i in range(xt.shape[0]):
            b = portable_raw_logits(xt[i], fitted.mean, fitted.scale, fitted.coef, fitted.intercept)
            np.testing.assert_allclose(a[i], b, rtol=1e-12, atol=1e-12)


class TestGenerateAndRuntime(unittest.TestCase):
    def test_tiny_bundle_bind_forecast_match(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        w = _tiny_world(td)
        out = os.path.join(td, "b.fcbundle")
        writes = {"n": 0}
        real = write_bundle_atomic

        def counting(path, doc):
            writes["n"] += 1
            return real(path, doc)

        with mock.patch("research.bundle.run.write_bundle_atomic", counting):
            doc, matched = generate_bundle(w["model"], w["recipe"], w["cal"], w["rank"], w["logits"], w["matrix"], out)
        self.assertFalse(matched)
        self.assertGreater(writes["n"], 0)
        blob = json.dumps(doc)
        self.assertNotIn("feature_tape_content_digest", blob)
        self.assertNotIn("beta", doc)
        self.assertNotIn("runtime", doc)
        self.assertIn("feature_plan_digest", doc["input_contract"])
        self.assertIn("target_digest", doc["output_contract"])
        self.assertEqual(list(doc["output_contract"]["class_order"]), list(CLASS_ORDER))
        n0 = writes["n"]
        with mock.patch("research.bundle.run.write_bundle_atomic", counting):
            _, matched2 = generate_bundle(w["model"], w["recipe"], w["cal"], w["rank"], w["logits"], w["matrix"], out)
        self.assertTrue(matched2)
        self.assertEqual(writes["n"], n0)
        loaded = load_forecast_bundle(out, w["model"], w["recipe"], w["cal"], w["rank"])
        self.assertIsInstance(loaded, LoadedForecastBundle)
        m = load_oof_matrix(w["matrix"], w["matrix_digest"])
        producer = FeatureProducerContract(tuple(m.feature_ids), m.header["feature_plan_digest"])
        bound = bind_feature_contract(loaded, producer)
        self.assertIsInstance(bound, BoundForecaster)
        ev = forecast(bound, list(m.x[0]))
        self.assertEqual(len(ev.probabilities), 3)
        self.assertTrue(-1.0 <= ev.directional_rank <= 1.0)
        with mock.patch("research.bundle.runtime.project_forecast", wraps=project_forecast) as pf:
            forecast(bound, list(m.x[0]))
            self.assertGreaterEqual(pf.call_count, 1)
        drift = FeatureProducerContract(tuple(m.feature_ids), "11" * 32)
        with self.assertRaises(RuntimeError):
            bind_feature_contract(loaded, drift)
        reordered = FeatureProducerContract(tuple(reversed(m.feature_ids)), m.header["feature_plan_digest"])
        with self.assertRaises(RuntimeError):
            bind_feature_contract(loaded, reordered)
        with self.assertRaises(BundleError):
            forecast(loaded, list(m.x[0]))  # type: ignore[arg-type]
        with self.assertRaises(BundleError):
            forecast(bound, [float("nan")] * len(m.feature_ids))
        with self.assertRaises(BundleError):
            forecast(bound, [0.0])
        huge = 1.7976931348623157e308
        with self.assertRaises(BundleError):
            portable_raw_logits(
                [huge] * len(loaded.mean),
                tuple(-huge for _ in loaded.mean),
                loaded.scale,
                loaded.coef,
                loaded.intercept,
            )
        with mock.patch(
            "research.modelfit.envpin.require_pinned_runtime",
            side_effect=RuntimeError("sklearn gate"),
        ):
            ev2 = forecast(bound, list(m.x[0]))
        self.assertEqual(ev.directional_rank, ev2.directional_rank)
        tampered = dict(doc)
        oc = dict(tampered["output_contract"])
        oc["target_digest"] = "22" * 32
        tampered["output_contract"] = oc
        other = os.path.join(td, "t.fcbundle")
        write_bundle_atomic(other, attach_digest(tampered))
        with self.assertRaises(RuntimeError):
            generate_bundle(w["model"], w["recipe"], w["cal"], w["rank"], w["logits"], w["matrix"], other)
        bad = os.path.join(td, "bad.fcbundle")
        with open(bad, "w") as fh:
            fh.write("{")
        with self.assertRaises(RuntimeError):
            generate_bundle(w["model"], w["recipe"], w["cal"], w["rank"], w["logits"], w["matrix"], bad)

    def test_wrong_logits(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        w = _tiny_world(td)
        out = os.path.join(td, "z.fcbundle")
        real = read_oof_logits
        h, rows, ft = real(w["logits"])

        def bad(_p):
            f2 = dict(ft)
            f2["content_digest"] = "aa" * 32
            return h, rows, f2

        with mock.patch("research.bundle.run.read_oof_logits", bad):
            with self.assertRaises(RuntimeError):
                generate_bundle(w["model"], w["recipe"], w["cal"], w["rank"], w["logits"], w["matrix"], out)


class TestCanonical(unittest.TestCase):
    def test_canonical_and_runtime_without_oof_reload(self):
        recs = list((ROOT / "research" / "recipes").glob("*.fcrecipe"))
        models = list((ROOT / "research" / "final_models").glob("*.finalmodel"))
        cals = [p for p in (ROOT / "research" / "calibrations").glob("*.tempcals") if "rejected" not in p.name]
        ranks = [p for p in (ROOT / "research" / "ranks").glob("*.emprank") if "rejected" not in p.name]
        logits = [p for p in (ROOT / "research" / "logits").glob("BINANCE_*.ooflogits") if "rejected" not in p.name]
        mats = list((ROOT / "research" / "oof").glob("*.oofmatrix"))
        if not (recs and models and cals and ranks and logits and mats):
            self.skipTest("canonical children not present")
        recipe = str(recs[0])
        if read_recipe(recipe)["content_digest"] != EXPECT_RECIPE:
            self.skipTest("canonical recipe digest is not the expected snapshot")
        out_dir = ROOT / "research" / "bundles"
        out_dir.mkdir(parents=True, exist_ok=True)
        from research.bundle.run import bundle_filename
        from research.finalfit.artifact import read_final_model
        from research.modelfit.hashwire import parse_digest_hex

        model_p = str(models[0])
        out = out_dir / bundle_filename(
            parse_digest_hex(EXPECT_RECIPE), parse_digest_hex(read_final_model(model_p)["content_digest"])
        )
        doc, _ = generate_bundle(model_p, recipe, str(cals[0]), str(ranks[0]), str(logits[0]), str(mats[0]), str(out))
        self.assertEqual(doc["format_version"], FORMAT)
        self.assertNotIn("feature_tape_content_digest", json.dumps(doc))
        loaded = load_forecast_bundle(str(out), model_p, recipe, str(cals[0]), str(ranks[0]))
        m = load_oof_matrix(str(mats[0]), read_oof_logits(str(logits[0]))[0]["source_oof_matrix_content_digest"])
        bound = bind_feature_contract(
            loaded, FeatureProducerContract(tuple(m.feature_ids), m.header["feature_plan_digest"])
        )
        ev = forecast(bound, [0.0] * len(loaded.feature_ids))
        self.assertEqual(len(ev.probabilities), 3)


if __name__ == "__main__":
    unittest.main()
