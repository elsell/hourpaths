import assert from 'node:assert/strict';
import test from 'node:test';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from '@hourpaths/client-core';
import { retainedTimers } from './studio/offline/adapters/retained-timers';

test('Studio retained controls durably stop only the retained account and disappear on replacement', async () => {
  const rows = new Map<string, TrackingSnapshot>();
  const store: TrackingStore = {
    read: async owner => structuredClone(rows.get(owner) ?? null),
    commit: async (owner, revision, next) => {
      if ((rows.get(owner)?.revision ?? 0) !== revision) return false;
      rows.set(owner, structuredClone(next)); return true;
    },
  };
  let now = Date.parse('2026-10-01T12:00:00Z'), sequence = 0;
  const ids = () => `id-${++sequence}`;
  const active = new OfflineTracking(store, 'alice', () => now, ids);
  await active.retainPaths([{ id: 'guitar', name: 'guitar', timeZone: 'utc' }]);
  const timer = await active.start('guitar'); active.dispose();
  let owner: string | null = 'alice'; now += 60000;
  const paused = retainedTimers('alice', store, () => owner === 'alice', () => now, ids);
  assert.equal((await paused.snapshot()).timers[0]?.id, timer.id);
  assert.equal('start' in paused, false);
  assert.equal('replay' in paused, false);
  const stopped = await paused.stop(timer.id);
  assert.equal(stopped.timers.length, 0);
  assert.equal(stopped.pending, true);
  assert.equal(rows.get('alice')?.operations.at(-1)?.endedAt, new Date(now).toISOString());
  owner = 'bob';
  await assert.rejects(paused.snapshot(), /account_changed/);
  await assert.rejects(paused.stop(timer.id), /account_changed/);
  assert.equal(rows.has('bob'), false);
  paused.dispose();
});
