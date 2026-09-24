import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { accountShellDestination } from './auth-shell-presentation';

describe('accountShellDestination', () => {
  it('keeps unresolved account state on its current root', () => {
    assert.equal(accountShellDestination({
      destinationKind: null,
      pathname: '/',
      ready: false,
    }), 'stay');
  });

  for (const destinationKind of [null, 'onboarding', 'duplicate_email_recovery'] as const) {
    it(`keeps ${destinationKind ?? 'signed-out'} account entry outside the tab shell`, () => {
      assert.equal(accountShellDestination({
        destinationKind,
        pathname: '/home',
        ready: true,
      }), 'account-entry');
    });
  }

  it('enters the tab shell only for an activated Home destination', () => {
    assert.equal(accountShellDestination({
      destinationKind: 'home',
      pathname: '/',
      ready: true,
    }), 'home-tabs');
  });

  it('does not redirect an already-correct account or Home surface', () => {
    assert.equal(accountShellDestination({ destinationKind: null, pathname: '/', ready: true }), 'stay');
    assert.equal(accountShellDestination({ destinationKind: 'home', pathname: '/home', ready: true }), 'stay');
  });
});
