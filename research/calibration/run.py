"""Orchestrate CALIBRATION-1. No metrics, rank, holdout, or sklearn gate."""

from __future__ import annotations

import os
from typing import Any, Dict, Optional, Tuple

import numpy as np

from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.modelfit.spec import CLASS_ORDER, CODE_BY_OUTCOME

from .artifact import (
    FORMAT,
    attach_digest,
    read_calibration,
    roundtrip_fit_fields,
    write_calibration_atomic,
)
from .env import pinned_calibration_runtime, require_calibration_runtime
from .fit import FIT_PARAMS, fit_temperature
from .spec import CalibrationSpec, pinned_calibration_spec, spec_from_json


def calibration_filename(logits_digest: bytes, spec_digest: bytes) -> str:
    return f"oof-{logits_digest[:8].hex()}_cal-{spec_digest[:8].hex()}.tempcals"


def generate_calibration(
    logits_path: str,
    expect_logits_digest: str,
    out_path: str,
    *,
    spec: Optional[CalibrationSpec] = None,
) -> Tuple[Dict[str, Any], bool]:
    if FIT_PARAMS != ("y", "logits", "spec"):
        raise RuntimeError("calibration: fit_temperature signature law broken")
    runtime = require_calibration_runtime()
    spec = spec or pinned_calibration_spec()
    pin = pinned_calibration_runtime()
    if os.path.exists(out_path):
        return _match_existing(out_path, expect_logits_digest, spec, runtime, pin)

    _, rows, footer = read_oof_logits(logits_path)
    src_hex = footer["content_digest"].lower()
    expect = expect_logits_digest.strip().lower()
    parse_digest_hex(expect)
    if src_hex != expect:
        raise RuntimeError("calibration: source ContentDigest != expected")
    y, z = _arrays(rows)
    fit = fit_temperature(y, z, spec)
    beta, gnorm, objective = roundtrip_fit_fields(fit.beta, fit.gradient_norm, fit.objective)
    doc = {
        "format_version": FORMAT,
        "source": {
            "oof_logits_content_digest": src_hex,
            "source_row_count": len(rows),
            "source_first_at": int(rows[0]["at"]),
            "source_last_at": int(rows[-1]["at"]),
        },
        "calibration": {
            "logic_version": spec.logic,
            "calibration_spec_digest": spec.digest_hex(),
            "resolved_spec": spec.as_json(),
            "class_order": list(CLASS_ORDER),
        },
        "runtime": dict(runtime),
        "fit": {
            "beta": beta,
            "success": True,
            "iterations": fit.iterations,
            "gradient_norm": gnorm,
            "objective": objective,
        },
    }
    doc = attach_digest(doc)
    write_calibration_atomic(out_path, doc)
    back = read_calibration(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("calibration: readback digest mismatch")
    return back, False


def _arrays(rows) -> Tuple[np.ndarray, np.ndarray]:
    y = np.empty(len(rows), dtype=np.int64)
    z = np.empty((len(rows), 3), dtype=np.float64)
    for i, r in enumerate(rows):
        y[i] = CODE_BY_OUTCOME[r["outcome"]]
        z[i, :] = np.asarray(r["logits"], dtype=np.float64)
    return y, z


def _match_existing(
    out_path: str,
    expect_logits_digest: str,
    spec: CalibrationSpec,
    runtime: Dict[str, str],
    pin: Dict[str, str],
) -> Tuple[Dict[str, Any], bool]:
    try:
        doc = read_calibration(out_path)
    except Exception as e:
        raise RuntimeError(f"calibration: refuse existing {out_path}: {e}") from e
    src = doc["source"]["oof_logits_content_digest"].lower()
    if src != expect_logits_digest.strip().lower():
        raise RuntimeError("calibration: refuse overwrite (source digest)")
    if doc["calibration"]["calibration_spec_digest"] != spec.digest_hex():
        raise RuntimeError("calibration: refuse overwrite (CalibrationSpecDigest)")
    if spec_from_json(doc["calibration"]["resolved_spec"]).payload() != spec.payload():
        raise RuntimeError("calibration: refuse overwrite (CalibrationSpec)")
    rec = doc.get("runtime") or {}
    if rec != pin or rec != runtime:
        raise RuntimeError("calibration: refuse existing runtime/pin mismatch")
    return doc, True
