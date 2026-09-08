"""decision-validation-plan-v1 JSON + DV1C digest."""

from __future__ import annotations

import json
import os
from typing import Any, Dict

from research.modelfit.hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_i64, put_string, put_u32

FORMAT = "decision-validation-plan-v1"
LOGIC_V1 = "decision-validation:walk-forward-v1"
AT_UNIT = "unix_ms"


def hash_plan(doc: Dict[str, Any]) -> bytes:
    h = new_sha()
    put_string(h, "DV1C")
    put_string(h, doc["format_version"])
    put_string(h, doc["decision_validation_logic_version"])
    src = doc["source"]
    put_digest(h, parse_digest_hex(src["evidence_content_digest"]))
    m = src["market"]
    put_string(h, m["venue"])
    put_string(h, m["instrument"])
    put_string(h, m["contract"])
    put_string(h, m["timeframe"])
    put_string(h, src["at_unit"])
    put_u32(h, int(src["row_count"]))
    put_i64(h, int(src["first_at"]))
    put_i64(h, int(src["last_at"]))
    put_digest(h, parse_digest_hex(src["target_digest"]))
    put_string(h, src["label_logic_version"])
    r = doc["rules"]
    put_i64(h, int(r["holdout_start_at"]))
    put_u32(h, int(r["validation_span_bars"]))
    put_u32(h, int(r["fold_count"]))
    put_u32(h, int(r["min_train_rows"]))
    put_u32(h, int(r["target_h"]))
    put_u32(h, int(r["extra_gap_bars"]))
    c = doc["compiled"]
    put_u32(h, int(c["total_causal_bars"]))
    put_i64(h, int(c["development_exclusive_end_at"]))
    put_u32(h, int(c["development_end_index"]))
    put_u32(h, int(c["holdout_begin_index"]))
    folds = c["folds"]
    put_u32(h, len(folds))
    for f in folds:
        put_u32(h, int(f["train_begin"]))
        put_u32(h, int(f["train_end"]))
        put_u32(h, int(f["val_begin"]))
        put_u32(h, int(f["val_end"]))
        put_i64(h, int(f["val_boundary_start_at"]))
        put_i64(h, int(f["val_boundary_end_at"]))
        put_i64(h, int(f["train_first_at"]))
        put_i64(h, int(f["train_last_at"]))
        put_i64(h, int(f["val_first_at"]))
        put_i64(h, int(f["val_last_at"]))
    return h.digest()


def write_plan_atomic(path: str, doc: Dict[str, Any]) -> None:
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


def read_plan(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        doc = json.load(f)
    if doc.get("format_version") != FORMAT:
        raise ValueError("decisionplan: unknown format")
    if doc.get("decision_validation_logic_version") != LOGIC_V1:
        raise ValueError("decisionplan: unknown logic")
    blob = json.dumps(doc)
    for banned in ("probabilities", "directional_rank", "feature_ids", "available_at", "validation_plan_digest"):
        if banned in blob:
            raise ValueError(f"decisionplan: forbidden field {banned}")
    stored = parse_digest_hex(doc["content_digest"])
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    got = hash_plan(body)
    if got != stored:
        raise ValueError("decisionplan: ContentDigest mismatch")
    return doc


def attach_digest(doc: Dict[str, Any]) -> Dict[str, Any]:
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    out = dict(body)
    out["content_digest"] = digest_hex(hash_plan(body))
    return out
