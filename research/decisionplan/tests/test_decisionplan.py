import datetime
import inspect
import json
import os
import shutil
import tempfile
import unittest
from pathlib import Path

from research.decisionplan.artifact import LOGIC_V1
from research.decisionplan.compile import compile_validation_plan, extract_at
from research.decisionplan.run import generate_decision_plan
from research.evidence.artifact import hash_evidence, read_evidence, write_evidence_atomic
from research.modelfit.hashwire import digest_hex
from research.modelfit.matrix import load_oof_matrix

ROOT = Path(__file__).resolve().parents[3]
PKG = ROOT / "research" / "decisionplan"
EVIDENCE = ROOT / "research" / "forecast_evidence" / "oof-e347ea29dfd08961_rec-410376806c418a37.ofev"
EXPECT_EV = "64378c3a124c6c2cea0737f7c65001abe70c8a4d4e55629c7f981a29f37aec73"
MATRIX = ROOT / "research" / "oof" / "BINANCE_BTCUSDT_FUTURES_PERP_15m_tape-1db5716813a49136_labels-d5f3764264f7a5df_valplan-154f839c13eb0138.oofmatrix"
MATRIX_DIGEST = "ead8b06c8f82fe041b1f53376f8a062eeae52e5a1a85a164c9a69a368c9c0ed8"


def _utc_ms(y, m, d, hh, mm):
    return int(datetime.datetime(y, m, d, hh, mm, tzinfo=datetime.timezone.utc).timestamp() * 1000)


class TestHygiene(unittest.TestCase):
    def test_no_python_packing_or_performance(self):
        for py in PKG.glob("*.py"):
            text = py.read_text(encoding="utf-8")
            self.assertNotIn("PreviousBarOpen", text)
            self.assertNotIn("HorizonEnd", text)
            self.assertNotIn("hopPrev", text)
            self.assertNotIn("NLL", text)
            self.assertNotIn("DecisionSpec", text)
            self.assertNotIn("AvailableAt", text)
            self.assertNotIn("read_rank", text)
            self.assertNotIn("project_forecast", text)


class TestExtract(unittest.TestCase):
    def test_at_only_and_unsorted(self):
        rows = [
            {"at": 1, "outcome": "UP_FIRST", "probabilities": [1, 0, 0], "directional_rank": 0.2},
            {"at": 2, "outcome": "DOWN_FIRST", "probabilities": [0, 1, 0], "directional_rank": -0.2},
        ]
        self.assertEqual(extract_at(rows), [1, 2])
        src = inspect.getsource(extract_at)
        self.assertIn('r["at"]', src)
        self.assertNotIn("probabilities", src)
        self.assertNotIn("directional_rank", src)
        self.assertNotIn("outcome", src)
        with self.assertRaises(RuntimeError):
            extract_at([{"at": 2}, {"at": 1}])
        src2 = inspect.getsource(compile_validation_plan)
        self.assertIn('"at"', src2)
        self.assertNotIn("probabilities", src2)


class TestBridgeRefuse(unittest.TestCase):
    def test_unsorted_go_refuse(self):
        with self.assertRaises(RuntimeError):
            compile_validation_plan([2, 1])


class TestCanonical(unittest.TestCase):
    def test_generate_match_independence_calendar(self):
        if not EVIDENCE.exists() or not MATRIX.exists():
            self.skipTest("canonical evidence/matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        out = os.path.join(td, "p.dvalplan")
        doc, matched = generate_decision_plan(str(EVIDENCE), EXPECT_EV, str(MATRIX), out)
        self.assertFalse(matched)
        self.assertEqual(doc["decision_validation_logic_version"], LOGIC_V1)
        self.assertEqual(doc["rules"]["validation_span_bars"], 8640)
        self.assertEqual(doc["rules"]["fold_count"], 4)
        self.assertEqual(doc["rules"]["min_train_rows"], 35040)
        self.assertEqual(doc["rules"]["extra_gap_bars"], 0)
        self.assertEqual(doc["rules"]["holdout_start_at"], 1767225600000)
        train0 = doc["compiled"]["folds"][0]["train_end"] - doc["compiled"]["folds"][0]["train_begin"]
        self.assertGreaterEqual(train0, 35040)
        want = [
            (_utc_ms(2025, 1, 5, 18, 0), _utc_ms(2025, 4, 5, 18, 0)),
            (_utc_ms(2025, 4, 5, 18, 0), _utc_ms(2025, 7, 4, 18, 0)),
            (_utc_ms(2025, 7, 4, 18, 0), _utc_ms(2025, 10, 2, 18, 0)),
            (_utc_ms(2025, 10, 2, 18, 0), _utc_ms(2025, 12, 31, 18, 0)),
        ]
        folds = doc["compiled"]["folds"]
        self.assertEqual(len(folds), 4)
        for i, f in enumerate(folds):
            self.assertEqual(int(f["val_boundary_start_at"]), want[i][0])
            self.assertEqual(int(f["val_boundary_end_at"]), want[i][1])
        matrix = load_oof_matrix(str(MATRIX), MATRIX_DIGEST)
        mf = matrix.header["folds"][0]
        self.assertNotEqual(int(folds[0]["val_begin"]), int(mf["validation_begin"]))
        blob = json.dumps(doc)
        self.assertNotIn("probabilities", blob)
        self.assertNotIn("directional_rank", blob)
        self.assertNotIn("feature_ids", blob)
        self.assertNotIn("validation_plan_digest", blob)
        doc2, matched2 = generate_decision_plan(str(EVIDENCE), EXPECT_EV, str(MATRIX), out)
        self.assertTrue(matched2)
        self.assertEqual(doc2["content_digest"], doc["content_digest"])
        with self.assertRaises(RuntimeError):
            generate_decision_plan(str(EVIDENCE), "00" * 32, str(MATRIX), os.path.join(td, "x.dvalplan"))
        with self.assertRaises(RuntimeError):
            generate_decision_plan(str(EVIDENCE), EXPECT_EV, str(EVIDENCE), os.path.join(td, "m.dvalplan"))
        h, rs, ft = read_evidence(str(EVIDENCE))
        for r in rs:
            r["outcome"] = "TIMEOUT" if r["outcome"] != "TIMEOUT" else "UP_FIRST"
            r["probabilities"] = [r["probabilities"][2], r["probabilities"][1], r["probabilities"][0]]
            r["directional_rank"] = -float(r["directional_rank"])
        foot = {"kind": "footer", "row_count": ft["row_count"], "first_at": ft["first_at"], "last_at": ft["last_at"]}
        foot["content_digest"] = digest_hex(hash_evidence(h, rs, foot))
        alt = os.path.join(td, "mut.ofev")
        write_evidence_atomic(alt, h, rs, foot)
        other, _ = generate_decision_plan(alt, foot["content_digest"], str(MATRIX), os.path.join(td, "b.dvalplan"))
        self.assertEqual(other["compiled"], doc["compiled"])
        self.assertEqual(other["rules"], doc["rules"])
        self.assertNotEqual(other["source"]["evidence_content_digest"], doc["source"]["evidence_content_digest"])
        with open(out, "w") as fh:
            fh.write("{")
        with self.assertRaises(RuntimeError):
            generate_decision_plan(str(EVIDENCE), EXPECT_EV, str(MATRIX), out)
