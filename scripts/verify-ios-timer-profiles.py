#!/usr/bin/env python3
"""Validate decoded signing profiles and write an escaped export-options plist."""
import datetime
import os
import pathlib
import plistlib
import sys


def validate(profile, *, bundle, team, name, group, now):
    entitlements = profile.get('Entitlements', {})
    if (profile.get('Name') != name or profile.get('TeamIdentifier') != [team]
            or entitlements.get('application-identifier') != f'{team}.{bundle}'
            or group not in entitlements.get('com.apple.security.application-groups', [])
            or entitlements.get('get-task-allow', False)
            or profile.get('ProvisionedDevices') or profile.get('ProvisionsAllDevices')
            or profile.get('ExpirationDate', now) <= now):
        raise ValueError(f'Invalid App Store timer signing profile for {bundle}')


if __name__ == '__main__':
    bundle = os.environ['APPLE_BUNDLE_ID']
    team = os.environ['APPLE_TEAM_ID']
    mappings = {}
    for path, identifier, name in [
        (sys.argv[1], bundle, os.environ['APPLE_PROVISIONING_PROFILE_NAME']),
        (sys.argv[2], f'{bundle}.ExpoWidgetsTarget', os.environ['APPLE_WIDGET_PROVISIONING_PROFILE_NAME']),
    ]:
        profile = plistlib.loads(pathlib.Path(path).read_bytes())
        validate(profile, bundle=identifier, team=team, name=name,
                 group=f'group.{bundle}', now=datetime.datetime.now(datetime.timezone.utc).replace(tzinfo=None))
        mappings[identifier] = name
    pathlib.Path(sys.argv[3]).write_bytes(plistlib.dumps({
        'method': 'app-store', 'signingStyle': 'manual', 'teamID': team,
        'provisioningProfiles': mappings,
    }))
