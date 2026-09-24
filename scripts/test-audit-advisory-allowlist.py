#!/usr/bin/env python3
from __future__ import annotations

import os
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHECK = ROOT / "scripts" / "check-audit-advisory-allowlist.py"
WORKSPACE = """auditConfig:
  ignoreGhsas:
    - GHSA-5p2g-fcmc-qvqq
    - GHSA-w3rx-r6r6-pgpr
overrides:
  nanoid: '3.3.18'
"""
LOCKFILE = """packages:
  image-size@1.2.1:
    resolution: {integrity: reviewed}
snapshots:
  image-size@1.2.1:
    dependencies: {}
"""


def run(workspace: str = WORKSPACE, lockfile: str = LOCKFILE, status: str = "missing") -> subprocess.CompletedProcess[str]:
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        (root / "pnpm-workspace.yaml").write_text(workspace, encoding="utf-8")
        (root / "pnpm-lock.yaml").write_text(lockfile, encoding="utf-8")
        env = {**os.environ, "HOURPATHS_IMAGE_SIZE_203_STATUS": status}
        return subprocess.run(
            ["python3", str(CHECK), "--workspace", str(root / "pnpm-workspace.yaml"), "--lockfile", str(root / "pnpm-lock.yaml")],
            capture_output=True,
            text=True,
            env=env,
            check=False,
        )


assert run().returncode == 0
assert run(workspace=WORKSPACE.replace("GHSA-w3rx-r6r6-pgpr", "GHSA-wrong-id")).returncode != 0
assert run(workspace=WORKSPACE.replace("  nanoid: '3.3.18'", "  image-size: '1.2.1'\n  nanoid: '3.3.18'")).returncode != 0
assert run(lockfile=LOCKFILE.replace("1.2.1", "2.0.2")).returncode != 0
assert run(status="published").returncode != 0
print("audit advisory allowlist tests passed")
