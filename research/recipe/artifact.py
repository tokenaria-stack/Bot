"""forecast-recipe-v1 JSON + FR1C digest. Bindings and laws only."""

from __future__ import annotations

import json
import os
from typing import Any, Dict

from research.modelfit.hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_string, put_u32
from .laws import FORMAT, LOGIC_V1, OUTPUTS, PROJECTION, require_class_order


def hash_recipe(doc: Dict[str, Any]) -> bytes:
    h = new_sha()
    put_string(h, "FR1C")
    put_string(h, doc["format_version"])
    put_string(h, doc["recipe_logic_version"])
    bm = doc["base_model"]
    put_string(h, bm["model_logic_version"])
    put_digest(h, parse_digest_hex(bm["model_spec_digest"]))
    put_digest(h, parse_digest_hex(doc["research_source"]["oof_logits_content_digest"]))
    put_digest(h, parse_digest_hex(doc["calibration"]["content_digest"]))
    put_digest(h, parse_digest_hex(doc["rank"]["content_digest"]))
    order = doc["class_order"]
    put_u32(h, len(order))
    for c in order:
        put_string(h, c)
    pr = doc["projection"]
    put_string(h, pr["probability_projection"])
    put_string(h, pr["beta_zero_law"])
    put_string(h, pr["stable_softmax_law"])
    put_string(h, pr["rank_projection"])
    put_string(h, pr["branch_independence"])
    out = doc["outputs"]
    put_string(h, out["probabilities"])
    put_string(h, out["directional_rank"])
    return h.digest()


def write_recipe_atomic(path: str, doc: Dict[str, Any]) -> None:
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


def read_recipe(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        doc = json.load(f)
    if doc.get("format_version") != FORMAT:
        raise ValueError("recipe: unknown format")
    if doc.get("recipe_logic_version") != LOGIC_V1:
        raise ValueError("recipe: unknown logic")
    require_class_order(doc.get("class_order"))
    if doc.get("projection") != PROJECTION:
        raise ValueError("recipe: projection law mismatch")
    if doc.get("outputs") != OUTPUTS:
        raise ValueError("recipe: output schema mismatch")
    forbidden = (
        "beta",
        "fit",
        "reference",
        "runtime",
        "logits_path",
        "calibration_path",
        "rank_path",
        "path",
        "sorted_directional_values",
    )
    for key in forbidden:
        if key in doc:
            raise ValueError(f"recipe: forbidden field {key}")
    blob = json.dumps(doc)
    if "sorted_directional_values" in blob:
        raise ValueError("recipe: rank reference must not be stored")
    stored = parse_digest_hex(doc["content_digest"])
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    got = hash_recipe(body)
    if got != stored:
        raise ValueError("recipe: ContentDigest mismatch")
    return doc


def attach_digest(doc: Dict[str, Any]) -> Dict[str, Any]:
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    d = hash_recipe(body)
    out = dict(body)
    out["content_digest"] = digest_hex(d)
    return out
