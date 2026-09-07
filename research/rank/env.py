"""Fail-closed RANK-1 runtime: Python, NumPy. Not SciPy, not sklearn."""

from __future__ import annotations

import sys
from typing import Dict

from research.modelfit.envpin import load_pin


def current_rank_runtime() -> Dict[str, str]:
    import numpy

    return {
        "python": f"{sys.version_info.major}.{sys.version_info.minor}",
        "numpy": numpy.__version__,
    }


def pinned_rank_runtime() -> Dict[str, str]:
    pin = load_pin()
    return {
        "python": f"{pin['python_major']}.{pin['python_minor']}",
        "numpy": pin["numpy"],
    }


def require_rank_runtime() -> Dict[str, str]:
    pin = load_pin()
    cur = current_rank_runtime()
    want = pinned_rank_runtime()
    if (sys.version_info.major, sys.version_info.minor) != (pin["python_major"], pin["python_minor"]):
        raise RuntimeError(f"rank: refuse Python {cur['python']} want {want['python']}")
    if cur["numpy"] != want["numpy"]:
        raise RuntimeError(f"rank: refuse NumPy {cur['numpy']} want {want['numpy']}")
    return cur
