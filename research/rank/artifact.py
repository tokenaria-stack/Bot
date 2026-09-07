"""empirical-rank-v1 single JSON object + ER1C digest."""

from __future__ import annotations

import json
import math
import os
from typing import Any, Dict, List

import numpy as np

from research.modelfit.hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_f64, put_string, put_u32
from research.modelfit.logits import json_roundtrip_array

from .spec import spec_from_json

FORMAT = "empirical-rank-v1"


def hash_rank(doc: Dict[str, Any]) -> bytes:
    h = new_sha()
    put_string(h, "ER1C")
    put_string(h, doc["format_version"])
    src = doc["source"]
    put_digest(h, parse_digest_hex(src["oof_logits_content_digest"]))
    put_u32(h, int(src["source_row_count"]))
    rk = doc["rank"]
    put_string(h, rk["rank_logic_version"])
    put_digest(h, parse_digest_hex(rk["rank_spec_digest"]))
    blob = json.dumps(rk["resolved_spec"], sort_keys=True, separators=(",", ":"), ensure_ascii=True)
    put_string(h, blob)
    rt = doc["runtime"]
    put_string(h, rt["python"])
    put_string(h, rt["numpy"])
    vals = doc["reference"]["sorted_directional_values"]
    put_u32(h, len(vals))
    for v in vals:
        put_f64(h, float(v))
    return h.digest()


def write_rank_atomic(path: str, doc: Dict[str, Any]) -> None:
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


def _require_sorted_finite(vals: List[float], n: int) -> None:
    if n <= 0:
        raise ValueError("rank: source_row_count must be > 0")
    if len(vals) != n:
        raise ValueError("rank: len(sorted_D) != source_row_count")
    prev = None
    for v in vals:
        x = float(v)
        if not math.isfinite(x):
            raise ValueError("rank: nonfinite sorted_D")
        if prev is not None and not (prev <= x):
            raise ValueError("rank: sorted_D is not sorted")
        prev = x


def read_rank(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        doc = json.load(f)
    if doc.get("format_version") != FORMAT:
        raise ValueError("rank: unknown format")
    n = int(doc["source"]["source_row_count"])
    vals = doc["reference"]["sorted_directional_values"]
    if not isinstance(vals, list):
        raise ValueError("rank: sorted_directional_values")
    _require_sorted_finite(vals, n)
    stored = parse_digest_hex(doc["content_digest"])
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    got = hash_rank(body)
    if got != stored:
        raise ValueError("rank: ContentDigest mismatch")
    spec = spec_from_json(doc["rank"]["resolved_spec"])
    if spec.digest_hex() != doc["rank"]["rank_spec_digest"]:
        raise ValueError("rank: RankSpecDigest mismatch")
    if spec.logic != doc["rank"]["rank_logic_version"]:
        raise ValueError("rank: logic version mismatch")
    rt = doc.get("runtime") or {}
    if "python" not in rt or "numpy" not in rt:
        raise ValueError("rank: runtime provenance")
    if "scipy" in rt or "sklearn" in rt:
        raise ValueError("rank: unexpected runtime keys")
    return doc


def attach_digest(doc: Dict[str, Any]) -> Dict[str, Any]:
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    d = hash_rank(body)
    out = dict(body)
    out["content_digest"] = digest_hex(d)
    return out


def roundtrip_sorted(values: np.ndarray) -> List[float]:
    return json_roundtrip_array(values)
