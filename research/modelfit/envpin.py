"""Fail-closed runtime pin: Python minor, NumPy, sklearn, SciPy."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Dict

PIN_PATH = Path(__file__).with_name("pin.json")


def load_pin() -> Dict:
    return json.loads(PIN_PATH.read_text(encoding="utf-8"))


def current_runtime() -> Dict[str, str]:
    import numpy
    import scipy
    import sklearn

    return {
        "python": f"{sys.version_info.major}.{sys.version_info.minor}",
        "numpy": numpy.__version__,
        "sklearn": sklearn.__version__,
        "scipy": scipy.__version__,
    }


def pinned_runtime() -> Dict[str, str]:
    pin = load_pin()
    return {
        "python": f"{pin['python_major']}.{pin['python_minor']}",
        "numpy": pin["numpy"],
        "sklearn": pin["sklearn"],
        "scipy": pin["scipy"],
    }


def require_pinned_runtime() -> Dict[str, str]:
    pin = load_pin()
    cur = current_runtime()
    want = pinned_runtime()
    if (sys.version_info.major, sys.version_info.minor) != (pin["python_major"], pin["python_minor"]):
        raise RuntimeError(f"modelfit: refuse Python {cur['python']} want {want['python']}")
    if cur["numpy"] != want["numpy"]:
        raise RuntimeError(f"modelfit: refuse NumPy {cur['numpy']} want {want['numpy']}")
    if cur["sklearn"] != want["sklearn"]:
        raise RuntimeError(f"modelfit: refuse sklearn {cur['sklearn']} want {want['sklearn']}")
    if cur["scipy"] != want["scipy"]:
        raise RuntimeError(f"modelfit: refuse SciPy {cur['scipy']} want {want['scipy']}")
    return cur
