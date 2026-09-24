#!/usr/bin/env python3
from __future__ import annotations

import json
import pathlib
import shutil
import subprocess
import tempfile
import unittest


REPOSITORY = pathlib.Path(__file__).resolve().parents[1]
CHECKER = REPOSITORY / "scripts/check-generated-contracts.py"


class GeneratedContractDriftTest(unittest.TestCase):
    def run_checker(self, expected_root: pathlib.Path) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["python3", str(CHECKER), "--expected-root", str(expected_root)],
            cwd=REPOSITORY,
            check=False,
            capture_output=True,
            text=True,
        )

    def test_clean_generated_contracts_match(self) -> None:
        result = self.run_checker(REPOSITORY / "packages" / "api-client")
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_deliberate_openapi_drift_fails_closed(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            expected_root = pathlib.Path(directory)
            shutil.copy(REPOSITORY / "packages/api-client/openapi.json", expected_root / "openapi.json")
            shutil.copy(REPOSITORY / "packages/api-client/src/schema.d.ts", expected_root / "schema.d.ts")
            contract = json.loads((expected_root / "openapi.json").read_text(encoding="utf-8"))
            contract["info"]["description"] = "FND10-DRIFT"
            (expected_root / "openapi.json").write_text(
                json.dumps(contract, separators=(",", ":")) + "\n",
                encoding="utf-8",
            )

            result = self.run_checker(expected_root)

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("FND10-DRIFT", result.stderr)

    def test_deliberate_typescript_schema_drift_fails_closed(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            expected_root = pathlib.Path(directory)
            shutil.copy(REPOSITORY / "packages/api-client/openapi.json", expected_root / "openapi.json")
            shutil.copy(REPOSITORY / "packages/api-client/src/schema.d.ts", expected_root / "schema.d.ts")
            schema = expected_root / "schema.d.ts"
            schema.write_text(
                schema.read_text(encoding="utf-8") + "\n// FND10-DRIFT\n",
                encoding="utf-8",
            )

            result = self.run_checker(expected_root)

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("FND10-DRIFT", result.stderr)
        self.assertIn("schema.d.ts", result.stderr)


if __name__ == "__main__":
    unittest.main()
