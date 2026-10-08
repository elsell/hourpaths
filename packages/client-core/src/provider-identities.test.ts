import test from 'node:test';
import assert from 'node:assert/strict';
import { ProviderIdentities, providerSignInScopes, type IdentityLinkIntent, type ProviderIdentityPorts } from './provider-identities';

function fixture() {
  let owner: string | null = 'owner';
  let pending: IdentityLinkIntent | null = null;
  let completed = 0;
  const now = Date.parse('2026-10-08T12:00:00Z');
  const ports: ProviderIdentityPorts = {
    currentOwner: () => owner,
    now: () => now,
    pending: {
      read: async () => pending,
      save: async intent => { pending = intent; },
      remove: async (account, id) => { if (pending?.owner === account && pending.id === id) pending = null; },
    },
    remote: {
      list: async () => [{ provider: 'google', canUnlink: false }],
      begin: async provider => ({ id: 'challenge', provider, nonce: 'hourpaths-link:' + 'a'.repeat(64), expiresAt: now + 600_000 }),
      complete: async () => { completed += 1; },
      unlink: async () => undefined,
    },
  };
  return { ports, service: new ProviderIdentities(ports), switchOwner: (value: string | null) => { owner = value; }, intent: () => pending, completed: () => completed };
}

test('a provider callback links without exchanging or replacing the current application session', async () => {
  const f = fixture();
  const intent = await f.service.begin('apple');
  assert.equal(intent.owner, 'owner');
  assert.equal(f.intent()?.id, intent.id);
  await f.service.complete('signed-provider-proof');
  assert.equal(f.completed(), 1);
  assert.equal(f.intent(), null);
  assert.equal(f.ports.currentOwner(), 'owner');
});

test('account replacement after authorization cannot attach the provider to the new account', async () => {
  const f = fixture();
  await f.service.begin('apple');
  f.switchOwner('replacement');
  await assert.rejects(f.service.complete('proof'), /account_changed/);
  assert.equal(f.completed(), 0);
});

test('changing accounts while obtaining a challenge does not persist an intent for the wrong account', async () => {
  const f = fixture();
  const begin = f.ports.remote.begin;
  f.ports.remote.begin = async provider => { const challenge = await begin(provider); f.switchOwner('replacement'); return challenge; };
  await assert.rejects(f.service.begin('apple'), /account_changed/);
  assert.equal(f.intent(), null);
});

test('expired or cancelled authorization cannot be submitted', async () => {
  const f = fixture();
  const intent = await f.service.begin('apple');
  f.ports.now = () => intent.expiresAt;
  await assert.rejects(f.service.complete('proof'), /identity_link_expired/);
  assert.equal(f.completed(), 0);
  await f.service.cancel(intent);
  await assert.rejects(f.service.complete('proof'), /identity_link_missing/);
});

test('a failed completion retains the same bounded intent and never changes session ownership', async () => {
  const f = fixture();
  const intent = await f.service.begin('apple');
  f.ports.remote.complete = async () => { throw new Error('offline'); };
  await assert.rejects(f.service.complete('proof'), /offline/);
  assert.deepEqual(f.intent(), intent);
  assert.equal(f.ports.currentOwner(), 'owner');
});


test('an old provider callback cannot consume a newer local intent', async () => {
  const f = fixture();
  await f.service.begin('apple');
  await assert.rejects(f.service.complete('old-proof', { owner: 'owner', id: 'earlier-challenge' }), /identity_link_superseded/);
  assert.equal(f.completed(), 0);
  assert.equal(f.intent()?.id, 'challenge');
});


test('ordinary sign-in requests provider metadata only from an issuer that supports it', () => {
  assert.deepEqual(providerSignInScopes(['openid', 'profile', 'email']), ['openid', 'profile', 'email']);
  assert.deepEqual(providerSignInScopes(['openid', 'profile', 'email', 'identities']), ['openid', 'profile', 'email', 'identities']);
});
