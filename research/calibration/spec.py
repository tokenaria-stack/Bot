"""CalibrationSpec-v1 and CalibrationSpecDigest (rules only)."""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Any, Dict, Tuple

from research.modelfit.spec import CLASS_ORDER

LOGIC_V1 = "calibration:temperature-scaling-v1"


@dataclass(frozen=True)
class CalibrationSpec:
    logic: str
    family: str
    canonical_parameter: str
    constraint: str
    transform: str
    objective: str
    weighting: str
    class_order: Tuple[str, ...]
    dtype: str
    optimizer: str
    method: str
    initial_beta: float
    max_iter: int
    gtol: float

    def payload(self) -> Dict[str, Any]:
        return {
            "canonical_parameter": self.canonical_parameter,
            "class_order": list(self.class_order),
            "constraint": self.constraint,
            "dtype": self.dtype,
            "family": self.family,
            "gtol": self.gtol,
            "initial_beta": self.initial_beta,
            "logic": self.logic,
            "max_iter": self.max_iter,
            "method": self.method,
            "objective": self.objective,
            "optimizer": self.optimizer,
            "transform": self.transform,
            "weighting": self.weighting,
        }

    def digest(self) -> bytes:
        blob = json.dumps(self.payload(), sort_keys=True, separators=(",", ":"), ensure_ascii=True)
        return hashlib.sha256(blob.encode("utf-8")).digest()

    def digest_hex(self) -> str:
        return self.digest().hex()

    def as_json(self) -> Dict[str, Any]:
        return self.payload()


def pinned_calibration_spec() -> CalibrationSpec:
    return CalibrationSpec(
        logic=LOGIC_V1,
        family="scalar_temperature",
        canonical_parameter="inverse_temperature_beta",
        constraint="beta>=0",
        transform="softmax(beta*logits)",
        objective="mean_multiclass_nll",
        weighting="uniform_rows",
        class_order=CLASS_ORDER,
        dtype="float64",
        optimizer="scipy.optimize.minimize",
        method="L-BFGS-B",
        initial_beta=1.0,
        max_iter=1000,
        gtol=1e-12,
    )


def spec_from_json(d: Dict[str, Any]) -> CalibrationSpec:
    return CalibrationSpec(
        logic=d["logic"],
        family=d["family"],
        canonical_parameter=d["canonical_parameter"],
        constraint=d["constraint"],
        transform=d["transform"],
        objective=d["objective"],
        weighting=d["weighting"],
        class_order=tuple(d["class_order"]),
        dtype=d["dtype"],
        optimizer=d["optimizer"],
        method=d["method"],
        initial_beta=float(d["initial_beta"]),
        max_iter=int(d["max_iter"]),
        gtol=float(d["gtol"]),
    )
