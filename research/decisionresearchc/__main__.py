"""python -m research.decisionresearchc --logits --expect-logits --matrix [--plan-out] [--out]"""

from __future__ import annotations

import argparse
import os
import sys

from research.catboostlogits.reader import read_catboost_oof_logits
from research.decisionresearchc.artifact import LOGIC_V1
from research.decisionresearchc.plan import generate_decision_plan_c, plan_filename
from research.decisionresearchc.plan_artifact import read_plan
from research.decisionresearchc.run import generate_decision_research_c, report_lines, research_filename
from research.modelfit.hashwire import parse_digest_hex


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.decisionresearchc", description="DECISION-RESEARCH-C")
    p.add_argument("--logits", required=True)
    p.add_argument("--expect-logits", required=True)
    p.add_argument("--matrix", required=True)
    p.add_argument("--plan-out", default="")
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    expect = args.expect_logits.strip().lower()
    parse_digest_hex(expect)
    logits = read_catboost_oof_logits(args.logits, expect)
    os.makedirs("research/decision_validation", exist_ok=True)
    os.makedirs("research/decision_research", exist_ok=True)
    plan_out = args.plan_out
    if not plan_out:
        plan_out = os.path.join(
            "research/decision_validation",
            plan_filename(parse_digest_hex(logits.content_digest)),
        )
    parent = os.path.dirname(plan_out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    plan, plan_matched = generate_decision_plan_c(logits, args.matrix, plan_out)
    print(
        f"DECISION-VALIDATION-PLAN-C {'MATCH' if plan_matched else 'WRITE'} "
        f"digest={plan['content_digest']} TargetH={plan['rules']['target_h']} "
        f"span={plan['rules']['validation_span_bars']}"
    )
    print(f"plan_path={plan_out}")
    out = args.out
    if not out:
        out = os.path.join(
            "research/decision_research",
            research_filename(parse_digest_hex(plan["content_digest"]), parse_digest_hex(logits.content_digest)),
        )
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    doc, matched = generate_decision_research_c(plan_out, plan["content_digest"], logits, out)
    elig = doc["aggregate"]["eligible_for_finalization"]
    print(
        f"DECISION-RESEARCH-C {'MATCH' if matched else 'WRITE'} logic={LOGIC_V1} "
        f"eligible_for_finalization={elig} digest={doc['content_digest']}"
    )
    print(f"path={out}")
    for line in report_lines(doc):
        print(line)
    if elig:
        print("ELIGIBLE_FOR_FINALIZATION")
        print("HARD STOP. Do not start FINALIZATION-C.")
    else:
        print("NOT_ELIGIBLE_FOR_FINALIZATION")
        print("HARD STOP. Freeze the negative result.")
    _ = read_plan(plan_out)
    return 0


if __name__ == "__main__":
    sys.exit(main())
