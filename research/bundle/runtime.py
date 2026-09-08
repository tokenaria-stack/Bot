"""Load bundle children and type-state feature bind + scalar forecast."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Sequence, Tuple

from research.calibration.artifact import read_calibration
from research.finalfit.artifact import read_final_model
from research.modelfit.spec import CLASS_ORDER, pinned_model_spec
from research.rank.artifact import read_rank
from research.recipe.artifact import read_recipe
from research.recipe.laws import require_class_order
from research.recipe.project import ForecastEvidence, LoadedForecastRecipe, load_forecast_recipe, project_forecast

from .artifact import read_bundle
from .portable import BundleError, portable_raw_logits

LOAD_PARAMS = ("bundle_path", "final_model_path", "recipe_path", "calibration_path", "rank_path")
BIND_PARAMS = ("loaded", "producer")
FORECAST_PARAMS = ("bound", "values")


@dataclass(frozen=True)
class FeatureProducerContract:
    feature_ids: Tuple[str, ...]
    feature_plan_digest: str


@dataclass(frozen=True)
class LoadedForecastBundle:
    bundle: dict
    mean: Tuple[float, ...]
    scale: Tuple[float, ...]
    coef: Tuple[Tuple[float, ...], ...]
    intercept: Tuple[float, ...]
    recipe: LoadedForecastRecipe
    feature_ids: Tuple[str, ...]
    feature_plan_digest: str
    target_digest: str
    label_logic_version: str


@dataclass(frozen=True)
class BoundForecaster:
    loaded: LoadedForecastBundle


def load_forecast_bundle(
    bundle_path: str,
    final_model_path: str,
    recipe_path: str,
    calibration_path: str,
    rank_path: str,
) -> LoadedForecastBundle:
    if LOAD_PARAMS != ("bundle_path", "final_model_path", "recipe_path", "calibration_path", "rank_path"):
        raise RuntimeError("bundle: load signature law broken")
    bundle = read_bundle(bundle_path)
    model = read_final_model(final_model_path)
    recipe_doc = read_recipe(recipe_path)
    cal = read_calibration(calibration_path)
    rank = read_rank(rank_path)
    b = bundle["bindings"]
    if model["content_digest"].lower() != b["final_model_content_digest"].lower():
        raise RuntimeError("bundle: final model digest != binding")
    if recipe_doc["content_digest"].lower() != b["recipe_content_digest"].lower():
        raise RuntimeError("bundle: recipe digest != binding")
    if cal["content_digest"].lower() != b["calibration_content_digest"].lower():
        raise RuntimeError("bundle: calibration digest != binding")
    if rank["content_digest"].lower() != b["rank_content_digest"].lower():
        raise RuntimeError("bundle: rank digest != binding")
    if cal["content_digest"].lower() != recipe_doc["calibration"]["content_digest"].lower():
        raise RuntimeError("bundle: calibration != recipe child")
    if rank["content_digest"].lower() != recipe_doc["rank"]["content_digest"].lower():
        raise RuntimeError("bundle: rank != recipe child")
    spec = pinned_model_spec()
    if model["model"]["model_spec_digest"].lower() != recipe_doc["base_model"]["model_spec_digest"].lower():
        raise RuntimeError("bundle: ModelSpecDigest mismatch")
    if model["model"]["model_spec_digest"].lower() != spec.digest_hex():
        raise RuntimeError("bundle: ModelSpecDigest != pinned")
    require_class_order(model["model"]["class_order"])
    require_class_order(bundle["output_contract"]["class_order"])
    if list(model["model"]["feature_ids"]) != list(bundle["input_contract"]["feature_ids"]):
        raise RuntimeError("bundle: FeatureIDs mismatch")
    loaded_recipe = load_forecast_recipe(recipe_path, calibration_path, rank_path)
    mean = tuple(float(v) for v in model["scaler"]["mean"])
    scale = tuple(float(v) for v in model["scaler"]["scale"])
    coef = tuple(tuple(float(v) for v in row) for row in model["logistic"]["coef"])
    intercept = tuple(float(v) for v in model["logistic"]["intercept"])
    return LoadedForecastBundle(
        bundle=bundle,
        mean=mean,
        scale=scale,
        coef=coef,
        intercept=intercept,
        recipe=loaded_recipe,
        feature_ids=tuple(bundle["input_contract"]["feature_ids"]),
        feature_plan_digest=bundle["input_contract"]["feature_plan_digest"].lower(),
        target_digest=bundle["output_contract"]["target_digest"].lower(),
        label_logic_version=str(bundle["output_contract"]["label_logic_version"]),
    )


def bind_feature_contract(loaded: LoadedForecastBundle, producer: FeatureProducerContract) -> BoundForecaster:
    if BIND_PARAMS != ("loaded", "producer"):
        raise RuntimeError("bundle: bind signature law broken")
    if list(producer.feature_ids) != list(loaded.feature_ids):
        raise RuntimeError("bundle: producer FeatureIDs mismatch")
    if producer.feature_plan_digest.strip().lower() != loaded.feature_plan_digest:
        raise RuntimeError("bundle: feature_plan_digest mismatch")
    return BoundForecaster(loaded=loaded)


def forecast(bound: BoundForecaster, values: Sequence[float]) -> ForecastEvidence:
    if FORECAST_PARAMS != ("bound", "values"):
        raise RuntimeError("bundle: forecast signature law broken")
    if not isinstance(bound, BoundForecaster):
        raise BundleError("bundle: forecast requires BoundForecaster")
    z = portable_raw_logits(
        values,
        bound.loaded.mean,
        bound.loaded.scale,
        bound.loaded.coef,
        bound.loaded.intercept,
    )
    return project_forecast(z, bound.loaded.recipe)
