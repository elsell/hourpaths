import test from 'node:test';
import assert from 'node:assert/strict';
import { AccountRecovery, type AccountRecoveryIntent, type AccountRecoveryPorts } from './account-recovery';

function fixture() {
  let lifecycle: string | null = 'enrollment-1';
  let pending: AccountRecoveryIntent | null = null;
  const requests: string[] = [], revoked: string[] = [];
  const now = Date.parse('2026-10-08T12:00:00Z');
  const credential = { token: 'recovered-credential', expiresAt: new Date(now + 60_000).toISOString(), nextAction: 'home' as const };
  const ports: AccountRecoveryPorts = {
    lifecycle: () => lifecycle, now: () => now,
    pending: {
      read: async () => pending,
      save: async intent => { pending = intent; },
      remove: async (key, id) => { if (pending?.lifecycle === key && pending.id === id) pending = null; },
    },
    remote: key => {
      assert.equal(key, lifecycle);
      return {
        begin: async () => ({ id: 'challenge', provider: 'apple', nonce: 'hourpaths-recovery:' + 'a'.repeat(64), expiresAt: now + 600_000 }),
        complete: async () => { requests.push(key); return credential; },
      };
    },
    revoke: async token => { revoked.push(token); },
  };
  return { ports, service: new AccountRecovery(ports), credential, requests, revoked,
    replace: () => { lifecycle = 'enrollment-2'; }, pending: () => pending };
}

test('recovery returns a home credential only for the originating enrollment', async () => {
  const f = fixture(), intent = await f.service.begin();
  assert.equal(intent.provider, 'apple');
  assert.deepEqual(await f.service.complete('proof', intent), f.credential);
  assert.deepEqual(f.requests, ['enrollment-1']);
  assert.equal(f.pending(), null);
});
test('same-account reauthentication and newer intents cannot consume an old callback', async () => {
  const f = fixture(), intent = await f.service.begin();
  await assert.rejects(f.service.complete('proof', { ...intent, id: 'old' }), /superseded/);
  f.replace();
  await assert.rejects(f.service.complete('proof', intent), /account_changed/);
  assert.deepEqual(f.requests, []);
});
test('a late successful recovery is revoked rather than adopted after logout', async () => {
  const f = fixture(), intent = await f.service.begin(), remote = f.ports.remote;
  f.ports.remote = key => {
    const bound = remote(key);
    return { ...bound, complete: async (id, proof) => { const result = await bound.complete(id, proof); f.replace(); return result; } };
  };
  await assert.rejects(f.service.complete('proof', intent), /account_changed/);
  assert.deepEqual(f.requests, ['enrollment-1']);
  assert.deepEqual(f.revoked, [f.credential.token]);
});
test('expired and cancelled recovery do not contact the completion boundary', async () => {
  const f = fixture(), intent = await f.service.begin();
  f.ports.now = () => intent.expiresAt;
  await assert.rejects(f.service.complete('proof', intent), /expired/);
  await f.service.cancel(intent);
  await assert.rejects(f.service.complete('proof', intent), /missing/);
  assert.deepEqual(f.requests, []);
});
test('failed completion leaves enrollment intact and cancellation cannot erase a later intent', async () => {
  const f = fixture(), intent = await f.service.begin(), remote = f.ports.remote;
  f.ports.remote = key => ({ ...remote(key), complete: async () => { throw new Error('offline'); } });
  await assert.rejects(f.service.complete('proof', intent), /offline/);
  assert.deepEqual(f.pending(), intent);
  const newer = { ...intent, id: 'newer' };
  await f.ports.pending.save(newer);
  await f.service.cancel(intent);
  assert.deepEqual(f.pending(), newer);
});
