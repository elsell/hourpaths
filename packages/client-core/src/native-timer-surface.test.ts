import assert from 'node:assert/strict';
import test from 'node:test';
import { NativeTimerSurfaceCoordinator, type RunningTimerSurface } from './native-timer-surface';

const timers: RunningTimerSurface[] = [
  { id: 'one', pathId: 'guitar', name: 'Guitar', startedAt: 1000 },
  { id: 'two', pathId: 'reading', name: 'Reading', startedAt: 2000 },
];

test('one replacement represents concurrent timers; removing the last clears the surface', async () => {
  let visible: readonly RunningTimerSurface[] = [];
  const coordinator = new NativeTimerSurfaceCoordinator({
    clear: async () => { visible = []; }, replace: async value => { visible = value; },
  });
  await coordinator.account('alice');
  await coordinator.publish('alice', timers);
  assert.deepEqual(visible, timers);
  await coordinator.publish('alice', timers.slice(1));
  assert.deepEqual(visible, timers.slice(1));
  await coordinator.publish('alice', []);
  assert.deepEqual(visible, []);
});

test('logout and account replacement clear an in-flight old-account presentation before the new one', async () => {
  let release!: () => void;
  let entered!: () => void;
  const started = new Promise<void>(resolve => { entered = resolve; });
  const pending = new Promise<void>(resolve => { release = resolve; });
  let visible: readonly RunningTimerSurface[] = [];
  const coordinator = new NativeTimerSurfaceCoordinator({
    clear: async () => { visible = []; },
    replace: async value => { if (value[0]?.id === 'one') { entered(); await pending; } visible = value; },
  });
  await coordinator.account('alice');
  const old = coordinator.publish('alice', timers);
  await started;
  const clear = coordinator.account(null);
  const adopt = coordinator.account('bob');
  const late = coordinator.publish('alice', timers);
  const next = coordinator.publish('bob', [{ ...timers[1], name: 'Bob’s reading' }]);
  release();
  await Promise.all([old, clear, adopt, late, next]);
  assert.deepEqual(visible, [{ ...timers[1], name: 'Bob’s reading' }]);
  await coordinator.account(null);
  assert.deepEqual(visible, []);
});

test('denied or unsupported presentation does not poison later reconciliation', async () => {
  let denied = true;
  let visible: readonly RunningTimerSurface[] = [];
  const coordinator = new NativeTimerSurfaceCoordinator({
    clear: async () => { visible = []; },
    replace: async value => { if (denied) throw new Error('permission_denied'); visible = value; },
  });
  await coordinator.account('alice');
  assert.equal(await coordinator.publish('alice', timers), false);
  denied = false;
  assert.equal(await coordinator.publish('alice', timers), true);
  assert.deepEqual(visible, timers);
});

test('account replacement during cleanup never presents the previous account again', async () => {
  let release!: () => void;
  let entered!: () => void;
  const started = new Promise<void>(resolve => { entered = resolve; });
  const pending = new Promise<void>(resolve => { release = resolve; });
  let clears = 0;
  const presentations: string[] = [];
  const coordinator = new NativeTimerSurfaceCoordinator({
    clear: async () => {
      if (++clears === 1) throw new Error('temporary_cleanup_failure');
      if (clears === 2) { entered(); await pending; }
    },
    replace: async value => { presentations.push(value[0].name); },
  });
  assert.equal(await coordinator.account('alice'), false);
  const old = coordinator.publish('alice', timers);
  await started;
  const adopt = coordinator.account('bob');
  const next = coordinator.publish('bob', [{ ...timers[1], name: 'Bob’s reading' }]);
  release();
  await Promise.all([old, adopt, next]);
  assert.deepEqual(presentations, ['Bob’s reading']);
});

test('account action revocation is independent of presentation failure and rejects queued old publications', async () => {
  let revoked = 0;
  const surface = new NativeTimerSurfaceCoordinator({
    clear: async () => { throw new Error('presentation denied'); },
    revokeActions: async () => { revoked++; },
    replace: async () => { throw new Error('old account must not publish'); },
  });
  await surface.account('alice');
  assert.equal(await surface.revokeAccount(), true);
  assert.equal(revoked, 1);
  assert.equal(await surface.publish('alice', []), false);
  const unavailable = new NativeTimerSurfaceCoordinator({
    clear: async () => undefined, replace: async () => undefined,
    revokeActions: async () => { throw new Error('durable storage unavailable'); },
  });
  assert.equal(await unavailable.revokeAccount(), false);
});


test('empty timer publication reaches adapter reconciliation so background restart controls survive', async () => {
  let retained = false;
  const coordinator = new NativeTimerSurfaceCoordinator({
    clear: async () => { retained = false; },
    replace: async () => { retained = true; },
  });
  await coordinator.account('alice');
  await coordinator.publish('alice', timers);
  await coordinator.publish('alice', []);
  assert.equal(retained, true);
  await coordinator.account(null);
  assert.equal(retained, false);
});


test('sign-out pause prevents refreshed projections from rebinding revoked native controls', async () => {
  let replacements = 0;
  const coordinator = new NativeTimerSurfaceCoordinator({
    clear: async () => {}, revokeActions: async () => {},
    replace: async () => { replacements++; },
  });
  await coordinator.account('alice');
  const resume = coordinator.pausePublications();
  await coordinator.revokeAccount();
  assert.equal(await coordinator.account('alice'), false);
  assert.equal(await coordinator.publish('alice', timers), false);
  assert.equal(replacements, 0);
  resume(); resume();
  await coordinator.account('alice');
  assert.equal(await coordinator.publish('alice', timers), true);
  assert.equal(replacements, 1);
});

test('transient native publication retries without another timer edit, and revocation cancels retry', async () => {
  let retry: (() => void) | undefined;
  let attempts = 0;
  let visible: readonly RunningTimerSurface[] = [];
  let reject = true;
  const coordinator = new NativeTimerSurfaceCoordinator({
    clear: async () => { visible = []; },
    replace: async value => { attempts++; if (reject) throw new Error('stale_local_revision'); visible = value; },
  }, work => { retry = work; return () => { retry = undefined; }; });
  await coordinator.account('alice');
  assert.equal(await coordinator.publish('alice', timers), false);
  assert.ok(retry);
  reject = false;
  retry!();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(visible, timers);
  reject = true;
  await coordinator.publish('alice', timers);
  const delayed = retry!;
  await coordinator.revokeAccount();
  const previousAttempts = attempts;
  delayed();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(attempts, previousAttempts);
});


test('first restored owner preserves verified controls; replacement and signout still clear', async () => {
  const events: string[] = [];
  const surface = new NativeTimerSurfaceCoordinator({
    restoreAccount: async owner => { events.push(`restore:${owner}`); return owner === 'alice'; },
    clear: async () => { events.push('clear'); }, replace: async () => {},
  });
  await surface.account('alice');
  assert.deepEqual(events, ['restore:alice']);
  await surface.account('bob');
  await surface.account(null);
  assert.deepEqual(events, ['restore:alice', 'clear', 'clear']);
});

test('unverified or failed restoration clears old controls before publishing', async () => {
  for (const fails of [false, true]) {
    const events: string[] = [];
    const surface = new NativeTimerSurfaceCoordinator({
      restoreAccount: async () => { if (fails) throw new Error('storage unavailable'); return false; },
      clear: async () => { events.push('clear'); }, replace: async () => { events.push('replace'); },
    });
    assert.equal(await surface.account('alice'), !fails);
    assert.deepEqual(events, fails ? [] : ['clear']);
    await surface.publish('alice', timers);
    assert.deepEqual(events, ['clear', 'replace']);
  }
});


test('repeated same-owner publications share pending restoration without revoking its lease', async () => {
  let resolve!: (value: boolean) => void;
  let entered!: () => void;
  const started = new Promise<void>(done => { entered = done; });
  const pending = new Promise<boolean>(done => { resolve = done; });
  const events: string[] = [];
  const surface = new NativeTimerSurfaceCoordinator({
    restoreAccount: async () => { entered(); return pending; },
    clear: async () => { events.push('clear'); }, replace: async () => { events.push('replace'); },
  });
  const first = surface.account('alice');
  await started;
  const duplicate = surface.account('alice');
  const publication = surface.publish('alice', timers);
  resolve(true);
  await Promise.all([first, duplicate, publication]);
  assert.deepEqual(events, ['replace']);
});
