"""Fail-closed CALIBRATION runtime: Python, NumPy, SciPy. Not sklearn."""

from __future__ import annotations

import sys
from typing import Dict

from research.modelfit.envpin import load_pin


def current_calibration_runtime() -> Dict[str, str]:
    import numpy
    import scipy

    return {
        "python": f"{sys.version_info.major}.{sys.version_info.minor}",
        "numpy": numpy.__version__,
        "scipy": scipy.__version__,
    }


def pinned_calibration_runtime() -> Dict[str, str]:
    pin = load_pin()
    return {
        "python": f"{pin['python_major']}.{pin['python_minor']}",
        "numpy": pin["numpy"],
        "scipy": pin["scipy"],
    }


def require_calibration_runtime() -> Dict[str, str]:
    pin = load_pin()
    cur = current_calibration_runtime()
    want = pinned_calibration_runtime()
    if (sys.version_info.major, sys.version_info.minor) != (pin["python_major"], pin["python_minor"]):
        raise RuntimeError(f"calibration: refuse Python {cur['python']} want {want['python']}")
    if cur["numpy"] != want["numpy"]:
        raise RuntimeError(f"calibration: refuse NumPy {cur['numpy']} want {want['numpy']}")
    if cur["scipy"] != want["scipy"]:
        raise RuntimeError(f"calibration: refuse SciPy {cur['scipy']} want {want['scipy']}")
    return cur
