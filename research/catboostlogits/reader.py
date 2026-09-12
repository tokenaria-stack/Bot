"""Read official Go catboost-oof-logits-v1 JSONL. No FeatureIDs, no trees, no OL1C."""

from __future__ import annotations

import json
import math
from dataclasses import dataclass
from typing import Any, Dict, List, Tuple

import numpy as np

from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.spec import CLASS_DOWN, CLASS_TIMEOUT, CLASS_UP, CODE_BY_OUTCOME

FORMAT = "catboost-oof-logits-v1"
CLASS_ORDER = (CLASS_UP, CLASS_DOWN, CLASS_TIMEOUT)
AT_UNIT = "unix_ms"


@dataclass(frozen=True)
class CatBoostOOFLogits:
    header: Dict[str, Any]
    at: np.ndarray
    outcome: np.ndarray
    y: np.ndarray
    logits: np.ndarray
    class_order: Tuple[str, ...]
    content_digest: str
    market: Dict[str, str]
    matrix_digest: str
    spec_digest: str
    fitplan_digest: str


def read_catboost_oof_logits(path: str, expect_digest_hex: str) -> CatBoostOOFLogits:
    expect = parse_digest_hex(expect_digest_hex.strip().lower()).hex()
    with open(path, "r", encoding="utf-8") as f:
        lines = [ln.strip() for ln in f if ln.strip()]
    if len(lines) < 3:
        raise ValueError("catboostlogits: file too short")
    header = json.loads(lines[0])
    footer = json.loads(lines[-1])
    if header.get("kind") != "header":
        raise ValueError("catboostlogits: first record must be header")
    if footer.get("kind") != "footer":
        raise ValueError("catboostlogits: last record must be footer")
    if header.get("format_version") != FORMAT:
        raise ValueError("catboostlogits: unknown format")
    if header.get("at_unit") != AT_UNIT:
        raise ValueError("catboostlogits: at_unit")
    order = tuple(header.get("class_order") or ())
    if order != CLASS_ORDER:
        raise ValueError(f"catboostlogits: class_order {order} != {CLASS_ORDER}")
    stored = str(footer.get("content_digest") or "").strip().lower()
    if stored != expect:
        raise ValueError("catboostlogits: ContentDigest != expected")
    rows = [json.loads(ln) for ln in lines[1:-1]]
    n = len(rows)
    if int(footer["row_count"]) != n:
        raise ValueError("catboostlogits: row_count")
    at = np.empty(n, dtype=np.int64)
    y = np.empty(n, dtype=np.int64)
    outcome = np.empty(n, dtype=object)
    z = np.empty((n, 3), dtype=np.float64)
    last = None
    for i, r in enumerate(rows):
        if r.get("kind") != "row":
            raise ValueError("catboostlogits: expected row")
        a = int(r["at"])
        if last is not None and a <= last:
            raise ValueError("catboostlogits: At not strictly increasing")
        last = a
        out = str(r["outcome"])
        if out not in CODE_BY_OUTCOME:
            raise ValueError(f"catboostlogits: illegal outcome {out!r}")
        lg = r["logits"]
        if len(lg) != 3:
            raise ValueError("catboostlogits: logits length")
        trip = (float(lg[0]), float(lg[1]), float(lg[2]))
        if any(not math.isfinite(x) for x in trip):
            raise ValueError("catboostlogits: nonfinite logits")
        at[i] = a
        outcome[i] = out
        y[i] = CODE_BY_OUTCOME[out]
        z[i, :] = trip
    if int(footer["first_at"]) != int(at[0]) or int(footer["last_at"]) != int(at[-1]):
        raise ValueError("catboostlogits: footer first/last At")
    m = header["market"]
    market = {
        "venue": str(m["venue"]),
        "instrument": str(m["instrument"]),
        "contract": str(m["contract"]),
        "timeframe": str(m["timeframe"]),
    }
    return CatBoostOOFLogits(
        header=header,
        at=at,
        outcome=outcome,
        y=y,
        logits=z,
        class_order=CLASS_ORDER,
        content_digest=stored,
        market=market,
        matrix_digest=str(header["source_oof_matrix_content_digest"]).lower(),
        spec_digest=str(header["catboost_spec_digest"]).lower(),
        fitplan_digest=str(header["catboost_fitplan_digest"]).lower(),
    )
