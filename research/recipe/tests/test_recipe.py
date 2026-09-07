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

from research.calibration.run import generate_calibration
from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.run import generate_oof_logits
from research.modelfit.spec import CLASS_ORDER, pinned_model_spec
from research.rank.reference import RankReference, query_rank
from research.rank.run import generate_rank
from research.rank.spec import pinned_rank_spec
from research.recipe.artifact import attach_digest, read_recipe, write_recipe_atomic
from research.recipe.laws import LOGIC_V1, OUTPUTS, PROJECTION, composition_payload
from research.recipe.project import (
    ForecastEvidence,
    LoadedForecastRecipe,
    RecipeError,
    calibrated_probabilities,
    load_forecast_recipe,
    project_forecast,
)
from research.recipe.run import generate_recipe, recipe_filename

ROOT = Path(__file__).resolve().parents[3]
TINY_OOF = ROOT / "research" / "modelfit" / "testdata" / "tiny.oofmatrix"
TINY_OOF_DIGEST = ROOT / "research" / "modelfit" / "testdata" / "tiny.contentdigest"
LOGITS_DIR = ROOT / "research" / "logits"
CAL_DIR = ROOT / "research" / "calibrations"
RANK_DIR = ROOT / "research" / "ranks"
RECIPE_DIR = ROOT / "research" / "recipes"
PKG = ROOT / "research" / "recipe"
EXPECT_LOGITS = "e347ea29dfd089615b99d8aab5754ece8ea453290c24bf20a8acfa1962d6ac18"
EXPECT_CAL = "2a3b5fe248cd3c75f9a61390973bfd6245154569cf6c61aee1cc75c8de88f775"
EXPECT_RANK = "c68405d029860ca711938800846ea14d22480a09033fbc7a562607ef6ea14f71"


def _tiny_bundle(td: str) -> dict:
    want = TINY_OOF_DIGEST.read_text().strip()
    logits = os.path.join(td, "t.ooflogits")
    generate_oof_logits(str(TINY_OOF), want, logits)
    from research.modelfit.logits import read_oof_logits

    _, _, ft = read_oof_logits(logits)
    ld = ft["content_digest"]
    cal = os.path.join(td, "c.tempcals")
    generate_calibration(logits, ld, cal)
    rank = os.path.join(td, "r.emprank")
    generate_rank(logits, ld, rank)
    from research.calibration.artifact import read_calibration
    from research.rank.artifact import read_rank

    return {
        "logits": logits,
        "logits_digest": ld,
        "cal": cal,
        "cal_digest": read_calibration(cal)["content_digest"],
        "rank": rank,
        "rank_digest": read_rank(rank)["content_digest"],
    }


def _loaded(beta: float, sorted_d) -> LoadedForecastRecipe:
    spec = pinned_rank_spec()
    ref = RankReference(sorted_d=np.asarray(sorted_d, dtype=np.float64), spec=spec)
    return LoadedForecastRecipe(
        beta=beta,
        rank_reference=ref,
        class_order=CLASS_ORDER,
        model_spec_digest=pinned_model_spec().digest_hex(),
    )


class TestSourceHygiene(unittest.TestCase):
    def test_no_fitter_or_second_rank_or_cli_glob(self):
        for py in PKG.glob("*.py"):
            text = py.read_text(encoding="utf-8")
            self.assertNotIn("fit_temperature", text)
            self.assertNotIn("searchsorted", text)
            self.assertNotIn("import glob", text)
            self.assertNotIn("Path.glob", text)
            self.assertNotIn("fit_fold_model", text)
        src = inspect.getsource(project_forecast)
        self.assertIn("query_rank", src)
        self.assertNotIn("searchsorted", src)


class TestGenerate(unittest.TestCase):
    def test_tiny_match_refuse_and_tiny_recipe(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        b = _tiny_bundle(td)
        out = os.path.join(td, "x.fcrecipe")
        writes = {"n": 0}
        real = write_recipe_atomic

        def counting(path, doc):
            writes["n"] += 1
            return real(path, doc)

        with mock.patch("research.recipe.run.write_recipe_atomic", counting):
            doc, matched = generate_recipe(
                b["logits"], b["logits_digest"], b["cal"], b["cal_digest"], b["rank"], b["rank_digest"], out
            )
        self.assertFalse(matched)
        self.assertGreater(writes["n"], 0)
        n0 = writes["n"]
        blob = json.dumps(doc)
        self.assertNotIn("sorted_directional_values", blob)
        self.assertNotIn("beta", doc)
        self.assertNotIn("runtime", doc)
        self.assertNotIn("path", doc)
        self.assertEqual(doc["projection"], PROJECTION)
        with mock.patch("research.recipe.run.write_recipe_atomic", counting):
            _, matched2 = generate_recipe(
                b["logits"], b["logits_digest"], b["cal"], b["cal_digest"], b["rank"], b["rank_digest"], out
            )
        self.assertTrue(matched2)
        self.assertEqual(writes["n"], n0)
        loaded = load_forecast_recipe(out, b["cal"], b["rank"])
        ev = project_forecast([0.2, -0.1, 0.0], loaded)
        self.assertIsInstance(ev, ForecastEvidence)
        with self.assertRaises(RuntimeError):
            generate_recipe(b["logits"], "00" * 32, b["cal"], b["cal_digest"], b["rank"], b["rank_digest"], os.path.join(td, "y.fcrecipe"))
        with self.assertRaises(RuntimeError):
            generate_recipe(b["logits"], b["logits_digest"], b["cal"], b["cal_digest"], b["rank"], "11" * 32, out)
        bad = os.path.join(td, "bad.fcrecipe")
        with open(bad, "w") as fh:
            fh.write("{")
        with self.assertRaises(RuntimeError):
            generate_recipe(b["logits"], b["logits_digest"], b["cal"], b["cal_digest"], b["rank"], b["rank_digest"], bad)

    def test_cross_source_and_model_spec(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        b = _tiny_bundle(td)
        out = os.path.join(td, "z.fcrecipe")
        real_rank = __import__("research.rank.artifact", fromlist=["read_rank"]).read_rank
        doc = real_rank(b["rank"])
        doc = dict(doc)
        src = dict(doc["source"])
        src["oof_logits_content_digest"] = "aa" * 32
        doc["source"] = src
        with mock.patch("research.recipe.run.read_rank", return_value=doc):
            with self.assertRaises(RuntimeError):
                generate_recipe(b["logits"], b["logits_digest"], b["cal"], b["cal_digest"], b["rank"], b["rank_digest"], out)
        with mock.patch(
            "research.recipe.run.pinned_model_spec",
            return_value=mock.Mock(digest_hex=lambda: "bb" * 32, class_order=CLASS_ORDER),
        ):
            with self.assertRaises(RuntimeError):
                generate_recipe(b["logits"], b["logits_digest"], b["cal"], b["cal_digest"], b["rank"], b["rank_digest"], out)


class TestLoadAndProject(unittest.TestCase):
    def test_runtime_sklearn_independence(self):
        loaded = _loaded(1.0, [1.0, 2.0, 2.0, 4.0])
        with mock.patch(
            "research.modelfit.envpin.require_pinned_runtime",
            side_effect=RuntimeError("sklearn gate"),
        ):
            ev = project_forecast([3.0, 0.0, 1.0], loaded)
        self.assertTrue(math.isfinite(ev.directional_rank))

    def test_load_runtime_mismatch(self):
        with mock.patch(
            "research.recipe.project.require_recipe_runtime",
            side_effect=RuntimeError("recipe: refuse NumPy"),
        ):
            with self.assertRaises(RuntimeError):
                load_forecast_recipe("a", "b", "c")

    def test_input_safety(self):
        loaded = _loaded(1.0, [0.0, 1.0])
        with self.assertRaises(RecipeError):
            project_forecast([1.0, 2.0], loaded)
        for z in ([float("nan"), 0.0, 0.0], [float("inf"), 0.0, 0.0], [0.0, float("-inf"), 0.0]):
            with self.assertRaises(RecipeError):
                project_forecast(z, loaded)

    def test_beta_zero_and_extreme(self):
        loaded0 = _loaded(0.0, [-1.0, 0.0, 1.0])
        huge = 1.7976931348623157e308
        p = calibrated_probabilities((huge, -huge, 0.0), 0.0)
        self.assertEqual(p, (1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0))
        ev = project_forecast([0.5, -0.25, 9.0], loaded0)
        self.assertEqual(ev.probabilities, (1.0 / 3.0, 1.0 / 3.0, 1.0 / 3.0))
        with self.assertRaises(RecipeError):
            project_forecast([huge, -huge, 0.0], loaded0)
        loaded_pos = _loaded(1.0, [0.0])
        p2 = calibrated_probabilities((huge, -huge, 0.0), 1.0)
        self.assertTrue(all(math.isfinite(x) and 0.0 <= x <= 1.0 for x in p2))
        self.assertAlmostEqual(sum(p2), 1.0, places=12)

    def test_independence(self):
        z = [1.0, -0.5, 0.2]
        a = _loaded(0.5, [0.0, 1.0, 2.0])
        b = _loaded(2.0, [0.0, 1.0, 2.0])
        ea, eb = project_forecast(z, a), project_forecast(z, b)
        self.assertEqual(ea.directional_rank, eb.directional_rank)
        self.assertNotEqual(ea.probabilities, eb.probabilities)
        c = _loaded(0.5, [-10.0, -9.0])
        ec = project_forecast(z, c)
        self.assertEqual(ea.probabilities, ec.probabilities)
        self.assertNotEqual(ea.directional_rank, ec.directional_rank)
        z_to = [1.0, -0.5, 50.0]
        self.assertEqual(project_forecast(z, a).directional_rank, project_forecast(z_to, a).directional_rank)

    def test_query_rank_owner(self):
        loaded = _loaded(1.0, [1.0, 2.0, 4.0])
        with mock.patch("research.recipe.project.query_rank", wraps=query_rank) as q:
            project_forecast([2.0, 0.0, 0.0], loaded)
            self.assertGreaterEqual(q.call_count, 1)


class TestCanonical(unittest.TestCase):
    def test_canonical_recipe(self):
        cands = list(LOGITS_DIR.glob("BINANCE_*.ooflogits")) if LOGITS_DIR.exists() else []
        cands = [p for p in cands if "rejected" not in p.name]
        cals = list(CAL_DIR.glob("*.tempcals")) if CAL_DIR.exists() else []
        cals = [p for p in cals if "rejected" not in p.name]
        ranks = list(RANK_DIR.glob("*.emprank")) if RANK_DIR.exists() else []
        ranks = [p for p in ranks if "rejected" not in p.name]
        if not (cands and cals and ranks):
            self.skipTest("canonical children not present")
        logits, cal, rank = str(cands[0]), str(cals[0]), str(ranks[0])
        from research.calibration.artifact import read_calibration
        from research.modelfit.logits import read_oof_logits
        from research.rank.artifact import read_rank

        _, _, ft = read_oof_logits(logits)
        if ft["content_digest"] != EXPECT_LOGITS:
            self.skipTest("canonical logits digest is not the expected snapshot")
        if read_calibration(cal)["content_digest"] != EXPECT_CAL:
            self.skipTest("canonical calibration digest is not the expected snapshot")
        if read_rank(rank)["content_digest"] != EXPECT_RANK:
            self.skipTest("canonical rank digest is not the expected snapshot")
        RECIPE_DIR.mkdir(parents=True, exist_ok=True)
        out = RECIPE_DIR / recipe_filename(
            parse_digest_hex(EXPECT_LOGITS), parse_digest_hex(EXPECT_CAL), parse_digest_hex(EXPECT_RANK)
        )
        doc, _ = generate_recipe(logits, EXPECT_LOGITS, cal, EXPECT_CAL, rank, EXPECT_RANK, str(out))
        self.assertEqual(doc["recipe_logic_version"], LOGIC_V1)
        self.assertEqual(doc["base_model"]["model_spec_digest"], pinned_model_spec().digest_hex())
        self.assertEqual(list(doc["class_order"]), list(CLASS_ORDER))
        loaded = load_forecast_recipe(str(out), cal, rank)
        ev = project_forecast([0.0, 0.0, 0.0], loaded)
        self.assertEqual(len(ev.probabilities), 3)
        self.assertTrue(-1.0 <= ev.directional_rank <= 1.0)


if __name__ == "__main__":
    unittest.main()
