#!/usr/bin/env python3
"""Run the complete audit with the owner's one expiring advisory exception."""
import datetime as dt
import json
import subprocess
import sys

DEADLINE = dt.datetime(2026, 10, 9, tzinfo=dt.timezone.utc)
ADVISORY = 'GHSA-86w9-cpqp-85rv'


def evaluate(report, returncode, now):
    if now >= DEADLINE:
        raise ValueError('node-forge exception expired; remove it or obtain renewed owner approval')
    if returncode not in (0, 1) or not isinstance(report, dict) or report.get('error'):
        raise ValueError('package audit failed')
    advisories = report.get('advisories')
    counts = report.get('metadata', {}).get('vulnerabilities')
    if not isinstance(advisories, dict) or not isinstance(counts, dict) or not counts:
        raise ValueError('unrecognized package audit report')
    if any(type(count) is not int or count < 0 for count in counts.values()):
        raise ValueError('invalid vulnerability counts')
    if not advisories and (returncode != 0 or any(counts.values())):
        raise ValueError('audit omitted reported vulnerabilities')
    exceptions = []
    for advisory in advisories.values():
        if not isinstance(advisory, dict):
            raise ValueError('malformed advisory')
        identity = advisory.get('github_advisory_id')
        if identity is None:
            identity = advisory.get('url', '').removeprefix('https://github.com/advisories/')
        findings = advisory.get('findings')
        if (identity != ADVISORY or advisory.get('module_name') != 'node-forge'
                or not isinstance(findings, list) or not findings
                or any(not isinstance(f, dict) or f.get('version') != '1.4.0' for f in findings)):
            raise ValueError('unaccepted vulnerability: ' + str(identity))
        exceptions.append(ADVISORY)
    return exceptions


def main():
    result = subprocess.run(['pnpm', 'audit', '--audit-level', 'low', '--json'],
                            capture_output=True, text=True, timeout=180)
    # Preserve the complete audit evidence, including excepted findings.
    print(result.stdout, end='')
    exceptions = evaluate(json.loads(result.stdout), result.returncode, dt.datetime.now(dt.timezone.utc))
    for advisory in exceptions:
        print(f'TEMPORARY OWNER EXCEPTION: {advisory}, node-forge@1.4.0; expires {DEADLINE.isoformat()}')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.TimeoutExpired) as error:
        print(f'package audit rejected: {error}', file=sys.stderr)
        sys.exit(1)
