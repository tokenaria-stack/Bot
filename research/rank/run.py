"""Orchestrate RANK-1. No metrics, calibration, holdout, or sklearn/SciPy gate."""

from __future__ import annotations

import os
from typing import Any, Dict, Optional, Tuple

import numpy as np

from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.modelfit.spec import CLASS_ORDER

from .artifact import FORMAT, attach_digest, read_rank, roundtrip_sorted, write_rank_atomic
from .env import pinned_rank_runtime, require_rank_runtime
from .reference import BUILD_PARAMS, build_rank_reference
from .spec import RankSpec, pinned_rank_spec, spec_from_json


def rank_filename(logits_digest: bytes, spec_digest: bytes) -> str:
    return f"oof-{logits_digest[:8].hex()}_rank-{spec_digest[:8].hex()}.emprank"


def require_class_order(order) -> None:
    if list(order) != list(CLASS_ORDER):
        raise RuntimeError("rank: source class_order mismatch")


def generate_rank(
    logits_path: str,
    expect_logits_digest: str,
    out_path: str,
    *,
    spec: Optional[RankSpec] = None,
) -> Tuple[Dict[str, Any], bool]:
    if BUILD_PARAMS != ("logits", "spec"):
        raise RuntimeError("rank: build_rank_reference signature law broken")
    runtime = require_rank_runtime()
    spec = spec or pinned_rank_spec()
    if tuple(spec.class_order) != CLASS_ORDER:
        raise RuntimeError("rank: RankSpec class_order mismatch")
    pin = pinned_rank_runtime()
    if os.path.exists(out_path):
        return _match_existing(out_path, expect_logits_digest, spec, runtime, pin)

    header, rows, footer = read_oof_logits(logits_path)
    require_class_order(header["class_order"])
    src_hex = footer["content_digest"].lower()
    expect = expect_logits_digest.strip().lower()
    parse_digest_hex(expect)
    if src_hex != expect:
        raise RuntimeError("rank: source ContentDigest != expected")
    z = _logits_only(rows)
    ref = build_rank_reference(z, spec)
    stored = roundtrip_sorted(ref.sorted_d)
    n = len(stored)
    if n != len(rows):
        raise RuntimeError("rank: reference count != source rows")
    doc = {
        "format_version": FORMAT,
        "source": {
            "oof_logits_content_digest": src_hex,
            "source_row_count": n,
        },
        "rank": {
            "rank_logic_version": spec.logic,
            "rank_spec_digest": spec.digest_hex(),
            "resolved_spec": spec.as_json(),
        },
        "runtime": dict(runtime),
        "reference": {"sorted_directional_values": stored},
    }
    doc = attach_digest(doc)
    write_rank_atomic(out_path, doc)
    back = read_rank(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("rank: readback digest mismatch")
    return back, False


def _logits_only(rows) -> np.ndarray:
    z = np.empty((len(rows), 3), dtype=np.float64)
    for i, r in enumerate(rows):
        z[i, :] = np.asarray(r["logits"], dtype=np.float64)
    return z


def _match_existing(
    out_path: str,
    expect_logits_digest: str,
    spec: RankSpec,
    runtime: Dict[str, str],
    pin: Dict[str, str],
) -> Tuple[Dict[str, Any], bool]:
    try:
        doc = read_rank(out_path)
    except Exception as e:
        raise RuntimeError(f"rank: refuse existing {out_path}: {e}") from e
    src = doc["source"]["oof_logits_content_digest"].lower()
    if src != expect_logits_digest.strip().lower():
        raise RuntimeError("rank: refuse overwrite (source digest)")
    if doc["rank"]["rank_spec_digest"] != spec.digest_hex():
        raise RuntimeError("rank: refuse overwrite (RankSpecDigest)")
    if spec_from_json(doc["rank"]["resolved_spec"]).payload() != spec.payload():
        raise RuntimeError("rank: refuse overwrite (RankSpec)")
    rec = doc.get("runtime") or {}
    if rec != pin or rec != runtime:
        raise RuntimeError("rank: refuse existing runtime/pin mismatch")
    return doc, True
