#!/usr/bin/env python3
"""Fail closed around the temporary unpatched image-size audit exception."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
from pathlib import Path

EXPECTED_ADVISORIES = {"GHSA-5p2g-fcmc-qvqq", "GHSA-w3rx-r6r6-pgpr"}
EXPECTED_VERSION = "1.2.1"
def fail(message: str) -> None:
    raise SystemExit(f"audit advisory allowlist: {message}")


def patched_release_status() -> str:
    controlled = os.environ.get("HOURPATHS_IMAGE_SIZE_203_STATUS")
    if controlled:
        return controlled
    try:
        result = subprocess.run(
            ["pnpm", "view", "image-size", "versions", "--json"],
            capture_output=True,
            text=True,
            timeout=15,
            check=False,
        )
        versions = json.loads(result.stdout)
    except (OSError, subprocess.TimeoutExpired, json.JSONDecodeError) as error:
        fail(f"registry metadata unavailable: {error}")
    if result.returncode != 0 or not isinstance(versions, list):
        fail("registry metadata unavailable or malformed")
    stable = {
        tuple(int(part) for part in version.split("."))
        for version in versions
        if isinstance(version, str) and re.fullmatch(r"\d+\.\d+\.\d+", version)
    }
    return "published" if any(version > (2, 0, 2) for version in stable) else "missing"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--workspace", default="pnpm-workspace.yaml")
    parser.add_argument("--lockfile", default="pnpm-lock.yaml")
    args = parser.parse_args()

    workspace = Path(args.workspace).read_text(encoding="utf-8")
    lockfile = Path(args.lockfile).read_text(encoding="utf-8")
    advisory_ids = set(re.findall(r"^\s+- (GHSA-[0-9a-z-]+)\s*$", workspace, re.MULTILINE))
    if advisory_ids != EXPECTED_ADVISORIES:
        fail(f"expected only {sorted(EXPECTED_ADVISORIES)}, got {sorted(advisory_ids)}")

    override = re.findall(r"^\s{2}image-size: '([^']+)'\s*$", workspace, re.MULTILINE)
    if override:
        fail("image-size must not be overridden while no patched release exists")
    resolved = set(re.findall(r"^\s{2}image-size@([^:]+):\s*$", lockfile, re.MULTILINE))
    if resolved != {EXPECTED_VERSION}:
        fail(f"image-size lock resolution must be exactly {EXPECTED_VERSION}, got {sorted(resolved)}")

    status = patched_release_status()
    if status == "published":
        fail("a stable image-size release newer than 2.0.2 is published; remove the exception and upgrade")
    if status != "missing":
        fail(f"unexpected registry status {status!r}")
    print("image-size advisory exception remains valid while no patched stable release exists")


if __name__ == "__main__":
    main()
