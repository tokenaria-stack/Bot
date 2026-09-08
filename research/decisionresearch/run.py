"""Orchestrate DECISION-RESEARCH-1. MATCH does not reenact the selector."""

from __future__ import annotations

import os
from typing import Any, Dict, Tuple

from research.decisionplan.artifact import read_plan
from research.decisionresearch.artifact import (
    AT_UNIT,
    BASELINE_LAW,
    DECISION_LOGIC,
    FORMAT,
    LOGIC_V1,
    OBJECTIVE_LAW,
    PROTOCOL_LAW,
    THRESHOLD_LAW,
    TIE_LAW,
    attach_digest,
    read_research,
    write_research_atomic,
)
from research.decisionresearch.worker import run_selector
from research.evidence.artifact import read_evidence
from research.modelfit.hashwire import parse_digest_hex


def research_filename(plan_digest: bytes, evidence_digest: bytes) -> str:
    return f"plan-{plan_digest[:8].hex()}_ev-{evidence_digest[:8].hex()}_dres-fixed-grid-v1.dres"


def generate_decision_research(
    plan_path: str,
    expect_plan_digest: str,
    evidence_path: str,
    out_path: str,
) -> Tuple[Dict[str, Any], bool]:
    expect = expect_plan_digest.strip().lower()
    parse_digest_hex(expect)
    plan = read_plan(plan_path)
    if plan["content_digest"].lower() != expect:
        raise RuntimeError("decisionresearch: plan ContentDigest != --expect-plan")
    header, rows, footer = read_evidence(evidence_path)
    ev_digest = footer["content_digest"].lower()
    if ev_digest != plan["source"]["evidence_content_digest"].lower():
        raise RuntimeError("decisionresearch: evidence ContentDigest != plan source")
    _prove_semantics(plan, header, footer, rows)
    if os.path.exists(out_path):
        return _match_existing(out_path, plan, header, footer)

    compiled = plan["compiled"]
    worker = run_selector(plan["source"]["target_digest"], rows, compiled["folds"])
    _require_worker(worker, plan)
    doc = _artifact(plan, footer, worker)
    write_research_atomic(out_path, doc)
    back = read_research(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("decisionresearch: readback mismatch")
    return back, False


def _prove_semantics(plan, header, footer, rows) -> None:
    src = plan["source"]
    if dict(src["market"]) != dict(header["source"]["market"]):
        raise RuntimeError("decisionresearch: market mismatch")
    if src["at_unit"] != AT_UNIT or header["source"]["at_unit"] != AT_UNIT:
        raise RuntimeError("decisionresearch: at_unit")
    if int(src["row_count"]) != int(footer["row_count"]) or int(footer["row_count"]) != len(rows):
        raise RuntimeError("decisionresearch: row_count")
    if src["target_digest"].lower() != header["truth_contract"]["target_digest"].lower():
        raise RuntimeError("decisionresearch: target_digest mismatch")
    if src["label_logic_version"] != header["truth_contract"]["label_logic_version"]:
        raise RuntimeError("decisionresearch: label_logic_version")


def _require_worker(worker: Dict[str, Any], plan: Dict[str, Any]) -> None:
    if worker.get("decision_research_logic_version") != LOGIC_V1:
        raise RuntimeError("decisionresearch: DecisionResearchLogicVersion")
    if worker.get("decision_logic_version") != DECISION_LOGIC:
        raise RuntimeError("decisionresearch: DecisionLogicVersion")
    if worker["target_digest"].lower() != plan["source"]["target_digest"].lower():
        raise RuntimeError("decisionresearch: resolved TargetSpec digest")
    folds = worker["folds"]
    if len(folds) != int(plan["rules"]["fold_count"]):
        raise RuntimeError("decisionresearch: fold_count")
    plan_folds = plan["compiled"]["folds"]
    for i, f in enumerate(folds):
        val_n = int(plan_folds[i]["val_end"]) - int(plan_folds[i]["val_begin"])
        kind = f["selection"]["kind"]
        calls = int(f["validation_apply_calls"])
        if kind == "DECISION_SPEC" and calls != val_n:
            raise RuntimeError("decisionresearch: validation searched")
        if kind == "ABSTAIN_BASELINE" and calls != 0:
            raise RuntimeError("decisionresearch: baseline validation ApplyDecision")


def _audit_from_worker(a: Dict[str, Any]) -> Dict[str, Any]:
    return {
        "n": int(a["n"]),
        "up_intent": int(a["up_intent"]),
        "down_intent": int(a["down_intent"]),
        "abstain": int(a["abstain"]),
        "up_intent_up_first": int(a["up_intent_up_first"]),
        "up_intent_down_first": int(a["up_intent_down_first"]),
        "up_intent_timeout": int(a["up_intent_timeout"]),
        "down_intent_up_first": int(a["down_intent_up_first"]),
        "down_intent_down_first": int(a["down_intent_down_first"]),
        "down_intent_timeout": int(a["down_intent_timeout"]),
        "abstain_up_first": int(a["abstain_up_first"]),
        "abstain_down_first": int(a["abstain_down_first"]),
        "abstain_timeout": int(a["abstain_timeout"]),
        "total_utility": float(a["total_utility"]),
    }


def _artifact(plan: Dict[str, Any], footer: Dict[str, Any], worker: Dict[str, Any]) -> Dict[str, Any]:
    fold_results = []
    for f in worker["folds"]:
        sel = f["selection"]
        selection: Dict[str, Any] = {"kind": sel["kind"]}
        if sel["kind"] == "DECISION_SPEC":
            selection["utility_decile"] = int(sel["utility_decile"])
            selection["rank_decile"] = int(sel["rank_decile"])
            selection["resolved"] = {
                "logic": sel["spec"]["logic"],
                "target_digest": sel["spec"]["target_digest"].lower(),
                "upper_barrier_utility": float(sel["spec"]["upper_barrier_utility"]),
                "lower_barrier_utility": float(sel["spec"]["lower_barrier_utility"]),
                "min_expected_target_utility": float(sel["spec"]["min_expected_target_utility"]),
                "min_abs_directional_rank": float(sel["spec"]["min_abs_directional_rank"]),
                "class_order": list(sel["spec"]["class_order"]),
            }
        fold_results.append({
            "fold_index": int(f["fold_index"]),
            "selection": selection,
            "train_audit": _audit_from_worker(f["train_audit"]),
            "validation_audit": _audit_from_worker(f["validation_audit"]),
            "validation_positive": bool(f["validation_positive"]),
        })
    p = worker["pooled_validation"]
    doc = {
        "format_version": FORMAT,
        "decision_research_logic_version": LOGIC_V1,
        "source": {
            "decision_validation_plan_content_digest": plan["content_digest"].lower(),
            "oof_forecast_evidence_content_digest": footer["content_digest"].lower(),
            "market": dict(plan["source"]["market"]),
            "at_unit": AT_UNIT,
        },
        "decision_contract": {"decision_logic_version": DECISION_LOGIC},
        "target_law": {
            "target_digest": worker["target_digest"].lower(),
            "class_order": list(worker["class_order"]),
            "upper_barrier_utility": float(worker["upper_barrier_utility"]),
            "lower_barrier_utility": float(worker["lower_barrier_utility"]),
        },
        "selector": {
            "utility_deciles": list(range(1, 10)),
            "rank_deciles": list(range(1, 10)),
            "candidate_count": 81,
            "canonical_threshold_resolution_law": THRESHOLD_LAW,
            "objective_law": OBJECTIVE_LAW,
            "baseline_law": BASELINE_LAW,
            "tie_law": TIE_LAW,
            "train_select_validation_evaluate_law": PROTOCOL_LAW,
        },
        "acceptance": {
            "pooled_total_utility_strictly_positive": True,
            "positive_fold_fraction_numerator": 3,
            "positive_fold_fraction_denominator": 4,
        },
        "fold_results": fold_results,
        "aggregate": {
            "validation_n": int(p["n"]),
            "pooled_up_intent": int(p["up_intent"]),
            "pooled_down_intent": int(p["down_intent"]),
            "pooled_abstain": int(p["abstain"]),
            "pooled_up_intent_up_first": int(p["up_intent_up_first"]),
            "pooled_up_intent_down_first": int(p["up_intent_down_first"]),
            "pooled_up_intent_timeout": int(p["up_intent_timeout"]),
            "pooled_down_intent_up_first": int(p["down_intent_up_first"]),
            "pooled_down_intent_down_first": int(p["down_intent_down_first"]),
            "pooled_down_intent_timeout": int(p["down_intent_timeout"]),
            "pooled_abstain_up_first": int(p["abstain_up_first"]),
            "pooled_abstain_down_first": int(p["abstain_down_first"]),
            "pooled_abstain_timeout": int(p["abstain_timeout"]),
            "pooled_total_utility": float(p["total_utility"]),
            "positive_fold_count": int(worker["positive_fold_count"]),
            "fold_count": int(worker["fold_count"]),
            "eligible_for_finalization": bool(worker["eligible_for_finalization"]),
        },
    }
    return attach_digest(doc)


def _match_existing(out_path, plan, header, footer):
    try:
        doc = read_research(out_path)
    except Exception as e:
        raise RuntimeError(f"decisionresearch: refuse existing {out_path}: {e}") from e
    if doc["source"]["decision_validation_plan_content_digest"].lower() != plan["content_digest"].lower():
        raise RuntimeError("decisionresearch: refuse overwrite (plan)")
    if doc["source"]["oof_forecast_evidence_content_digest"].lower() != footer["content_digest"].lower():
        raise RuntimeError("decisionresearch: refuse overwrite (evidence)")
    if doc["decision_research_logic_version"] != LOGIC_V1:
        raise RuntimeError("decisionresearch: refuse overwrite (research logic)")
    if doc["decision_contract"]["decision_logic_version"] != DECISION_LOGIC:
        raise RuntimeError("decisionresearch: refuse overwrite (decision logic)")
    if doc["target_law"]["target_digest"].lower() != plan["source"]["target_digest"].lower():
        raise RuntimeError("decisionresearch: refuse overwrite (target)")
    sel = doc["selector"]
    if sel["utility_deciles"] != list(range(1, 10)) or sel["rank_deciles"] != list(range(1, 10)) or int(sel["candidate_count"]) != 81:
        raise RuntimeError("decisionresearch: refuse overwrite (grid)")
    if sel["canonical_threshold_resolution_law"] != THRESHOLD_LAW:
        raise RuntimeError("decisionresearch: refuse overwrite (threshold law)")
    acc = doc["acceptance"]
    if int(acc["positive_fold_fraction_numerator"]) != 3 or int(acc["positive_fold_fraction_denominator"]) != 4:
        raise RuntimeError("decisionresearch: refuse overwrite (acceptance)")
    if dict(doc["source"]["market"]) != dict(plan["source"]["market"]):
        raise RuntimeError("decisionresearch: refuse overwrite (market)")
    return doc, True


def report_lines(doc: Dict[str, Any]) -> list[str]:
    lines = []
    for f in doc["fold_results"]:
        s = f["selection"]
        kind = s["kind"]
        extra = ""
        if kind == "DECISION_SPEC":
            extra = f" du={s['utility_decile']} dr={s['rank_decile']}"
        ta, va = f["train_audit"], f["validation_audit"]
        lines.append(
            f"fold {f['fold_index']} kind={kind}{extra} "
            f"train_n={ta['n']} train_U={ta['total_utility']} "
            f"val_n={va['n']} val_U={va['total_utility']} positive={f['validation_positive']}"
        )
    ag = doc["aggregate"]
    lines.append(
        f"pooled_n={ag['validation_n']} pooled_U={ag['pooled_total_utility']} "
        f"positive_folds={ag['positive_fold_count']}/{ag['fold_count']} "
        f"eligible_for_finalization={ag['eligible_for_finalization']}"
    )
    return lines
