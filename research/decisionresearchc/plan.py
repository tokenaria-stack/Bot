"""Compile DecisionValidationPlan-C from CatBoost OOF At[]. Existing Go compiler, Target C."""

from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path
from typing import Any, Dict, List, Tuple

from research.catboostlogits.reader import CatBoostOOFLogits
from research.decisionplan.compile import extract_at
from research.decisionresearchc.plan_artifact import AT_UNIT, FORMAT, LOGIC_V1, attach_digest, read_plan, write_plan_atomic
from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.matrix import load_oof_matrix

ROOT = Path(__file__).resolve().parents[2]


def compile_validation_plan_c(at: List[int]) -> Dict[str, Any]:
    payload = json.dumps({"at": [int(x) for x in at], "target": "c"}, separators=(",", ":"))
    proc = subprocess.run(
        ["go", "run", "./cmd/research_decision_plan"],
        cwd=str(ROOT),
        input=payload.encode("utf-8"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if proc.returncode != 0:
        err = proc.stderr.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"decisionplan_c: CompileValidationPlan: {err}")
    doc = json.loads(proc.stdout.decode("utf-8"))
    if doc.get("logic") != LOGIC_V1:
        raise RuntimeError("decisionplan_c: unexpected compiler logic")
    if int(doc.get("target_h") or 0) != 72:
        raise RuntimeError(f"decisionplan_c: TargetH={doc.get('target_h')} want 72")
    if int(doc.get("validation_span_bars") or 0) != 8640:
        raise RuntimeError("decisionplan_c: ValidationSpanBars must remain 8640")
    return doc


def plan_filename(logits_digest: bytes) -> str:
    return f"logits-{logits_digest[:8].hex()}_dval-walk-forward-c.dvalplan"


def generate_decision_plan_c(
    logits: CatBoostOOFLogits,
    matrix_path: str,
    out_path: str,
) -> Tuple[Dict[str, Any], bool]:
    matrix = load_oof_matrix(matrix_path, logits.matrix_digest)
    if dict(matrix.header["market"]) != dict(logits.market):
        raise RuntimeError("decisionplan_c: market mismatch")
    wall = int(matrix.header["holdout_start_at"])
    tf = logits.market["timeframe"]
    label_logic = str(matrix.header["label_logic_version"])
    if os.path.exists(out_path):
        return _match_existing(out_path, logits, matrix, wall, tf, label_logic)

    at = [int(x) for x in logits.at.tolist()]
    extract_at([{"at": a} for a in at])
    compiled = compile_validation_plan_c(at)
    _require_resolved(compiled, logits, matrix, wall, tf, len(at))
    doc = _artifact(logits, compiled, label_logic)
    write_plan_atomic(out_path, doc)
    back = read_plan(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("decisionplan_c: readback mismatch")
    return back, False


def _require_resolved(compiled: Dict[str, Any], logits: CatBoostOOFLogits, matrix, wall: int, tf: str, n: int) -> None:
    if compiled["timeframe"] != tf:
        raise RuntimeError("decisionplan_c: timeframe mismatch")
    if int(compiled["holdout_start_at"]) != wall:
        raise RuntimeError("decisionplan_c: HoldoutStartAt != sealed wall")
    if compiled["target_digest"].lower() != str(matrix.header["target_digest"]).lower():
        raise RuntimeError("decisionplan_c: TargetSpec digest != matrix target_digest")
    if int(compiled["validation_span_bars"]) != 8640:
        raise RuntimeError("decisionplan_c: ValidationSpanBars")
    if int(compiled["fold_count"]) != 4:
        raise RuntimeError("decisionplan_c: FoldCount")
    if int(compiled["min_train_rows"]) != 35040:
        raise RuntimeError("decisionplan_c: MinTrainRows")
    if int(compiled["extra_gap_bars"]) != 0:
        raise RuntimeError("decisionplan_c: ExtraGapBars")
    if int(compiled["target_h"]) != 72:
        raise RuntimeError("decisionplan_c: TargetH")
    if int(compiled["target_h"]) != int(compiled["total_causal_bars"]):
        raise RuntimeError("decisionplan_c: TotalCausalBars != TargetH + ExtraGapBars")
    folds = compiled["folds"]
    if len(folds) != 4:
        raise RuntimeError("decisionplan_c: fold count")
    train0 = int(folds[0]["train_end"]) - int(folds[0]["train_begin"])
    if train0 < int(compiled["min_train_rows"]):
        raise RuntimeError(f"decisionplan_c: Fold0 train rows {train0} < MinTrainRows")
    if int(compiled["holdout_begin_index"]) != n:
        raise RuntimeError("decisionplan_c: holdout index must be past all logits rows")


def _artifact(logits: CatBoostOOFLogits, compiled: Dict[str, Any], label_logic: str) -> Dict[str, Any]:
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
    n = int(logits.at.shape[0])
    doc = {
        "format_version": FORMAT,
        "decision_validation_logic_version": LOGIC_V1,
        "source": {
            "oof_logits_content_digest": logits.content_digest.lower(),
            "market": dict(logits.market),
            "at_unit": AT_UNIT,
            "row_count": n,
            "first_at": int(logits.at[0]),
            "last_at": int(logits.at[-1]),
            "target_digest": compiled["target_digest"].lower(),
            "label_logic_version": label_logic,
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


def _match_existing(out_path, logits, matrix, wall, tf, label_logic):
    try:
        doc = read_plan(out_path)
    except Exception as e:
        raise RuntimeError(f"decisionplan_c: refuse existing {out_path}: {e}") from e
    at = [int(x) for x in logits.at.tolist()]
    compiled = compile_validation_plan_c(at)
    _require_resolved(compiled, logits, matrix, wall, tf, len(at))
    fresh = _artifact(logits, compiled, label_logic)
    if doc["content_digest"] != fresh["content_digest"]:
        raise RuntimeError("decisionplan_c: refuse overwrite (digest)")
    return doc, True
