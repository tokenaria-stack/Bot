"""Extract At[] and call existing Go CompileValidationPlan. No calendar packing here."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Any, Dict, List

ROOT = Path(__file__).resolve().parents[2]


def extract_at(rows: List[Dict[str, Any]]) -> List[int]:
    at: List[int] = []
    last = None
    for r in rows:
        a = int(r["at"])
        if last is not None and a <= last:
            raise RuntimeError("decisionplan: At not strictly increasing")
        last = a
        at.append(a)
    if not at:
        raise RuntimeError("decisionplan: empty At[]")
    return at


def compile_validation_plan(at: List[int]) -> Dict[str, Any]:
    payload = json.dumps({"at": [int(x) for x in at]}, separators=(",", ":"))
    proc = subprocess.run(
        ["go", "run", "./cmd/research_decision_plan"],
        cwd=str(ROOT),
        input=payload.encode("utf-8"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if proc.returncode != 0:
        err = proc.stderr.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"decisionplan: CompileValidationPlan: {err}")
    doc = json.loads(proc.stdout.decode("utf-8"))
    if doc.get("logic") != "decision-validation:walk-forward-v1":
        raise RuntimeError("decisionplan: unexpected compiler logic")
    return doc
