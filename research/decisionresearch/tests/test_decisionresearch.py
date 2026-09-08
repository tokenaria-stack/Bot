import inspect
import json
import os
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from research.decisionplan.artifact import read_plan
from research.decisionresearch.artifact import LOGIC_V1, hash_research, read_research
from research.decisionresearch.run import generate_decision_research, report_lines
from research.decisionresearch.worker import extract_bridge_arrays
from research.modelfit.hashwire import parse_digest_hex

ROOT = Path(__file__).resolve().parents[3]
PKG = ROOT / "research" / "decisionresearch"
EVIDENCE = ROOT / "research" / "forecast_evidence" / "oof-e347ea29dfd08961_rec-410376806c418a37.ofev"
PLAN = ROOT / "research" / "decision_validation" / "ev-64378c3a124c6c2c_dval-walk-forward-v1.dvalplan"
EXPECT_PLAN = "5a5796522f7676a94fd263daedad3ed73368a1fada26a09e6300512d145280b5"
GO_SEL = ROOT / "decisionresearch"


class TestHygiene(unittest.TestCase):
    def test_no_python_decision_twin_or_scoreboard(self):
        for py in PKG.glob("*.py"):
            text = py.read_text(encoding="utf-8")
            self.assertNotIn("ExpectedTargetUtility", text)
            self.assertNotIn("EU_DOWN", text)
            self.assertNotIn("Sharpe", text)
            self.assertNotIn("--expect-evidence", text)
            self.assertNotIn("--recipe", text)
            if py.name != "artifact.py":
                self.assertNotIn("leaderboard", text)
                self.assertNotIn("heatmap", text)
        run = (PKG / "run.py").read_text(encoding="utf-8")
        self.assertNotIn("Holdout", run)
        self.assertNotIn("FINAL-DECISION", run)
        src = inspect.getsource(extract_bridge_arrays)
        self.assertNotIn('["at"]', src)
        self.assertIn("probabilities", src)
        self.assertIn("outcome", src)


class TestGoOneBrain(unittest.TestCase):
    def test_selector_calls_apply(self):
        text = (GO_SEL / "research.go").read_text(encoding="utf-8")
        self.assertIn("decision.ApplyDecision", text)
        self.assertNotIn("expectedTargetUtility(", text)


class TestProvenance(unittest.TestCase):
    def test_wrong_expect_plan(self):
        if not PLAN.exists() or not EVIDENCE.exists():
            self.skipTest("canonical plan/evidence missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        out = os.path.join(td, "x.dres")
        with self.assertRaises(RuntimeError):
            generate_decision_research(str(PLAN), "0" * 64, str(EVIDENCE), out)


class TestCanonical(unittest.TestCase):
    def test_write_match_refuse(self):
        if not PLAN.exists() or not EVIDENCE.exists():
            self.skipTest("canonical plan/evidence missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        out = os.path.join(td, "r.dres")
        doc, matched = generate_decision_research(str(PLAN), EXPECT_PLAN, str(EVIDENCE), out)
        self.assertFalse(matched)
        self.assertEqual(doc["decision_research_logic_version"], LOGIC_V1)
        self.assertEqual(doc["selector"]["candidate_count"], 81)
        blob = json.dumps(doc)
        self.assertNotIn("candidate_utilities", blob)
        self.assertNotIn("leaderboard", blob)
        self.assertNotIn("probabilities", blob)
        self.assertNotIn("directional_rank", blob)
        for line in report_lines(doc):
            self.assertNotIn("second-best", line)
            self.assertNotIn("heatmap", line)
        plan = read_plan(str(PLAN))
        self.assertEqual(doc["source"]["decision_validation_plan_content_digest"], plan["content_digest"])
        with mock.patch("research.decisionresearch.run.run_selector") as rs:
            doc2, matched2 = generate_decision_research(str(PLAN), EXPECT_PLAN, str(EVIDENCE), out)
            rs.assert_not_called()
        self.assertTrue(matched2)
        self.assertEqual(doc2["content_digest"], doc["content_digest"])
        body = {k: v for k, v in doc.items() if k != "content_digest"}
        self.assertEqual(hash_research(body), parse_digest_hex(doc["content_digest"]))
        read_research(out)

        good = Path(out).read_bytes()
        Path(out).write_bytes(good.replace(b"decision-research-v1", b"decision-research-xx", 1))
        with self.assertRaises(RuntimeError):
            generate_decision_research(str(PLAN), EXPECT_PLAN, str(EVIDENCE), out)
        Path(out).write_bytes(good)
        _, matched3 = generate_decision_research(str(PLAN), EXPECT_PLAN, str(EVIDENCE), out)
        self.assertTrue(matched3)
