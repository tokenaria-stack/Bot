"""Numeric CatBoost fitter. No folds, class names, or trading semantics."""

from __future__ import annotations

import json
import sys
from typing import Any, Dict, List

import numpy as np
from catboost import CatBoostClassifier, Pool


def _load(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def _load_xy(x_path: str, y_path: str):
    with open(x_path, "r", encoding="utf-8") as f:
        X = np.asarray(json.load(f), dtype=np.float64)
    with open(y_path, "r", encoding="utf-8") as f:
        y = np.asarray(json.load(f), dtype=np.int64)
    if X.ndim != 2 or y.ndim != 1 or len(X) != len(y):
        raise RuntimeError("brain3.fit: X/y shape")
    if not np.isfinite(X).all():
        raise RuntimeError("brain3.fit: nonfinite X")
    if y.min() < 0 or y.max() > 2:
        raise RuntimeError("brain3.fit: y out of 0..2")
    return X, y


def _loss(params: Dict[str, Any]) -> str:
    return str(params.get("loss_function") or params.get("loss") or "")


def fit(plan: Dict[str, Any]) -> None:
    X, y = _load_xy(plan["x_path"], plan["y_path"])
    params = dict(plan["catboost_params"])
    params["iterations"] = int(plan["iterations"])
    if params.get("use_best_model"):
        raise RuntimeError("brain3.fit: use_best_model forbidden")
    if "eval_set" in params:
        raise RuntimeError("brain3.fit: eval_set forbidden")
    names = [str(n) for n in plan["feature_names"]]
    if len(names) != X.shape[1]:
        raise RuntimeError("brain3.fit: width")
    loss = _loss(params)
    model = CatBoostClassifier(**params)
    model.fit(Pool(X, y, feature_names=names))
    classes = [int(c) for c in model.classes_]
    if loss == "Logloss":
        if classes != [0, 1]:
            raise RuntimeError("brain3.fit: binary classes != [0,1]")
        if int(y.max()) > 1:
            raise RuntimeError("brain3.fit: binary y has class > 1")
    elif loss == "MultiClass":
        if classes != [0, 1, 2]:
            raise RuntimeError("brain3.fit: classes != [0,1,2]")
    else:
        raise RuntimeError("brain3.fit: unknown loss %s" % loss)
    if int(model.tree_count_) != int(plan["iterations"]):
        raise RuntimeError("brain3.fit: tree_count")
    model.save_model(plan["out_json"], format="json")
    wit = {
        "tree_count": int(model.tree_count_),
        "classes": classes,
        "loss": loss,
        "catboost_version": getattr(model, "__module__", "catboost"),
        "get_params": model.get_params(),
    }
    with open(plan["out_witness"], "w", encoding="utf-8") as f:
        json.dump(wit, f, default=str)
    print("brain3.fit ok trees=%d loss=%s" % (model.tree_count_, loss), file=sys.stderr)


def predict(plan: Dict[str, Any]) -> None:
    X, _ = _load_xy(plan["x_path"], plan["y_path"])
    model = CatBoostClassifier()
    model.load_model(plan["model_json"], format="json")
    raw = model.predict(X, prediction_type="RawFormulaVal")
    arr = np.asarray(raw, dtype=np.float64)
    proba = None
    if arr.ndim == 1:
        proba = np.asarray(model.predict_proba(X), dtype=np.float64)
        if proba.ndim != 2 or proba.shape[1] != 2:
            raise RuntimeError("brain3.fit: binary predict_proba shape")
        out = {"raw": arr.tolist(), "p1": proba[:, 1].tolist()}
    else:
        out = arr.tolist()
    with open(plan["out_pred"], "w", encoding="utf-8") as f:
        json.dump(out, f)


def main(argv: List[str]) -> int:
    if len(argv) != 2 or argv[0] not in ("fit", "predict"):
        print("usage: python research/brain3/fit.py fit|predict <plan.json>", file=sys.stderr)
        return 2
    plan = _load(argv[1])
    if argv[0] == "fit":
        fit(plan)
    else:
        predict(plan)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
