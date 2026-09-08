"""Portable standardize-then-ordered-affine logits. Application only."""

from __future__ import annotations

import math
from typing import Sequence, Tuple


class BundleError(ValueError):
    pass


def portable_raw_logits(
    values: Sequence[float],
    mean: Sequence[float],
    scale: Sequence[float],
    coef: Sequence[Sequence[float]],
    intercept: Sequence[float],
) -> Tuple[float, float, float]:
    f = len(mean)
    if len(values) != f or len(scale) != f:
        raise BundleError("bundle: feature width mismatch")
    if len(coef) != 3 or len(intercept) != 3:
        raise BundleError("bundle: logit layout")
    if any(len(row) != f for row in coef):
        raise BundleError("bundle: coef width")
    x = []
    for v in values:
        fv = float(v)
        if not math.isfinite(fv):
            raise BundleError("bundle: nonfinite feature")
        x.append(fv)
    scaled = []
    for j in range(f):
        delta = x[j] - float(mean[j])
        if not math.isfinite(delta):
            raise BundleError("bundle: nonfinite delta")
        sj = delta / float(scale[j])
        if not math.isfinite(sj):
            raise BundleError("bundle: nonfinite scaled")
        scaled.append(sj)
    logits = []
    for k in range(3):
        s = float(intercept[k])
        row = coef[k]
        for j in range(f):
            s = s + float(row[j]) * scaled[j]
            if not math.isfinite(s):
                raise BundleError("bundle: nonfinite logit")
        logits.append(s)
    return (logits[0], logits[1], logits[2])
