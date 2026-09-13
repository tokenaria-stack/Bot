"""Assemble SIGNAL-DIAGNOSTICS-C tables from frozen files."""

from __future__ import annotations

from pathlib import Path
from typing import Any, Dict, List

import numpy as np

from research.catboostlogits.reader import read_catboost_oof_logits
from research.decisionresearchc.plan_artifact import read_plan
from research.modelfit.matrix import load_oof_matrix
from research.signaldiagnosticsc.tables import (
    COVERAGES,
    autopsy_from_plan,
    catboost_fold_slices,
    coverage_table,
    directional_d,
    event_baseline,
    fold_coverage_tables,
    join_matrix_features,
    population_row,
    q_from_p,
    rank_baseline,
    softmax_beta1,
)

ROOT = Path(__file__).resolve().parents[2]
EXPECT_LOGITS = "71213ef705dc390ee7d551695c1fc9c275044dab4625b86f8a6176fbc67c96ad"
EXPECT_MATRIX = "6793d8fe01a8533fc20ec372801b4c864396adde0c493aaeb9f3eed107c8e4e7"
EXPECT_PLAN = "54f349437d3f27a73a8399dd34b6a6d7a0a7e8298d955ae7cf3d4ad771c31d38"
LOGITS_PATH = ROOT / "research" / "catboost" / (
    "BINANCE_BTCUSDT_FUTURES_PERP_15m_matrix-6793d8fe01a8533f_spec-0ff37a077dc2911c.catboostlogits"
)
MATRIX_PATH = ROOT / "research" / "oof" / (
    "BINANCE_BTCUSDT_FUTURES_PERP_15m_tape-5ba899ebe5a1bf59_labels-491c7b8274a27291_valplan-b328000202d3512c.oofmatrix"
)
PLAN_PATH = ROOT / "research" / "decision_validation" / "logits-71213ef705dc390e_dval-walk-forward-c.dvalplan"


def run_diagnostics() -> Dict[str, Any]:
    logits = read_catboost_oof_logits(str(LOGITS_PATH), EXPECT_LOGITS)
    d = directional_d(logits.logits)
    p = softmax_beta1(logits.logits)
    q = q_from_p(p)
    spear = float(np.corrcoef(d.argsort().argsort(), q.argsort().argsort())[0, 1])
    if spear < 0.999:
        raise RuntimeError("signaldiagnosticsc: q is not a monotone readout of D")
    slices = catboost_fold_slices(logits.header, int(d.shape[0]))
    pop = population_row(logits.y, p)
    bull = coverage_table(d, logits.y, p, "bull")
    bear = coverage_table(d, logits.y, p, "bear")
    bull_folds = fold_coverage_tables(d, logits.y, p, slices, "bull")
    bear_folds = fold_coverage_tables(d, logits.y, p, slices, "bear")

    matrix = load_oof_matrix(str(MATRIX_PATH), EXPECT_MATRIX)
    feats = join_matrix_features(logits.at, matrix.at, matrix.x, matrix.feature_ids)
    baselines: List[Dict[str, Any]] = []
    baselines.extend(rank_baseline(feats["rsx_value"], logits.y, "bull", "rsx_value"))
    baselines.extend(rank_baseline(feats["rsx_value"], logits.y, "bear", "rsx_value"))
    baselines.extend(rank_baseline(feats["rsx_delta_1"], logits.y, "bull", "rsx_delta_1"))
    baselines.extend(rank_baseline(feats["rsx_delta_1"], logits.y, "bear", "rsx_delta_1"))
    baselines.append(event_baseline(logits.y, feats["tv_bull_present"], "bull", "tv_bull_present"))
    baselines.append(event_baseline(logits.y, feats["tv_bear_present"], "bear", "tv_bear_present"))
    baselines.append(event_baseline(logits.y, feats["rsx_50_cross_up_present"], "bull", "rsx_50_cross_up_present"))
    baselines.append(event_baseline(logits.y, feats["rsx_50_cross_down_present"], "bear", "rsx_50_cross_down_present"))

    if not PLAN_PATH.exists():
        raise RuntimeError("signaldiagnosticsc: DecisionValidationPlan-C missing (needed for du/dr autopsy only)")
    plan = read_plan(str(PLAN_PATH))
    if plan["content_digest"].lower() != EXPECT_PLAN:
        raise RuntimeError("signaldiagnosticsc: plan digest != frozen DECISION-RESEARCH-C plan")
    autopsy = autopsy_from_plan(logits, plan)

    return {
        "n": int(d.shape[0]),
        "logits_digest": logits.content_digest,
        "population": pop,
        "bull": bull,
        "bear": bear,
        "bull_folds": bull_folds,
        "bear_folds": bear_folds,
        "baselines": baselines,
        "autopsy": autopsy,
        "oof_fold_n": [e - b for b, e in slices],
        "coverages": list(COVERAGES),
        "q_note": "q is β=1 softmax readout of D; not a second signal; not causal β",
    }


def _fmt_rate(x: Any) -> str:
    if x is None or (isinstance(x, float) and (np.isnan(x) or np.isinf(x))):
        return "—"
    return f"{100.0 * float(x):.2f}%"


def _fmt_u(x: Any) -> str:
    if x is None or (isinstance(x, float) and (np.isnan(x) or np.isinf(x))):
        return "—"
    return f"{float(x):.4f}"


def _line_cov(rec: Dict[str, Any], extra: str = "") -> str:
    pct = rec.get("coverage_pct", "")
    return (
        f"| {pct} | {rec['n']} | {rec['coverage']:.4f} | "
        f"{_fmt_rate(rec['intended_hit_rate'])} | {_fmt_rate(rec['reverse_hit_rate'])} | "
        f"{_fmt_rate(rec['timeout_rate'])} | {_fmt_u(rec['utility_per_obs'])} | "
        f"{_fmt_rate(rec.get('mean_q'))} | {_fmt_u(rec.get('mean_a'))} |{extra}"
    )


def render_report(doc: Dict[str, Any]) -> str:
    pop = doc["population"]
    lines = [
        "# SIGNAL-DIAGNOSTICS-C",
        "",
        "Read-only autopsy of frozen CatBoost OOF logits on Target C. Not eligibility. Not a 2R ticket.",
        "",
        f"- rows: {doc['n']}",
        f"- logits: `{doc['logits_digest']}`",
        f"- CatBoost OOF fold sizes: {doc['oof_fold_n']}",
        f"- {doc['q_note']}",
        f"- Target-C utility if the tail is treated as always-UP (bull) or always-DOWN (bear): U=L=2.0.",
        "",
        "## Population base rates (all OOF)",
        "",
        f"- N={pop['n']} UP={_fmt_rate(pop['up_first_rate'])} DOWN={_fmt_rate(pop['down_first_rate'])} TO={_fmt_rate(pop['timeout_rate'])}",
        f"- mean q={_fmt_rate(pop['mean_q'])} mean a={_fmt_u(pop['mean_a'])} mean P_TO={_fmt_rate(pop['mean_p_timeout'])}",
        "",
        "## 1. Bullish quality vs coverage (highest D)",
        "",
        "| pct | N | coverage | intended UP | reverse DOWN | TIMEOUT | U/obs | mean q | mean a |",
        "|-----|---|----------|-------------|--------------|---------|-------|--------|--------|",
    ]
    for rec in doc["bull"]:
        lines.append(_line_cov(rec))
    lines += [
        "",
        "## 2. Bearish quality vs coverage (lowest D)",
        "",
        "| pct | N | coverage | intended DOWN | reverse UP | TIMEOUT | U/obs | mean q | mean a |",
        "|-----|---|----------|---------------|------------|---------|-------|--------|--------|",
    ]
    for rec in doc["bear"]:
        lines.append(_line_cov(rec))
    lines += ["", "## 3. Same tails inside each CatBoost OOF fold (percentiles local to the fold)", ""]
    for side, key in (("bull", "bull_folds"), ("bear", "bear_folds")):
        lines.append(f"### {side}")
        lines.append("")
        lines.append("| fold | pct | N | intended | reverse | TIMEOUT | U/obs |")
        lines.append("|------|-----|---|----------|---------|---------|-------|")
        for rec in doc[key]:
            lines.append(
                f"| {rec['oof_fold']} | {rec['coverage_pct']} | {rec['n']} | "
                f"{_fmt_rate(rec['intended_hit_rate'])} | {_fmt_rate(rec['reverse_hit_rate'])} | "
                f"{_fmt_rate(rec['timeout_rate'])} | {_fmt_u(rec['utility_per_obs'])} |"
            )
        lines.append("")
    lines += [
        "## 4. Simple baselines (predeclared; not searched)",
        "",
        "Rank baselines use the same coverage knots. Event baselines are the raw present=1 sets (one operating point).",
        "",
        "| baseline | side | pct/event | N | intended | reverse | TIMEOUT | U/obs |",
        "|----------|------|-----------|---|----------|---------|---------|-------|",
    ]
    for rec in doc["baselines"]:
        knot = rec["coverage_pct"] if rec["kind"] == "rank" else "present=1"
        lines.append(
            f"| {rec['baseline']} | {rec['side']} | {knot} | {rec['n']} | "
            f"{_fmt_rate(rec['intended_hit_rate'])} | {_fmt_rate(rec['reverse_hit_rate'])} | "
            f"{_fmt_rate(rec['timeout_rate'])} | {_fmt_u(rec['utility_per_obs'])} |"
        )
    lines += [
        "",
        "## 5. du=1 / dr=8 autopsy",
        "",
        "Causal fold-local β/rank (Decision-Research-C law). Among |rank|≥0.8, |EU| bands are the frozen du grid (0.2 steps). "
        "`fires_du1` is false for |EU|<0.2 (would not pass du=1).",
        "",
        "| fold | era | side | |EU| band | fires_du1 | N | intended | reverse | TIMEOUT | U total | U/obs |",
        "|------|-----|------|-----------|-----------|---|----------|---------|---------|---------|-------|",
    ]
    for rec in doc["autopsy"]:
        if rec["n"] == 0:
            continue
        lines.append(
            f"| {rec['decision_fold']} | {rec['era']} | {rec['side']} | "
            f"[{rec['eu_abs_lo']:.1f},{rec['eu_abs_hi']:.1f}) | {rec['fires_du1']} | {rec['n']} | "
            f"{_fmt_rate(rec['intended_hit_rate'])} | {_fmt_rate(rec['reverse_hit_rate'])} | "
            f"{_fmt_rate(rec['timeout_rate'])} | {_fmt_u(rec['utility_total'])} | {_fmt_u(rec['utility_per_obs'])} |"
        )
    lines += ["", "## 6. Interpretation", ""]
    lines += interpret_lines(doc)
    return "\n".join(lines) + "\n"


def interpret_lines(doc: Dict[str, Any]) -> list:
    bull1 = next(r for r in doc["bull"] if r["coverage_pct"] == 1)
    bull50 = next(r for r in doc["bull"] if r["coverage_pct"] == 50)
    bear1 = next(r for r in doc["bear"] if r["coverage_pct"] == 1)
    bear50 = next(r for r in doc["bear"] if r["coverage_pct"] == 50)
    pop = doc["population"]
    return [
        "**Verdict: C. MIXED.** Not eligibility. Not a 2R result.",
        "",
        "CatBoost OOF ranking on Target C is **not** a strong stable situation scorer. "
        "The **bearish D tail is weakly useful and roughly monotone**. The **bullish D tail is inverted in aggregate** "
        "(elite highest-D states hit DOWN_FIRST more often than UP_FIRST) and **breaks in later chronological OOF folds**.",
        "",
        f"- Population: UP {100*pop['up_first_rate']:.2f}% / DOWN {100*pop['down_first_rate']:.2f}% / TO {100*pop['timeout_rate']:.2f}%. "
        "TIMEOUT is rare (~2%). Target C ±2 ATR within 72×15m almost always resolves on this BTC sample. "
        "Do not import 'large TIMEOUT mass' folklore from other markets into this autopsy.",
        f"- Bull 50%→1% intended UP: {100*bull50['intended_hit_rate']:.2f}% → {100*bull1['intended_hit_rate']:.2f}% "
        f"(worse); reverse rises; U/obs {bull50['utility_per_obs']:.3f} → {bull1['utility_per_obs']:.3f}.",
        f"- Bear 50%→1% intended DOWN: {100*bear50['intended_hit_rate']:.2f}% → {100*bear1['intended_hit_rate']:.2f}% "
        f"(better); reverse falls; U/obs {bear50['utility_per_obs']:.3f} → {bear1['utility_per_obs']:.3f}.",
        "- Fold-local: bull elite helps in OOF folds 0–1 and fails hard in 2–3. Bear elite is strongest in fold 1, "
        "present in 2–3 at 1%, weak in fold 0. Ordering is **not** reasonably consistent.",
        "- Baselines: raw `rsx_value` / `rsx_delta_1` tails and TV/50-cross present=1 sets are near coin-flip. "
        "CatBoost **bear 1%** beats `rsx_value` bear 1% (61.5% vs 56.4% intended) but that is a small, one-sided edge. "
        "CatBoost **bull 1%** is worse than population and worse than those facts.",
        "- du=1/dr=8: among |rank|≥0.8 almost all mass sits in |EU|∈[0.2,0.4). Higher-EU bands are tiny. "
        "The extra volume that du=1 admits **is** the weak-edge rank tail. Train often mildly green; "
        "later val (especially bull) is red. That matches Decision-Research-C: stable in-sample cell, failed forward robustness.",
        "",
        "Nonlinear ranking **exists on the short side of this y**, not as a two-sided situation microscope. "
        "Do not treat this as proof that FeatureSpec2 atoms are useless as **events** (meta-label populations). "
        "It does mean: do not spend the next chapter explaining trees/SHAP, and do not expect a Target-C score slider "
        "to look like 'elite longs'. Next discussion: candidate atoms + ticket geometry, not CatBoostSpec2.",
        "",
        "HARD STOP. No ticket geometry, ResearchWindow, SHAP, UI, or new CatBoost in this chapter.",
        "",
    ]

