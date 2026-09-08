"""Orchestrate OOF-FORECAST-EVIDENCE-1. Matrix is target-law witness only."""

from __future__ import annotations

import math
import os
from typing import Any, Dict, List, Tuple

from research.calibration.artifact import read_calibration
from research.modelfit.hashwire import digest_hex, parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.modelfit.matrix import load_oof_matrix
from research.modelfit.spec import CLASS_ORDER, pinned_model_spec
from research.recipe.artifact import read_recipe
from research.recipe.env import pinned_recipe_runtime, require_recipe_runtime
from research.recipe.laws import require_class_order
from research.recipe.project import ForecastEvidence, LoadedForecastRecipe, load_forecast_recipe, project_forecast

from .artifact import AT_UNIT, FORMAT, LOGIC_V1, hash_evidence, read_evidence, roundtrip_evidence_fields, write_evidence_atomic


def evidence_filename(logits_digest: bytes, recipe_digest: bytes) -> str:
    return f"oof-{logits_digest[:8].hex()}_rec-{recipe_digest[:8].hex()}.ofev"


def project_evidence_row(at: int, outcome: str, logits, loaded: LoadedForecastRecipe) -> Dict[str, Any]:
    ev = project_forecast(logits, loaded)
    return _row_from_evidence(at, outcome, ev)


def _row_from_evidence(at: int, outcome: str, ev: ForecastEvidence) -> Dict[str, Any]:
    p, rk = roundtrip_evidence_fields(list(ev.probabilities), ev.directional_rank)
    _require_row(p, rk)
    if outcome not in CLASS_ORDER:
        raise RuntimeError("evidence: illegal outcome")
    return {
        "kind": "row",
        "at": int(at),
        "outcome": str(outcome),
        "probabilities": p,
        "directional_rank": rk,
    }


def generate_evidence(
    recipe_path: str,
    expect_recipe_digest: str,
    logits_path: str,
    calibration_path: str,
    rank_path: str,
    matrix_path: str,
    out_path: str,
) -> Tuple[Dict[str, Any], List[Dict[str, Any]], Dict[str, Any], bool]:
    expect = expect_recipe_digest.strip().lower()
    parse_digest_hex(expect)
    recipe = read_recipe(recipe_path)
    if recipe["content_digest"].lower() != expect:
        raise RuntimeError("evidence: recipe ContentDigest != expected")
    header_l, rows_l, footer_l = read_oof_logits(logits_path)
    if footer_l["content_digest"].lower() != recipe["research_source"]["oof_logits_content_digest"].lower():
        raise RuntimeError("evidence: logits != recipe research source")
    try:
        matrix = load_oof_matrix(matrix_path, header_l["source_oof_matrix_content_digest"])
    except ValueError as e:
        raise RuntimeError(f"evidence: logits ↔ matrix: {e}") from e
    _require_model_class(recipe, header_l)
    _require_market_time(header_l, matrix.header)
    if os.path.exists(out_path):
        return _match_existing(out_path, recipe, header_l, footer_l, matrix)

    cal = read_calibration(calibration_path)
    if cal["content_digest"].lower() != recipe["calibration"]["content_digest"].lower():
        raise RuntimeError("evidence: calibration != recipe child")
    from research.rank.artifact import read_rank

    rank = read_rank(rank_path)
    if rank["content_digest"].lower() != recipe["rank"]["content_digest"].lower():
        raise RuntimeError("evidence: rank != recipe child")
    runtime = require_recipe_runtime()
    loaded = load_forecast_recipe(recipe_path, calibration_path, rank_path)
    out_rows: List[Dict[str, Any]] = []
    for r in rows_l:
        out_rows.append(project_evidence_row(int(r["at"]), str(r["outcome"]), r["logits"], loaded))
    n = len(out_rows)
    if n != len(rows_l) or n != int(footer_l["row_count"]):
        raise RuntimeError("evidence: row_count != logits")
    first_at = int(out_rows[0]["at"]) if n else 0
    last_at = int(out_rows[-1]["at"]) if n else 0
    if first_at != int(footer_l["first_at"]) or last_at != int(footer_l["last_at"]):
        raise RuntimeError("evidence: first/last At != logits")
    header = {
        "kind": "header",
        "format_version": FORMAT,
        "evidence_logic_version": LOGIC_V1,
        "source": {
            "oof_logits_content_digest": footer_l["content_digest"].lower(),
            "oof_matrix_content_digest": digest_hex(matrix.content_digest),
            "market": dict(header_l["market"]),
            "at_unit": AT_UNIT,
        },
        "recipe": {"content_digest": recipe["content_digest"].lower()},
        "truth_contract": {
            "class_order": list(CLASS_ORDER),
            "target_digest": matrix.header["target_digest"].lower(),
            "label_logic_version": str(matrix.header["label_logic_version"]),
        },
        "runtime": dict(runtime),
    }
    footer = {
        "kind": "footer",
        "row_count": n,
        "first_at": first_at,
        "last_at": last_at,
    }
    footer["content_digest"] = digest_hex(hash_evidence(header, out_rows, footer))
    write_evidence_atomic(out_path, header, out_rows, footer)
    back_h, back_rows, back_f = read_evidence(out_path)
    if back_f["content_digest"] != footer["content_digest"] or len(back_rows) != n:
        raise RuntimeError("evidence: readback mismatch")
    return back_h, back_rows, back_f, False


def _require_model_class(recipe, logits_header) -> None:
    spec = pinned_model_spec()
    a = recipe["base_model"]["model_spec_digest"].lower()
    b = logits_header["model_spec_digest"].lower()
    c = spec.digest_hex().lower()
    if not (a == b == c):
        raise RuntimeError("evidence: ModelSpecDigest mismatch")
    require_class_order(recipe["class_order"])
    require_class_order(logits_header["class_order"])


def _require_market_time(logits_header, matrix_header) -> None:
    if logits_header.get("at_unit") != AT_UNIT or matrix_header.get("at_unit") != AT_UNIT:
        raise RuntimeError("evidence: at_unit mismatch")
    if dict(logits_header["market"]) != dict(matrix_header["market"]):
        raise RuntimeError("evidence: market mismatch")
    parse_digest_hex(matrix_header["target_digest"])
    if not str(matrix_header.get("label_logic_version") or ""):
        raise RuntimeError("evidence: label_logic_version missing")


def _require_row(p, rk: float) -> None:
    if len(p) != 3:
        raise RuntimeError("evidence: probabilities length")
    for x in p:
        v = float(x)
        if not math.isfinite(v) or v < 0.0 or v > 1.0:
            raise RuntimeError("evidence: illegal probability")
    r = float(rk)
    if not math.isfinite(r) or r < -1.0 or r > 1.0:
        raise RuntimeError("evidence: illegal directional_rank")


def _match_existing(out_path, recipe, logits_header, footer_l, matrix):
    try:
        header, rows, footer = read_evidence(out_path)
    except Exception as e:
        raise RuntimeError(f"evidence: refuse existing {out_path}: {e}") from e
    pin = pinned_recipe_runtime()
    if dict(header.get("runtime") or {}) != pin:
        raise RuntimeError("evidence: refuse overwrite (recorded runtime provenance)")
    if header["recipe"]["content_digest"].lower() != recipe["content_digest"].lower():
        raise RuntimeError("evidence: refuse overwrite (recipe)")
    if header["source"]["oof_logits_content_digest"].lower() != footer_l["content_digest"].lower():
        raise RuntimeError("evidence: refuse overwrite (logits)")
    if header["source"]["oof_matrix_content_digest"].lower() != digest_hex(matrix.content_digest).lower():
        raise RuntimeError("evidence: refuse overwrite (matrix)")
    tc = header["truth_contract"]
    if tc["target_digest"].lower() != matrix.header["target_digest"].lower():
        raise RuntimeError("evidence: refuse overwrite (target_digest)")
    if str(tc["label_logic_version"]) != str(matrix.header["label_logic_version"]):
        raise RuntimeError("evidence: refuse overwrite (label_logic_version)")
    if list(tc["class_order"]) != list(CLASS_ORDER):
        raise RuntimeError("evidence: refuse overwrite (class_order)")
    if dict(header["source"]["market"]) != dict(logits_header["market"]):
        raise RuntimeError("evidence: refuse overwrite (market)")
    if header["source"]["at_unit"] != AT_UNIT:
        raise RuntimeError("evidence: refuse overwrite (at_unit)")
    if int(footer["row_count"]) != int(footer_l["row_count"]):
        raise RuntimeError("evidence: refuse overwrite (row_count)")
    return header, rows, footer, True
