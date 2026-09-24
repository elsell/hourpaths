#!/usr/bin/env python3
from __future__ import annotations

import pathlib
import subprocess
import sys


def main() -> int:
    repository = pathlib.Path(__file__).resolve().parents[1]
    root = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    result = subprocess.run(
        ["node", str(repository / "scripts/check-client-api-boundary.mjs"), str(root)],
        check=False,
    )
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
