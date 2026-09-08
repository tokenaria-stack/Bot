"""forecast-bundle-v1 JSON + FB1C digest."""

from __future__ import annotations

import json
import os
from typing import Any, Dict

from research.modelfit.hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_string, put_u32
from research.recipe.laws import require_class_order

from .laws import COMPOSITION, FORMAT, LOGIC_V1, PUBLIC_OUTPUT, RAW_MODEL_PROJECTION


def hash_bundle(doc: Dict[str, Any]) -> bytes:
    h = new_sha()
    put_string(h, "FB1C")
    put_string(h, doc["format_version"])
    put_string(h, doc["bundle_logic_version"])
    b = doc["bindings"]
    put_digest(h, parse_digest_hex(b["final_model_content_digest"]))
    put_digest(h, parse_digest_hex(b["recipe_content_digest"]))
    put_digest(h, parse_digest_hex(b["calibration_content_digest"]))
    put_digest(h, parse_digest_hex(b["rank_content_digest"]))
    ic = doc["input_contract"]
    ids = ic["feature_ids"]
    put_u32(h, len(ids))
    for fid in ids:
        put_string(h, fid)
    put_digest(h, parse_digest_hex(ic["feature_plan_digest"]))
    oc = doc["output_contract"]
    order = oc["class_order"]
    put_u32(h, len(order))
    for c in order:
        put_string(h, c)
    put_digest(h, parse_digest_hex(oc["target_digest"]))
    put_string(h, oc["label_logic_version"])
    put_string(h, doc["raw_model_projection"])
    put_string(h, doc["composition"])
    po = doc["public_output"]
    put_string(h, po["probabilities"])
    put_string(h, po["directional_rank"])
    return h.digest()


def write_bundle_atomic(path: str, doc: Dict[str, Any]) -> None:
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


def read_bundle(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        doc = json.load(f)
    if doc.get("format_version") != FORMAT:
        raise ValueError("bundle: unknown format")
    if doc.get("bundle_logic_version") != LOGIC_V1:
        raise ValueError("bundle: unknown logic")
    if doc.get("raw_model_projection") != RAW_MODEL_PROJECTION:
        raise ValueError("bundle: raw-model law mismatch")
    if doc.get("composition") != COMPOSITION:
        raise ValueError("bundle: composition law mismatch")
    if doc.get("public_output") != PUBLIC_OUTPUT:
        raise ValueError("bundle: public output mismatch")
    require_class_order(doc["output_contract"]["class_order"])
    parse_digest_hex(doc["input_contract"]["feature_plan_digest"])
    parse_digest_hex(doc["output_contract"]["target_digest"])
    if not str(doc["output_contract"].get("label_logic_version") or ""):
        raise ValueError("bundle: label_logic_version required")
    forbidden = (
        "beta",
        "runtime",
        "scaler",
        "logistic",
        "sorted_directional_values",
        "feature_tape_content_digest",
        "path",
    )
    for key in forbidden:
        if key in doc:
            raise ValueError(f"bundle: forbidden field {key}")
    ic = doc["input_contract"]
    if "feature_tape_content_digest" in ic:
        raise ValueError("bundle: historical tape identity in live contract")
    stored = parse_digest_hex(doc["content_digest"])
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    got = hash_bundle(body)
    if got != stored:
        raise ValueError("bundle: ContentDigest mismatch")
    return doc


def attach_digest(doc: Dict[str, Any]) -> Dict[str, Any]:
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    d = hash_bundle(body)
    out = dict(body)
    out["content_digest"] = digest_hex(d)
    return out
