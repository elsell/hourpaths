import assert from 'node:assert/strict';
import test from 'node:test';
import { parseWebConfig } from './config.js';

test('web server injection uses the same fail-closed public configuration contract', () => {
  assert.deepEqual(parseWebConfig({
    environment: 'production',
    apiURL: 'https://api.example.com',
    oidcIssuer: 'https://identity.example.com',
    oidcClientId: 'hourpaths-web',
  }), {
    environment: 'production',
    apiURL: 'https://api.example.com',
    oidcIssuer: 'https://identity.example.com',
    oidcClientId: 'hourpaths-web',
  });
  assert.throws(() => parseWebConfig({
    environment: 'production',
    apiURL: 'http://localhost:8080',
    oidcIssuer: 'https://identity.example.com',
    oidcClientId: 'hourpaths-web',
  }));
});
