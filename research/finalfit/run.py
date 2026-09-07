"""Orchestrate FINAL-MODEL-FIT-1. Recipe is precommit only."""

from __future__ import annotations

import os
from typing import Any, Dict, Tuple

from research.modelfit.envpin import require_pinned_runtime
from research.modelfit.fit import fit_fold_model
from research.modelfit.hashwire import digest_hex, parse_digest_hex
from research.modelfit.logits import json_roundtrip_array, json_roundtrip_matrix, read_oof_logits
from research.modelfit.matrix import load_oof_matrix
from research.modelfit.spec import CLASS_ORDER, pinned_model_spec
from research.recipe.artifact import read_recipe
from research.recipe.laws import require_class_order

from .artifact import FIT_LOGIC_V1, FORMAT, attach_digest, read_final_model, write_final_model_atomic


def final_model_filename(market: Dict[str, str], matrix_digest: bytes, spec_digest: bytes) -> str:
    short = lambda d: d[:8].hex()
    return (
        f"{market['venue']}_{market['instrument']}_{market['contract']}_{market['timeframe']}"
        f"_matrix-{short(matrix_digest)}_model-{short(spec_digest)}.finalmodel"
    )


def generate_final_model(
    recipe_path: str,
    expect_recipe_digest: str,
    logits_path: str,
    matrix_path: str,
    out_path: str,
) -> Tuple[Dict[str, Any], bool]:
    recipe, logits_header, matrix = _prove_chain(recipe_path, expect_recipe_digest, logits_path, matrix_path)
    spec = pinned_model_spec()
    _require_model_and_schema(recipe, logits_header, matrix, spec)
    if os.path.exists(out_path):
        return _match_existing(out_path, matrix, spec)

    runtime = require_pinned_runtime()
    fitted = fit_fold_model(matrix.x, matrix.y, spec)
    if any(float(v) <= 0 for v in fitted.scale):
        raise RuntimeError("finalfit: nonpositive scale")
    fitted.scaler = None
    fitted.model = None
    mean = json_roundtrip_array(fitted.mean)
    scale = json_roundtrip_array(fitted.scale)
    coef = json_roundtrip_matrix(fitted.coef)
    intercept = json_roundtrip_array(fitted.intercept)
    n = int(matrix.at.shape[0])
    doc = {
        "format_version": FORMAT,
        "fit_logic_version": FIT_LOGIC_V1,
        "source": {
            "oof_matrix_content_digest": digest_hex(matrix.content_digest),
            "market": dict(matrix.header["market"]),
            "row_count": n,
            "first_at": int(matrix.at[0]),
            "last_at": int(matrix.at[-1]),
        },
        "model": {
            "model_logic_version": spec.logic,
            "model_spec_digest": spec.digest_hex(),
            "feature_ids": list(matrix.feature_ids),
            "class_order": list(CLASS_ORDER),
        },
        "runtime": dict(runtime),
        "scaler": {"mean": mean, "scale": scale},
        "logistic": {"coef": coef, "intercept": intercept, "n_iter": int(fitted.n_iter)},
    }
    doc = attach_digest(doc)
    write_final_model_atomic(out_path, doc)
    back = read_final_model(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("finalfit: readback digest mismatch")
    return back, False


def _prove_chain(recipe_path: str, expect_recipe_digest: str, logits_path: str, matrix_path: str):
    expect = expect_recipe_digest.strip().lower()
    parse_digest_hex(expect)
    recipe = read_recipe(recipe_path)
    if recipe["content_digest"].lower() != expect:
        raise RuntimeError("finalfit: recipe ContentDigest != expected")
    header, _, footer = read_oof_logits(logits_path)
    logits_d = footer["content_digest"].lower()
    want_logits = recipe["research_source"]["oof_logits_content_digest"].lower()
    if logits_d != want_logits:
        raise RuntimeError("finalfit: logits ContentDigest != recipe research source")
    mtx_expect = header["source_oof_matrix_content_digest"]
    try:
        matrix = load_oof_matrix(matrix_path, mtx_expect)
    except ValueError as e:
        raise RuntimeError(f"finalfit: matrix provenance: {e}") from e
    return recipe, header, matrix


def _require_model_and_schema(recipe, logits_header, matrix, spec) -> None:
    r = recipe["base_model"]["model_spec_digest"].lower()
    l = logits_header["model_spec_digest"].lower()
    p = spec.digest_hex().lower()
    if not (r == l == p):
        raise RuntimeError("finalfit: ModelSpecDigest triple mismatch")
    if list(matrix.feature_ids) != list(logits_header["feature_ids"]):
        raise RuntimeError("finalfit: FeatureIDs mismatch")
    require_class_order(recipe["class_order"])
    require_class_order(logits_header["class_order"])
    require_class_order(spec.class_order)


def _match_existing(out_path: str, matrix, spec) -> Tuple[Dict[str, Any], bool]:
    try:
        doc = read_final_model(out_path)
    except Exception as e:
        raise RuntimeError(f"finalfit: refuse existing {out_path}: {e}") from e
    got = doc["source"]["oof_matrix_content_digest"].lower()
    if got != digest_hex(matrix.content_digest).lower():
        raise RuntimeError("finalfit: refuse overwrite (matrix digest)")
    if doc["model"]["model_spec_digest"].lower() != spec.digest_hex():
        raise RuntimeError("finalfit: refuse overwrite (ModelSpecDigest)")
    if list(doc["model"]["feature_ids"]) != list(matrix.feature_ids):
        raise RuntimeError("finalfit: refuse overwrite (FeatureIDs)")
    if list(doc["model"]["class_order"]) != list(CLASS_ORDER):
        raise RuntimeError("finalfit: refuse overwrite (class_order)")
    return doc, True
