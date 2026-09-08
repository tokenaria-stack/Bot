import inspect
import json
import os
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from research.calibration.run import generate_calibration
from research.evidence.artifact import FORMAT, LOGIC_V1, read_evidence
from research.evidence.run import generate_evidence, project_evidence_row
from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.modelfit.run import generate_oof_logits
from research.modelfit.spec import CLASS_ORDER
from research.rank.run import generate_rank
from research.recipe.artifact import read_recipe
from research.recipe.project import ForecastEvidence, load_forecast_recipe, project_forecast
from research.recipe.run import generate_recipe

ROOT = Path(__file__).resolve().parents[3]
TINY_OOF = ROOT / "research" / "modelfit" / "testdata" / "tiny.oofmatrix"
TINY_OOF_DIGEST = ROOT / "research" / "modelfit" / "testdata" / "tiny.contentdigest"
PKG = ROOT / "research" / "evidence"
EXPECT_RECIPE = "410376806c418a376c692ef4ebf36062dde9ec10836fc1ad692abe8e2d593366"
EXPECT_LOGITS = "e347ea29dfd089615b99d8aab5754ece8ea453290c24bf20a8acfa1962d6ac18"
CANON_MATRIX = ROOT / "research" / "oof"


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
    return {
        "matrix": matrix,
        "logits": logits,
        "cal": cal,
        "rank": rankp,
        "recipe": rec,
        "recipe_digest": read_recipe(rec)["content_digest"],
        "logits_digest": ld,
        "matrix_digest": want,
        "cal_digest": read_calibration(cal)["content_digest"],
        "rank_digest": read_rank(rankp)["content_digest"],
    }


class TestHygiene(unittest.TestCase):
    def test_no_bundle_metrics_decisions_holdout(self):
        for py in PKG.glob("*.py"):
            text = py.read_text(encoding="utf-8")
            self.assertNotIn("portable_raw_logits", text)
            self.assertNotIn("BoundForecaster", text)
            self.assertNotIn("fit_fold_model", text)
            self.assertNotIn("final-base-model", text)
            self.assertNotIn("forecast-bundle", text)
            self.assertNotIn("NLL", text)
            self.assertNotIn("Brier", text)
            self.assertNotIn("holdout", text.lower())
            self.assertNotIn("DecisionSpec", text)
            self.assertNotIn("AvailableAt", text)


class TestEvidence(unittest.TestCase):
    def test_tiny_generate_match_refusals(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        w = _tiny_world(td)
        out = os.path.join(td, "e.ofev")
        writes = {"n": 0}
        real = __import__("research.evidence.artifact", fromlist=["write_evidence_atomic"]).write_evidence_atomic

        def counting(path, header, rows, footer):
            writes["n"] += 1
            return real(path, header, rows, footer)

        proj = {"n": 0}
        real_p = __import__("research.evidence.run", fromlist=["project_forecast"]).project_forecast

        def counting_p(*a, **k):
            proj["n"] += 1
            return real_p(*a, **k)

        with mock.patch("research.evidence.run.write_evidence_atomic", counting), mock.patch(
            "research.evidence.run.project_forecast", counting_p
        ):
            h, rows, ft, matched = generate_evidence(
                w["recipe"], w["recipe_digest"], w["logits"], w["cal"], w["rank"], w["matrix"], out
            )
        self.assertFalse(matched)
        self.assertEqual(h["format_version"], FORMAT)
        self.assertEqual(h["evidence_logic_version"], LOGIC_V1)
        self.assertEqual(h["recipe"]["content_digest"], w["recipe_digest"])
        self.assertEqual(h["source"]["oof_logits_content_digest"], w["logits_digest"])
        self.assertEqual(h["source"]["oof_matrix_content_digest"], w["matrix_digest"])
        self.assertNotIn("calibration_content_digest", json.dumps(h))
        self.assertNotIn("rank_content_digest", json.dumps(h))
        self.assertNotIn("feature_ids", h)
        self.assertNotIn("feature_plan_digest", h)
        self.assertNotIn("calibration", h)
        self.assertNotIn("rank", h)
        self.assertNotIn("logits", rows[0])
        self.assertNotIn("features", rows[0])
        self.assertNotIn("fold", rows[0])
        blob = json.dumps(h)
        self.assertNotIn("sorted_directional_values", blob)
        self.assertNotIn("final_model", blob)
        self.assertNotIn("forecast_bundle", blob)
        _, src_rows, src_ft = read_oof_logits(w["logits"])
        self.assertEqual(len(rows), len(src_rows))
        self.assertEqual(int(ft["first_at"]), int(src_ft["first_at"]))
        self.assertEqual(int(ft["last_at"]), int(src_ft["last_at"]))
        self.assertEqual([r["at"] for r in rows], [r["at"] for r in src_rows])
        self.assertEqual(proj["n"], len(src_rows))
        self.assertGreater(writes["n"], 0)
        n0, p0 = writes["n"], proj["n"]

        loaded = load_forecast_recipe(w["recipe"], w["cal"], w["rank"])
        z = src_rows[0]["logits"]
        a = project_evidence_row(int(src_rows[0]["at"]), "UP_FIRST", z, loaded)
        b = project_evidence_row(int(src_rows[0]["at"]), "DOWN_FIRST", z, loaded)
        self.assertEqual(a["probabilities"], b["probabilities"])
        self.assertEqual(a["directional_rank"], b["directional_rank"])
        self.assertNotEqual(a["outcome"], b["outcome"])

        with mock.patch("research.evidence.run.write_evidence_atomic", counting), mock.patch(
            "research.evidence.run.project_forecast", counting_p
        ), mock.patch("research.evidence.run.load_forecast_recipe") as load_spy, mock.patch(
            "research.rank.artifact.read_rank"
        ) as rank_spy, mock.patch(
            "research.evidence.run.require_recipe_runtime",
            side_effect=RuntimeError("should not execute"),
        ):
            h2, _, _, matched2 = generate_evidence(
                w["recipe"], w["recipe_digest"], w["logits"], w["cal"], w["rank"], w["matrix"], out
            )
        self.assertTrue(matched2)
        self.assertEqual(writes["n"], n0)
        self.assertEqual(proj["n"], p0)
        load_spy.assert_not_called()
        rank_spy.assert_not_called()
        self.assertEqual(h2["runtime"]["python"], "3.9")
        self.assertEqual(h2["runtime"]["numpy"], "1.26.4")

        with self.assertRaises(RuntimeError):
            generate_evidence(w["recipe"], "00" * 32, w["logits"], w["cal"], w["rank"], w["matrix"], os.path.join(td, "x.ofev"))
        from research.recipe.artifact import attach_digest as attach_recipe, write_recipe_atomic
        from research.calibration.artifact import attach_digest as attach_cal, read_calibration, write_calibration_atomic
        from research.rank.artifact import attach_digest as attach_rank, read_rank, write_rank_atomic

        bad_rec = os.path.join(td, "badsrc.fcrecipe")
        rdoc = dict(read_recipe(w["recipe"]))
        rdoc["research_source"] = dict(rdoc["research_source"])
        rdoc["research_source"]["oof_logits_content_digest"] = "11" * 32
        rdoc = attach_recipe(rdoc)
        write_recipe_atomic(bad_rec, rdoc)
        with self.assertRaises(RuntimeError):
            generate_evidence(bad_rec, rdoc["content_digest"], w["logits"], w["cal"], w["rank"], w["matrix"], os.path.join(td, "m.ofev"))

        bad_cal = os.path.join(td, "bad.tempcals")
        c0 = read_calibration(w["cal"])
        cdoc = attach_cal({**c0, "fit": {**c0["fit"], "beta": 0.123456789}})
        write_calibration_atomic(bad_cal, cdoc)
        with self.assertRaises(RuntimeError):
            generate_evidence(w["recipe"], w["recipe_digest"], w["logits"], bad_cal, w["rank"], w["matrix"], os.path.join(td, "c.ofev"))

        bad_rank = os.path.join(td, "bad.emprank")
        rk0 = read_rank(w["rank"])
        src = dict(rk0["source"])
        src["oof_logits_content_digest"] = "22" * 32
        rk = attach_rank({**rk0, "source": src})
        write_rank_atomic(bad_rank, rk)
        with self.assertRaises(RuntimeError):
            generate_evidence(w["recipe"], w["recipe_digest"], w["logits"], w["cal"], bad_rank, w["matrix"], os.path.join(td, "r.ofev"))
        with open(out, "w") as fh:
            fh.write("{")
        with self.assertRaises(RuntimeError):
            generate_evidence(w["recipe"], w["recipe_digest"], w["logits"], w["cal"], w["rank"], w["matrix"], out)

    def test_wrong_matrix_and_runtime_and_invalid_row(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        w = _tiny_world(td)
        with self.assertRaises(RuntimeError):
            generate_evidence(
                w["recipe"],
                w["recipe_digest"],
                w["logits"],
                w["cal"],
                w["rank"],
                w["logits"],
                os.path.join(td, "badm.ofev"),
            )
        with mock.patch("research.evidence.run.require_recipe_runtime", side_effect=RuntimeError("pin")):
            with mock.patch("research.evidence.run.project_forecast") as spy:
                with self.assertRaises(RuntimeError):
                    generate_evidence(
                        w["recipe"], w["recipe_digest"], w["logits"], w["cal"], w["rank"], w["matrix"], os.path.join(td, "rt.ofev")
                    )
                spy.assert_not_called()
        out = os.path.join(td, "ok.ofev")
        generate_evidence(w["recipe"], w["recipe_digest"], w["logits"], w["cal"], w["rank"], w["matrix"], out)

        def boom(*a, **k):
            return ForecastEvidence(probabilities=(1.5, 0.0, 0.0), directional_rank=0.0)

        with self.assertRaises(RuntimeError):
            with mock.patch("research.evidence.run.project_forecast", boom):
                generate_evidence(
                    w["recipe"], w["recipe_digest"], w["logits"], w["cal"], w["rank"], w["matrix"], os.path.join(td, "badp.ofev")
                )

    def test_source_inspect_project_forecast(self):
        src = inspect.getsource(project_evidence_row)
        self.assertIn("project_forecast", src)
        self.assertNotIn("outcome", inspect.signature(project_forecast).parameters)


class TestCanonicalIfPresent(unittest.TestCase):
    def test_canonical_paths_optional(self):
        recipes = list((ROOT / "research" / "recipes").glob("*.fcrecipe")) if (ROOT / "research" / "recipes").is_dir() else []
        if not recipes:
            self.skipTest("canonical recipe artifact not on disk")
        rec = str(recipes[0])
        doc = read_recipe(rec)
        self.assertEqual(doc["content_digest"].lower(), EXPECT_RECIPE)


