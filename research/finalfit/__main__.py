"""python -m research.finalfit --recipe --expect-recipe --logits --matrix [--out]"""

from __future__ import annotations

import argparse
import os
import sys

from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits
from research.modelfit.spec import pinned_model_spec
from research.recipe.artifact import read_recipe

from .artifact import FIT_LOGIC_V1
from .run import final_model_filename, generate_final_model


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.finalfit", description="FINAL-MODEL-FIT-1 all-development base model")
    p.add_argument("--recipe", required=True)
    p.add_argument("--expect-recipe", required=True)
    p.add_argument("--logits", required=True)
    p.add_argument("--matrix", required=True)
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    parse_digest_hex(args.expect_recipe.strip().lower())
    out = args.out
    if not out:
        recipe = read_recipe(args.recipe)
        if recipe["content_digest"].lower() != args.expect_recipe.strip().lower():
            raise SystemExit("finalfit: recipe ContentDigest != expected")
        header, _, footer = read_oof_logits(args.logits)
        if footer["content_digest"].lower() != recipe["research_source"]["oof_logits_content_digest"].lower():
            raise SystemExit("finalfit: logits ContentDigest != recipe research source")
        spec = pinned_model_spec()
        os.makedirs("research/final_models", exist_ok=True)
        out = os.path.join(
            "research/final_models",
            final_model_filename(
                header["market"],
                parse_digest_hex(header["source_oof_matrix_content_digest"]),
                spec.digest(),
            ),
        )
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    doc, matched = generate_final_model(args.recipe, args.expect_recipe, args.logits, args.matrix, out)
    status = "MATCH" if matched else "WRITE"
    src = doc["source"]
    print(f"FINAL-MODEL-FIT-1 {status} logic={FIT_LOGIC_V1} rows={src['row_count']} digest={doc['content_digest']}")
    print(f"path={out}")
    print(f"n_iter={doc['logistic']['n_iter']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
