"""final-base-model-v1 JSON + FM1C digest."""

from __future__ import annotations

import json
import math
import os
from typing import Any, Dict, List

from research.modelfit.hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_f64, put_i64, put_string, put_u32
from research.modelfit.spec import CLASS_ORDER

FORMAT = "final-base-model-v1"
FIT_LOGIC_V1 = "final-model-fit:all-development-v1"


def hash_final_model(doc: Dict[str, Any]) -> bytes:
    h = new_sha()
    put_string(h, "FM1C")
    put_string(h, doc["format_version"])
    put_string(h, doc["fit_logic_version"])
    src = doc["source"]
    put_digest(h, parse_digest_hex(src["oof_matrix_content_digest"]))
    m = src["market"]
    put_string(h, m["venue"])
    put_string(h, m["instrument"])
    put_string(h, m["contract"])
    put_string(h, m["timeframe"])
    put_u32(h, int(src["row_count"]))
    put_i64(h, int(src["first_at"]))
    put_i64(h, int(src["last_at"]))
    md = doc["model"]
    put_string(h, md["model_logic_version"])
    put_digest(h, parse_digest_hex(md["model_spec_digest"]))
    ids = md["feature_ids"]
    put_u32(h, len(ids))
    for fid in ids:
        put_string(h, fid)
    order = md["class_order"]
    put_u32(h, len(order))
    for c in order:
        put_string(h, c)
    rt = doc["runtime"]
    put_string(h, rt["python"])
    put_string(h, rt["numpy"])
    put_string(h, rt["sklearn"])
    put_string(h, rt["scipy"])
    mean = doc["scaler"]["mean"]
    put_u32(h, len(mean))
    for v in mean:
        put_f64(h, float(v))
    scale = doc["scaler"]["scale"]
    put_u32(h, len(scale))
    for v in scale:
        put_f64(h, float(v))
    coef = doc["logistic"]["coef"]
    put_u32(h, len(coef))
    put_u32(h, len(coef[0]) if coef else 0)
    for row in coef:
        for v in row:
            put_f64(h, float(v))
    intercept = doc["logistic"]["intercept"]
    put_u32(h, len(intercept))
    for v in intercept:
        put_f64(h, float(v))
    put_u32(h, int(doc["logistic"]["n_iter"]))
    return h.digest()


def write_final_model_atomic(path: str, doc: Dict[str, Any]) -> None:
    parent = os.path.dirname(path)
    if parent and not os.path.isdir(parent):
        raise FileNotFoundError(parent)
    tmp = path + ".tmp"
    try:
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump(doc, f, ensure_ascii=True, separators=(",", ":"), allow_nan=False)
            f.write("\n")
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, path)
    except Exception:
        if os.path.exists(tmp):
            os.remove(tmp)
        raise


def _require_params(doc: Dict[str, Any]) -> None:
    ids = doc["model"]["feature_ids"]
    f = len(ids)
    if f <= 0:
        raise ValueError("finalfit: FeatureIDs required")
    mean = doc["scaler"]["mean"]
    scale = doc["scaler"]["scale"]
    if len(mean) != f or len(scale) != f:
        raise ValueError("finalfit: scaler width")
    for v in mean:
        if not math.isfinite(float(v)):
            raise ValueError("finalfit: nonfinite mean")
    for v in scale:
        x = float(v)
        if not math.isfinite(x) or x <= 0:
            raise ValueError("finalfit: illegal scale")
    coef = doc["logistic"]["coef"]
    intercept = doc["logistic"]["intercept"]
    if len(coef) != 3 or any(len(row) != f for row in coef):
        raise ValueError("finalfit: coef shape")
    if len(intercept) != 3:
        raise ValueError("finalfit: intercept shape")
    for row in coef:
        for v in row:
            if not math.isfinite(float(v)):
                raise ValueError("finalfit: nonfinite coef")
    for v in intercept:
        if not math.isfinite(float(v)):
            raise ValueError("finalfit: nonfinite intercept")
    n_iter = int(doc["logistic"]["n_iter"])
    if n_iter < 1:
        raise ValueError("finalfit: illegal n_iter")
    if list(doc["model"]["class_order"]) != list(CLASS_ORDER):
        raise ValueError("finalfit: class_order")


def read_final_model(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        doc = json.load(f)
    if doc.get("format_version") != FORMAT:
        raise ValueError("finalfit: unknown format")
    if doc.get("fit_logic_version") != FIT_LOGIC_V1:
        raise ValueError("finalfit: unknown fit logic")
    if "recipe_content_digest" in doc or "beta" in doc or "rank" in doc:
        raise ValueError("finalfit: forbidden composition fields")
    _require_params(doc)
    stored = parse_digest_hex(doc["content_digest"])
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    got = hash_final_model(body)
    if got != stored:
        raise ValueError("finalfit: ContentDigest mismatch")
    rt = doc.get("runtime") or {}
    for k in ("python", "numpy", "sklearn", "scipy"):
        if k not in rt:
            raise ValueError("finalfit: runtime provenance")
    return doc


def attach_digest(doc: Dict[str, Any]) -> Dict[str, Any]:
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    d = hash_final_model(body)
    out = dict(body)
    out["content_digest"] = digest_hex(d)
    return out
