"""Fold-local causal projection using existing calibration/rank/recipe math."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Dict, List, Tuple

import numpy as np

from research.calibration.fit import fit_temperature
from research.calibration.spec import CalibrationSpec
from research.modelfit.spec import CLASS_ORDER
from research.rank.reference import build_rank_reference
from research.rank.spec import RankSpec
from research.recipe.project import LoadedForecastRecipe, project_forecast


@dataclass(frozen=True)
class FoldProjection:
    beta: float
    rank_reference_n: int
    train_rows: List[Dict[str, Any]]
    val_rows: List[Dict[str, Any]]


def project_decision_fold(
    logits: np.ndarray,
    y: np.ndarray,
    outcomes,
    train_begin: int,
    train_end: int,
    val_begin: int,
    val_end: int,
    cal_spec: CalibrationSpec,
    rank_spec: RankSpec,
    model_spec_digest: str,
) -> FoldProjection:
    if train_end <= train_begin or val_end <= val_begin:
        raise RuntimeError("decisionresearchc: empty train or val")
    z_tr = np.asarray(logits[train_begin:train_end], dtype=np.float64)
    y_tr = np.asarray(y[train_begin:train_end], dtype=np.int64)
    fit = fit_temperature(y_tr, z_tr, cal_spec)
    ref = build_rank_reference(z_tr, rank_spec)
    loaded = LoadedForecastRecipe(
        beta=fit.beta,
        rank_reference=ref,
        class_order=CLASS_ORDER,
        model_spec_digest=model_spec_digest,
    )
    train_rows = _project_slice(logits, outcomes, train_begin, train_end, loaded)
    val_rows = _project_slice(logits, outcomes, val_begin, val_end, loaded)
    return FoldProjection(
        beta=float(fit.beta),
        rank_reference_n=int(ref.n),
        train_rows=train_rows,
        val_rows=val_rows,
    )


def _project_slice(logits, outcomes, begin: int, end: int, loaded: LoadedForecastRecipe) -> List[Dict[str, Any]]:
    rows: List[Dict[str, Any]] = []
    for i in range(begin, end):
        ev = project_forecast(logits[i], loaded)
        rows.append({
            "probabilities": list(ev.probabilities),
            "directional_rank": float(ev.directional_rank),
            "outcome": str(outcomes[i]),
        })
    return rows


def selector_payload(proj: FoldProjection) -> Tuple[List[Dict[str, Any]], List[Dict[str, int]]]:
    n_tr = len(proj.train_rows)
    n_va = len(proj.val_rows)
    rows = list(proj.train_rows) + list(proj.val_rows)
    folds = [{
        "train_begin": 0,
        "train_end": n_tr,
        "val_begin": n_tr,
        "val_end": n_tr + n_va,
    }]
    return rows, folds
