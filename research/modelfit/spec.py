"""Pinned ModelSpec-v1 and ModelSpecDigest (rules only)."""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Any, Dict, List, Tuple

LOGIC_V1 = "model-fit:multinomial-logistic-v1"

CLASS_UP = "UP_FIRST"
CLASS_DOWN = "DOWN_FIRST"
CLASS_TIMEOUT = "TIMEOUT"
CLASS_ORDER: Tuple[str, ...] = (CLASS_UP, CLASS_DOWN, CLASS_TIMEOUT)
CODE_BY_OUTCOME = {CLASS_UP: 0, CLASS_DOWN: 1, CLASS_TIMEOUT: 2}
OUTCOME_BY_CODE = {0: CLASS_UP, 1: CLASS_DOWN, 2: CLASS_TIMEOUT}


@dataclass(frozen=True)
class ModelSpec:
    logic: str
    preprocessor: str
    with_mean: bool
    with_std: bool
    penalty: str
    C: float
    fit_intercept: bool
    solver: str
    tol: float
    max_iter: int
    class_weight: None
    warm_start: bool
    dtype: str
    class_order: Tuple[str, ...]

    def payload(self) -> Dict[str, Any]:
        return {
            "C": self.C,
            "class_order": list(self.class_order),
            "class_weight": self.class_weight,
            "dtype": self.dtype,
            "fit_intercept": self.fit_intercept,
            "logic": self.logic,
            "max_iter": self.max_iter,
            "penalty": self.penalty,
            "preprocessor": self.preprocessor,
            "solver": self.solver,
            "tol": self.tol,
            "warm_start": self.warm_start,
            "with_mean": self.with_mean,
            "with_std": self.with_std,
        }

    def digest(self) -> bytes:
        blob = json.dumps(self.payload(), sort_keys=True, separators=(",", ":"), ensure_ascii=True)
        return hashlib.sha256(blob.encode("utf-8")).digest()

    def digest_hex(self) -> str:
        return self.digest().hex()

    def as_json(self) -> Dict[str, Any]:
        return self.payload()


def pinned_model_spec() -> ModelSpec:
    return ModelSpec(
        logic=LOGIC_V1,
        preprocessor="standard_scaler",
        with_mean=True,
        with_std=True,
        penalty="l2",
        C=1.0,
        fit_intercept=True,
        solver="lbfgs",
        tol=1e-9,
        max_iter=2000,
        class_weight=None,
        warm_start=False,
        dtype="float64",
        class_order=CLASS_ORDER,
    )


def spec_from_json(d: Dict[str, Any]) -> ModelSpec:
    order = tuple(d["class_order"])
    return ModelSpec(
        logic=d["logic"],
        preprocessor=d["preprocessor"],
        with_mean=d["with_mean"],
        with_std=d["with_std"],
        penalty=d["penalty"],
        C=float(d["C"]),
        fit_intercept=d["fit_intercept"],
        solver=d["solver"],
        tol=float(d["tol"]),
        max_iter=int(d["max_iter"]),
        class_weight=d["class_weight"],
        warm_start=d["warm_start"],
        dtype=d["dtype"],
        class_order=order,
    )
