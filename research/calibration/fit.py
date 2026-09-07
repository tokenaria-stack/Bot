"""Train one global β from OOF (y, logits) only. No X, no folds."""

from __future__ import annotations

import inspect
import math
from dataclasses import dataclass

import numpy as np
from scipy.optimize import OptimizeResult, minimize

from .objective import nll_and_grad
from .spec import CalibrationSpec


class CalibrationError(RuntimeError):
    pass


@dataclass
class TemperatureFit:
    beta: float
    success: bool
    iterations: int
    gradient_norm: float
    objective: float


def fit_temperature(y: np.ndarray, logits: np.ndarray, spec: CalibrationSpec) -> TemperatureFit:
    """Fit inverse temperature on all OOF rows. Must not accept X or folds."""
    y = np.asarray(y, dtype=np.int64)
    logits = np.asarray(logits, dtype=np.float64)
    if logits.ndim != 2 or logits.shape[1] != 3 or y.shape[0] != logits.shape[0]:
        raise CalibrationError("calibration: y/logits shape mismatch")
    present = set(int(v) for v in np.unique(y))
    if present != {0, 1, 2}:
        raise CalibrationError(f"calibration: classes {sorted(present)} != {{0,1,2}}")
    if spec.method != "L-BFGS-B":
        raise CalibrationError("calibration: unknown optimizer method")

    def obj(x):
        nll, g = nll_and_grad(float(x[0]), y, logits)
        return nll, np.array([g], dtype=np.float64)

    result: OptimizeResult = minimize(
        obj,
        np.array([spec.initial_beta], dtype=np.float64),
        jac=True,
        method="L-BFGS-B",
        bounds=[(0.0, None)],
        options={"maxiter": spec.max_iter, "gtol": spec.gtol, "ftol": spec.ftol},
    )
    if not bool(result.success):
        raise CalibrationError(f"calibration: optimizer failed: {result.message}")
    beta = float(result.x[0])
    jac = np.asarray(result.jac, dtype=np.float64).reshape(-1)
    gnorm = float(np.abs(jac[0])) if jac.size else float("nan")
    nll0, _ = nll_and_grad(beta, y, logits)
    objective = float(result.fun) if result.fun is not None else nll0
    nit = result.nit
    if nit is None:
        raise CalibrationError("calibration: missing iteration count")
    _validate_fit(beta, objective, gnorm, int(nit))
    return TemperatureFit(
        beta=beta,
        success=True,
        iterations=int(nit),
        gradient_norm=gnorm,
        objective=objective,
    )


def _validate_fit(beta: float, objective: float, gnorm: float, nit: int) -> None:
    if not math.isfinite(beta) or beta < 0:
        raise CalibrationError("calibration: illegal beta")
    if not math.isfinite(objective) or not math.isfinite(gnorm):
        raise CalibrationError("calibration: nonfinite optimizer audit")
    if nit < 0:
        raise CalibrationError("calibration: illegal iteration count")


FIT_PARAMS = tuple(inspect.signature(fit_temperature).parameters)
