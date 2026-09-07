import ast
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

from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.run import generate_oof_logits
from research.modelfit.spec import CLASS_ORDER
from research.rank.artifact import FORMAT, attach_digest, read_rank, write_rank_atomic
from research.rank.reference import BUILD_PARAMS, QUERY_PARAMS, RankError, RankReference, build_rank_reference, query_rank
from research.rank.run import generate_rank, require_class_order
from research.rank.spec import RankSpec, pinned_rank_spec

ROOT = Path(__file__).resolve().parents[3]
TINY_OOF = ROOT / "research" / "modelfit" / "testdata" / "tiny.oofmatrix"
TINY_OOF_DIGEST = ROOT / "research" / "modelfit" / "testdata" / "tiny.contentdigest"
LOGITS_DIR = ROOT / "research" / "logits"
RANK_DIR = ROOT / "research" / "ranks"
EXPECT_LOGITS = "e347ea29dfd089615b99d8aab5754ece8ea453290c24bf20a8acfa1962d6ac18"
RANK_PKG = ROOT / "research" / "rank"


def _tiny_logits(td: str) -> tuple:
    want = TINY_OOF_DIGEST.read_text().strip()
    path = os.path.join(td, "t.ooflogits")
    generate_oof_logits(str(TINY_OOF), want, path)
    from research.modelfit.logits import read_oof_logits

    _, _, ft = read_oof_logits(path)
    return path, ft["content_digest"]


def _spec_payload_change(**kwargs) -> RankSpec:
    a = pinned_rank_spec()
    d = dict(a.__dict__)
    d.update(kwargs)
    return RankSpec(**d)


class TestAPI(unittest.TestCase):
    def test_signatures_logits_only(self):
        self.assertEqual(BUILD_PARAMS, ("logits", "spec"))
        self.assertEqual(tuple(inspect.signature(build_rank_reference).parameters), ("logits", "spec"))
        self.assertEqual(QUERY_PARAMS, ("reference", "d"))
        self.assertEqual(tuple(inspect.signature(query_rank).parameters), ("reference", "d"))
        src = inspect.getsource(build_rank_reference)
        self.assertNotIn("outcome", src.lower())
        self.assertNotIn("fold", src.lower())

    def test_no_calibration_import_or_cli(self):
        for py in RANK_PKG.glob("*.py"):
            text = py.read_text(encoding="utf-8")
            self.assertNotIn("research.calibration", text)
            self.assertNotIn("--beta", text)
            self.assertNotIn("--calibration", text)
        tree = ast.parse((RANK_PKG / "__main__.py").read_text(encoding="utf-8"))
        for node in ast.walk(tree):
            if isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute):
                if node.func.attr == "add_argument":
                    for a in node.args:
                        if isinstance(a, ast.Constant) and isinstance(a.value, str):
                            self.assertNotIn("beta", a.value)
                            self.assertNotIn("calibration", a.value)

    def test_uniform_weighting_only(self):
        spec = pinned_rank_spec()
        self.assertEqual(spec.reference_weighting, "uniform_rows")
        text = (RANK_PKG / "reference.py").read_text(encoding="utf-8")
        self.assertNotIn("recency", text)
        self.assertNotIn("class_weight", text)
        self.assertNotIn("fold_weight", text)


class TestSpecIdentity(unittest.TestCase):
    def test_rules_digest(self):
        a = pinned_rank_spec()
        self.assertEqual(a.digest_hex(), pinned_rank_spec().digest_hex())
        other = _spec_payload_change(reference_weighting="recency")
        self.assertNotEqual(a.digest_hex(), other.digest_hex())
        blob = json.dumps(a.payload())
        self.assertNotIn("numpy.searchsorted", blob)
        self.assertNotIn("quicksort", blob)
        self.assertNotIn("source", a.payload())
        self.assertNotIn("numpy", a.payload())
        payload = json.dumps(a.payload())
        self.assertNotIn("e347ea29", payload)


class TestQueryLaw(unittest.TestCase):
    def setUp(self):
        spec = pinned_rank_spec()
        self.ref = RankReference(sorted_d=np.array([1.0, 2.0, 2.0, 4.0], dtype=np.float64), spec=spec)

    def test_boundaries(self):
        self.assertEqual(query_rank(self.ref, 0.5), -1.0)
        self.assertEqual(query_rank(self.ref, 2.0), 0.0)
        self.assertEqual(query_rank(self.ref, 3.0), 0.5)
        self.assertEqual(query_rank(self.ref, 5.0), 1.0)

    def test_nonfinite_query(self):
        for d in (float("nan"), float("inf"), float("-inf")):
            with self.assertRaises(RankError):
                query_rank(self.ref, d)

    def test_empty_query(self):
        empty = RankReference(sorted_d=np.array([], dtype=np.float64), spec=pinned_rank_spec())
        with self.assertRaises(RankError):
            query_rank(empty, 0.0)

    def test_exact_tie_no_epsilon(self):
        r = query_rank(self.ref, 2.0)
        self.assertEqual(r, 0.0)
        self.assertNotEqual(query_rank(self.ref, 2.0 + 1e-12), 0.0)

    def test_signed_zero(self):
        spec = pinned_rank_spec()
        ref = RankReference(sorted_d=np.array([-0.0, 0.0], dtype=np.float64), spec=spec)
        self.assertEqual(query_rank(ref, 0.0), query_rank(ref, -0.0))


class TestConstruction(unittest.TestCase):
    def test_empty(self):
        with self.assertRaises(RankError):
            build_rank_reference(np.zeros((0, 3), dtype=np.float64), pinned_rank_spec())

    def test_nonfinite(self):
        z = np.array([[1.0, 0.0, 0.0], [np.inf, 0.0, 0.0]], dtype=np.float64)
        with self.assertRaises(RankError):
            build_rank_reference(z, pinned_rank_spec())

    def test_timeout_independence(self):
        spec = pinned_rank_spec()
        a = np.array([[1.0, -1.0, 0.1], [2.0, 0.5, -3.0], [0.0, 0.2, 9.0]], dtype=np.float64)
        b = a.copy()
        b[:, 2] = np.array([100.0, -100.0, 50.0])
        ra = build_rank_reference(a, spec)
        rb = build_rank_reference(b, spec)
        self.assertTrue(np.array_equal(ra.sorted_d, rb.sorted_d))
        self.assertEqual(query_rank(ra, 1.5), query_rank(rb, 1.5))

    def test_class_order_refuse(self):
        with self.assertRaises(RuntimeError):
            require_class_order(["DOWN_FIRST", "UP_FIRST", "TIMEOUT"])
        require_class_order(list(CLASS_ORDER))


class TestGenerate(unittest.TestCase):
    def test_match_refuse_runtime_and_sklearn_independence(self):
        if not TINY_OOF.exists():
            self.skipTest("tiny oof-matrix missing")
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        logits_path, digest = _tiny_logits(td)
        out = os.path.join(td, "r.emprank")
        calls = {"n": 0}
        real = build_rank_reference

        def counting(logits, spec):
            calls["n"] += 1
            return real(logits, spec)

        with mock.patch("research.rank.run.build_rank_reference", counting):
            with mock.patch(
                "research.modelfit.envpin.require_pinned_runtime",
                side_effect=RuntimeError("sklearn gate"),
            ):
                doc, matched = generate_rank(logits_path, digest, out)
        self.assertFalse(matched)
        self.assertGreater(calls["n"], 0)
        self.assertNotIn("sklearn", doc["runtime"])
        self.assertNotIn("scipy", doc["runtime"])
        n0 = calls["n"]
        with mock.patch("research.rank.run.build_rank_reference", counting):
            _, matched2 = generate_rank(logits_path, digest, out)
        self.assertTrue(matched2)
        self.assertEqual(calls["n"], n0)
        with self.assertRaises(RuntimeError):
            generate_rank(logits_path, "00" * 32, os.path.join(td, "x.emprank"))
        bad = os.path.join(td, "bad.emprank")
        with open(bad, "w") as fh:
            fh.write("{")
        with self.assertRaises(RuntimeError):
            generate_rank(logits_path, digest, bad)
        other = os.path.join(td, "other.emprank")
        shutil.copy(out, other)
        with self.assertRaises(RuntimeError):
            generate_rank(logits_path, "11" * 32, other)

    def test_runtime_mismatch(self):
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        with mock.patch(
            "research.rank.run.require_rank_runtime",
            side_effect=RuntimeError("rank: refuse NumPy"),
        ):
            with self.assertRaises(RuntimeError):
                generate_rank("x", "00" * 32, os.path.join(td, "z.emprank"))

    def test_source_vs_rules_digest(self):
        spec = pinned_rank_spec()
        runtime = {"python": "3.9", "numpy": "1.26.4"}

        def doc_for(src: str, vals):
            body = {
                "format_version": FORMAT,
                "source": {"oof_logits_content_digest": src, "source_row_count": len(vals)},
                "rank": {
                    "rank_logic_version": spec.logic,
                    "rank_spec_digest": spec.digest_hex(),
                    "resolved_spec": spec.as_json(),
                },
                "runtime": runtime,
                "reference": {"sorted_directional_values": vals},
            }
            return attach_digest(body)

        a = doc_for("aa" * 32, [1.0, 2.0])
        b = doc_for("bb" * 32, [1.0, 3.0])
        self.assertEqual(a["rank"]["rank_spec_digest"], b["rank"]["rank_spec_digest"])
        self.assertNotEqual(a["content_digest"], b["content_digest"])

    def test_unsorted_reader_refuses(self):
        spec = pinned_rank_spec()
        td = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, td)
        path = os.path.join(td, "u.emprank")
        body = {
            "format_version": FORMAT,
            "source": {"oof_logits_content_digest": "aa" * 32, "source_row_count": 2},
            "rank": {
                "rank_logic_version": spec.logic,
                "rank_spec_digest": spec.digest_hex(),
                "resolved_spec": spec.as_json(),
            },
            "runtime": {"python": "3.9", "numpy": "1.26.4"},
            "reference": {"sorted_directional_values": [2.0, 1.0]},
        }
        write_rank_atomic(path, attach_digest(body))
        with self.assertRaises(ValueError):
            read_rank(path)


class TestCanonical(unittest.TestCase):
    def test_canonical_logits(self):
        cands = list(LOGITS_DIR.glob("BINANCE_*.ooflogits")) if LOGITS_DIR.exists() else []
        cands = [p for p in cands if "rejected" not in p.name]
        if not cands:
            self.skipTest("canonical oof-logits-v1 not present")
        path = str(cands[0])
        from research.modelfit.logits import read_oof_logits

        header, rows, ft = read_oof_logits(path)
        if ft["content_digest"] != EXPECT_LOGITS:
            self.skipTest("canonical logits digest is not the expected snapshot")
        self.assertEqual(header["class_order"], list(CLASS_ORDER))
        z = np.array([r["logits"] for r in rows], dtype=np.float64)
        d = z[:, 0] - z[:, 1]
        self.assertTrue(np.all(np.isfinite(d)))
        RANK_DIR.mkdir(parents=True, exist_ok=True)
        spec = pinned_rank_spec()
        from research.rank.run import rank_filename

        out = RANK_DIR / rank_filename(parse_digest_hex(EXPECT_LOGITS), spec.digest())
        doc, _ = generate_rank(path, EXPECT_LOGITS, str(out))
        self.assertEqual(doc["format_version"], FORMAT)
        self.assertEqual(doc["source"]["oof_logits_content_digest"], EXPECT_LOGITS)
        self.assertEqual(doc["source"]["source_row_count"], len(rows))
        vals = doc["reference"]["sorted_directional_values"]
        self.assertEqual(len(vals), len(rows))
        for i in range(len(vals) - 1):
            self.assertLessEqual(vals[i], vals[i + 1])


if __name__ == "__main__":
    unittest.main()
