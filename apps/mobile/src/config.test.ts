import assert from 'node:assert/strict';
import test from 'node:test';
import buildAppConfig from '../app.config.js';
import { loadMobileConfig } from './config.js';

const production = {
  hourpaths: {
    environment: 'production',
    apiURL: 'https://api.example.com',
    oidcIssuer: 'https://identity.example.com',
    oidcClientId: 'hourpaths-mobile',
    pushProjectId: '3f14cb3e-753d-48eb-89e4-54df740de5c4',
  },
};

test('mobile configuration accepts validated Expo extra', () => {
  assert.deepEqual(loadMobileConfig(production), production.hourpaths);
});

test('mobile production configuration requires a valid Expo push project identity', () => {
  for (const pushProjectId of [undefined, null, '', 'project', 7]) {
    assert.throws(() => loadMobileConfig({
      hourpaths: { ...production.hourpaths, pushProjectId },
    }));
  }
});

test('mobile development configuration can defer Expo project linking', () => {
  for (const pushProjectId of [undefined, null]) {
    assert.equal(loadMobileConfig({
      hourpaths: {
        ...production.hourpaths,
        environment: 'development',
        apiURL: 'http://localhost:8080',
        oidcIssuer: 'http://localhost:5556/dex',
        pushProjectId,
      },
    }).pushProjectId, null);
  }
});

test('development app config omits unlinked Expo project values', () => {
  const previous = process.env.HOURPATHS_EXPO_PROJECT_ID;
  delete process.env.HOURPATHS_EXPO_PROJECT_ID;
  try {
    const config = buildAppConfig({ config: {} });
    const extra = config.extra as Record<string, unknown>;
    const hourpaths = extra.hourpaths as Record<string, unknown>;
    assert.equal('eas' in extra, false);
    assert.equal('pushProjectId' in hourpaths, false);
  } finally {
    if (previous === undefined) delete process.env.HOURPATHS_EXPO_PROJECT_ID;
    else process.env.HOURPATHS_EXPO_PROJECT_ID = previous;
  }
});

test('mobile configuration rejects malformed deployment environments', () => {
  for (const environment of ['prod', 'preview', '', 7, null]) {
    assert.throws(() => loadMobileConfig({ hourpaths: { ...production.hourpaths, environment } }));
  }
});

test('mobile configuration rejects blank OIDC client identifiers', () => {
  for (const oidcClientId of ['', ' ', '\t', 'mobile client']) {
    assert.throws(() => loadMobileConfig({ hourpaths: { ...production.hourpaths, oidcClientId } }));
  }
});

test('mobile production configuration rejects unsafe endpoints', () => {
  for (const endpoint of ['http://api.example.com', 'https://localhost', 'https://sub.localhost/path', 'https://127.0.0.2', 'https://[::1]', 'https://[::]', 'https://[::ffff:7f00:1]', 'https://[::ffff:a00:204]', 'https://[::ffff:ac14:102]', 'https://[::ffff:c0a8:102]', 'https://[::ffff:a9fe:304]', 'https://10.0.2.2', 'https://10.2.3.4', 'https://172.20.1.2', 'https://192.168.1.2', 'https://169.254.3.4']) {
    assert.throws(() => loadMobileConfig({ hourpaths: { ...production.hourpaths, apiURL: endpoint } }));
    assert.throws(() => loadMobileConfig({ hourpaths: { ...production.hourpaths, oidcIssuer: endpoint } }));
  }
  assert.equal(loadMobileConfig({ hourpaths: { ...production.hourpaths, apiURL: 'https://fc.example.com' } }).apiURL, 'https://fc.example.com');
});

test('mobile configuration fails closed when Expo extra is absent or malformed', () => {
  for (const value of [undefined, null, {}, { hourpaths: null }, { hourpaths: 'production' }, { hourpaths: { environment: 'development' } }]) {
    assert.throws(() => loadMobileConfig(value));
  }
});
