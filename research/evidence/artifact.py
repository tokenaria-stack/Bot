"""oof-forecast-evidence-v1 JSONL + OE1C digest."""

from __future__ import annotations

import json
import math
import os
from typing import Any, Dict, List, Tuple

from research.modelfit.hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_f64, put_i64, put_string, put_u32
from research.modelfit.logits import json_roundtrip_array, json_roundtrip_float, write_jsonl_atomic
from research.modelfit.spec import CLASS_ORDER
from research.recipe.laws import require_class_order

FORMAT = "oof-forecast-evidence-v1"
LOGIC_V1 = "oof-evidence:recipe-projection-v1"
AT_UNIT = "unix_ms"


def hash_evidence(header: Dict[str, Any], rows: List[Dict[str, Any]], footer: Dict[str, Any]) -> bytes:
    h = new_sha()
    put_string(h, "OE1C")
    put_string(h, header["format_version"])
    put_string(h, header["evidence_logic_version"])
    src = header["source"]
    put_digest(h, parse_digest_hex(src["oof_logits_content_digest"]))
    put_digest(h, parse_digest_hex(src["oof_matrix_content_digest"]))
    m = src["market"]
    put_string(h, m["venue"])
    put_string(h, m["instrument"])
    put_string(h, m["contract"])
    put_string(h, m["timeframe"])
    put_string(h, src["at_unit"])
    put_digest(h, parse_digest_hex(header["recipe"]["content_digest"]))
    tc = header["truth_contract"]
    order = tc["class_order"]
    put_u32(h, len(order))
    for c in order:
        put_string(h, c)
    put_digest(h, parse_digest_hex(tc["target_digest"]))
    put_string(h, tc["label_logic_version"])
    rt = header["runtime"]
    put_string(h, rt["python"])
    put_string(h, rt["numpy"])
    for r in rows:
        put_i64(h, int(r["at"]))
        put_string(h, str(r["outcome"]))
        p = r["probabilities"]
        put_u32(h, len(p))
        for v in p:
            put_f64(h, float(v))
        put_f64(h, float(r["directional_rank"]))
    put_u32(h, int(footer["row_count"]))
    put_i64(h, int(footer["first_at"]))
    put_i64(h, int(footer["last_at"]))
    return h.digest()


def read_evidence(path: str) -> Tuple[Dict[str, Any], List[Dict[str, Any]], Dict[str, Any]]:
    with open(path, "r", encoding="utf-8") as f:
        lines = [ln.strip() for ln in f if ln.strip()]
    if len(lines) < 3:
        raise ValueError("evidence: too short")
    header = json.loads(lines[0])
    footer = json.loads(lines[-1])
    rows = [json.loads(ln) for ln in lines[1:-1]]
    if header.get("kind") != "header" or footer.get("kind") != "footer":
        raise ValueError("evidence: header/footer kind")
    if header.get("format_version") != FORMAT:
        raise ValueError("evidence: unknown format")
    if header.get("evidence_logic_version") != LOGIC_V1:
        raise ValueError("evidence: unknown logic")
    if header.get("source", {}).get("at_unit") != AT_UNIT:
        raise ValueError("evidence: at_unit")
    require_class_order(header["truth_contract"]["class_order"])
    parse_digest_hex(header["source"]["oof_logits_content_digest"])
    parse_digest_hex(header["source"]["oof_matrix_content_digest"])
    parse_digest_hex(header["recipe"]["content_digest"])
    parse_digest_hex(header["truth_contract"]["target_digest"])
    if not str(header["truth_contract"].get("label_logic_version") or ""):
        raise ValueError("evidence: label_logic_version")
    rt = header.get("runtime") or {}
    if "python" not in rt or "numpy" not in rt:
        raise ValueError("evidence: runtime provenance")
    if "sklearn" in rt or "scipy" in rt:
        raise ValueError("evidence: unexpected runtime keys")
    blob = json.dumps(header)
    for banned in (
        "feature_ids",
        "feature_plan_digest",
        "feature_tape_content_digest",
        "sorted_directional_values",
        "final_model",
        "bundle",
        "calibration_content_digest",
        "rank_content_digest",
    ):
        if banned in blob:
            raise ValueError(f"evidence: forbidden header field {banned}")
    last = None
    for r in rows:
        if r.get("kind") != "row":
            raise ValueError("evidence: expected row")
        if "logits" in r or "features" in r or "fold" in r:
            raise ValueError("evidence: forbidden row field")
        if r.get("outcome") not in CLASS_ORDER:
            raise ValueError("evidence: illegal outcome")
        p = r.get("probabilities")
        if not isinstance(p, list) or len(p) != 3:
            raise ValueError("evidence: probabilities")
        for v in p:
            x = float(v)
            if not math.isfinite(x) or x < 0.0 or x > 1.0:
                raise ValueError("evidence: illegal probability")
        rk = float(r["directional_rank"])
        if not math.isfinite(rk) or rk < -1.0 or rk > 1.0:
            raise ValueError("evidence: illegal directional_rank")
        a = int(r["at"])
        if last is not None and a <= last:
            raise ValueError("evidence: At must be strictly increasing")
        last = a
    n = len(rows)
    if int(footer["row_count"]) != n:
        raise ValueError("evidence: row_count")
    if n:
        if int(footer["first_at"]) != int(rows[0]["at"]) or int(footer["last_at"]) != int(rows[-1]["at"]):
            raise ValueError("evidence: first/last At")
    got = hash_evidence(header, rows, footer)
    if got != parse_digest_hex(footer["content_digest"]):
        raise ValueError("evidence: ContentDigest mismatch")
    return header, rows, footer


def write_evidence_atomic(path: str, header: Dict[str, Any], rows: List[Dict[str, Any]], footer: Dict[str, Any]) -> None:
    parent = os.path.dirname(path)
    if parent and not os.path.isdir(parent):
        raise FileNotFoundError(parent)
    write_jsonl_atomic(path, [header] + rows + [footer])


def roundtrip_evidence_fields(probabilities, rank: float) -> Tuple[List[float], float]:
    return json_roundtrip_array(probabilities), json_roundtrip_float(rank)
