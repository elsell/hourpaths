#!/usr/bin/env python3
import subprocess
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
FIXTURE = ROOT / "scripts" / "identity-fixture.py"


class IdentityFixtureTest(unittest.TestCase):
    def run_fixture(self, *arguments: str, input_text: str | None = None) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, str(FIXTURE), *arguments],
            cwd=ROOT,
            capture_output=True,
            text=True,
            input=input_text,
            check=False,
        )

    def test_user_id_matches_go_known_answer(self) -> None:
        result = self.run_fixture("user-id", "http://localhost:5556/dex", "subject")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "4ce4570c-8fdd-5c76-8fa3-69c92efba210")

    def test_malformed_identity_token_fails_closed(self) -> None:
        result = self.run_fixture("subject", input_text="malformed")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, "")


if __name__ == "__main__":
    unittest.main()
