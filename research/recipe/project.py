"""LoadedForecastRecipe and scalar project_forecast."""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Tuple

import numpy as np

from research.calibration.artifact import read_calibration
from research.modelfit.spec import CLASS_ORDER
from research.rank.artifact import read_rank
from research.rank.reference import RankReference, query_rank
from research.rank.spec import spec_from_json

from .artifact import read_recipe
from .env import require_recipe_runtime
from .laws import require_class_order

PROJECT_PARAMS = ("logits", "loaded_recipe")
LOAD_PARAMS = ("recipe_path", "calibration_path", "rank_path")


class RecipeError(ValueError):
    pass


@dataclass(frozen=True)
class ForecastEvidence:
    probabilities: Tuple[float, float, float]
    directional_rank: float


@dataclass(frozen=True)
class LoadedForecastRecipe:
    beta: float
    rank_reference: RankReference
    class_order: Tuple[str, ...]
    model_spec_digest: str


def load_forecast_recipe(recipe_path: str, calibration_path: str, rank_path: str) -> LoadedForecastRecipe:
    if LOAD_PARAMS != ("recipe_path", "calibration_path", "rank_path"):
        raise RuntimeError("recipe: load signature law broken")
    require_recipe_runtime()
    recipe = read_recipe(recipe_path)
    cal = read_calibration(calibration_path)
    rank = read_rank(rank_path)
    if cal["content_digest"].lower() != recipe["calibration"]["content_digest"].lower():
        raise RuntimeError("recipe: calibration ContentDigest != recipe binding")
    if rank["content_digest"].lower() != recipe["rank"]["content_digest"].lower():
        raise RuntimeError("recipe: rank ContentDigest != recipe binding")
    require_class_order(recipe["class_order"])
    require_class_order(cal["calibration"]["class_order"])
    require_class_order(rank["rank"]["resolved_spec"]["class_order"])
    beta = float(cal["fit"]["beta"])
    if not math.isfinite(beta) or beta < 0:
        raise RecipeError("recipe: illegal beta")
    spec = spec_from_json(rank["rank"]["resolved_spec"])
    arr = np.asarray(rank["reference"]["sorted_directional_values"], dtype=np.float64)
    ref = RankReference(sorted_d=arr, spec=spec)
    return LoadedForecastRecipe(
        beta=beta,
        rank_reference=ref,
        class_order=tuple(CLASS_ORDER),
        model_spec_digest=recipe["base_model"]["model_spec_digest"],
    )


def calibrated_probabilities(z: Tuple[float, float, float], beta: float) -> Tuple[float, float, float]:
    if not math.isfinite(beta) or beta < 0:
        raise RecipeError("recipe: illegal beta")
    if beta == 0.0:
        u = 1.0 / 3.0
        return (u, u, u)
    m = max(z)
    e = []
    for zk in z:
        scaled = beta * (zk - m)
        e.append(math.exp(scaled))
    s = sum(e)
    if not math.isfinite(s) or s <= 0:
        raise RecipeError("recipe: softmax denominator")
    p = tuple(ek / s for ek in e)
    _require_probabilities(p)
    return p


def project_forecast(logits, loaded_recipe: LoadedForecastRecipe) -> ForecastEvidence:
    z = _finite_logits3(logits)
    p = calibrated_probabilities(z, loaded_recipe.beta)
    d = z[0] - z[1]
    if not math.isfinite(d):
        raise RecipeError("recipe: nonfinite directional evidence")
    r = query_rank(loaded_recipe.rank_reference, d)
    if not math.isfinite(r) or r < -1.0 or r > 1.0:
        raise RecipeError("recipe: illegal directional_rank")
    return ForecastEvidence(probabilities=p, directional_rank=r)


def _finite_logits3(logits) -> Tuple[float, float, float]:
    try:
        seq = list(logits)
    except TypeError as e:
        raise RecipeError("recipe: logits must have length 3") from e
    if len(seq) != 3:
        raise RecipeError("recipe: logits must have length 3")
    z = tuple(float(x) for x in seq)
    if any(not math.isfinite(x) for x in z):
        raise RecipeError("recipe: nonfinite logits")
    return z


def _require_probabilities(p: Tuple[float, float, float]) -> None:
    if len(p) != 3:
        raise RecipeError("recipe: probabilities length")
    s = 0.0
    for x in p:
        if not math.isfinite(x) or x < 0.0 or x > 1.0:
            raise RecipeError("recipe: illegal probability")
        s += x
    if not math.isfinite(s) or s <= 0.0:
        raise RecipeError("recipe: illegal probability sum")
