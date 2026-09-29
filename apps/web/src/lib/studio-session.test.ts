import assert from 'node:assert/strict';
import { test } from 'node:test';
import { SessionController } from './studio/session/application/session-controller';
import { SessionUnavailable, type Session } from './studio/session/domain/session';
const initial: Session = { token: 'original', expiresAt: 1_000_000, destination: 'home' };
test('sign-out invalidates a late refresh and never persists its credential', async () => {
  let stored: Session | null = initial;
  let resolve!: (value: Session) => void;
  const revoked: string[] = [];
  const controller = new SessionController(initial, { read: () => stored, write: value => { stored = value; }, clear: () => { stored = null; } }, {
    refresh: () => new Promise(done => { resolve = done; }),
    revoke: async value => { revoked.push(value.token); },
  }, () => 0, () => undefined);
  const pending = controller.refresh();
  controller.signOut();
  resolve({ ...initial, token: 'replacement' });
  await assert.rejects(pending);
  assert.equal(stored, null);
  assert.deepEqual(revoked.sort(), ['original', 'replacement']);
});
test('temporary refresh failure retains a valid credential and refreshes are single-flight', async () => {
  let calls = 0;
  const controller = new SessionController(initial, { read: () => initial, write: () => undefined, clear: () => assert.fail('credential_cleared') }, {
    refresh: async () => { calls++; throw new SessionUnavailable(true); }, revoke: async () => undefined,
  }, () => 0, () => undefined);
  await Promise.allSettled([controller.refresh(), controller.refresh()]);
  assert.equal(calls, 1);
  assert.equal(controller.token(), 'original');
});
test('replacement account is neither exposed through the old controller nor cleared by it', () => {
  let stored: Session | null = initial;
  let lost = 0;
  const controller = new SessionController(initial, { read: () => stored, write: value => { stored = value; }, clear: () => { stored = null; } }, {
    refresh: async value => value, revoke: async () => undefined,
  }, () => 0, () => { lost++; });
  stored = { ...initial, token: 'another-account' };
  assert.equal(controller.token(), null);
  controller.signOut();
  assert.equal(stored?.token, 'another-account');
  assert.equal(lost, 1);
});
test('an expired credential is discarded before any authenticated request', () => {
  let stored: Session | null = initial;
  const controller = new SessionController(initial, { read: () => stored, write: value => { stored = value; }, clear: () => { stored = null; } }, {
    refresh: async value => value, revoke: async () => undefined,
  }, () => initial.expiresAt, () => undefined);
  assert.equal(controller.token(), null);
  assert.equal(stored, null);
});
