"""Train-only sklearn fit; validation-only decision_function."""

from __future__ import annotations

import warnings
from dataclasses import dataclass
from typing import Any

import numpy as np
from sklearn.exceptions import ConvergenceWarning
from sklearn.linear_model import LogisticRegression
from sklearn.preprocessing import StandardScaler

from .spec import CLASS_ORDER, ModelSpec


class FitError(RuntimeError):
    pass


@dataclass
class FittedFold:
    scaler: Any
    model: Any
    mean: np.ndarray
    scale: np.ndarray
    coef: np.ndarray
    intercept: np.ndarray
    n_iter: int


def _require_finite(name: str, arr: np.ndarray) -> None:
    if not np.isfinite(arr).all():
        raise FitError(f"modelfit: nonfinite {name}")


def fit_fold_model(x_train: np.ndarray, y_train: np.ndarray, spec: ModelSpec) -> FittedFold:
    """Fit scaler + logistic on TRAIN only. Must not accept X_val or y_val."""
    if spec.logic != "model-fit:multinomial-logistic-v1":
        raise FitError("modelfit: unknown ModelLogicVersion")
    x_train = np.asarray(x_train, dtype=np.float64)
    y_train = np.asarray(y_train, dtype=np.int8)
    if x_train.ndim != 2 or y_train.ndim != 1 or x_train.shape[0] != y_train.shape[0]:
        raise FitError("modelfit: train shape mismatch")
    present = set(int(v) for v in np.unique(y_train))
    if present != {0, 1, 2}:
        raise FitError(f"modelfit: train classes {sorted(present)} != {{0,1,2}}")
    scaler = StandardScaler(with_mean=spec.with_mean, with_std=spec.with_std)
    scaler.fit(x_train)
    x_scaled = scaler.transform(x_train)
    model = LogisticRegression(
        penalty=spec.penalty,
        C=spec.C,
        fit_intercept=spec.fit_intercept,
        solver=spec.solver,
        tol=spec.tol,
        max_iter=spec.max_iter,
        class_weight=spec.class_weight,
        warm_start=spec.warm_start,
    )
    with warnings.catch_warnings():
        warnings.simplefilter("error", ConvergenceWarning)
        try:
            model.fit(x_scaled, y_train)
        except ConvergenceWarning as e:
            raise FitError("modelfit: ConvergenceWarning") from e
    classes = [int(c) for c in model.classes_]
    if classes != [0, 1, 2]:
        raise FitError(f"modelfit: classes_ {classes} != [0,1,2]")
    mean = np.asarray(scaler.mean_, dtype=np.float64).copy()
    scale = np.asarray(scaler.scale_, dtype=np.float64).copy()
    coef = np.asarray(model.coef_, dtype=np.float64).copy()
    intercept = np.asarray(model.intercept_, dtype=np.float64).copy()
    n_iter = int(np.max(model.n_iter_))
    _require_finite("scaler.mean_", mean)
    _require_finite("scaler.scale_", scale)
    _require_finite("coef_", coef)
    _require_finite("intercept_", intercept)
    if coef.shape != (3, x_train.shape[1]) or intercept.shape != (3,):
        raise FitError("modelfit: unexpected coef/intercept shape")
    return FittedFold(
        scaler=scaler,
        model=model,
        mean=mean,
        scale=scale,
        coef=coef,
        intercept=intercept,
        n_iter=n_iter,
    )


def predict_fold_logits(fitted: FittedFold, x_val: np.ndarray) -> np.ndarray:
    """Validation X only. Must not accept y_val."""
    x_val = np.asarray(x_val, dtype=np.float64)
    if x_val.ndim != 2:
        raise FitError("modelfit: X_val must be 2d")
    x_scaled = fitted.scaler.transform(x_val)
    logits = np.asarray(fitted.model.decision_function(x_scaled), dtype=np.float64)
    if logits.ndim == 1:
        raise FitError("modelfit: expected multinomial logits")
    if logits.shape != (x_val.shape[0], 3):
        raise FitError(f"modelfit: logits shape {logits.shape} != [{x_val.shape[0]}, 3]")
    _require_finite("logits", logits)
    return logits
