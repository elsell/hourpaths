import test from 'node:test';
import assert from 'node:assert/strict';
import { PolicyTimersController } from './policy-timers';

const timer = { id: 'remote', pathId: 'guitar', name: 'Guitar', startedAt: '2026-10-09T00:00:00Z' };
test('policy timer controls keep device stops durable and hide their server counterpart until replay', async () => {
  let stopped = false;
  const controller = new PolicyTimersController({ list: async () => ({ timers: [timer], nextCursor: '' }), stop: async () => { throw new Error('must not bypass durable stop'); } }, () => 'key', {
    snapshot: async () => ({ timers: stopped ? [] : [{ ...timer, id: 'local', serverId: 'remote' }], pendingStops: stopped ? [{ id: 'local', serverId: 'remote' }] : [] }),
    stop: async value => { assert.equal(value.id, 'local'); stopped = true; },
  });
  await controller.refresh();
  assert.equal(controller.state.timers.length, 1);
  assert.equal(controller.state.timers[0].source, 'device');
  await controller.stop('local', 'device');
  assert.equal(controller.state.timers.length, 0);
  assert.equal(controller.state.pending, true);
});
test('ambiguous remote stops retry the reviewed timer with the same key and disposed results stay private', async () => {
  const keys: string[] = [];
  let first = true, resolve!: () => void;
  const controller = new PolicyTimersController({ list: async () => ({ timers: [timer], nextCursor: '' }), stop: async (value, key) => {
    assert.equal(value.id, timer.id); keys.push(key);
    if (first) { first = false; throw new Error('response lost'); }
    await new Promise<void>(done => { resolve = done; });
  } }, () => 'stable-key');
  await controller.refresh(); await controller.stop(timer.id, 'server');
  assert.equal(controller.state.error, true); assert.equal(controller.state.timers.length, 1);
  const retry = controller.stop(timer.id, 'server'); controller.dispose(); resolve(); await retry;
  assert.deepEqual(keys, ['stable-key', 'stable-key']);
  assert.equal(controller.state.timers.length, 1);
});

test('policy device controls persist the stop through the durable tracking port and reject an account replacement', async () => {
  const { OfflineTracking } = await import('./offline-tracking');
  const { retainedPolicyTimers } = await import('./policy-timers');
  const rows = new Map<string, import('./offline-tracking').TrackingSnapshot>();
  const store: import('./offline-tracking').TrackingStore = {
    read: async owner => structuredClone(rows.get(owner) ?? null),
    commit: async (owner, revision, next) => {
      if ((rows.get(owner)?.revision ?? 0) !== revision) return false;
      rows.set(owner, structuredClone(next)); return true;
    },
  };
  let now = Date.parse(timer.startedAt), sequence = 0, owner = 'alice';
  const id = () => 'local-' + ++sequence;
  const tracking = new OfflineTracking(store, owner, () => now, id);
  await tracking.retainPaths([{ id: timer.pathId, name: timer.name, timeZone: 'Etc/UTC' }]);
  const active = await tracking.start(timer.pathId);
  const controls = retainedPolicyTimers(async () => store, () => owner, () => true, () => now, id);
  const before = await controls.snapshot();
  now += 60000;
  await controls.stop({ ...before.timers[0], source: 'device' });
  assert.equal((await controls.snapshot()).pendingStops[0].id, active.id);
  assert.equal(rows.get('alice')?.operations.at(-1)?.endedAt, new Date(now).toISOString());
  owner = 'bob';
  await assert.rejects(controls.snapshot(), /owner_changed/);
  await assert.rejects(controls.stop({ ...before.timers[0], source: 'device' }), /owner_changed/);
  assert.equal(rows.has('bob'), false);
});
