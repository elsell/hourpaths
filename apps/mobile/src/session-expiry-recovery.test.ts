import assert from 'node:assert/strict';
import test from 'node:test';
import { recoverRetainedSession, retainedAccount, type SessionExchangeCredential } from '@hourpaths/client-core';
import { createSerializedMobileSessionStorage } from './session-state';
import { restoreStoredSession } from './session-restoration';

const credential: SessionExchangeCredential = { token: 'old-session', ownerId: 'alice', nextAction: 'home', expiresAt: '2026-10-01T12:00:00Z' };
function storage() {
  let raw: string | null = JSON.stringify(credential);
  return createSerializedMobileSessionStorage({ read: async () => raw,
    write: async value => { raw = JSON.stringify(value); },
    pause: async owner => { raw = JSON.stringify(retainedAccount(owner)); },
    discard: async () => { raw = null; },
  });
}
test('cold expiry replaces the credential with only its verified retained owner', async () => {
  const stored = storage();
  let recovered: unknown;
  await restoreStoredSession({ read: stored.read, now: () => Date.parse('2026-10-02T12:00:00Z'), refreshLeadMs: 60000,
    current: () => true, refresh: async () => assert.fail('expired credential cannot refresh'), expiryAdvanced: () => false,
    activate: async () => assert.fail('expired credential cannot activate'), revokeSuperseded: async () => {},
    handleUnreadable: async () => assert.fail('valid expired credential is readable'),
    handleFailure: async (failure, current) => {
      assert.deepEqual(failure, { kind: 'expired' });
      recovered = await recoverRetainedSession({ credential: current, current: () => true,
        restore: async owner => ({ owner, value: 'retained-timer' }), pause: stored.pause });
    },
  });
  assert.deepEqual(recovered, { kind: 'retained', value: 'retained-timer' });
  assert.deepEqual(JSON.parse((await stored.read())!), retainedAccount('alice'));
});
test('late recovery cannot overwrite a newer sign-in or publish the old partition', async () => {
  const stored = storage();
  let live = true;
  const result = await recoverRetainedSession({ credential, current: () => live,
    restore: async owner => {
      await stored.persist({ ...credential, token: 'new-session', ownerId: 'bob' }, () => true);
      live = false;
      return { owner, value: 'old-timer' };
    }, pause: stored.pause,
  });
  assert.deepEqual(result, { kind: 'superseded' });
  assert.equal(JSON.parse((await stored.read())!).ownerId, 'bob');
});
test('unbound credentials and mismatched retained owners cannot enter recovery', async () => {
  const stored = storage();
  assert.deepEqual(await recoverRetainedSession({ credential: { ...credential, ownerId: undefined }, current: () => true,
    restore: async () => assert.fail('unbound owner must not be read'), pause: stored.pause }), { kind: 'unavailable' });
  assert.deepEqual(await recoverRetainedSession({ credential, current: () => true,
    restore: async () => ({ owner: 'bob', value: 'other-timer' }), pause: stored.pause }), { kind: 'unavailable' });
  assert.equal(JSON.parse((await stored.read())!).token, credential.token);
});
