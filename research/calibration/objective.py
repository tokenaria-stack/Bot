"""Mean multiclass NLL and analytic dL/dβ. Production uses this only."""

from __future__ import annotations

import math
from typing import Tuple

import numpy as np
from scipy.special import logsumexp


def nll_and_grad(beta: float, y: np.ndarray, logits: np.ndarray) -> Tuple[float, float]:
    """L(β)=mean[logsumexp(βz) − β z_y]; dL/dβ=mean[∑ p_k z_k − z_y]."""
    if not math.isfinite(beta):
        raise ValueError("calibration: nonfinite beta")
    z = np.asarray(logits, dtype=np.float64)
    y = np.asarray(y, dtype=np.int64)
    if z.ndim != 2 or z.shape[1] != 3 or y.shape != (z.shape[0],):
        raise ValueError("calibration: y/logits shape mismatch")
    scaled = beta * z
    lse = logsumexp(scaled, axis=1)
    nll = float(np.mean(lse - beta * z[np.arange(z.shape[0]), y]))
    log_p = scaled - lse[:, None]
    p = np.exp(log_p)
    g = float(np.mean(np.sum(p * z, axis=1) - z[np.arange(z.shape[0]), y]))
    return nll, g
