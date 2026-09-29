import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createProviderLogout, validProviderLogoutReturn } from './provider-logout';
test('canceled logout survives a new controller and gates subsequent authorization', async () => {
  let pending = false;
  let completes = false;
  let calls = 0;
  const ports = { pending: async () => pending, markPending: async () => { pending = true; }, clearPending: async () => { pending = false; }, endSession: async () => { calls++; return completes; } };
  assert.equal(await createProviderLogout(ports).signOut(), false);
  assert.equal(pending, true);
  const restarted = createProviderLogout(ports);
  assert.equal(await restarted.beforeSignIn(), false);
  completes = true;
  assert.equal(await restarted.beforeSignIn(), true);
  assert.equal(pending, false);
  assert.equal(calls, 3);
});
test('network and storage failures never authorize sign-in past pending logout', async () => {
  for (const fails of ['pending', 'mark', 'end', 'clear']) {
    const fail = () => { throw new Error('unavailable'); };
    const flow = createProviderLogout({ pending: async () => fails === 'pending' ? fail() : true, markPending: async () => { if (fails === 'mark') fail(); }, clearPending: async () => { if (fails === 'clear') fail(); }, endSession: async () => fails === 'end' ? fail() : true });
    assert.equal(await flow.beforeSignIn(), false);
  }
});
test('logout callbacks require exact destination, state, and no authorization payload', () => {
  const uri = 'hourpaths://logout';
  assert.equal(validProviderLogoutReturn(uri + '?state=nonce', uri, 'nonce'), true);
  for (const url of ['hourpaths://logout-other?state=nonce', 'hourpaths://callback?state=nonce', 'hourpaths://logout?state=wrong', 'hourpaths://logout?state=nonce&state=nonce', 'hourpaths://logout?state=nonce&error=denied', 'hourpaths://logout?state=nonce&code=code', 'https://logout?state=nonce']) assert.equal(validProviderLogoutReturn(url, uri, 'nonce'), false);
});
test('concurrent attempts share one browser operation and a failed attempt can retry', async () => {
  let calls = 0;
  let finish!: (value: boolean) => void;
  const flow = createProviderLogout({ pending: async () => true, markPending: async () => undefined, clearPending: async () => undefined, endSession: () => { calls++; return new Promise(resolve => { finish = resolve; }); } });
  const first = flow.signOut();
  const duplicate = flow.signOut();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(calls, 1);
  finish(false);
  assert.deepEqual(await Promise.all([first, duplicate]), [false, false]);
  const retry = flow.signOut();
  await new Promise(resolve => setImmediate(resolve));
  finish(true);
  assert.equal(await retry, true);
  assert.equal(calls, 2);
});
