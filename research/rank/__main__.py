"""python -m research.rank --logits PATH --expect-logits HEX [--out PATH]"""

from __future__ import annotations

import argparse
import os
import sys

from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits

from .env import require_rank_runtime
from .run import generate_rank, rank_filename
from .spec import pinned_rank_spec


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.rank", description="RANK-1 exact empirical directional rank")
    p.add_argument("--logits", required=True)
    p.add_argument("--expect-logits", required=True)
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    parse_digest_hex(args.expect_logits.strip().lower())
    spec = pinned_rank_spec()
    out = args.out
    if not out:
        require_rank_runtime()
        _, _, footer = read_oof_logits(args.logits)
        os.makedirs("research/ranks", exist_ok=True)
        out = os.path.join(
            "research/ranks",
            rank_filename(parse_digest_hex(footer["content_digest"]), spec.digest()),
        )
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    doc, matched = generate_rank(args.logits, args.expect_logits, out, spec=spec)
    status = "MATCH" if matched else "WRITE"
    n = doc["source"]["source_row_count"]
    print(f"RANK-1 {status} rows={n} digest={doc['content_digest']}")
    print(f"path={out}")
    print(f"RankSpecDigest={doc['rank']['rank_spec_digest']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
