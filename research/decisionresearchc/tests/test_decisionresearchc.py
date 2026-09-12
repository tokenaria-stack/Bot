import inspect
import json
import math
import os
import tempfile
import unittest
from pathlib import Path
import numpy as np

from research.calibration.spec import pinned_calibration_spec
from research.catboostlogits.reader import CLASS_ORDER, FORMAT, read_catboost_oof_logits
from research.decisionresearchc.plan import compile_validation_plan_c
from research.decisionresearchc.project import project_decision_fold, selector_payload
from research.decisionresearchc.specs import RANK_POPULATION_C, decision_fold_rank_spec
from research.rank.spec import pinned_rank_spec

ROOT = Path(__file__).resolve().parents[3]
C_PKG = ROOT / "research" / "decisionresearchc"
CB_PKG = ROOT / "research" / "catboostlogits"
V1_DRES = ROOT / "research" / "decisionresearch"
V1_PLAN = ROOT / "research" / "decisionplan"
EXPECT_LOGITS = "71213ef705dc390ee7d551695c1fc9c275044dab4625b86f8a6176fbc67c96ad"
LOGITS_PATH = ROOT / "research" / "catboost" / (
    "BINANCE_BTCUSDT_FUTURES_PERP_15m_matrix-6793d8fe01a8533f_spec-0ff37a077dc2911c.catboostlogits"
)
MATRIX_PATH = ROOT / "research" / "oof" / (
    "BINANCE_BTCUSDT_FUTURES_PERP_15m_tape-5ba899ebe5a1bf59_labels-491c7b8274a27291_valplan-b328000202d3512c.oofmatrix"
)


def _pkg_text(pkg: Path) -> str:
    return "\n".join(p.read_text(encoding="utf-8") for p in pkg.glob("*.py"))


class TestHygiene(unittest.TestCase):
    def test_native_door_not_ol1c(self):
        imports = []
        for pkg in (C_PKG, CB_PKG):
            for p in pkg.glob("*.py"):
                for line in p.read_text(encoding="utf-8").splitlines():
                    s = line.strip()
                    if s.startswith("from ") or s.startswith("import "):
                        imports.append(s)
        blob = "\n".join(imports)
        self.assertNotIn("read_oof_logits", blob)
        self.assertNotIn("research.modelfit.logits", blob)
        src = inspect.getsource(read_catboost_oof_logits)
        self.assertNotIn("sklearn", src)
        self.assertNotIn("StandardScaler", src)
        from research.catboostlogits import reader as cbreader
        self.assertEqual(cbreader.FORMAT, "catboost-oof-logits-v1")
    def test_v1_packages_unmodified_identity(self):
        art = (V1_DRES / "artifact.py").read_text(encoding="utf-8")
        self.assertIn("decision-research-v1", art)
        self.assertIn("oof_forecast_evidence_content_digest", art)
        self.assertIn("DR1C", art)
        plan = (V1_PLAN / "artifact.py").read_text(encoding="utf-8")
        self.assertIn("decision-validation-plan-v1", plan)
        self.assertIn("evidence_content_digest", plan)
        self.assertIn("DV1C", plan)

    def test_no_durable_global_projection_writers(self):
        text = _pkg_text(C_PKG)
        self.assertNotIn("generate_calibration", text)
        self.assertNotIn("generate_rank", text)
        self.assertNotIn("generate_recipe", text)
        self.assertNotIn("write_evidence", text)
        self.assertNotIn("ProjectionEngine", text)


class TestLogitsDoor(unittest.TestCase):
    def test_class_order_and_finite_fixture(self):
        hdr = {
            "kind": "header",
            "format_version": FORMAT,
            "at_unit": "unix_ms",
            "market": {"venue": "BINANCE", "instrument": "BTCUSDT", "contract": "FUTURES_PERP", "timeframe": "15m"},
            "feature_ids": ["a"],
            "source_oof_matrix_content_digest": "11" * 32,
            "catboost_spec_digest": "22" * 32,
            "catboost_fitplan_digest": "33" * 32,
            "class_order": list(CLASS_ORDER),
            "folds": [],
        }
        rows = [
            {"kind": "row", "at": 10, "outcome": "UP_FIRST", "logits": [0.1, -0.2, 0.0]},
            {"kind": "row", "at": 20, "outcome": "DOWN_FIRST", "logits": [0.0, 0.3, -0.1]},
            {"kind": "row", "at": 30, "outcome": "TIMEOUT", "logits": [0.2, 0.2, 0.4]},
        ]
        digest = "aa" * 32
        ft = {"kind": "footer", "row_count": 3, "first_at": 10, "last_at": 30, "content_digest": digest}
        td = tempfile.mkdtemp()
        path = os.path.join(td, "x.catboostlogits")
        with open(path, "w", encoding="utf-8") as f:
            f.write(json.dumps(hdr) + "\n")
            for r in rows:
                f.write(json.dumps(r) + "\n")
            f.write(json.dumps(ft) + "\n")
        got = read_catboost_oof_logits(path, digest)
        self.assertEqual(got.class_order, CLASS_ORDER)
        self.assertEqual(got.class_order[0], "UP_FIRST")
        self.assertEqual(got.class_order[1], "DOWN_FIRST")
        self.assertEqual(got.class_order[2], "TIMEOUT")
        self.assertTrue(np.all(np.diff(got.at) > 0))
        self.assertTrue(np.all(np.isfinite(got.logits)))
        self.assertEqual(int(got.y[0]), 0)
        self.assertEqual(int(got.y[1]), 1)
        self.assertEqual(int(got.y[2]), 2)

    def test_wrong_class_order_refuses(self):
        hdr = {
            "kind": "header",
            "format_version": FORMAT,
            "at_unit": "unix_ms",
            "market": {"venue": "BINANCE", "instrument": "BTCUSDT", "contract": "FUTURES_PERP", "timeframe": "15m"},
            "feature_ids": [],
            "source_oof_matrix_content_digest": "11" * 32,
            "catboost_spec_digest": "22" * 32,
            "catboost_fitplan_digest": "33" * 32,
            "class_order": ["TIMEOUT", "UP_FIRST", "DOWN_FIRST"],
            "folds": [],
        }
        td = tempfile.mkdtemp()
        path = os.path.join(td, "x.catboostlogits")
        with open(path, "w", encoding="utf-8") as f:
            f.write(json.dumps(hdr) + "\n")
            f.write(json.dumps({"kind": "row", "at": 1, "outcome": "UP_FIRST", "logits": [0, 0, 0]}) + "\n")
            f.write(json.dumps({"kind": "footer", "row_count": 1, "first_at": 1, "last_at": 1, "content_digest": "aa" * 32}) + "\n")
        with self.assertRaises(ValueError):
            read_catboost_oof_logits(path, "aa" * 32)


class TestCausalProjection(unittest.TestCase):
    def _data(self, n=90):
        rng = np.random.default_rng(0)
        z = rng.normal(size=(n, 3))
        y = np.array([i % 3 for i in range(n)], dtype=np.int64)
        out = np.array(["UP_FIRST", "DOWN_FIRST", "TIMEOUT"] * (n // 3), dtype=object)
        return z, y, out

    def test_beta_and_rank_from_train_only(self):
        z, y, out = self._data()
        z2 = z.copy()
        z2[60:] += 50.0
        y2 = y.copy()
        y2[60:] = 0
        cal = pinned_calibration_spec()
        rank = decision_fold_rank_spec()
        a = project_decision_fold(z, y, out, 0, 60, 60, 90, cal, rank, "ab" * 32)
        b = project_decision_fold(z2, y2, out, 0, 60, 60, 90, cal, rank, "ab" * 32)
        self.assertEqual(a.beta, b.beta)
        self.assertEqual(a.rank_reference_n, 60)
        self.assertEqual(b.rank_reference_n, 60)
        self.assertEqual(a.rank_reference_n, b.rank_reference_n)
        for i in range(60):
            self.assertEqual(a.train_rows[i]["directional_rank"], b.train_rows[i]["directional_rank"])
            self.assertEqual(a.train_rows[i]["probabilities"], b.train_rows[i]["probabilities"])

    def test_same_beta_rank_on_train_and_val(self):
        z, y, out = self._data()
        cal = pinned_calibration_spec()
        rank = decision_fold_rank_spec()
        proj = project_decision_fold(z, y, out, 0, 60, 60, 90, cal, rank, "ab" * 32)
        from research.recipe.project import LoadedForecastRecipe, project_forecast
        from research.rank.reference import build_rank_reference
        from research.calibration.fit import fit_temperature
        fit = fit_temperature(y[:60], z[:60], cal)
        ref = build_rank_reference(z[:60], rank)
        loaded = LoadedForecastRecipe(beta=fit.beta, rank_reference=ref, class_order=CLASS_ORDER, model_spec_digest="ab" * 32)
        self.assertEqual(proj.beta, fit.beta)
        for i, row in enumerate(proj.train_rows):
            ev = project_forecast(z[i], loaded)
            self.assertEqual(row["probabilities"], list(ev.probabilities))
            self.assertEqual(row["directional_rank"], float(ev.directional_rank))
        for i, row in enumerate(proj.val_rows):
            ev = project_forecast(z[60 + i], loaded)
            self.assertEqual(row["probabilities"], list(ev.probabilities))
            self.assertEqual(row["directional_rank"], float(ev.directional_rank))

    def test_selector_payload_is_train_then_val(self):
        z, y, out = self._data()
        proj = project_decision_fold(z, y, out, 0, 60, 60, 90, pinned_calibration_spec(), decision_fold_rank_spec(), "ab" * 32)
        rows, folds = selector_payload(proj)
        self.assertEqual(folds, [{"train_begin": 0, "train_end": 60, "val_begin": 60, "val_end": 90}])
        self.assertEqual(len(rows), 90)
        self.assertEqual(rows[:60], proj.train_rows)
        self.assertEqual(rows[60:], proj.val_rows)

    def test_val_outcome_change_does_not_change_selection_inputs(self):
        z, y, out = self._data()
        out2 = out.copy()
        out2[60:] = "TIMEOUT"
        cal = pinned_calibration_spec()
        rank = decision_fold_rank_spec()
        a = project_decision_fold(z, y, out, 0, 60, 60, 90, cal, rank, "ab" * 32)
        b = project_decision_fold(z, y, out2, 0, 60, 60, 90, cal, rank, "ab" * 32)
        self.assertEqual(a.beta, b.beta)
        self.assertEqual([r["probabilities"] for r in a.train_rows], [r["probabilities"] for r in b.train_rows])
        self.assertEqual([r["directional_rank"] for r in a.train_rows], [r["directional_rank"] for r in b.train_rows])
        self.assertEqual(a.val_rows[0]["probabilities"], b.val_rows[0]["probabilities"])
        self.assertNotEqual(str(out[60]), str(out2[60]))
        captured = []

        def fake_selector(target, rows, folds):
            captured.append(([r["probabilities"] for r in rows[:60]], [r["outcome"] for r in rows[60:]], list(folds)))
            return {
                "decision_research_logic_version": "decision-research:fixed-grid-v1",
                "decision_logic_version": "decision:target-utility-rank-gate-v1",
                "target_digest": "00" * 32,
                "upper_barrier_utility": 2.0,
                "lower_barrier_utility": 2.0,
                "class_order": list(CLASS_ORDER),
                "folds": [{
                    "fold_index": 0,
                    "selection": {"kind": "ABSTAIN_BASELINE"},
                    "train_audit": {
                        "n": 60, "up_intent": 0, "down_intent": 0, "abstain": 60,
                        "up_intent_up_first": 0, "up_intent_down_first": 0, "up_intent_timeout": 0,
                        "down_intent_up_first": 0, "down_intent_down_first": 0, "down_intent_timeout": 0,
                        "abstain_up_first": 20, "abstain_down_first": 20, "abstain_timeout": 20,
                        "total_utility": 0.0,
                    },
                    "validation_audit": {
                        "n": 30, "up_intent": 0, "down_intent": 0, "abstain": 30,
                        "up_intent_up_first": 0, "up_intent_down_first": 0, "up_intent_timeout": 0,
                        "down_intent_up_first": 0, "down_intent_down_first": 0, "down_intent_timeout": 0,
                        "abstain_up_first": 10, "abstain_down_first": 10, "abstain_timeout": 10,
                        "total_utility": 0.0,
                    },
                    "validation_positive": False,
                    "validation_apply_calls": 0,
                }],
                "pooled_validation": {
                    "n": 30, "up_intent": 0, "down_intent": 0, "abstain": 30,
                    "up_intent_up_first": 0, "up_intent_down_first": 0, "up_intent_timeout": 0,
                    "down_intent_up_first": 0, "down_intent_down_first": 0, "down_intent_timeout": 0,
                    "abstain_up_first": 10, "abstain_down_first": 10, "abstain_timeout": 10,
                    "total_utility": 0.0,
                },
                "positive_fold_count": 0,
                "fold_count": 1,
                "eligible_for_finalization": False,
            }

        from research.decisionresearchc.project import selector_payload as sp
        ra, fa = sp(a)
        rb, fb = sp(b)
        fake_selector("t", ra, fa)
        fake_selector("t", rb, fb)
        self.assertEqual(captured[0][0], captured[1][0])
        self.assertEqual(captured[0][2], captured[1][2])
        self.assertNotEqual(captured[0][1], captured[1][1])

    def test_rank_spec_does_not_lie_v1_population(self):
        v1 = pinned_rank_spec()
        c = decision_fold_rank_spec()
        self.assertEqual(v1.reference_population, "all_rows_of_source_oof_logits_v1")
        self.assertEqual(c.reference_population, RANK_POPULATION_C)
        self.assertNotEqual(v1.digest_hex(), c.digest_hex())
        self.assertEqual(v1.directional_evidence, c.directional_evidence)
        self.assertEqual(v1.tie_law, c.tie_law)


class TestCompilePayload(unittest.TestCase):
    def test_c_compiler_sends_target_c(self):
        src = inspect.getsource(compile_validation_plan_c)
        self.assertIn('"target": "c"', src)
        self.assertIn("target_h", src)
        v1 = (ROOT / "research" / "decisionplan" / "compile.py").read_text(encoding="utf-8")
        self.assertNotIn('"target": "c"', v1)


@unittest.skipUnless(os.environ.get("DECISION_RESEARCH_C") == "1", "set DECISION_RESEARCH_C=1 for canonical logits")
class TestCanonicalRun(unittest.TestCase):
    def test_two_runs_and_geometry(self):
        from research.decisionresearchc.plan import generate_decision_plan_c
        from research.decisionresearchc.run import generate_decision_research_c

        self.assertTrue(LOGITS_PATH.exists(), "canonical CatBoost logits missing")
        self.assertTrue(MATRIX_PATH.exists(), "OOF-MATRIX-C missing")
        logits = read_catboost_oof_logits(str(LOGITS_PATH), EXPECT_LOGITS)
        self.assertEqual(logits.content_digest, EXPECT_LOGITS)
        self.assertEqual(int(logits.at.shape[0]), 70264)
        td = tempfile.mkdtemp()
        plan_a = os.path.join(td, "a.dvalplan")
        plan_b = os.path.join(td, "b.dvalplan")
        out_a = os.path.join(td, "a.dres")
        out_b = os.path.join(td, "b.dres")
        pa, _ = generate_decision_plan_c(logits, str(MATRIX_PATH), plan_a)
        pb, _ = generate_decision_plan_c(logits, str(MATRIX_PATH), plan_b)
        self.assertEqual(pa["content_digest"], pb["content_digest"])
        self.assertEqual(int(pa["rules"]["target_h"]), 72)
        self.assertEqual(int(pa["rules"]["validation_span_bars"]), 8640)
        da, _ = generate_decision_research_c(plan_a, pa["content_digest"], logits, out_a)
        db, _ = generate_decision_research_c(plan_b, pb["content_digest"], logits, out_b)
        self.assertEqual(da["content_digest"], db["content_digest"])
        for f in da["fold_results"]:
            self.assertEqual(f["rank_reference_n"], int(f["train_end"]) - int(f["train_begin"]))
            self.assertTrue(math.isfinite(f["projection_beta"]))
        self.assertNotIn("oof_forecast_evidence_content_digest", json.dumps(da))
        cal_dir = ROOT / "research" / "calibrations"
        if cal_dir.exists():
            before = {p.name for p in cal_dir.glob("*")}
        else:
            before = set()
        after = {p.name for p in cal_dir.glob("*")} if cal_dir.exists() else set()
        self.assertEqual(before, after)


if __name__ == "__main__":
    unittest.main()
