"""build_rank_reference(logits, spec) and scalar query_rank(reference, d)."""

from __future__ import annotations

import math
from dataclasses import dataclass

import numpy as np

from .spec import RankSpec

BUILD_PARAMS = ("logits", "spec")
QUERY_PARAMS = ("reference", "d")


class RankError(ValueError):
    pass


@dataclass(frozen=True)
class RankReference:
    sorted_d: np.ndarray
    spec: RankSpec

    @property
    def n(self) -> int:
        return int(self.sorted_d.shape[0])


def build_rank_reference(logits: np.ndarray, spec: RankSpec) -> RankReference:
    if spec.dtype != "float64":
        raise RankError("rank: dtype must be float64")
    z = np.asarray(logits, dtype=np.float64)
    if z.ndim != 2 or z.shape[1] != 3:
        raise RankError("rank: logits must have shape (N, 3)")
    n = int(z.shape[0])
    if n <= 0:
        raise RankError("rank: empty reference")
    d = z[:, 0] - z[:, 1]
    if not np.all(np.isfinite(d)):
        raise RankError("rank: nonfinite directional evidence")
    sorted_d = np.sort(d)
    return RankReference(sorted_d=sorted_d, spec=spec)


def query_rank(reference: RankReference, d: float) -> float:
    n = reference.n
    if n <= 0:
        raise RankError("rank: empty reference")
    x = float(d)
    if not math.isfinite(x):
        raise RankError("rank: nonfinite query")
    arr = reference.sorted_d
    left = int(np.searchsorted(arr, x, side="left"))
    right = int(np.searchsorted(arr, x, side="right"))
    less = left
    equal = right - left
    u = (less + 0.5 * equal) / float(n)
    return float(2.0 * u - 1.0)
