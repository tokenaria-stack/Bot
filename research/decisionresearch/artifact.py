"""decision-research-v1 JSON + DR1C digest."""

from __future__ import annotations

import json
import os
from typing import Any, Dict, List

from research.modelfit.hashwire import digest_hex, new_sha, parse_digest_hex, put_digest, put_f64, put_string, put_u32, put_u8

FORMAT = "decision-research-v1"
LOGIC_V1 = "decision-research:fixed-grid-v1"
DECISION_LOGIC = "decision:target-utility-rank-gate-v1"
AT_UNIT = "unix_ms"
THRESHOLD_LAW = "base=min(U,L); min_EU=base*(float64(du)/10.0); min_rank=float64(dr)/10.0"
OBJECTIVE_LAW = "maximize_train_TotalUtility_all_rows"
BASELINE_LAW = "ABSTAIN_BASELINE TotalUtility=0; replace iff candidate TotalUtility>incumbent"
TIE_LAW = "equal_positive_TotalUtility: higher utility_decile then higher rank_decile"
PROTOCOL_LAW = "train_select_one; validation_evaluate_selected_only"

_BANNED = (
    "probabilities",
    "directional_rank",
    "feature_ids",
    "available_at",
    "ohlcv",
    "logits",
    "pnl",
    "sharpe",
    "drawdown",
    "holdout",
    "candidate_utilities",
    "heatmap",
    "leaderboard",
    "second_best",
)


def _put_market(h, m: Dict[str, Any]) -> None:
    put_string(h, m["venue"])
    put_string(h, m["instrument"])
    put_string(h, m["contract"])
    put_string(h, m["timeframe"])


def _put_audit(h, a: Dict[str, Any]) -> None:
    put_u32(h, int(a["n"]))
    put_u32(h, int(a["up_intent"]))
    put_u32(h, int(a["down_intent"]))
    put_u32(h, int(a["abstain"]))
    put_u32(h, int(a["up_intent_up_first"]))
    put_u32(h, int(a["up_intent_down_first"]))
    put_u32(h, int(a["up_intent_timeout"]))
    put_u32(h, int(a["down_intent_up_first"]))
    put_u32(h, int(a["down_intent_down_first"]))
    put_u32(h, int(a["down_intent_timeout"]))
    put_u32(h, int(a["abstain_up_first"]))
    put_u32(h, int(a["abstain_down_first"]))
    put_u32(h, int(a["abstain_timeout"]))
    put_f64(h, float(a["total_utility"]))


def _put_spec(h, s: Dict[str, Any]) -> None:
    put_string(h, s["logic"])
    put_digest(h, parse_digest_hex(s["target_digest"]))
    put_f64(h, float(s["upper_barrier_utility"]))
    put_f64(h, float(s["lower_barrier_utility"]))
    put_f64(h, float(s["min_expected_target_utility"]))
    put_f64(h, float(s["min_abs_directional_rank"]))
    order = s["class_order"]
    put_u32(h, len(order))
    for c in order:
        put_string(h, c)


def hash_research(doc: Dict[str, Any]) -> bytes:
    h = new_sha()
    put_string(h, "DR1C")
    put_string(h, doc["format_version"])
    put_string(h, doc["decision_research_logic_version"])
    src = doc["source"]
    put_digest(h, parse_digest_hex(src["decision_validation_plan_content_digest"]))
    put_digest(h, parse_digest_hex(src["oof_forecast_evidence_content_digest"]))
    _put_market(h, src["market"])
    put_string(h, src["at_unit"])
    dc = doc["decision_contract"]
    put_string(h, dc["decision_logic_version"])
    tl = doc["target_law"]
    put_digest(h, parse_digest_hex(tl["target_digest"]))
    order = tl["class_order"]
    put_u32(h, len(order))
    for c in order:
        put_string(h, c)
    put_f64(h, float(tl["upper_barrier_utility"]))
    put_f64(h, float(tl["lower_barrier_utility"]))
    sel = doc["selector"]
    ud = sel["utility_deciles"]
    rd = sel["rank_deciles"]
    put_u32(h, len(ud))
    for x in ud:
        put_u32(h, int(x))
    put_u32(h, len(rd))
    for x in rd:
        put_u32(h, int(x))
    put_u32(h, int(sel["candidate_count"]))
    put_string(h, sel["canonical_threshold_resolution_law"])
    put_string(h, sel["objective_law"])
    put_string(h, sel["baseline_law"])
    put_string(h, sel["tie_law"])
    put_string(h, sel["train_select_validation_evaluate_law"])
    acc = doc["acceptance"]
    put_u8(h, 1 if acc["pooled_total_utility_strictly_positive"] else 0)
    put_u32(h, int(acc["positive_fold_fraction_numerator"]))
    put_u32(h, int(acc["positive_fold_fraction_denominator"]))
    folds: List[Dict[str, Any]] = doc["fold_results"]
    put_u32(h, len(folds))
    for f in folds:
        put_u32(h, int(f["fold_index"]))
        s = f["selection"]
        put_string(h, s["kind"])
        if s["kind"] == "DECISION_SPEC":
            put_u32(h, int(s["utility_decile"]))
            put_u32(h, int(s["rank_decile"]))
            _put_spec(h, s["resolved"])
        _put_audit(h, f["train_audit"])
        _put_audit(h, f["validation_audit"])
        put_u8(h, 1 if f["validation_positive"] else 0)
    ag = doc["aggregate"]
    put_u32(h, int(ag["validation_n"]))
    _put_audit(h, {
        "n": ag["validation_n"],
        "up_intent": ag["pooled_up_intent"],
        "down_intent": ag["pooled_down_intent"],
        "abstain": ag["pooled_abstain"],
        "up_intent_up_first": ag["pooled_up_intent_up_first"],
        "up_intent_down_first": ag["pooled_up_intent_down_first"],
        "up_intent_timeout": ag["pooled_up_intent_timeout"],
        "down_intent_up_first": ag["pooled_down_intent_up_first"],
        "down_intent_down_first": ag["pooled_down_intent_down_first"],
        "down_intent_timeout": ag["pooled_down_intent_timeout"],
        "abstain_up_first": ag["pooled_abstain_up_first"],
        "abstain_down_first": ag["pooled_abstain_down_first"],
        "abstain_timeout": ag["pooled_abstain_timeout"],
        "total_utility": ag["pooled_total_utility"],
    })
    put_u32(h, int(ag["positive_fold_count"]))
    put_u32(h, int(ag["fold_count"]))
    put_u8(h, 1 if ag["eligible_for_finalization"] else 0)
    return h.digest()


def write_research_atomic(path: str, doc: Dict[str, Any]) -> None:
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


def _forbid(doc: Dict[str, Any]) -> None:
    blob = json.dumps(doc)
    for banned in _BANNED:
        if banned in blob:
            raise ValueError(f"decisionresearch: forbidden field {banned}")


def _require_audit(a: Dict[str, Any], n_key: str = "n") -> None:
    n = int(a[n_key] if n_key in a else a["n"])
    if int(a["up_intent"]) + int(a["down_intent"]) + int(a["abstain"]) != n:
        raise ValueError("decisionresearch: intent counts != N")
    cells = (
        int(a["up_intent_up_first"]) + int(a["up_intent_down_first"]) + int(a["up_intent_timeout"])
        + int(a["down_intent_up_first"]) + int(a["down_intent_down_first"]) + int(a["down_intent_timeout"])
        + int(a["abstain_up_first"]) + int(a["abstain_down_first"]) + int(a["abstain_timeout"])
    )
    if cells != n:
        raise ValueError("decisionresearch: contingency sum != N")


def read_research(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        doc = json.load(f)
    if doc.get("format_version") != FORMAT:
        raise ValueError("decisionresearch: unknown format")
    if doc.get("decision_research_logic_version") != LOGIC_V1:
        raise ValueError("decisionresearch: unknown logic")
    _forbid(doc)
    stored = parse_digest_hex(doc["content_digest"])
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    got = hash_research(body)
    if got != stored:
        raise ValueError("decisionresearch: ContentDigest mismatch")
    for f in doc["fold_results"]:
        _require_audit(f["train_audit"])
        _require_audit(f["validation_audit"])
    ag = doc["aggregate"]
    pooled = {
        "n": ag["validation_n"],
        "up_intent": ag["pooled_up_intent"],
        "down_intent": ag["pooled_down_intent"],
        "abstain": ag["pooled_abstain"],
        "up_intent_up_first": ag["pooled_up_intent_up_first"],
        "up_intent_down_first": ag["pooled_up_intent_down_first"],
        "up_intent_timeout": ag["pooled_up_intent_timeout"],
        "down_intent_up_first": ag["pooled_down_intent_up_first"],
        "down_intent_down_first": ag["pooled_down_intent_down_first"],
        "down_intent_timeout": ag["pooled_down_intent_timeout"],
        "abstain_up_first": ag["pooled_abstain_up_first"],
        "abstain_down_first": ag["pooled_abstain_down_first"],
        "abstain_timeout": ag["pooled_abstain_timeout"],
        "total_utility": ag["pooled_total_utility"],
    }
    _require_audit(pooled)
    return doc


def attach_digest(doc: Dict[str, Any]) -> Dict[str, Any]:
    body = {k: v for k, v in doc.items() if k != "content_digest"}
    out = dict(body)
    out["content_digest"] = digest_hex(hash_research(body))
    return out
