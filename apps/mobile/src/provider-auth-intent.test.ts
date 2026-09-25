import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { redirectSystemPath } from '../app/+native-intent';

describe('redirectSystemPath', () => {
  for (const response of [
    'hourpaths://callback?code=authorization-code&state=expected-state',
    'hourpaths://callback?error=access_denied&state=expected-state',
    'hourpaths://callback#code=authorization-code&state=expected-state',
  ]) {
    it('keeps an OIDC response in the active AuthSession instead of routing it', () => {
      assert.equal(redirectSystemPath({ initial: false, path: response }), null);
    });
  }

  it('fails closed at account entry when a callback cold-starts the app', () => {
    assert.equal(redirectSystemPath({
      initial: true,
      path: 'hourpaths://callback?code=authorization-code&state=orphaned-state',
    }), null);
  });

  for (const path of [
    'hourpaths://callback.evil.example?code=authorization-code',
    'hourpaths://callback/child?code=authorization-code',
    'hourpaths://path/path-id',
    'https://login.hourpaths.com/callback?code=authorization-code',
    'not a url',
  ]) {
    it(`leaves unrelated intent unchanged: ${path}`, () => {
      assert.equal(redirectSystemPath({ initial: false, path }), path);
    });
  }
});
