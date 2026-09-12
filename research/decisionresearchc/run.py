"""Orchestrate DECISION-RESEARCH-C. Per-fold causal projection, then existing Go selector."""

from __future__ import annotations

import os
from typing import Any, Dict, List, Tuple

from research.calibration.spec import pinned_calibration_spec
from research.catboostlogits.reader import CatBoostOOFLogits
from research.decisionresearch.artifact import BASELINE_LAW, DECISION_LOGIC, OBJECTIVE_LAW, PROTOCOL_LAW, THRESHOLD_LAW, TIE_LAW
from research.decisionresearch.worker import run_selector
from research.decisionresearchc.artifact import AT_UNIT, FORMAT, LOGIC_V1, attach_digest, read_research, write_research_atomic
from research.decisionresearchc.plan_artifact import read_plan
from research.decisionresearchc.project import project_decision_fold, selector_payload
from research.decisionresearchc.specs import PROJECTION_LOGIC, RANK_POPULATION_C, decision_fold_rank_spec
from research.modelfit.hashwire import parse_digest_hex


def research_filename(plan_digest: bytes, logits_digest: bytes) -> str:
    return f"plan-{plan_digest[:8].hex()}_logits-{logits_digest[:8].hex()}_dres-c.dres"


def generate_decision_research_c(
    plan_path: str,
    expect_plan_digest: str,
    logits: CatBoostOOFLogits,
    out_path: str,
) -> Tuple[Dict[str, Any], bool]:
    expect = expect_plan_digest.strip().lower()
    parse_digest_hex(expect)
    plan = read_plan(plan_path)
    if plan["content_digest"].lower() != expect:
        raise RuntimeError("decisionresearchc: plan ContentDigest != --expect-plan")
    if plan["source"]["oof_logits_content_digest"].lower() != logits.content_digest.lower():
        raise RuntimeError("decisionresearchc: logits ContentDigest != plan source")
    if os.path.exists(out_path):
        return _match_existing(out_path, plan, logits)

    cal = pinned_calibration_spec()
    rank = decision_fold_rank_spec()
    fold_results: List[Dict[str, Any]] = []
    pooled = _empty_audit()
    workers: List[Dict[str, Any]] = []
    compiled_folds = plan["compiled"]["folds"]
    for i, fr in enumerate(compiled_folds):
        tb, te = int(fr["train_begin"]), int(fr["train_end"])
        vb, ve = int(fr["val_begin"]), int(fr["val_end"])
        proj = project_decision_fold(
            logits.logits, logits.y, logits.outcome,
            tb, te, vb, ve, cal, rank, logits.spec_digest,
        )
        if proj.rank_reference_n != (te - tb):
            raise RuntimeError("decisionresearchc: rank reference n != train n")
        rows, local_folds = selector_payload(proj)
        worker = run_selector(plan["source"]["target_digest"], rows, local_folds)
        _require_worker_fold(worker, ve - vb)
        workers.append(worker)
        wf = worker["folds"][0]
        fold_results.append(_fold_result(i, proj, wf, fr))
        _add_audit(pooled, _audit_from_worker(wf["validation_audit"]))

    u = float(workers[0]["upper_barrier_utility"])
    l = float(workers[0]["lower_barrier_utility"])
    pooled["total_utility"] = u * (pooled["up_intent_up_first"] - pooled["down_intent_up_first"]) + l * (
        pooled["down_intent_down_first"] - pooled["up_intent_down_first"]
    )
    positive = sum(1 for f in fold_results if f["validation_positive"])
    fold_count = len(fold_results)
    eligible = (pooled["total_utility"] > 0) and (positive * 4 >= fold_count * 3)
    doc = _artifact(plan, logits, cal, rank, fold_results, pooled, positive, fold_count, eligible, workers[0])
    write_research_atomic(out_path, doc)
    back = read_research(out_path)
    if back["content_digest"] != doc["content_digest"]:
        raise RuntimeError("decisionresearchc: readback mismatch")
    return back, False


def _require_worker_fold(worker: Dict[str, Any], val_n: int) -> None:
    if worker.get("decision_research_logic_version") != LOGIC_V1:
        raise RuntimeError("decisionresearchc: DecisionResearchLogicVersion")
    if worker.get("decision_logic_version") != DECISION_LOGIC:
        raise RuntimeError("decisionresearchc: DecisionLogicVersion")
    if len(worker["folds"]) != 1:
        raise RuntimeError("decisionresearchc: expected one selector fold")
    f = worker["folds"][0]
    kind = f["selection"]["kind"]
    calls = int(f["validation_apply_calls"])
    if kind == "DECISION_SPEC" and calls != val_n:
        raise RuntimeError("decisionresearchc: validation searched")
    if kind == "ABSTAIN_BASELINE" and calls != 0:
        raise RuntimeError("decisionresearchc: baseline validation ApplyDecision")


def _empty_audit() -> Dict[str, Any]:
    return {
        "n": 0, "up_intent": 0, "down_intent": 0, "abstain": 0,
        "up_intent_up_first": 0, "up_intent_down_first": 0, "up_intent_timeout": 0,
        "down_intent_up_first": 0, "down_intent_down_first": 0, "down_intent_timeout": 0,
        "abstain_up_first": 0, "abstain_down_first": 0, "abstain_timeout": 0,
        "total_utility": 0.0,
    }


def _add_audit(dst: Dict[str, Any], src: Dict[str, Any]) -> None:
    for k, v in src.items():
        if k == "total_utility":
            continue
        dst[k] = int(dst[k]) + int(v)


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


def _fold_result(index: int, proj, wf: Dict[str, Any], fr: Dict[str, Any]) -> Dict[str, Any]:
    sel = wf["selection"]
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
    return {
        "fold_index": index,
        "train_begin": int(fr["train_begin"]),
        "train_end": int(fr["train_end"]),
        "val_begin": int(fr["val_begin"]),
        "val_end": int(fr["val_end"]),
        "train_first_at": int(fr["train_first_at"]),
        "train_last_at": int(fr["train_last_at"]),
        "val_first_at": int(fr["val_first_at"]),
        "val_last_at": int(fr["val_last_at"]),
        "projection_beta": float(proj.beta),
        "rank_reference_n": int(proj.rank_reference_n),
        "selection": selection,
        "train_audit": _audit_from_worker(wf["train_audit"]),
        "validation_audit": _audit_from_worker(wf["validation_audit"]),
        "validation_positive": bool(wf["validation_positive"]),
    }


def _artifact(plan, logits, cal, rank, fold_results, pooled, positive, fold_count, eligible, worker0) -> Dict[str, Any]:
    doc = {
        "format_version": FORMAT,
        "decision_research_logic_version": LOGIC_V1,
        "source": {
            "decision_validation_plan_content_digest": plan["content_digest"].lower(),
            "oof_logits_content_digest": logits.content_digest.lower(),
            "market": dict(plan["source"]["market"]),
            "at_unit": AT_UNIT,
        },
        "projection": {
            "calibration_spec_digest": cal.digest_hex(),
            "rank_spec_digest": rank.digest_hex(),
            "projection_logic_version": PROJECTION_LOGIC,
            "rank_population": RANK_POPULATION_C,
        },
        "decision_contract": {"decision_logic_version": DECISION_LOGIC},
        "target_law": {
            "target_digest": worker0["target_digest"].lower(),
            "class_order": list(worker0["class_order"]),
            "upper_barrier_utility": float(worker0["upper_barrier_utility"]),
            "lower_barrier_utility": float(worker0["lower_barrier_utility"]),
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
            "validation_n": int(pooled["n"]),
            "pooled_up_intent": int(pooled["up_intent"]),
            "pooled_down_intent": int(pooled["down_intent"]),
            "pooled_abstain": int(pooled["abstain"]),
            "pooled_up_intent_up_first": int(pooled["up_intent_up_first"]),
            "pooled_up_intent_down_first": int(pooled["up_intent_down_first"]),
            "pooled_up_intent_timeout": int(pooled["up_intent_timeout"]),
            "pooled_down_intent_up_first": int(pooled["down_intent_up_first"]),
            "pooled_down_intent_down_first": int(pooled["down_intent_down_first"]),
            "pooled_down_intent_timeout": int(pooled["down_intent_timeout"]),
            "pooled_abstain_up_first": int(pooled["abstain_up_first"]),
            "pooled_abstain_down_first": int(pooled["abstain_down_first"]),
            "pooled_abstain_timeout": int(pooled["abstain_timeout"]),
            "pooled_total_utility": float(pooled["total_utility"]),
            "positive_fold_count": int(positive),
            "fold_count": int(fold_count),
            "eligible_for_finalization": bool(eligible),
        },
    }
    return attach_digest(doc)


def _match_existing(out_path, plan, logits):
    try:
        doc = read_research(out_path)
    except Exception as e:
        raise RuntimeError(f"decisionresearchc: refuse existing {out_path}: {e}") from e
    if doc["source"]["decision_validation_plan_content_digest"].lower() != plan["content_digest"].lower():
        raise RuntimeError("decisionresearchc: refuse overwrite (plan)")
    if doc["source"]["oof_logits_content_digest"].lower() != logits.content_digest.lower():
        raise RuntimeError("decisionresearchc: refuse overwrite (logits)")
    return doc, True


def report_lines(doc: Dict[str, Any]) -> list:
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
            f"train[{f['train_begin']}:{f['train_end']}] n={ta['n']} "
            f"val[{f['val_begin']}:{f['val_end']}] n={va['n']} "
            f"beta={f['projection_beta']} rank_n={f['rank_reference_n']} "
            f"train_U={ta['total_utility']} val_U={va['total_utility']} "
            f"positive={f['validation_positive']} "
            f"train_at=[{f['train_first_at']},{f['train_last_at']}] "
            f"val_at=[{f['val_first_at']},{f['val_last_at']}]"
        )
    ag = doc["aggregate"]
    lines.append(
        f"pooled_n={ag['validation_n']} pooled_U={ag['pooled_total_utility']} "
        f"positive_folds={ag['positive_fold_count']}/{ag['fold_count']} "
        f"eligible_for_finalization={ag['eligible_for_finalization']}"
    )
    return lines
