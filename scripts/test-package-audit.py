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

if __name__ == '__main__':
    unittest.main()
