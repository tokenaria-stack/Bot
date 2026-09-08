"""Orchestrate DECISION-VALIDATION-PLAN-1. Geometry from At[] only."""

from __future__ import annotations

import os
from typing import Any, Dict, Tuple

from research.decisionplan.artifact import FORMAT, LOGIC_V1, AT_UNIT, attach_digest, read_plan, write_plan_atomic
from research.decisionplan.compile import compile_validation_plan, extract_at
from research.evidence.artifact import read_evidence
from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.matrix import load_oof_matrix


def plan_filename(evidence_digest: bytes) -> str:
    return f"ev-{evidence_digest[:8].hex()}_dval-walk-forward-v1.dvalplan"


def generate_decision_plan(
    evidence_path: str,
    expect_evidence_digest: str,
    matrix_path: str,
    out_path: str,
) -> Tuple[Dict[str, Any], bool]:
    expect = expect_evidence_digest.strip().lower()
    parse_digest_hex(expect)
    header, rows, footer = read_evidence(evidence_path)
    if footer["content_digest"].lower() != expect:
        raise RuntimeError("decisionplan: evidence ContentDigest != expected")
    want_m = header["source"]["oof_matrix_content_digest"]
    try:
        matrix = load_oof_matrix(matrix_path, want_m)
    except ValueError as e:
        raise RuntimeError(f"decisionplan: matrix witness: {e}") from e
    if str(matrix.header["target_digest"]).lower() != header["truth_contract"]["target_digest"].lower():
        raise RuntimeError("decisionplan: target_digest mismatch")
    if str(matrix.header["label_logic_version"]) != str(header["truth_contract"]["label_logic_version"]):
        raise RuntimeError("decisionplan: label_logic_version mismatch")
    if dict(matrix.header["market"]) != dict(header["source"]["market"]):
        raise RuntimeError("decisionplan: market mismatch")
    tf = header["source"]["market"]["timeframe"]
    wall = int(matrix.header["holdout_start_at"])
    if os.path.exists(out_path):
        return _match_existing(out_path, header, footer, rows, matrix, wall, tf)

    at = extract_at(rows)
    compiled = compile_validation_plan(at)
    _require_resolved(compiled, header, matrix, wall, tf, len(at))
    doc = _artifact(header, footer, compiled)
    write_plan_atomic(out_path, doc)
    back = read_plan(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("decisionplan: readback mismatch")
    return back, False


def _require_resolved(compiled: Dict[str, Any], header, matrix, wall: int, tf: str, n: int) -> None:
    if compiled["timeframe"] != tf:
        raise RuntimeError("decisionplan: timeframe mismatch")
    if int(compiled["holdout_start_at"]) != wall:
        raise RuntimeError("decisionplan: HoldoutStartAt != sealed wall")
    if compiled["target_digest"].lower() != header["truth_contract"]["target_digest"].lower():
        raise RuntimeError("decisionplan: TargetSpec digest != evidence target_digest")
    if compiled["target_digest"].lower() != str(matrix.header["target_digest"]).lower():
        raise RuntimeError("decisionplan: TargetSpec digest != matrix target_digest")
    if int(compiled["validation_span_bars"]) != 8640:
        raise RuntimeError("decisionplan: ValidationSpanBars")
    if int(compiled["fold_count"]) != 4:
        raise RuntimeError("decisionplan: FoldCount")
    if int(compiled["min_train_rows"]) != 35040:
        raise RuntimeError("decisionplan: MinTrainRows")
    if int(compiled["extra_gap_bars"]) != 0:
        raise RuntimeError("decisionplan: ExtraGapBars")
    if int(compiled["target_h"]) != int(compiled["total_causal_bars"]):
        raise RuntimeError("decisionplan: TotalCausalBars != TargetH + ExtraGapBars")
    folds = compiled["folds"]
    if len(folds) != 4:
        raise RuntimeError("decisionplan: fold count")
    train0 = int(folds[0]["train_end"]) - int(folds[0]["train_begin"])
    if train0 < int(compiled["min_train_rows"]):
        raise RuntimeError(f"decisionplan: Fold0 train rows {train0} < MinTrainRows")
    if int(compiled["holdout_begin_index"]) != n:
        raise RuntimeError("decisionplan: holdout index must be past all evidence rows")


def _artifact(header, footer, compiled: Dict[str, Any]) -> Dict[str, Any]:
    folds = []
    for f in compiled["folds"]:
        folds.append({
            "train_begin": int(f["train_begin"]),
            "train_end": int(f["train_end"]),
            "val_begin": int(f["val_begin"]),
            "val_end": int(f["val_end"]),
            "val_boundary_start_at": int(f["val_boundary_start_at"]),
            "val_boundary_end_at": int(f["val_boundary_end_at"]),
            "train_first_at": int(f["train_first_at"]),
            "train_last_at": int(f["train_last_at"]),
            "val_first_at": int(f["val_first_at"]),
            "val_last_at": int(f["val_last_at"]),
        })
    doc = {
        "format_version": FORMAT,
        "decision_validation_logic_version": LOGIC_V1,
        "source": {
            "evidence_content_digest": footer["content_digest"].lower(),
            "market": dict(header["source"]["market"]),
            "at_unit": AT_UNIT,
            "row_count": int(footer["row_count"]),
            "first_at": int(footer["first_at"]),
            "last_at": int(footer["last_at"]),
            "target_digest": header["truth_contract"]["target_digest"].lower(),
            "label_logic_version": str(header["truth_contract"]["label_logic_version"]),
        },
        "rules": {
            "holdout_start_at": int(compiled["holdout_start_at"]),
            "validation_span_bars": int(compiled["validation_span_bars"]),
            "fold_count": int(compiled["fold_count"]),
            "min_train_rows": int(compiled["min_train_rows"]),
            "target_h": int(compiled["target_h"]),
            "extra_gap_bars": int(compiled["extra_gap_bars"]),
        },
        "compiled": {
            "total_causal_bars": int(compiled["total_causal_bars"]),
            "development_exclusive_end_at": int(compiled["development_exclusive_end_at"]),
            "development_end_index": int(compiled["development_end_index"]),
            "holdout_begin_index": int(compiled["holdout_begin_index"]),
            "folds": folds,
        },
    }
    return attach_digest(doc)


def _match_existing(out_path, header, footer, rows, matrix, wall, tf):
    try:
        doc = read_plan(out_path)
    except Exception as e:
        raise RuntimeError(f"decisionplan: refuse existing {out_path}: {e}") from e
    at = extract_at(rows)
    compiled = compile_validation_plan(at)
    _require_resolved(compiled, header, matrix, wall, tf, len(at))
    fresh = _artifact(header, footer, compiled)
    if doc["content_digest"] != fresh["content_digest"]:
        raise RuntimeError("decisionplan: refuse overwrite (digest)")
    if doc["compiled"] != fresh["compiled"] or doc["rules"] != fresh["rules"]:
        raise RuntimeError("decisionplan: refuse overwrite (geometry)")
    return doc, True
