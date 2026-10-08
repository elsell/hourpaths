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
