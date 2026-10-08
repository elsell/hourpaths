#!/usr/bin/env python3
"""Run the complete audit with the owner's exact, expiring advisory exceptions."""
import datetime as dt
import json
import subprocess
import sys

DEADLINE = dt.datetime(2026, 10, 15, tzinfo=dt.timezone.utc)
ADVISORY = 'GHSA-86w9-cpqp-85rv'
EXCEPTIONS = {
    ADVISORY: ('node-forge', '1.4.0'),
    'GHSA-vfj7-8cjw-p6xm': ('braces', '3.0.3'),
    'GHSA-hp3w-g68c-fv3c': ('sprintf-js', '1.0.3'),
}


def evaluate(report, returncode, now):
    if now >= DEADLINE:
        raise ValueError('package exceptions expired; remove them or obtain renewed owner approval')
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
        allowed = EXCEPTIONS.get(identity)
        if (allowed is None or advisory.get('module_name') != allowed[0]
                or not isinstance(findings, list) or not findings
                or any(not isinstance(f, dict) or f.get('version') != allowed[1] for f in findings)):
            raise ValueError('unaccepted vulnerability: ' + str(identity))
        exceptions.append(identity)
    return exceptions


def main():
    result = subprocess.run(['pnpm', 'audit', '--audit-level', 'low', '--json'],
                            capture_output=True, text=True, timeout=180)
    # Preserve the complete audit evidence, including excepted findings.
    print(result.stdout, end='')
    exceptions = evaluate(json.loads(result.stdout), result.returncode, dt.datetime.now(dt.timezone.utc))
    for advisory in exceptions:
        package, version = EXCEPTIONS[advisory]
        print(f'TEMPORARY OWNER EXCEPTION: {advisory}, {package}@{version}; expires {DEADLINE.isoformat()}')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.TimeoutExpired) as error:
        print(f'package audit rejected: {error}', file=sys.stderr)
        sys.exit(1)
