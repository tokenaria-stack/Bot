"""python -m research.signaldiagnosticsc"""

from __future__ import annotations

import sys

from research.signaldiagnosticsc.run import render_report, run_diagnostics


def main(argv=None) -> int:
    doc = run_diagnostics()
    print(render_report(doc), end="")
    return 0


if __name__ == "__main__":
    sys.exit(main())
