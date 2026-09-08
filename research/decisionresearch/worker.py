"""Go selector worker. Arrays only — no At."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Any, Dict, List

ROOT = Path(__file__).resolve().parents[2]


def extract_bridge_arrays(rows: List[Dict[str, Any]]) -> Dict[str, Any]:
    p_up, p_down, p_to, rank, outcome = [], [], [], [], []
    for r in rows:
        p = r["probabilities"]
        if len(p) != 3:
            raise RuntimeError("decisionresearch: probabilities length")
        p_up.append(float(p[0]))
        p_down.append(float(p[1]))
        p_to.append(float(p[2]))
        rank.append(float(r["directional_rank"]))
        outcome.append(str(r["outcome"]))
    return {
        "p_up": p_up,
        "p_down": p_down,
        "p_timeout": p_to,
        "rank": rank,
        "outcome": outcome,
    }


def run_selector(target_digest: str, rows: List[Dict[str, Any]], folds: List[Dict[str, Any]]) -> Dict[str, Any]:
    arrays = extract_bridge_arrays(rows)
    payload = {
        "target_digest": target_digest.lower(),
        **arrays,
        "folds": [
            {
                "train_begin": int(f["train_begin"]),
                "train_end": int(f["train_end"]),
                "val_begin": int(f["val_begin"]),
                "val_end": int(f["val_end"]),
            }
            for f in folds
        ],
    }
    if "at" in payload:
        raise RuntimeError("decisionresearch: At leaked into selector worker")
    proc = subprocess.run(
        ["go", "run", "./cmd/research_decision_research"],
        cwd=str(ROOT),
        input=json.dumps(payload, separators=(",", ":")).encode("utf-8"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if proc.returncode != 0:
        err = proc.stderr.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"decisionresearch: selector: {err}")
    return json.loads(proc.stdout.decode("utf-8"))
