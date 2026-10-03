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

test('maintenance retries temporary failure without losing the session and stops refreshing at absolute expiry', async () => {
  let now = 950_000, stored: Session | null = initial, calls = 0;
  const controller = new SessionController(initial, { read: () => stored, write: value => { stored = value; }, clear: () => { stored = null; } }, {
    refresh: async value => { calls++; if (calls === 1) throw new SessionUnavailable(true); return { ...value, token: 'rotated' }; }, revoke: async () => undefined,
  }, () => now, () => undefined);
  await controller.maintain(); assert.equal(controller.token(), 'original');
  now += 10_000; await controller.maintain(); assert.equal(calls, 1);
  now += 20_000; await controller.maintain(); assert.equal(controller.token(), 'rotated');
  now += 1000; await controller.maintain(); assert.equal(calls, 2);
  now = initial.expiresAt; await controller.maintain(); assert.equal(stored, null);
});

test('offline owner binding is retained with the session and a late profile cannot bind a replacement account', async () => {
  let stored: Session | null = initial;
  const controller = new SessionController(initial, { read: () => stored, write: value => { stored = value; }, clear: () => { stored = null; } }, {
    refresh: async value => value, revoke: async () => undefined,
  }, () => 0, () => undefined);
  assert.equal(await controller.bindOwner('original', 'alice'), true);
  assert.equal(controller.owner(), 'alice');
  assert.equal(stored?.ownerId, 'alice');
  stored = { ...initial, token: 'replacement-account', ownerId: 'bob' };
  assert.equal(await controller.bindOwner('original', 'alice'), false);
  assert.equal(controller.owner(), null);
  assert.equal(stored.ownerId, 'bob');
});

test('rejection atomically replaces a verified credential with a retained account reference', () => {
  let stored: Session | null = { ...initial, ownerId: 'alice' };
  let retained: string | null = null;
  const controller = new SessionController(stored, {
    read: () => stored, write: value => { stored = value; }, clear: () => { stored = null; retained = null; },
    pause: owner => { stored = null; retained = owner; }, retainedOwner: () => retained,
  }, { refresh: async value => value, revoke: async () => undefined }, () => 0, () => undefined);
  controller.reject(initial.token);
  assert.equal(controller.token(), null);
  assert.equal(stored, null);
  assert.equal(retained, 'alice');
});

test('foreground expiry retains only the verified owner for local Stop and never refreshes', async () => {
  let stored: Session | null = { ...initial, ownerId: 'alice' };
  let retained: string | null = null;
  let now = 0;
  const controller = new SessionController(stored, {
    read: () => stored, write: value => { stored = value; },
    clear: () => { stored = null; retained = null; },
    pause: owner => { stored = null; retained = owner; }, retainedOwner: () => retained,
  }, { refresh: async () => {
      assert.fail('expired session must not refresh');
    }, revoke: async () => undefined }, () => now, () => undefined);
  now = initial.expiresAt;
  await controller.maintain();
  assert.equal(controller.token(), null);
  assert.equal(controller.owner(), null);
  assert.equal(stored, null);
  assert.equal(retained, 'alice');
});

test('two tabs serialize token rotation and the losing controller never revokes the replacement', async () => {
  let stored: Session | null = initial;
  let tail: Promise<unknown> = Promise.resolve();
  let finish!: (value: Session) => void;
  let calls = 0, remounts = 0;
  const revoked: string[] = [];
  const store = {
    read: () => stored,
    write: (value: Session) => { stored = value; },
    clear: () => { stored = null; },
    refreshExclusive<T>(operation: () => Promise<T>): Promise<T> {
      const result = tail.then(operation);
      tail = result.catch(() => undefined);
      return result;
    },
  };
  const service = {
    refresh: async () => { calls++; return new Promise<Session>(resolve => { finish = resolve; }); },
    revoke: async (value: Session) => { revoked.push(value.token); },
  };
  const first = new SessionController(initial, store, service, () => 0, () => { remounts++; });
  const second = new SessionController(initial, store, service, () => 0, () => { remounts++; });
  const pending = [first.refresh(), second.refresh()];
  await Promise.resolve();
  finish({ ...initial, token: 'rotated' });
  const result = await Promise.allSettled(pending);
  assert.deepEqual(result.map(value => value.status), ['fulfilled', 'rejected']);
  assert.equal(calls, 1);
  assert.equal(stored?.token, 'rotated');
  assert.equal(first.token(), 'rotated');
  assert.equal(second.token(), null);
  assert.equal(remounts, 1);
  assert.deepEqual(revoked, []);
});
