"""python -m research.calibration --logits PATH --expect-logits HEX [--out PATH]"""

from __future__ import annotations

import argparse
import os
import sys

from research.modelfit.hashwire import parse_digest_hex
from research.modelfit.logits import read_oof_logits

from .env import require_calibration_runtime
from .run import calibration_filename, generate_calibration
from .spec import pinned_calibration_spec


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="research.calibration", description="CALIBRATION-1 temperature scaling")
    p.add_argument("--logits", required=True)
    p.add_argument("--expect-logits", required=True)
    p.add_argument("--out", default="")
    args = p.parse_args(argv)
    parse_digest_hex(args.expect_logits.strip().lower())
    spec = pinned_calibration_spec()
    out = args.out
    if not out:
        require_calibration_runtime()
        _, _, footer = read_oof_logits(args.logits)
        os.makedirs("research/calibrations", exist_ok=True)
        from research.modelfit.hashwire import parse_digest_hex as parse_src

        out = os.path.join(
            "research/calibrations",
            calibration_filename(parse_src(footer["content_digest"]), spec.digest()),
        )
    parent = os.path.dirname(out)
    if parent:
        os.makedirs(parent, exist_ok=True)
    doc, matched = generate_calibration(args.logits, args.expect_logits, out, spec=spec)
    status = "MATCH" if matched else "WRITE"
    fit = doc["fit"]
    print(f"CALIBRATION-1 {status} beta={fit['beta']} nit={fit['iterations']} digest={doc['content_digest']}")
    print(f"path={out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
