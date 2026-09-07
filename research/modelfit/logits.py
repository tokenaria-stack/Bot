"""oof-logits-v1 JSONL reader/writer and ContentDigest (OL1C)."""

from __future__ import annotations

import json
import math
import os
from typing import Any, Dict, List, Tuple

from .hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_f64, put_i64, put_string, put_u32
from .spec import CLASS_ORDER, ModelSpec, spec_from_json

FORMAT = "oof-logits-v1"
AT_UNIT = "unix_ms"


def json_roundtrip_float(v: float) -> float:
    return float(json.loads(json.dumps(float(v))))


def json_roundtrip_array(arr) -> List:
    return json.loads(json.dumps([json_roundtrip_float(float(x)) for x in arr]))


def json_roundtrip_matrix(mat) -> List[List[float]]:
    return [json_roundtrip_array(row) for row in mat]


def write_jsonl_atomic(path: str, records: List[Dict[str, Any]]) -> None:
    parent = os.path.dirname(path)
    if parent and not os.path.isdir(parent):
        raise FileNotFoundError(parent)
    tmp = path + ".tmp"
    try:
        with open(tmp, "w", encoding="utf-8") as f:
            for rec in records:
                f.write(json.dumps(rec, ensure_ascii=True, separators=(",", ":"), allow_nan=False))
                f.write("\n")
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, path)
    except Exception:
        if os.path.exists(tmp):
            os.remove(tmp)
        raise


def hash_oof_logits(header: Dict[str, Any], rows: List[Dict[str, Any]]) -> bytes:
    h = new_sha()
    put_string(h, "OL1C")
    put_string(h, header["format_version"])
    put_string(h, header["at_unit"])
    put_digest(h, parse_digest_hex(header["source_oof_matrix_content_digest"]))
    m = header["market"]
    put_string(h, m["venue"])
    put_string(h, m["instrument"])
    put_string(h, m["contract"])
    put_string(h, m["timeframe"])
    ids = header["feature_ids"]
    put_u32(h, len(ids))
    for fid in ids:
        put_string(h, fid)
    put_string(h, header["model_logic_version"])
    put_digest(h, parse_digest_hex(header["model_spec_digest"]))
    blob = json.dumps(header["model_spec"], sort_keys=True, separators=(",", ":"), ensure_ascii=True)
    put_string(h, blob)
    order = header["class_order"]
    put_u32(h, len(order))
    for c in order:
        put_string(h, c)
    rt = header["runtime"]
    put_string(h, rt["python"])
    put_string(h, rt["numpy"])
    put_string(h, rt["sklearn"])
    put_string(h, rt["scipy"])
    folds = header["folds"]
    put_u32(h, len(folds))
    for fj in folds:
        put_u32(h, int(fj["source_train_begin"]))
        put_u32(h, int(fj["source_train_end"]))
        put_u32(h, int(fj["source_validation_begin"]))
        put_u32(h, int(fj["source_validation_end"]))
        put_u32(h, int(fj["output_begin"]))
        put_u32(h, int(fj["output_end"]))
        mean = fj["scaler_mean"]
        put_u32(h, len(mean))
        for v in mean:
            put_f64(h, float(v))
        scale = fj["scaler_scale"]
        put_u32(h, len(scale))
        for v in scale:
            put_f64(h, float(v))
        coef = fj["coef"]
        put_u32(h, len(coef))
        put_u32(h, len(coef[0]) if coef else 0)
        for row in coef:
            for v in row:
                put_f64(h, float(v))
        intercept = fj["intercept"]
        put_u32(h, len(intercept))
        for v in intercept:
            put_f64(h, float(v))
        put_u32(h, int(fj["n_iter"]))
    for r in rows:
        put_i64(h, int(r["at"]))
        put_string(h, str(r["outcome"]))
        logits = r["logits"]
        put_u32(h, len(logits))
        for v in logits:
            put_f64(h, float(v))
    put_u32(h, len(rows))
    if rows:
        put_i64(h, int(rows[0]["at"]))
        put_i64(h, int(rows[-1]["at"]))
    return h.digest()


def read_oof_logits(path: str) -> Tuple[Dict[str, Any], List[Dict[str, Any]], Dict[str, Any]]:
    with open(path, "r", encoding="utf-8") as f:
        lines = [ln.strip() for ln in f if ln.strip()]
    if len(lines) < 3:
        raise ValueError("modelfit: oof-logits too short")
    header = json.loads(lines[0])
    footer = json.loads(lines[-1])
    rows = [json.loads(ln) for ln in lines[1:-1]]
    if header.get("kind") != "header" or footer.get("kind") != "footer":
        raise ValueError("modelfit: oof-logits header/footer kind")
    if header.get("format_version") != FORMAT or header.get("at_unit") != AT_UNIT:
        raise ValueError("modelfit: unknown oof-logits format/AtUnit")
    if header.get("class_order") != list(CLASS_ORDER):
        raise ValueError("modelfit: class_order mismatch")
    last = None
    for r in rows:
        if r.get("kind") != "row":
            raise ValueError("modelfit: expected logits row")
        if r.get("outcome") not in CLASS_ORDER:
            raise ValueError("modelfit: illegal outcome")
        logits = r.get("logits")
        if not isinstance(logits, list) or len(logits) != 3:
            raise ValueError("modelfit: logits must have 3 columns")
        for v in logits:
            if not math.isfinite(float(v)):
                raise ValueError("modelfit: nonfinite logit")
        a = int(r["at"])
        if last is not None and a <= last:
            raise ValueError("modelfit: logits At must be strictly increasing")
        last = a
    n = len(rows)
    if footer.get("row_count") != n:
        raise ValueError("modelfit: logits row count mismatch")
    got = hash_oof_logits(header, rows)
    if got != parse_digest_hex(footer["content_digest"]):
        raise ValueError("modelfit: oof-logits ContentDigest mismatch")
    _validate_output_folds(header.get("folds") or [], n)
    spec = spec_from_json(header["model_spec"])
    if spec.digest_hex() != header["model_spec_digest"]:
        raise ValueError("modelfit: ModelSpecDigest does not match encoded spec")
    return header, rows, footer


def _validate_output_folds(folds: List[Dict[str, Any]], n: int) -> None:
    cursor = 0
    for i, fj in enumerate(folds):
        ob, oe = int(fj["output_begin"]), int(fj["output_end"])
        vb, ve = int(fj["source_validation_begin"]), int(fj["source_validation_end"])
        if ob != cursor:
            raise ValueError("modelfit: output ranges not contiguous from 0")
        if oe < ob:
            raise ValueError("modelfit: empty output range")
        if (oe - ob) != (ve - vb):
            raise ValueError("modelfit: output length != source validation length")
        cursor = oe
    if cursor != n:
        raise ValueError("modelfit: output ranges do not partition rows")
