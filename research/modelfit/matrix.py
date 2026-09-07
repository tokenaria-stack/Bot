"""oof-matrix-v1 interchange loader + OM1C content digest (wire only)."""

from __future__ import annotations

import json
import math
from dataclasses import dataclass
from typing import Any, Dict, List, Tuple

import numpy as np

from .hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_f64, put_i64, put_string, put_u32
from .spec import CLASS_ORDER, CODE_BY_OUTCOME

LEGAL_OUTCOMES = set(CLASS_ORDER)


@dataclass
class MatrixFold:
    train_begin: int
    train_end: int
    val_begin: int
    val_end: int
    val_boundary_start_at: int
    val_boundary_end_at: int
    train_first_at: int
    train_last_at: int
    val_first_at: int
    val_last_at: int


@dataclass
class OOFMatrix:
    header: Dict[str, Any]
    at: np.ndarray
    x: np.ndarray
    y: np.ndarray
    outcomes: List[str]
    folds: List[MatrixFold]
    content_digest: bytes
    feature_ids: List[str]


def load_oof_matrix(path: str, expect_digest_hex: str) -> OOFMatrix:
    expect = parse_digest_hex(expect_digest_hex.strip().lower())
    with open(path, "r", encoding="utf-8") as f:
        lines = [ln.strip() for ln in f if ln.strip()]
    if len(lines) < 3:
        raise ValueError("modelfit: oof-matrix too short")
    header = json.loads(lines[0])
    footer = json.loads(lines[-1])
    if header.get("kind") != "header":
        raise ValueError("modelfit: first record must be header")
    if footer.get("kind") != "footer":
        raise ValueError("modelfit: last record must be footer")
    if header.get("format_version") != "oof-matrix-v1":
        raise ValueError("modelfit: unknown oof-matrix format")
    if header.get("at_unit") != "unix_ms":
        raise ValueError("modelfit: AtUnit must be unix_ms")
    rows_json = [json.loads(ln) for ln in lines[1:-1]]
    for r in rows_json:
        if r.get("kind") != "row":
            raise ValueError("modelfit: expected row records")
    n = len(rows_json)
    if header.get("development_row_count") != n or footer.get("row_count") != n:
        raise ValueError("modelfit: row count mismatch")
    feature_ids = list(header["feature_ids"])
    width = len(feature_ids)
    if width == 0:
        raise ValueError("modelfit: FeatureIDs required")
    at = np.empty(n, dtype=np.int64)
    x = np.empty((n, width), dtype=np.float64)
    y = np.empty(n, dtype=np.int8)
    outcomes: List[str] = []
    last = None
    for i, r in enumerate(rows_json):
        a = int(r["at"])
        if last is not None and a <= last:
            raise ValueError("modelfit: At must be strictly increasing")
        last = a
        feats = r["features"]
        if len(feats) != width:
            raise ValueError("modelfit: feature width mismatch")
        for v in feats:
            fv = float(v)
            if not math.isfinite(fv):
                raise ValueError("modelfit: nonfinite feature")
        out = r["outcome"]
        if out not in LEGAL_OUTCOMES:
            raise ValueError(f"modelfit: illegal outcome {out!r}")
        at[i] = a
        x[i, :] = np.array(feats, dtype=np.float64)
        y[i] = CODE_BY_OUTCOME[out]
        outcomes.append(out)
    folds = [_parse_fold(fj) for fj in header.get("folds") or []]
    _validate_folds(folds, n)
    got = hash_oof_matrix(header, rows_json)
    footer_d = parse_digest_hex(footer["content_digest"])
    if got != footer_d:
        raise ValueError("modelfit: oof-matrix ContentDigest mismatch")
    if got != expect:
        raise ValueError("modelfit: oof-matrix ContentDigest != expected")
    if footer.get("first_at") != int(at[0]) or footer.get("last_at") != int(at[-1]):
        raise ValueError("modelfit: footer first/last At mismatch")
    return OOFMatrix(
        header=header,
        at=at,
        x=x,
        y=y,
        outcomes=outcomes,
        folds=folds,
        content_digest=got,
        feature_ids=feature_ids,
    )


def _parse_fold(fj: Dict[str, Any]) -> MatrixFold:
    return MatrixFold(
        train_begin=int(fj["train_begin"]),
        train_end=int(fj["train_end"]),
        val_begin=int(fj["validation_begin"]),
        val_end=int(fj["validation_end"]),
        val_boundary_start_at=int(fj["validation_boundary_start_at"]),
        val_boundary_end_at=int(fj["validation_boundary_end_at"]),
        train_first_at=int(fj["train_first_at"]),
        train_last_at=int(fj["train_last_at"]),
        val_first_at=int(fj["validation_first_actual_at"]),
        val_last_at=int(fj["validation_last_actual_at"]),
    )


def _validate_folds(folds: List[MatrixFold], n: int) -> None:
    for i, f in enumerate(folds):
        if not (0 <= f.train_begin <= f.train_end <= n):
            raise ValueError(f"modelfit: fold {i} train range outside matrix")
        if not (0 <= f.val_begin <= f.val_end <= n):
            raise ValueError(f"modelfit: fold {i} val range outside matrix")
        if f.val_end <= f.val_begin:
            raise ValueError(f"modelfit: fold {i} empty validation")
        for j in range(i + 1, len(folds)):
            g = folds[j]
            if f.val_end > g.val_begin and g.val_end > f.val_begin:
                raise ValueError("modelfit: validation ranges overlap")


def hash_oof_matrix(header: Dict[str, Any], rows: List[Dict[str, Any]]) -> bytes:
    h = new_sha()
    put_string(h, "OM1C")
    _hash_header(h, header)
    for r in rows:
        put_i64(h, int(r["at"]))
        feats = r["features"]
        put_u32(h, len(feats))
        for v in feats:
            put_f64(h, float(v))
        put_string(h, str(r["outcome"]))
    put_u32(h, len(rows))
    if rows:
        put_i64(h, int(rows[0]["at"]))
        put_i64(h, int(rows[-1]["at"]))
    return h.digest()


def hash_oof_matrix_file(path: str) -> str:
    with open(path, "r", encoding="utf-8") as f:
        lines = [ln.strip() for ln in f if ln.strip()]
    header = json.loads(lines[0])
    rows = [json.loads(ln) for ln in lines[1:-1]]
    return digest_hex(hash_oof_matrix(header, rows))


def _hx(s: str) -> bytes:
    return parse_digest_hex(s)


def _hash_header(h, header: Dict[str, Any]) -> None:
    put_string(h, header["format_version"])
    put_string(h, header["at_unit"])
    m = header["market"]
    put_string(h, m["venue"])
    put_string(h, m["instrument"])
    put_string(h, m["contract"])
    put_string(h, m["timeframe"])
    ids = header["feature_ids"]
    put_u32(h, len(ids))
    for fid in ids:
        put_string(h, fid)
    put_digest(h, _hx(header["feature_plan_digest"]))
    put_digest(h, _hx(header["feature_tape_source_range_digest"]))
    put_digest(h, _hx(header["feature_tape_content_digest"]))
    put_digest(h, _hx(header["label_set_content_digest"]))
    put_digest(h, _hx(header["target_digest"]))
    put_digest(h, _hx(header["label_source_range_digest"]))
    finer = header.get("finer_source_digest") or ("00" * 32)
    put_digest(h, _hx(finer))
    put_u32(h, int(header.get("finer_window_count") or 0))
    put_digest(h, _hx(header["label_feature_tape_plan_digest"]))
    put_digest(h, _hx(header["label_feature_tape_source_range_digest"]))
    put_digest(h, _hx(header["label_feature_tape_content_digest"]))
    put_string(h, header.get("label_logic_version") or "")
    put_digest(h, _hx(header["validation_plan_digest"]))
    vp = header["validation_plan"]
    put_string(h, vp["logic"])
    put_string(h, vp["timeframe"])
    put_i64(h, int(vp["holdout_start_at"]))
    put_u32(h, int(vp["validation_span_bars"]))
    put_u32(h, int(vp["fold_count"]))
    put_u32(h, int(vp["target_h"]))
    put_u32(h, int(vp["extra_gap_bars"]))
    put_u32(h, int(vp["min_train_rows"]))
    put_i64(h, int(header["development_exclusive_end_at"]))
    put_u32(h, int(header["development_row_count"]))
    put_i64(h, int(header["holdout_start_at"]))
    folds = header.get("folds") or []
    put_u32(h, len(folds))
    for fj in folds:
        put_u32(h, int(fj["train_begin"]))
        put_u32(h, int(fj["train_end"]))
        put_u32(h, int(fj["validation_begin"]))
        put_u32(h, int(fj["validation_end"]))
        put_i64(h, int(fj["validation_boundary_start_at"]))
        put_i64(h, int(fj["validation_boundary_end_at"]))
        put_i64(h, int(fj["train_first_at"]))
        put_i64(h, int(fj["train_last_at"]))
        put_i64(h, int(fj["validation_first_actual_at"]))
        put_i64(h, int(fj["validation_last_actual_at"]))
