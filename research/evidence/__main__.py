"""python -m research.evidence --recipe --expect-recipe --logits --calibration --rank --matrix [--out]"""

from __future__ import annotations

import argparse
import os
import sys

from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.recipe.artifact import read_recipe

from .artifact import LOGIC_V1
from .run import evidence_filename, generate_evidence


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.evidence", description="OOF-FORECAST-EVIDENCE-1 recipe projection snapshot")
    p.add_argument("--recipe", required=True)
    p.add_argument("--expect-recipe", required=True)
    p.add_argument("--logits", required=True)
    p.add_argument("--calibration", required=True)
    p.add_argument("--rank", required=True)
    p.add_argument("--matrix", required=True)
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    parse_digest_hex(args.expect_recipe.strip().lower())
    out = args.out
    if not out:
        recipe = read_recipe(args.recipe)
        if recipe["content_digest"].lower() != args.expect_recipe.strip().lower():
            raise SystemExit("evidence: recipe ContentDigest != expected")
        _, _, footer = read_oof_logits(args.logits)
        os.makedirs("research/forecast_evidence", exist_ok=True)
        out = os.path.join(
            "research/forecast_evidence",
            evidence_filename(parse_digest_hex(footer["content_digest"]), parse_digest_hex(recipe["content_digest"])),
        )
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    header, _rows, footer, matched = generate_evidence(
        args.recipe,
        args.expect_recipe,
        args.logits,
        args.calibration,
        args.rank,
        args.matrix,
        out,
    )
    status = "MATCH" if matched else "WRITE"
    print(f"OOF-FORECAST-EVIDENCE-1 {status} logic={LOGIC_V1} rows={footer['row_count']} digest={footer['content_digest']}")
    print(f"path={out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
