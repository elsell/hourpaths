#!/usr/bin/env python3
"""Table-driven tests for the fail-safe CI change classifier."""

from __future__ import annotations

import unittest
from pathlib import Path
import re
import subprocess

import ci_changes


ALL = ci_changes.Plan(acceptance=True, api_image=True, web_image=True, native=True)
NONE = ci_changes.Plan()


class ClassifyChangesTest(unittest.TestCase):
    def test_representative_change_sets(self) -> None:
        cases = {
            "spec only": (["specs/paths/core.spec.md"], NONE),
            "platform documentation only": (["specs/platform/testing.spec.md"], NONE),
            "api code": (
                ["apps/api/internal/domain/identity/user.go"],
                ci_changes.Plan(acceptance=True, api_image=True),
            ),
            "web code": (
                ["apps/web/src/routes/+page.svelte"],
                ci_changes.Plan(acceptance=True, web_image=True),
            ),
            "mobile code": (["apps/mobile/app/index.tsx"], ci_changes.Plan(native=True)),
            "shared client code": (
                ["packages/client-core/src/index.ts"],
                ci_changes.Plan(acceptance=True, web_image=True, native=True),
            ),
            "root javascript lock": (
                ["pnpm-lock.yaml"],
                ci_changes.Plan(acceptance=True, web_image=True, native=True),
            ),
            "compose topology": (["compose.yaml"], ci_changes.Plan(acceptance=True)),
            "browser acceptance": (
                ["scripts/web-browser-acceptance.mjs"],
                ci_changes.Plan(acceptance=True),
            ),
            "goal update browser acceptance": (
                ["scripts/web-goal-update-acceptance.mjs"],
                ci_changes.Plan(acceptance=True),
            ),
            "Scalar acceptance guard": (
                ["scripts/scalar-try-retry.mjs", "scripts/scalar-try-retry.test.mjs"],
                ci_changes.Plan(acceptance=True),
            ),
            "docker context": (
                [".dockerignore"],
                ci_changes.Plan(acceptance=True, api_image=True, web_image=True),
            ),
            "CI routing itself": ([".github/workflows/ci.yml"], ALL),
            "classifier contract": (
                ["scripts/ci_changes.py", "scripts/test_ci_changes.py"],
                NONE,
            ),
            "unclassified path fails safe": (["future/runtime/new.input"], ALL),
            "sets are unioned": (
                ["apps/api/cmd/server/main.go", "apps/mobile/app/index.tsx"],
                ci_changes.Plan(acceptance=True, api_image=True, native=True),
            ),
        }
        for name, (paths, expected) in cases.items():
            with self.subTest(name=name):
                self.assertEqual(ci_changes.classify(paths), expected)

    def test_empty_or_malformed_input_fails_safe(self) -> None:
        for paths in ([], [""], ["/absolute"], ["../escape"], ["a\\b"]):
            with self.subTest(paths=paths):
                self.assertEqual(ci_changes.classify(paths), ALL)

    def test_force_full_overrides_safe_paths(self) -> None:
        self.assertEqual(ci_changes.classify(["README.md"], force_full=True), ALL)

    def test_git_range_policy(self) -> None:
        sha_a = "a" * 40
        sha_b = "b" * 40
        cases = {
            "pull request": (
                ("pull_request", "refs/pull/7/merge", sha_a, sha_b, sha_a),
                f"{sha_b}...{sha_a}",
            ),
            "feature push": (
                ("push", "refs/heads/codex/topic", sha_b, "", sha_a),
                f"{sha_b}..{sha_a}",
            ),
            "main push uses exact landed diff": (
                ("push", "refs/heads/main", sha_b, "", sha_a),
                f"{sha_b}..{sha_a}",
            ),
            "manual is full": (("workflow_dispatch", "refs/heads/topic", "", "", sha_a), None),
            "missing predecessor is full": (("push", "refs/heads/topic", "0" * 40, "", sha_a), None),
            "unknown event is full": (("schedule", "refs/heads/topic", sha_b, "", sha_a), None),
        }
        for name, (arguments, expected) in cases.items():
            with self.subTest(name=name):
                self.assertEqual(ci_changes.select_diff_range(*arguments), expected)


class WorkflowRoutingGuardTest(unittest.TestCase):
    workflow = (Path(__file__).parent.parent / ".github/workflows/ci.yml").read_text(encoding="utf-8")
    release_workflow = (Path(__file__).parent.parent / ".github/workflows/release.yml").read_text(encoding="utf-8")
    testflight_workflow = (Path(__file__).parent.parent / ".github/workflows/testflight.yml").read_text(encoding="utf-8")
    stable_jobs = ("verify", "acceptance", "android-native", "ios-native")

    def test_testflight_archive_verifies_export_compliance(self) -> None:
        self.assertIn("ITSAppUsesNonExemptEncryption", self.testflight_workflow)
        self.assertIn("info.get('ITSAppUsesNonExemptEncryption') is not False", self.testflight_workflow)

    def test_push_runs_only_after_changes_land_on_main(self) -> None:
        triggers = re.search(
            r"(?ms)^on:\n(?P<body>.*?)(?=^[A-Za-z][A-Za-z0-9_-]*:\n)",
            self.workflow,
        )
        self.assertIsNotNone(triggers, "missing top-level workflow triggers")
        self.assertEqual(
            triggers.group("body"),  # type: ignore[union-attr]
            "  push:\n"
            "    branches: [main]\n"
            "  pull_request:\n"
            "  workflow_dispatch:\n",
        )

    def _job_body(self, job: str) -> str:
        match = re.search(rf"(?ms)^  {re.escape(job)}:\n(?P<body>.*?)(?=^  [a-z][a-z0-9-]*:\n|\Z)", self.workflow)
        self.assertIsNotNone(match, f"missing stable job {job}")
        return match.group("body")  # type: ignore[union-attr]

    def _guard_script(self, job: str) -> str:
        match = re.search(
            r"(?ms)^      - id: routing\n.*?^        run: \|\n(?P<script>(?:^          .*\n)+)^        env:\n",
            self._job_body(job),
        )
        self.assertIsNotNone(match, f"missing first routing guard in {job}")
        return match.group("script")  # type: ignore[union-attr]

    def test_stable_jobs_always_run_and_gate_every_later_step(self) -> None:
        for job in self.stable_jobs:
            with self.subTest(job=job):
                body = self._job_body(job)
                self.assertIn("    if: always()\n", body)
                self.assertIn("      - id: routing\n", body)
                self.assertLess(body.index("      - id: routing\n"), body.index("      - uses:"))
                later_steps = body[body.index("      - uses:") :]
                step_count = len(re.findall(r"(?m)^      - (?:uses:|name:|if:)", later_steps))
                guarded_count = later_steps.count("steps.routing.outcome == 'success'")
                self.assertEqual(guarded_count, step_count)

    def test_guard_rejects_dependency_failure_and_invalid_outputs(self) -> None:
        script = self._guard_script("verify")
        cases = {
            "valid": ("success", "true", "false", "true", "false", 0),
            "dependency failed": ("failure", "true", "true", "true", "true", 1),
            "missing output": ("success", "", "false", "false", "false", 1),
            "malformed output": ("success", "yes", "false", "false", "false", 1),
        }
        for name, (result, acceptance, api_image, web_image, native, expected) in cases.items():
            with self.subTest(name=name):
                completed = subprocess.run(
                    ["bash", "-c", script],
                    check=False,
                    env={
                        "CHANGES_RESULT": result,
                        "ACCEPTANCE": acceptance,
                        "API_IMAGE": api_image,
                        "WEB_IMAGE": web_image,
                        "NATIVE": native,
                    },
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                )
                self.assertEqual(completed.returncode, expected)

    def test_stable_jobs_use_the_correct_hosted_runners(self) -> None:
        self.assertNotIn("self-hosted", self.workflow)
        self.assertNotIn("hourpaths-", self.workflow)
        expected_runners = {
            "verify": "ubuntu-24.04",
            "acceptance": "ubuntu-24.04",
            "android-native": "ubuntu-24.04",
            "ios-native": "macos-26",
        }
        for job, runner in expected_runners.items():
            with self.subTest(job=job):
                self.assertIn(f"runs-on: {runner}", self._job_body(job))
        android_body = self._job_body("android-native")
        self.assertIn('"cmake;3.22.1"', android_body)
        self.assertIn('"ndk;27.1.12297006"', android_body)
        self.assertIn('"ndk;27.0.12077973"', android_body)
        self.assertIn("check-hosted-android-toolchain.sh", android_body)
        body = self._job_body("ios-native")
        self.assertIn("make mobile-build-ios", body)
        self.assertNotIn("ios_local_xcode_evidence", body)
        self.assertNotIn("ruby/setup-ruby", body)
        self.assertIn("brew --prefix ruby@3.4", body)
        self.assertIn("gem install bundler --version 2.6.9", body)
        self.assertIn("bundle exec pod --version", body)
        self.assertIn("needs.changes.outputs.native == 'true'", body)
        self.assertIn("needs.changes.outputs.native != 'true'", body)

    def test_release_requires_all_ci_platform_jobs(self) -> None:
        self.assertIn("Require complete Linux, Android, and macOS CI checks", self.release_workflow)
        self.assertIn('gh run view "$ci_run_id" --json jobs', self.release_workflow)
        for job in ("changes", "verify", "acceptance", "android-native", "ios-native"):
            with self.subTest(job=job):
                self.assertIn(job, self.release_workflow)
        self.assertIn("testflight:", self.release_workflow)
        self.assertIn("needs: [plan, publish]", self.release_workflow)
        self.assertIn("release_tag: ${{ needs.plan.outputs.tag }}", self.release_workflow)

    def test_testflight_uses_the_deployed_logto_mobile_app_id(self) -> None:
        self.assertIn(
            "HOURPATHS_MOBILE_OIDC_CLIENT_ID: ctdb003l6t7f3d5hidfm7",
            self.testflight_workflow,
        )
        self.assertNotIn(
            "HOURPATHS_MOBILE_OIDC_CLIENT_ID: hourpaths-mobile",
            self.testflight_workflow,
        )


if __name__ == "__main__":
    unittest.main()
