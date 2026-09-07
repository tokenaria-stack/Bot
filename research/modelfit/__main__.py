"""python -m research.modelfit --matrix PATH --expect-matrix HEX [--out PATH]"""

from __future__ import annotations

import argparse
import os
import sys

from .hashwire import parse_digest_hex
from .run import generate_oof_logits, logits_filename
from .spec import pinned_model_spec


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.modelfit", description="MODEL-FIT-1 OOF logits")
    p.add_argument("--matrix", required=True, help="path to oof-matrix-v1")
    p.add_argument("--expect-matrix", required=True, help="expected OOF Matrix ContentDigest hex")
    p.add_argument("--out", default="", help="output oof-logits-v1 path")
    args = p.parse_args(argv)
    parse_digest_hex(args.expect_matrix.strip().lower())
    spec = pinned_model_spec()
    out = args.out
    if not out:
        from .matrix import load_oof_matrix
        from .envpin import require_pinned_runtime

        require_pinned_runtime()
        m = load_oof_matrix(args.matrix, args.expect_matrix)
        os.makedirs("research/logits", exist_ok=True)
        out = os.path.join("research/logits", logits_filename(m.header["market"], m.content_digest, spec.digest()))
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    header, footer, matched = generate_oof_logits(args.matrix, args.expect_matrix, out, spec=spec)
    status = "MATCH" if matched else "WRITE"
    print(f"MODEL-FIT-1 {status} rows={footer['row_count']} digest={footer['content_digest']}")
    print(f"path={out}")
    print(f"folds={len(header['folds'])} converged")
    return 0


if __name__ == "__main__":
    sys.exit(main())
