import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const script = fileURLToPath(new URL('./validate-mobile-release-env.mjs', import.meta.url));
const valid = {
  HOURPATHS_APP_ENV: 'production',
  HOURPATHS_API_URL: 'https://api.example.com',
  HOURPATHS_OIDC_ISSUER: 'https://identity.example.com',
  HOURPATHS_MOBILE_OIDC_CLIENT_ID: 'hourpaths-mobile',
  HOURPATHS_EXPO_PROJECT_ID: '3f14cb3e-753d-48eb-89e4-54df740de5c4',
};

function validate(overrides = {}) {
  execFileSync(process.execPath, [script], { env: { ...valid, ...overrides }, stdio: 'pipe' });
}

test('signed release validation accepts explicit public production endpoints', () => {
  assert.doesNotThrow(() => validate());
});

test('signed release validation rejects unspecified and IPv4-mapped local IPv6 endpoints', () => {
  for (const endpoint of ['https://[::]', 'https://[::ffff:7f00:1]', 'https://[::ffff:a00:204]', 'https://[::ffff:ac14:102]', 'https://[::ffff:c0a8:102]', 'https://[::ffff:a9fe:304]']) {
    assert.throws(() => validate({ HOURPATHS_API_URL: endpoint }), endpoint);
    assert.throws(() => validate({ HOURPATHS_OIDC_ISSUER: endpoint }), endpoint);
  }
});

test('signed release validation requires an Expo project UUID', () => {
  for (const value of ['', 'project', '3f14cb3e-753d-48eb-89e4']) {
    assert.throws(() => validate({ HOURPATHS_EXPO_PROJECT_ID: value }), value);
  }
});
