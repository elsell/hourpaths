#!/usr/bin/env python3
from __future__ import annotations

import argparse
import pathlib
import subprocess
import sys
import tempfile


REPOSITORY = pathlib.Path(__file__).resolve().parents[1]
API_CLIENT = REPOSITORY / "packages" / "api-client"


def generate(destination: pathlib.Path) -> None:
    destination.mkdir(parents=True)
    (destination / "src").mkdir()
    result = subprocess.run(
        ["go", "run", "./cmd/openapi"],
        cwd=REPOSITORY / "apps" / "api",
        check=True,
        capture_output=True,
    )
    (destination / "openapi.json").write_bytes(result.stdout)
    candidates = list((REPOSITORY / "node_modules" / ".pnpm").glob(
        "openapi-typescript@*/node_modules/openapi-typescript/bin/cli.js"
    ))
    if len(candidates) != 1:
        raise OSError("pinned openapi-typescript CLI is unavailable or ambiguous")
    subprocess.run(
        ["node", str(candidates[0]), "openapi.json", "-o", "src/schema.d.ts"],
        cwd=destination,
        check=True,
        capture_output=True,
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--expected-root", type=pathlib.Path, default=API_CLIENT)
    args = parser.parse_args()
    expected_root = args.expected_root.resolve()
    expected_schema = expected_root / "src" / "schema.d.ts"
    if not expected_schema.exists():
        expected_schema = expected_root / "schema.d.ts"

    try:
        with tempfile.TemporaryDirectory() as directory:
            generated_root = pathlib.Path(directory) / "api-client"
            generate(generated_root)
            pairs = [
                (expected_root / "openapi.json", generated_root / "openapi.json"),
                (expected_schema, generated_root / "src" / "schema.d.ts"),
            ]
            drifted = []
            for expected, actual in pairs:
                if not expected.exists() or expected.read_bytes() != actual.read_bytes():
                    drifted.append(expected)
            if drifted:
                detail = ""
                for expected in drifted:
                    if expected.exists() and "FND10-DRIFT" in expected.read_text(encoding="utf-8"):
                        detail = " (FND10-DRIFT)"
                        break
                print(f"generated contract drift{detail}: {', '.join(str(path) for path in drifted)}", file=sys.stderr)
                return 1
    except (OSError, subprocess.CalledProcessError) as error:
        print(f"generated contract verification failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
