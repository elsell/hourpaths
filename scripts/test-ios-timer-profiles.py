import copy
import datetime
import importlib.util
import pathlib
import unittest

spec = importlib.util.spec_from_file_location('profiles', pathlib.Path(__file__).with_name('verify-ios-timer-profiles.py'))
profiles = importlib.util.module_from_spec(spec)
spec.loader.exec_module(profiles)


class SigningProfiles(unittest.TestCase):
    def test_app_store_profile_requires_matching_identity_group_and_unexpired_distribution(self):
        now = datetime.datetime(2026, 10, 8)
        profile = {
            'Name': 'Timer extension', 'TeamIdentifier': ['TEAM'],
            'ExpirationDate': now + datetime.timedelta(days=1),
            'Entitlements': {
                'application-identifier': 'TEAM.com.hourpaths.mobile.ExpoWidgetsTarget',
                'com.apple.security.application-groups': ['group.com.hourpaths.mobile'],
                'get-task-allow': False,
            },
        }
        def verify(value):
            profiles.validate(value, bundle='com.hourpaths.mobile.ExpoWidgetsTarget', team='TEAM',
                              name='Timer extension', group='group.com.hourpaths.mobile', now=now)
        verify(profile)
        for key, value in [('Name', 'Wrong'), ('TeamIdentifier', ['OTHER']), ('ExpirationDate', now),
                           ('ProvisionedDevices', ['device']), ('ProvisionsAllDevices', True)]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                verify({**profile, key: value})
        for key, value in [('application-identifier', 'TEAM.com.hourpaths.mobile'),
                           ('com.apple.security.application-groups', []), ('get-task-allow', True)]:
            invalid = copy.deepcopy(profile)
            invalid['Entitlements'][key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                verify(invalid)


if __name__ == '__main__':
    unittest.main()
