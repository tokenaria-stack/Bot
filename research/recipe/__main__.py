"""python -m research.recipe --logits --expect-logits --calibration --expect-calibration --rank --expect-rank [--out]"""

from __future__ import annotations

import argparse
import os
import sys

from research.calibration.artifact import read_calibration
from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.rank.artifact import read_rank

from .laws import LOGIC_V1
from .run import generate_recipe, recipe_filename


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.recipe", description="RECIPE-FREEZE-1 forecast composition")
    p.add_argument("--logits", required=True)
    p.add_argument("--expect-logits", required=True)
    p.add_argument("--calibration", required=True)
    p.add_argument("--expect-calibration", required=True)
    p.add_argument("--rank", required=True)
    p.add_argument("--expect-rank", required=True)
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    parse_digest_hex(args.expect_logits.strip().lower())
    parse_digest_hex(args.expect_calibration.strip().lower())
    parse_digest_hex(args.expect_rank.strip().lower())
    out = args.out
    if not out:
        _, _, footer = read_oof_logits(args.logits)
        cal = read_calibration(args.calibration)
        rank = read_rank(args.rank)
        os.makedirs("research/recipes", exist_ok=True)
        out = os.path.join(
            "research/recipes",
            recipe_filename(
                parse_digest_hex(footer["content_digest"]),
                parse_digest_hex(cal["content_digest"]),
                parse_digest_hex(rank["content_digest"]),
            ),
        )
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    doc, matched = generate_recipe(
        args.logits,
        args.expect_logits,
        args.calibration,
        args.expect_calibration,
        args.rank,
        args.expect_rank,
        out,
    )
    status = "MATCH" if matched else "WRITE"
    print(f"RECIPE-FREEZE-1 {status} logic={LOGIC_V1} digest={doc['content_digest']}")
    print(f"path={out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
