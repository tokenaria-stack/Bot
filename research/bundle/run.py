"""Orchestrate FORECAST-BUNDLE-1. OOF artifacts are construction witnesses only."""

from __future__ import annotations

import os
from typing import Any, Dict, Tuple

from research.calibration.artifact import read_calibration
from research.finalfit.artifact import read_final_model
from research.modelfit.hashwire import digest_hex, parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.modelfit.matrix import load_oof_matrix
from research.modelfit.spec import pinned_model_spec
from research.rank.artifact import read_rank
from research.recipe.artifact import read_recipe
from research.recipe.laws import require_class_order

from .artifact import attach_digest, read_bundle, write_bundle_atomic
from .laws import COMPOSITION, FORMAT, LOGIC_V1, PUBLIC_OUTPUT, RAW_MODEL_PROJECTION, bundle_payload


def bundle_filename(recipe_digest: bytes, model_digest: bytes) -> str:
    return f"rec-{recipe_digest[:8].hex()}_model-{model_digest[:8].hex()}.fcbundle"


def generate_bundle(
    final_model_path: str,
    recipe_path: str,
    calibration_path: str,
    rank_path: str,
    logits_path: str,
    matrix_path: str,
    out_path: str,
) -> Tuple[Dict[str, Any], bool]:
    ctx = _prove_world(final_model_path, recipe_path, calibration_path, rank_path, logits_path, matrix_path)
    if os.path.exists(out_path):
        return _match_existing(out_path, ctx)

    doc = bundle_payload(
        ctx["model"]["content_digest"].lower(),
        ctx["recipe"]["content_digest"].lower(),
        ctx["cal"]["content_digest"].lower(),
        ctx["rank"]["content_digest"].lower(),
        list(ctx["matrix"].feature_ids),
        ctx["matrix"].header["feature_plan_digest"].lower(),
        ctx["matrix"].header["target_digest"].lower(),
        str(ctx["matrix"].header["label_logic_version"]),
    )
    if doc["format_version"] != FORMAT or doc["bundle_logic_version"] != LOGIC_V1:
        raise RuntimeError("bundle: law tokens broken")
    if doc["raw_model_projection"] != RAW_MODEL_PROJECTION or doc["composition"] != COMPOSITION:
        raise RuntimeError("bundle: projection law broken")
    if doc["public_output"] != PUBLIC_OUTPUT:
        raise RuntimeError("bundle: public output broken")
    doc = attach_digest(doc)
    write_bundle_atomic(out_path, doc)
    back = read_bundle(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("bundle: readback digest mismatch")
    return back, False


def _prove_world(final_model_path, recipe_path, calibration_path, rank_path, logits_path, matrix_path):
    recipe = read_recipe(recipe_path)
    header, _, footer = read_oof_logits(logits_path)
    logits_d = footer["content_digest"].lower()
    if recipe["research_source"]["oof_logits_content_digest"].lower() != logits_d:
        raise RuntimeError("bundle: recipe ↔ logits mismatch")
    try:
        matrix = load_oof_matrix(matrix_path, header["source_oof_matrix_content_digest"])
    except ValueError as e:
        raise RuntimeError(f"bundle: logits ↔ matrix mismatch: {e}") from e
    model = read_final_model(final_model_path)
    if model["source"]["oof_matrix_content_digest"].lower() != digest_hex(matrix.content_digest).lower():
        raise RuntimeError("bundle: final model ↔ matrix mismatch")
    cal = read_calibration(calibration_path)
    rank = read_rank(rank_path)
    if recipe["calibration"]["content_digest"].lower() != cal["content_digest"].lower():
        raise RuntimeError("bundle: recipe ↔ calibration mismatch")
    if recipe["rank"]["content_digest"].lower() != rank["content_digest"].lower():
        raise RuntimeError("bundle: recipe ↔ rank mismatch")
    if cal["source"]["oof_logits_content_digest"].lower() != logits_d:
        raise RuntimeError("bundle: calibration ↔ logits mismatch")
    if rank["source"]["oof_logits_content_digest"].lower() != logits_d:
        raise RuntimeError("bundle: rank ↔ logits mismatch")
    spec = pinned_model_spec()
    a = model["model"]["model_spec_digest"].lower()
    b = recipe["base_model"]["model_spec_digest"].lower()
    c = header["model_spec_digest"].lower()
    d = spec.digest_hex().lower()
    if not (a == b == c == d):
        raise RuntimeError("bundle: ModelSpecDigest world mismatch")
    if list(matrix.feature_ids) != list(header["feature_ids"]):
        raise RuntimeError("bundle: FeatureIDs logits/matrix mismatch")
    if list(model["model"]["feature_ids"]) != list(matrix.feature_ids):
        raise RuntimeError("bundle: FeatureIDs model/matrix mismatch")
    require_class_order(recipe["class_order"])
    require_class_order(header["class_order"])
    require_class_order(model["model"]["class_order"])
    parse_digest_hex(matrix.header["feature_plan_digest"])
    parse_digest_hex(matrix.header["target_digest"])
    if not str(matrix.header.get("label_logic_version") or ""):
        raise RuntimeError("bundle: label_logic_version missing")
    return {
        "recipe": recipe,
        "header": header,
        "matrix": matrix,
        "model": model,
        "cal": cal,
        "rank": rank,
    }


def _match_existing(out_path: str, ctx) -> Tuple[Dict[str, Any], bool]:
    try:
        doc = read_bundle(out_path)
    except Exception as e:
        raise RuntimeError(f"bundle: refuse existing {out_path}: {e}") from e
    b = doc["bindings"]
    if b["final_model_content_digest"].lower() != ctx["model"]["content_digest"].lower():
        raise RuntimeError("bundle: refuse overwrite (final model)")
    if b["recipe_content_digest"].lower() != ctx["recipe"]["content_digest"].lower():
        raise RuntimeError("bundle: refuse overwrite (recipe)")
    if b["calibration_content_digest"].lower() != ctx["cal"]["content_digest"].lower():
        raise RuntimeError("bundle: refuse overwrite (calibration)")
    if b["rank_content_digest"].lower() != ctx["rank"]["content_digest"].lower():
        raise RuntimeError("bundle: refuse overwrite (rank)")
    ic = doc["input_contract"]
    if list(ic["feature_ids"]) != list(ctx["matrix"].feature_ids):
        raise RuntimeError("bundle: refuse overwrite (FeatureIDs)")
    if ic["feature_plan_digest"].lower() != ctx["matrix"].header["feature_plan_digest"].lower():
        raise RuntimeError("bundle: refuse overwrite (feature_plan_digest)")
    oc = doc["output_contract"]
    if oc["target_digest"].lower() != ctx["matrix"].header["target_digest"].lower():
        raise RuntimeError("bundle: refuse overwrite (target_digest)")
    if str(oc["label_logic_version"]) != str(ctx["matrix"].header["label_logic_version"]):
        raise RuntimeError("bundle: refuse overwrite (label_logic_version)")
    if "feature_tape_content_digest" in ic:
        raise RuntimeError("bundle: refuse historical tape in live contract")
    return doc, True
