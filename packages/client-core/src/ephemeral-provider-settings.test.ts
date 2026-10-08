import test from 'node:test';
import assert from 'node:assert/strict';
import { ephemeralProviderSettings } from './ephemeral-provider-settings';
import type { IdentityProvider } from './provider-identities';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(done => { resolve = done; });
  return { promise, resolve };
}
function fixture() {
  let current = { owner: 'owner', token: 'first-session' };
  const authenticated = deferred<void>();
  const proof = deferred<string | null>();
  const completed: string[] = [];
  let count = 0;
  let holdChallenge = false;
  const challengeReady = deferred<void>();
  const challengeRelease = deferred<void>();
  const service = ephemeralProviderSettings({
    current: () => current,
    now: () => 100,
    remote: token => ({
      list: async () => [{ provider: 'google', canUnlink: false }],
      unlink: async () => undefined,
      begin: async (provider: IdentityProvider) => {
        const id = String(++count);
        if (holdChallenge) { challengeReady.resolve(); await challengeRelease.promise; }
        return { id, provider, nonce: 'hourpaths-link:' + 'a'.repeat(64), expiresAt: 1000 };
      },
      complete: async () => { completed.push(token); },
    }),
    authenticate: async () => { authenticated.resolve(); return proof.promise; },
  });
  return { service, authenticated, proof, completed, challengeReady, challengeRelease,
    replace: (owner = 'owner') => { service.invalidate(); current = { owner, token: 'new-session' }; },
    rotate: () => { current = { ...current, token: 'rotated-session' }; },
    holdChallenge: () => { holdChallenge = true; },
  };
}

test('same-account reauthentication discards a held proof and permits a fresh link', async () => {
  const f = fixture();
  const old = f.service.link('apple');
  const rejected = assert.rejects(old, /account_changed/);
  await f.authenticated.promise;
  f.replace();
  const fresh = f.service.link('apple');
  f.proof.resolve('signed-proof');
  await rejected;
  await fresh;
  assert.deepEqual(f.completed, ['new-session']);
});

test('an invalidated challenge cannot open provider authentication or submit a proof', async () => {
  const f = fixture();
  f.holdChallenge();
  const old = f.service.link('apple');
  const rejected = assert.rejects(old, /account_changed/);
  await f.challengeReady.promise;
  f.replace('replacement');
  f.challengeRelease.resolve();
  await rejected;
  assert.deepEqual(f.completed, []);
});

test('routine same-account rotation preserves the attempt and dispatches with the rotated credential', async () => {
  const f = fixture();
  const work = f.service.link('apple');
  await f.authenticated.promise;
  f.rotate();
  f.proof.resolve('signed-proof');
  await work;
  assert.deepEqual(f.completed, ['rotated-session']);
});

test('dismissing provider authentication sends no link mutation and releases admission', async () => {
  const f = fixture();
  const work = f.service.link('apple');
  await f.authenticated.promise;
  f.proof.resolve(null);
  await work;
  await f.service.link('apple');
  assert.deepEqual(f.completed, []);
});
