"""python -m research.decisionplan --evidence --expect-evidence --matrix [--out]"""

from __future__ import annotations

import argparse
import os
import sys

from research.decisionplan.artifact import LOGIC_V1
from research.decisionplan.run import generate_decision_plan, plan_filename
from research.evidence.artifact import read_evidence
from research.modelfit.hashwire import parse_digest_hex


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.decisionplan", description="DECISION-VALIDATION-PLAN-1 geometry")
    p.add_argument("--evidence", required=True)
    p.add_argument("--expect-evidence", required=True)
    p.add_argument("--matrix", required=True)
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    parse_digest_hex(args.expect_evidence.strip().lower())
    out = args.out
    if not out:
        _, _, footer = read_evidence(args.evidence)
        os.makedirs("research/decision_validation", exist_ok=True)
        out = os.path.join("research/decision_validation", plan_filename(parse_digest_hex(footer["content_digest"])))
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    doc, matched = generate_decision_plan(args.evidence, args.expect_evidence, args.matrix, out)
    status = "MATCH" if matched else "WRITE"
    print(f"DECISION-VALIDATION-PLAN-1 {status} logic={LOGIC_V1} folds={doc['rules']['fold_count']} digest={doc['content_digest']}")
    print(f"path={out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
