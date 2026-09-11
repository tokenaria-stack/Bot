"""CatBoost executor. Does not own folds, classes, or iteration selection."""

from __future__ import annotations

import json
import math
import os
import sys
from typing import Any, Dict, List

import numpy as np
from catboost import CatBoostClassifier, Pool


OUTCOME_TO_Y = {"UP_FIRST": 0, "DOWN_FIRST": 1, "TIMEOUT": 2}


def _load_plan(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def _load_indexed_xy(matrix_path: str, expect_hex: str, indexes: List[int], y: List[int], feature_ids: List[str]):
    xs: List[List[float]] = []
    ys: List[int] = []
    want = set(indexes)
    by_i = {idx: n for n, idx in enumerate(indexes)}
    got_y = [None] * len(indexes)
    got_x = [None] * len(indexes)
    with open(matrix_path, "r", encoding="utf-8") as f:
        header = json.loads(f.readline())
        if header.get("format_version") != "oof-matrix-v1":
            raise RuntimeError("catboost1: unexpected matrix format")
        hdr_ids = header.get("feature_ids")
        if hdr_ids is not None and list(hdr_ids) != list(feature_ids):
            raise RuntimeError("catboost1: FeatureIDs order mismatch")
        i_row = 0
        for line in f:
            rec = json.loads(line)
            if rec.get("kind") == "footer":
                if rec.get("content_digest") != expect_hex:
                    raise RuntimeError("MATRIX_IDENTITY_MISMATCH")
                break
            if rec.get("kind") != "row":
                continue
            if i_row in want:
                feats = rec["features"]
                if len(feats) != len(feature_ids):
                    raise RuntimeError("catboost1: feature width")
                for v in feats:
                    if not math.isfinite(float(v)):
                        raise RuntimeError("catboost1: nonfinite X")
                pos = by_i[i_row]
                got_x[pos] = [float(v) for v in feats]
                oc = rec.get("outcome")
                if oc not in OUTCOME_TO_Y:
                    raise RuntimeError("CLASS_ORDER_MISMATCH")
                got_y[pos] = OUTCOME_TO_Y[oc]
            i_row += 1
    for i, yi in enumerate(y):
        if yi not in (0, 1, 2):
            raise RuntimeError("CLASS_ORDER_MISMATCH")
        if got_x[i] is None:
            raise RuntimeError("catboost1: missing row index")
        if got_y[i] != yi:
            raise RuntimeError("CLASS_ORDER_MISMATCH")
        ys.append(int(yi))
        xs.append(got_x[i])
    X = np.asarray(xs, dtype=np.float64)
    Y = np.asarray(ys, dtype=np.int64)
    if not np.isfinite(X).all():
        raise RuntimeError("catboost1: nonfinite X")
    return X, Y


def _param_match(key: str, want: Any, got: Any) -> bool:
    if isinstance(want, (int, float)) and isinstance(got, (int, float)):
        if key in ("learning_rate", "bayesian_matrix_reg"):
            return bool(np.float32(want) == np.float32(got))
        return float(want) == float(got)
    return want == got


def _assert_expect_params(model: CatBoostClassifier, plan: Dict[str, Any]) -> None:
    gp = model.get_params()
    if int(gp.get("thread_count") or 0) != 1:
        raise RuntimeError("CATBOOST_PARAM_MISMATCH thread_count")
    allp = model.get_all_params()
    expect = plan.get("expect_all_params") or {}
    for k, want in expect.items():
        got = allp.get(k)
        if not _param_match(k, want, got):
            raise RuntimeError("CATBOOST_PARAM_MISMATCH %s want=%r got=%r" % (k, want, got))


def fit_from_plan(plan_path: str) -> None:
    plan = _load_plan(plan_path)
    X, y = _load_indexed_xy(
        plan["matrix_path"],
        plan["expect_matrix"],
        plan["row_indexes"],
        plan["y"],
        plan["feature_ids"],
    )
    params = dict(plan["catboost_params"])
    params["iterations"] = int(plan["iterations"])
    if params.get("use_best_model"):
        raise RuntimeError("catboost1: use_best_model forbidden")
    if "eval_set" in params:
        raise RuntimeError("catboost1: eval_set forbidden")
    model = CatBoostClassifier(**params)
    pool = Pool(X, y, feature_names=list(plan["feature_ids"]))
    model.fit(pool)
    classes = [int(c) for c in model.classes_]
    if classes != [0, 1, 2]:
        raise RuntimeError("CLASS_ORDER_MISMATCH")
    if int(model.tree_count_) != int(plan["iterations"]):
        raise RuntimeError("catboost1: tree_count != iterations")
    _assert_expect_params(model, plan)
    os.makedirs(os.path.dirname(plan["out_json"]) or ".", exist_ok=True)
    model.save_model(plan["out_json"], format="json")
    if plan.get("out_cbm"):
        model.save_model(plan["out_cbm"], format="cbm")
    witness = {
        "classes": classes,
        "tree_count": int(model.tree_count_),
        "get_params": model.get_params(),
        "get_all_params": model.get_all_params(),
        "get_all_params_thread_count": model.get_all_params().get("thread_count"),
    }
    with open(plan["out_witness"], "w", encoding="utf-8") as f:
        json.dump(witness, f, default=str)
    print("catboost1 fit ok trees=%d classes=%s" % (model.tree_count_, classes), file=sys.stderr)


def main(argv: List[str]) -> int:
    if len(argv) != 2 or argv[0] != "fit":
        print("usage: python -m research.catboost1 fit <plan.json>", file=sys.stderr)
        return 2
    fit_from_plan(argv[1])
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
