"""Orchestrate RECIPE-FREEZE-1. No learning, metrics, holdout, or child discovery."""

from __future__ import annotations

import os
from typing import Any, Dict, Tuple

from research.calibration.artifact import read_calibration
from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.modelfit.spec import CLASS_ORDER, pinned_model_spec
from research.rank.artifact import read_rank

from .artifact import FORMAT, attach_digest, read_recipe, write_recipe_atomic
from .laws import LOGIC_V1, OUTPUTS, PROJECTION, composition_payload, require_class_order


def recipe_filename(logits_digest: bytes, cal_digest: bytes, rank_digest: bytes) -> str:
    return f"oof-{logits_digest[:8].hex()}_cal-{cal_digest[:8].hex()}_rank-{rank_digest[:8].hex()}.fcrecipe"


def generate_recipe(
    logits_path: str,
    expect_logits_digest: str,
    calibration_path: str,
    expect_calibration_digest: str,
    rank_path: str,
    expect_rank_digest: str,
    out_path: str,
) -> Tuple[Dict[str, Any], bool]:
    exp_l = expect_logits_digest.strip().lower()
    exp_c = expect_calibration_digest.strip().lower()
    exp_r = expect_rank_digest.strip().lower()
    parse_digest_hex(exp_l)
    parse_digest_hex(exp_c)
    parse_digest_hex(exp_r)
    if os.path.exists(out_path):
        return _match_existing(out_path, exp_l, exp_c, exp_r)

    header, _, footer = read_oof_logits(logits_path)
    cal = read_calibration(calibration_path)
    rank = read_rank(rank_path)
    src_l = footer["content_digest"].lower()
    src_c = cal["content_digest"].lower()
    src_r = rank["content_digest"].lower()
    if src_l != exp_l:
        raise RuntimeError("recipe: logits ContentDigest != expected")
    if src_c != exp_c:
        raise RuntimeError("recipe: calibration ContentDigest != expected")
    if src_r != exp_r:
        raise RuntimeError("recipe: rank ContentDigest != expected")
    cal_src = cal["source"]["oof_logits_content_digest"].lower()
    rank_src = rank["source"]["oof_logits_content_digest"].lower()
    if not (cal_src == rank_src == src_l):
        raise RuntimeError("recipe: calibration/rank/logits source mismatch")
    pinned = pinned_model_spec()
    if header["model_spec_digest"].lower() != pinned.digest_hex():
        raise RuntimeError("recipe: ModelSpecDigest != pinned_model_spec")
    require_class_order(header["class_order"])
    require_class_order(cal["calibration"]["class_order"])
    require_class_order(rank["rank"]["resolved_spec"]["class_order"])
    require_class_order(pinned.class_order)
    doc = composition_payload(pinned.digest_hex(), src_l, src_c, src_r, CLASS_ORDER)
    if doc["format_version"] != FORMAT or doc["recipe_logic_version"] != LOGIC_V1:
        raise RuntimeError("recipe: law tokens broken")
    if doc["projection"] != PROJECTION or doc["outputs"] != OUTPUTS:
        raise RuntimeError("recipe: composition law broken")
    doc = attach_digest(doc)
    write_recipe_atomic(out_path, doc)
    back = read_recipe(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("recipe: readback digest mismatch")
    return back, False


def _match_existing(out_path: str, exp_l: str, exp_c: str, exp_r: str) -> Tuple[Dict[str, Any], bool]:
    try:
        doc = read_recipe(out_path)
    except Exception as e:
        raise RuntimeError(f"recipe: refuse existing {out_path}: {e}") from e
    if doc["research_source"]["oof_logits_content_digest"].lower() != exp_l:
        raise RuntimeError("recipe: refuse overwrite (logits digest)")
    if doc["calibration"]["content_digest"].lower() != exp_c:
        raise RuntimeError("recipe: refuse overwrite (calibration digest)")
    if doc["rank"]["content_digest"].lower() != exp_r:
        raise RuntimeError("recipe: refuse overwrite (rank digest)")
    if doc["base_model"]["model_spec_digest"].lower() != pinned_model_spec().digest_hex():
        raise RuntimeError("recipe: refuse overwrite (ModelSpecDigest)")
    if doc["projection"] != PROJECTION or doc["outputs"] != OUTPUTS:
        raise RuntimeError("recipe: refuse overwrite (composition law)")
    return doc, True
