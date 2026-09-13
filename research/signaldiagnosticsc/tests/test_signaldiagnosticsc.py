import unittest

import numpy as np

from research.recipe.project import calibrated_probabilities
from research.signaldiagnosticsc.tables import (
    COVERAGES,
    directional_d,
    q_from_p,
    softmax_beta1,
    tail_indices,
    target_c_utility_up,
)


class TestHygiene(unittest.TestCase):
    def test_no_ol1c_shap_strategy(self):
        from pathlib import Path
        pkg = Path(__file__).resolve().parents[1]
        text = "\n".join(p.read_text(encoding="utf-8") for p in pkg.glob("*.py"))
        self.assertNotIn("read_oof_logits", text)
        self.assertNotIn("import shap", text.lower())
        self.assertNotIn("eligible_for_finalization", text)
        self.assertNotIn("P_TP", text)
        self.assertEqual(COVERAGES, (50, 20, 10, 5, 2, 1))


class TestLaws(unittest.TestCase):
    def test_q_is_monotone_readout_of_d(self):
        rng = np.random.default_rng(0)
        z = rng.normal(size=(200, 3))
        d = directional_d(z)
        p = softmax_beta1(z)
        q = q_from_p(p)
        spear = float(np.corrcoef(d.argsort().argsort(), q.argsort().argsort())[0, 1])
        self.assertGreater(spear, 0.999)
        for i, row in enumerate(z):
            p1 = calibrated_probabilities(tuple(row), 1.0)
            self.assertAlmostEqual(q[i], p1[0] / (p1[0] + p1[1]))

    def test_tail_is_deterministic_and_k_floor(self):
        s = np.array([0.0, 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0])
        idx = tail_indices(s, 20, high=True)
        self.assertEqual(sorted(idx.tolist()), [8, 9])
        idxb = tail_indices(s, 20, high=False)
        self.assertEqual(sorted(idxb.tolist()), [0, 1])

    def test_utility_always_up(self):
        y = np.array([0, 1, 2], dtype=np.int64)
        u = target_c_utility_up(y)
        self.assertEqual(u.tolist(), [2.0, -2.0, 0.0])


if __name__ == "__main__":
    unittest.main()
