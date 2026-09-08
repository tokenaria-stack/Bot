"""python -m research.bundle --final-model --recipe --calibration --rank --logits --matrix [--out]"""

from __future__ import annotations

import argparse
import os
import sys

from research.finalfit.artifact import read_final_model
from research.modelfit.hashwire import parse_digest_hex
from research.recipe.artifact import read_recipe

from .laws import LOGIC_V1
from .run import bundle_filename, generate_bundle


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.bundle", description="FORECAST-BUNDLE-1 executable forecast contract")
    p.add_argument("--final-model", required=True)
    p.add_argument("--recipe", required=True)
    p.add_argument("--calibration", required=True)
    p.add_argument("--rank", required=True)
    p.add_argument("--logits", required=True)
    p.add_argument("--matrix", required=True)
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    out = args.out
    if not out:
        recipe = read_recipe(args.recipe)
        model = read_final_model(args.final_model)
        os.makedirs("research/bundles", exist_ok=True)
        out = os.path.join(
            "research/bundles",
            bundle_filename(
                parse_digest_hex(recipe["content_digest"]),
                parse_digest_hex(model["content_digest"]),
            ),
        )
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    doc, matched = generate_bundle(
        args.final_model,
        args.recipe,
        args.calibration,
        args.rank,
        args.logits,
        args.matrix,
        out,
    )
    status = "MATCH" if matched else "WRITE"
    print(f"FORECAST-BUNDLE-1 {status} logic={LOGIC_V1} digest={doc['content_digest']}")
    print(f"path={out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
