"""Read-only Target-C ranking tables from frozen CatBoost OOF logits."""

from __future__ import annotations

from typing import Any, Dict, List, Optional, Sequence, Tuple

import numpy as np

from research.calibration.spec import pinned_calibration_spec
from research.catboostlogits.reader import CatBoostOOFLogits
from research.decisionresearchc.project import project_decision_fold
from research.decisionresearchc.specs import decision_fold_rank_spec
from research.recipe.project import calibrated_probabilities

COVERAGES = (50, 20, 10, 5, 2, 1)
U = 2.0
L = 2.0
EU_EDGES = tuple(U * d / 10.0 for d in range(0, 11))  # 0.0, 0.2, ..., 2.0
RANK_GATE = 0.8  # dr=8
POPULATION = "all_oof"

BASELINE_RANK_IDS = ("rsx_value", "rsx_delta_1")
BASELINE_EVENT_IDS = ("tv_bull_present", "tv_bear_present", "rsx_50_cross_up_present", "rsx_50_cross_down_present")


def directional_d(logits: np.ndarray) -> np.ndarray:
    z = np.asarray(logits, dtype=np.float64)
    return z[:, 0] - z[:, 1]


def softmax_beta1(logits: np.ndarray) -> np.ndarray:
    """β=1 readout of the same logits. q ranking ≡ D ranking. Not causal calibration."""
    n = logits.shape[0]
    p = np.empty((n, 3), dtype=np.float64)
    for i in range(n):
        p[i, :] = calibrated_probabilities(tuple(float(x) for x in logits[i]), 1.0)
    return p


def q_from_p(p: np.ndarray) -> np.ndarray:
    den = p[:, 0] + p[:, 1]
    q = np.full(p.shape[0], 0.5, dtype=np.float64)
    ok = den > 0
    q[ok] = p[ok, 0] / den[ok]
    return q


def a_from_p(p: np.ndarray) -> np.ndarray:
    return p[:, 0] + p[:, 1]


def target_c_utility_up(y: np.ndarray) -> np.ndarray:
    """Always-UP intent on each row. +U UP_FIRST, −L DOWN_FIRST, 0 TIMEOUT."""
    u = np.zeros(y.shape[0], dtype=np.float64)
    u[y == 0] = U
    u[y == 1] = -L
    return u


def target_c_utility_down(y: np.ndarray) -> np.ndarray:
    u = np.zeros(y.shape[0], dtype=np.float64)
    u[y == 1] = L
    u[y == 0] = -U
    return u


def _k(n: int, pct: int) -> int:
    k = int(np.floor(n * (pct / 100.0)))
    return max(k, 1 if n else 0)


def tail_indices(score: np.ndarray, pct: int, high: bool) -> np.ndarray:
    n = int(score.shape[0])
    k = _k(n, pct)
    if k <= 0:
        return np.zeros(0, dtype=np.int64)
    order = np.argsort(score, kind="mergesort")
    if high:
        return order[-k:]
    return order[:k]


def summarize_side(y: np.ndarray, idx: np.ndarray, side: str, p: Optional[np.ndarray] = None) -> Dict[str, Any]:
    n_all = int(y.shape[0])
    sel = np.asarray(y)[idx]
    n = int(sel.shape[0])
    up = int(np.sum(sel == 0))
    down = int(np.sum(sel == 1))
    to = int(np.sum(sel == 2))
    if side == "bull":
        intended, reverse = up, down
        util = float(np.sum(target_c_utility_up(sel)))
    elif side == "bear":
        intended, reverse = down, up
        util = float(np.sum(target_c_utility_down(sel)))
    else:
        raise ValueError(side)
    row: Dict[str, Any] = {
        "n": n,
        "coverage": (n / n_all) if n_all else 0.0,
        "up_first": up,
        "down_first": down,
        "timeout": to,
        "up_first_rate": up / n if n else float("nan"),
        "down_first_rate": down / n if n else float("nan"),
        "timeout_rate": to / n if n else float("nan"),
        "intended_hit_rate": intended / n if n else float("nan"),
        "reverse_hit_rate": reverse / n if n else float("nan"),
        "utility_total": util,
        "utility_per_obs": util / n if n else float("nan"),
    }
    if p is not None and n:
        pp = p[idx]
        row["mean_p_up"] = float(np.mean(pp[:, 0]))
        row["mean_p_down"] = float(np.mean(pp[:, 1]))
        row["mean_p_timeout"] = float(np.mean(pp[:, 2]))
        row["mean_q"] = float(np.mean(q_from_p(pp)))
        row["mean_a"] = float(np.mean(a_from_p(pp)))
    return row


def population_row(y: np.ndarray, p: Optional[np.ndarray] = None) -> Dict[str, Any]:
    n = int(y.shape[0])
    up = int(np.sum(y == 0))
    down = int(np.sum(y == 1))
    to = int(np.sum(y == 2))
    row = {
        "label": POPULATION,
        "n": n,
        "coverage": 1.0,
        "up_first": up,
        "down_first": down,
        "timeout": to,
        "up_first_rate": up / n,
        "down_first_rate": down / n,
        "timeout_rate": to / n,
    }
    if p is not None:
        row["mean_p_up"] = float(np.mean(p[:, 0]))
        row["mean_p_down"] = float(np.mean(p[:, 1]))
        row["mean_p_timeout"] = float(np.mean(p[:, 2]))
        row["mean_q"] = float(np.mean(q_from_p(p)))
        row["mean_a"] = float(np.mean(a_from_p(p)))
    return row


def coverage_table(d: np.ndarray, y: np.ndarray, p: np.ndarray, side: str) -> List[Dict[str, Any]]:
    high = side == "bull"
    rows = []
    for pct in COVERAGES:
        idx = tail_indices(d, pct, high=high)
        rec = summarize_side(y, idx, side, p)
        rec["coverage_pct"] = pct
        rec["side"] = side
        rec["score"] = "D"
        rows.append(rec)
    return rows


def catboost_fold_slices(header: Dict[str, Any], n: int) -> List[Tuple[int, int]]:
    folds = header.get("folds") or []
    out = []
    for f in folds:
        b = int(f["output_begin"])
        e = int(f["output_end"])
        if b < 0 or e > n or b >= e:
            raise RuntimeError(f"signaldiagnosticsc: bad CatBoost output range [{b}:{e}] n={n}")
        out.append((b, e))
    if not out:
        raise RuntimeError("signaldiagnosticsc: logits header has no folds")
    return out


def fold_coverage_tables(d: np.ndarray, y: np.ndarray, p: np.ndarray, slices: List[Tuple[int, int]], side: str) -> List[Dict[str, Any]]:
    rows = []
    for i, (b, e) in enumerate(slices):
        sub = coverage_table(d[b:e], y[b:e], p[b:e], side)
        for rec in sub:
            rec = dict(rec)
            rec["oof_fold"] = i
            rec["fold_n"] = e - b
            rows.append(rec)
    return rows


def event_baseline(y: np.ndarray, present: np.ndarray, side: str, name: str) -> Dict[str, Any]:
    idx = np.flatnonzero(np.asarray(present) > 0.5)
    rec = summarize_side(y, idx, side)
    rec["baseline"] = name
    rec["kind"] = "event"
    rec["side"] = side
    return rec


def rank_baseline(score: np.ndarray, y: np.ndarray, side: str, name: str, p: Optional[np.ndarray] = None) -> List[Dict[str, Any]]:
    high = side == "bull"
    rows = []
    for pct in COVERAGES:
        idx = tail_indices(score, pct, high=high)
        rec = summarize_side(y, idx, side, p)
        rec["coverage_pct"] = pct
        rec["baseline"] = name
        rec["kind"] = "rank"
        rec["side"] = side
        rows.append(rec)
    return rows


def _eu(p_up: float, p_down: float) -> float:
    return U * p_up - L * p_down


def autopsy_bands(rows: List[Dict[str, Any]], era: str, fold: int) -> List[Dict[str, Any]]:
    """Among |rank|>=0.8, band |EU| on existing du grid. Descriptive only."""
    out = []
    for side, rank_ok, eu_signed in (
        ("bull", lambda r: r["directional_rank"] >= RANK_GATE, 1.0),
        ("bear", lambda r: r["directional_rank"] <= -RANK_GATE, -1.0),
    ):
        picked = [r for r in rows if rank_ok(r)]
        for i in range(len(EU_EDGES) - 1):
            lo, hi = EU_EDGES[i], EU_EDGES[i + 1]
            bucket = []
            for r in picked:
                p = r["probabilities"]
                mag = eu_signed * _eu(p[0], p[1])
                last = i == len(EU_EDGES) - 2
                if (mag >= lo and mag < hi) or (last and mag >= lo):
                    bucket.append(r)
            y = np.array(
                [0 if x["outcome"] == "UP_FIRST" else 1 if x["outcome"] == "DOWN_FIRST" else 2 for x in bucket],
                dtype=np.int64,
            )
            idx = np.arange(len(bucket), dtype=np.int64)
            rec = summarize_side(y, idx, side) if len(bucket) else {
                "n": 0, "coverage": float("nan"), "up_first": 0, "down_first": 0, "timeout": 0,
                "up_first_rate": float("nan"), "down_first_rate": float("nan"), "timeout_rate": float("nan"),
                "intended_hit_rate": float("nan"), "reverse_hit_rate": float("nan"),
                "utility_total": 0.0, "utility_per_obs": float("nan"),
            }
            rec["era"] = era
            rec["decision_fold"] = fold
            rec["side"] = side
            rec["rank_gate"] = RANK_GATE
            rec["eu_abs_lo"] = lo
            rec["eu_abs_hi"] = hi
            rec["fires_du1"] = lo >= 0.2 - 1e-12
            out.append(rec)
    return out


def autopsy_from_plan(logits: CatBoostOOFLogits, plan: Dict[str, Any]) -> List[Dict[str, Any]]:
    cal = pinned_calibration_spec()
    rank = decision_fold_rank_spec()
    rows: List[Dict[str, Any]] = []
    for i, fr in enumerate(plan["compiled"]["folds"]):
        proj = project_decision_fold(
            logits.logits, logits.y, logits.outcome,
            int(fr["train_begin"]), int(fr["train_end"]),
            int(fr["val_begin"]), int(fr["val_end"]),
            cal, rank, logits.spec_digest,
        )
        rows.extend(autopsy_bands(proj.train_rows, "train", i))
        rows.extend(autopsy_bands(proj.val_rows, "val", i))
    return rows


def join_matrix_features(logits_at: np.ndarray, matrix_at: np.ndarray, x: np.ndarray, ids: Sequence[str]) -> Dict[str, np.ndarray]:
    pos = {name: i for i, name in enumerate(ids)}
    missing = [c for c in BASELINE_RANK_IDS + BASELINE_EVENT_IDS if c not in pos]
    if missing:
        raise RuntimeError(f"signaldiagnosticsc: missing FeatureIDs {missing}")
    ix = np.searchsorted(matrix_at, logits_at)
    if np.any(ix < 0) or np.any(ix >= matrix_at.shape[0]) or not np.array_equal(matrix_at[ix], logits_at):
        raise RuntimeError("signaldiagnosticsc: logits At[] is not a subset of OOF-MATRIX-C At[]")
    out = {}
    for name in BASELINE_RANK_IDS + BASELINE_EVENT_IDS:
        out[name] = np.asarray(x[ix, pos[name]], dtype=np.float64)
    return out
