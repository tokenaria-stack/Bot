"""Orchestrate MODEL-FIT-1: matrix in, oof-logits-v1 out. No metrics."""

from __future__ import annotations

import inspect
import os
from typing import Any, Dict, List, Optional, Tuple

import numpy as np

from . import fit as fitmod
from .envpin import pinned_runtime, require_pinned_runtime
from .fit import fit_fold_model, predict_fold_logits
from .hashwire import digest_hex
from .logits import (
    AT_UNIT,
    FORMAT,
    hash_oof_logits,
    json_roundtrip_array,
    json_roundtrip_float,
    json_roundtrip_matrix,
    read_oof_logits,
    write_jsonl_atomic,
)
from .matrix import OOFMatrix, load_oof_matrix
from .spec import CLASS_ORDER, ModelSpec, pinned_model_spec


FIT_PARAMS = tuple(inspect.signature(fit_fold_model).parameters)
PREDICT_PARAMS = tuple(inspect.signature(predict_fold_logits).parameters)


def logits_filename(market: Dict[str, str], matrix_digest: bytes, spec_digest: bytes) -> str:
    def short(d: bytes) -> str:
        return d[:8].hex()

    return (
        f"{market['venue']}_{market['instrument']}_{market['contract']}_{market['timeframe']}"
        f"_oof-{short(matrix_digest)}_model-{short(spec_digest)}.ooflogits"
    )


def assignment_expected(n: int, folds) -> np.ndarray:
    expected = np.zeros(n, dtype=bool)
    for f in folds:
        expected[f.val_begin : f.val_end] = True
    return expected


def generate_oof_logits(
    matrix_path: str,
    expect_matrix_digest: str,
    out_path: str,
    *,
    spec: Optional[ModelSpec] = None,
) -> Tuple[Dict[str, Any], Dict[str, Any], bool]:
    if FIT_PARAMS != ("x_train", "y_train", "spec"):
        raise RuntimeError("modelfit: fit_fold_model signature law broken")
    if PREDICT_PARAMS != ("fitted", "x_val"):
        raise RuntimeError("modelfit: predict_fold_logits signature law broken")
    runtime = require_pinned_runtime()
    spec = spec or pinned_model_spec()
    spec_hex = spec.digest_hex()
    pin = pinned_runtime()
    if os.path.exists(out_path):
        return _match_existing(out_path, expect_matrix_digest, spec, runtime, pin)

    matrix = load_oof_matrix(matrix_path, expect_matrix_digest)
    header, rows, footer = _fit_all(matrix, spec, runtime)
    records = [header] + rows + [footer]
    write_jsonl_atomic(out_path, records)
    back_h, back_rows, back_f = read_oof_logits(out_path)
    if back_f["content_digest"] != footer["content_digest"] or len(back_rows) != len(rows):
        raise RuntimeError("modelfit: oof-logits readback mismatch")
    if back_h["format_version"] != FORMAT or back_h["at_unit"] != AT_UNIT:
        raise RuntimeError("modelfit: oof-logits readback format")
    return back_h, back_f, False


def _match_existing(
    out_path: str,
    expect_matrix_digest: str,
    spec: ModelSpec,
    runtime: Dict[str, str],
    pin: Dict[str, str],
) -> Tuple[Dict[str, Any], Dict[str, Any], bool]:
    try:
        header, rows, footer = read_oof_logits(out_path)
    except Exception as e:
        raise RuntimeError(f"modelfit: refuse existing oof-logits {out_path}: {e}") from e
    src = header.get("source_oof_matrix_content_digest", "").lower()
    if src != expect_matrix_digest.strip().lower():
        raise RuntimeError("modelfit: refuse overwrite of different oof-logits (matrix digest)")
    if header.get("model_spec_digest") != spec.digest_hex():
        raise RuntimeError("modelfit: refuse overwrite of different oof-logits (ModelSpecDigest)")
    if spec_from_header(header).payload() != spec.payload():
        raise RuntimeError("modelfit: refuse overwrite of different oof-logits (ModelSpec)")
    rec = header.get("runtime") or {}
    if rec != pin or rec != runtime:
        raise RuntimeError("modelfit: refuse existing oof-logits runtime/pin mismatch")
    return header, footer, True


def spec_from_header(header: Dict[str, Any]) -> ModelSpec:
    from .spec import spec_from_json

    return spec_from_json(header["model_spec"])


def _fit_all(matrix: OOFMatrix, spec: ModelSpec, runtime: Dict[str, str]) -> Tuple[Dict, List[Dict], Dict]:
    n = int(matrix.at.shape[0])
    expected = assignment_expected(n, matrix.folds)
    assigned = np.zeros(n, dtype=bool)
    out_rows: List[Dict[str, Any]] = []
    fold_json: List[Dict[str, Any]] = []
    cursor = 0
    for f in matrix.folds:
        sl = slice(f.val_begin, f.val_end)
        if assigned[sl].any():
            raise RuntimeError("modelfit: duplicate validation assignment")
        x_train = matrix.x[f.train_begin : f.train_end]
        y_train = matrix.y[f.train_begin : f.train_end]
        fitted = fit_fold_model(x_train, y_train, spec)
        x_val = matrix.x[sl]
        logits = predict_fold_logits(fitted, x_val)
        assigned[sl] = True
        val_n = f.val_end - f.val_begin
        ob, oe = cursor, cursor + val_n
        cursor = oe
        fold_json.append(
            {
                "source_train_begin": f.train_begin,
                "source_train_end": f.train_end,
                "source_validation_begin": f.val_begin,
                "source_validation_end": f.val_end,
                "output_begin": ob,
                "output_end": oe,
                "scaler_mean": json_roundtrip_array(fitted.mean),
                "scaler_scale": json_roundtrip_array(fitted.scale),
                "coef": json_roundtrip_matrix(fitted.coef),
                "intercept": json_roundtrip_array(fitted.intercept),
                "n_iter": int(fitted.n_iter),
            }
        )
        for i in range(val_n):
            idx = f.val_begin + i
            out_rows.append(
                {
                    "kind": "row",
                    "at": int(matrix.at[idx]),
                    "outcome": matrix.outcomes[idx],
                    "logits": [json_roundtrip_float(v) for v in logits[i]],
                }
            )
        fitted.scaler = None
        fitted.model = None
    if not np.array_equal(assigned, expected):
        missing = int(np.sum(expected & ~assigned))
        extra = int(np.sum(assigned & ~expected))
        raise RuntimeError(f"modelfit: assignment mask mismatch missing={missing} extra={extra}")
    last = None
    for r in out_rows:
        a = r["at"]
        if last is not None and a <= last:
            raise RuntimeError("modelfit: output At not strictly increasing")
        last = a
    header = {
        "kind": "header",
        "format_version": FORMAT,
        "at_unit": AT_UNIT,
        "source_oof_matrix_content_digest": digest_hex(matrix.content_digest),
        "market": matrix.header["market"],
        "feature_ids": list(matrix.feature_ids),
        "model_logic_version": spec.logic,
        "model_spec_digest": spec.digest_hex(),
        "model_spec": spec.as_json(),
        "class_order": list(CLASS_ORDER),
        "runtime": dict(runtime),
        "folds": fold_json,
    }
    digest = hash_oof_logits(header, out_rows)
    footer = {
        "kind": "footer",
        "row_count": len(out_rows),
        "first_at": out_rows[0]["at"] if out_rows else 0,
        "last_at": out_rows[-1]["at"] if out_rows else 0,
        "content_digest": digest_hex(digest),
    }
    return header, out_rows, footer
