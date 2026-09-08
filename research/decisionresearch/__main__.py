"""python -m research.decisionresearch --plan --expect-plan --evidence [--out]"""

from __future__ import annotations

import argparse
import os
import sys

from research.decisionplan.artifact import read_plan
from research.decisionresearch.artifact import LOGIC_V1
from research.decisionresearch.run import generate_decision_research, report_lines, research_filename
from research.evidence.artifact import read_evidence
from research.modelfit.hashwire import parse_digest_hex


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.decisionresearch", description="DECISION-RESEARCH-1 selector")
    p.add_argument("--plan", required=True)
    p.add_argument("--expect-plan", required=True)
    p.add_argument("--evidence", required=True)
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    parse_digest_hex(args.expect_plan.strip().lower())
    out = args.out
    if not out:
        plan = read_plan(args.plan)
        _, _, footer = read_evidence(args.evidence)
        os.makedirs("research/decision_research", exist_ok=True)
        out = os.path.join(
            "research/decision_research",
            research_filename(parse_digest_hex(plan["content_digest"]), parse_digest_hex(footer["content_digest"])),
        )
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    doc, matched = generate_decision_research(args.plan, args.expect_plan, args.evidence, out)
    status = "MATCH" if matched else "WRITE"
    elig = doc["aggregate"]["eligible_for_finalization"]
    print(
        f"DECISION-RESEARCH-1 {status} logic={LOGIC_V1} "
        f"eligible_for_finalization={elig} digest={doc['content_digest']}"
    )
    print(f"path={out}")
    for line in report_lines(doc):
        print(line)
    return 0


if __name__ == "__main__":
    sys.exit(main())
