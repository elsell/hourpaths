#!/usr/bin/env python3
import copy
import datetime as dt
import importlib.util
import pathlib
import unittest

spec = importlib.util.spec_from_file_location('audit', pathlib.Path(__file__).with_name('check-package-audit.py'))
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)

class AuditPolicyTest(unittest.TestCase):
    def test_only_owner_approved_advisory_and_installed_version_are_accepted(self):
        now = dt.datetime(2026, 10, 2, tzinfo=dt.timezone.utc)
        report = {'metadata': {'vulnerabilities': {'high': 1}}, 'advisories': {'1': {
            'github_advisory_id': audit.ADVISORY, 'module_name': 'node-forge',
            'findings': [{'version': '1.4.0'}]}}}
        self.assertEqual(audit.evaluate(report, 1, now), [audit.ADVISORY])
        for field, value in [('github_advisory_id', 'GHSA-other'), ('module_name', 'another'), ('findings', [{'version': '1.3.0'}]), ('findings', [])]:
            changed = copy.deepcopy(report)
            changed['advisories']['1'][field] = value
            with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                audit.evaluate(changed, 1, now)
        mixed = copy.deepcopy(report)
        mixed['advisories']['2'] = {'github_advisory_id': 'GHSA-other'}
        for data, code, instant in [(mixed, 1, now), (report, 2, now), (report, 1, audit.DEADLINE), ({}, 0, now), ({'error': 'registry unavailable'}, 1, now)]:
            with self.assertRaises(ValueError):
                audit.evaluate(data, code, instant)
        clean = {'metadata': {'vulnerabilities': {'high': 0}}, 'advisories': {}}
        self.assertEqual(audit.evaluate(clean, 0, now), [])

    def test_braces_exception_is_exact_and_expires_at_the_approved_deadline(self):
        now = dt.datetime(2026, 10, 3, tzinfo=dt.timezone.utc)
        report = {'metadata': {'vulnerabilities': {'high': 1}}, 'advisories': {'1': {
            'github_advisory_id': 'GHSA-vfj7-8cjw-p6xm', 'module_name': 'braces',
            'findings': [{'version': '3.0.3'}]}}}
        self.assertEqual(audit.evaluate(report, 1, now), ['GHSA-vfj7-8cjw-p6xm'])
        for field, value in [('github_advisory_id', 'GHSA-other'), ('module_name', 'brace-expansion'),
                             ('findings', [{'version': '3.0.2'}]), ('findings', [{'version': '3.0.3'}, {'version': '3.0.2'}])]:
            changed = copy.deepcopy(report)
            changed['advisories']['1'][field] = value
            with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                audit.evaluate(changed, 1, now)
        with self.assertRaises(ValueError):
            audit.evaluate(report, 1, dt.datetime(2026, 10, 15, tzinfo=dt.timezone.utc))

    def test_sprintf_exception_rejects_different_versions_and_deadline(self):
        report = {'metadata': {'vulnerabilities': {'moderate': 1}}, 'advisories': {'1': {
            'github_advisory_id': 'GHSA-hp3w-g68c-fv3c', 'module_name': 'sprintf-js',
            'findings': [{'version': '1.0.3'}]}}}
        now = dt.datetime(2026, 10, 8, tzinfo=dt.timezone.utc)
        self.assertEqual(audit.evaluate(report, 1, now), ['GHSA-hp3w-g68c-fv3c'])
        with self.assertRaises(ValueError):
            audit.evaluate(report, 1, dt.datetime(2026, 10, 15, tzinfo=dt.timezone.utc))
        report['advisories']['1']['findings'].append({'version': '1.1.3'})
        with self.assertRaises(ValueError):
            audit.evaluate(report, 1, now)

if __name__ == '__main__':
    unittest.main()
