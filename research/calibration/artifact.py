"""temperature-calibration-v1 single JSON object + TC1C digest."""

from __future__ import annotations

import json
import math
import os
from typing import Any, Dict

from research.modelfit.hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_f64, put_i64, put_string, put_u32, put_u8
from research.modelfit.logits import json_roundtrip_float

from .spec import CalibrationSpec, spec_from_json

FORMAT = "temperature-calibration-v1"


def hash_calibration(doc: Dict[str, Any]) -> bytes:
    h = new_sha()
    put_string(h, "TC1C")
    put_string(h, doc["format_version"])
    src = doc["source"]
    put_digest(h, parse_digest_hex(src["oof_logits_content_digest"]))
    put_u32(h, int(src["source_row_count"]))
    put_i64(h, int(src["source_first_at"]))
    put_i64(h, int(src["source_last_at"]))
    cal = doc["calibration"]
    put_string(h, cal["logic_version"])
    put_digest(h, parse_digest_hex(cal["calibration_spec_digest"]))
    blob = json.dumps(cal["resolved_spec"], sort_keys=True, separators=(",", ":"), ensure_ascii=True)
    put_string(h, blob)
    order = cal["class_order"]
    put_u32(h, len(order))
    for c in order:
        put_string(h, c)
    rt = doc["runtime"]
    put_string(h, rt["python"])
    put_string(h, rt["numpy"])
    put_string(h, rt["scipy"])
    fit = doc["fit"]
    put_f64(h, float(fit["beta"]))
    put_u8(h, 1 if fit["success"] else 0)
    put_u32(h, int(fit["iterations"]))
    put_f64(h, float(fit["gradient_norm"]))
    put_f64(h, float(fit["objective"]))
    return h.digest()


def write_calibration_atomic(path: str, doc: Dict[str, Any]) -> None:
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


def read_calibration(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        doc = json.load(f)
    if doc.get("format_version") != FORMAT:
        raise ValueError("calibration: unknown format")
    stored = parse_digest_hex(doc["content_digest"])
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    got = hash_calibration(body)
    if got != stored:
        raise ValueError("calibration: ContentDigest mismatch")
    spec = spec_from_json(doc["calibration"]["resolved_spec"])
    if spec.digest_hex() != doc["calibration"]["calibration_spec_digest"]:
        raise ValueError("calibration: CalibrationSpecDigest mismatch")
    beta = float(doc["fit"]["beta"])
    if not math.isfinite(beta) or beta < 0:
        raise ValueError("calibration: illegal stored beta")
    if not doc["fit"]["success"]:
        raise ValueError("calibration: stored optimizer not successful")
    if not math.isfinite(float(doc["fit"]["objective"])) or not math.isfinite(float(doc["fit"]["gradient_norm"])):
        raise ValueError("calibration: nonfinite stored audit")
    return doc


def attach_digest(doc: Dict[str, Any]) -> Dict[str, Any]:
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    d = hash_calibration(body)
    out = dict(body)
    out["content_digest"] = digest_hex(d)
    return out


def roundtrip_fit_fields(beta: float, gnorm: float, objective: float) -> tuple:
    return json_roundtrip_float(beta), json_roundtrip_float(gnorm), json_roundtrip_float(objective)
