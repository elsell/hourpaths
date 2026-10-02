const utcTimeZone = 'utc';
import assert from 'node:assert/strict';
import test from 'node:test';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from '@hourpaths/client-core';
import { durableHistoryRepository } from './studio/offline/adapters/durable-history-repository';
import type { HistoryRepository } from './studio/history/ports/history-source';
import type { HistoryCursor, RecordedActivity } from './studio/history/domain/activity';

const now = Date.parse('2026-10-02T12:00:00Z');
function setup() {
  let saved: TrackingSnapshot | null = null;
  let clock = now;
  let id = 0;
  const store: TrackingStore = {
    read: async owner => saved?.owner === owner ? structuredClone(saved) : null,
    commit: async (owner, revision, next) => {
      assert.equal(next.owner, owner);
      if ((saved?.revision ?? 0) !== revision) return false;
      saved = structuredClone(next); return true;
    },
  };
  const tracking = new OfflineTracking(store, 'alice', () => clock, () => `local-${++id}`);
  return { tracking, store, tick: () => { clock += 60000; }, runtime: async () => ({ owner: 'alice', tracking, timeZone: utcTimeZone, assertCurrent() {}, wake() {} }) };
}
const entry = (id: string, instant = now - 120000): RecordedActivity => ({ id, pathId: 'path', pathName: 'guitar', startedAt: instant, endedAt: instant + 60000, seconds: 60, timeZone: utcTimeZone });
const cursor: HistoryCursor = { participantId: 'alice', streams: [] };

test('fully hydrated history survives restart, keeps original ends and names, and includes pending stops', async () => {
  const env = setup();
  let offline = false;
  const remote: HistoryRepository = { page: async value => {
    if (offline) throw new TypeError('network');
    return value ? { items: [entry('second', now - 240000)], next: null } : { items: [entry('first')], next: cursor };
  } };
  const history = durableHistoryRepository(remote, env.runtime, failure => failure instanceof TypeError, 2, () => now);
  assert.equal(await history.refresh(), true);
  await env.tracking.retainPaths([{ id: 'path', name: 'guitar', timeZone: utcTimeZone }]);
  const timer = await env.tracking.start('path'); env.tick(); await env.tracking.stop(timer.id);
  offline = true;
  const restored = new OfflineTracking(env.store, 'alice', () => now + 60000, () => 'unused');
  const local = durableHistoryRepository(remote, async () => ({ ...await env.runtime(), tracking: restored }), failure => failure instanceof TypeError, 2, () => now);
  const first = await local.page(null);
  assert.equal(first.retained, true);
  assert.equal(first.incomplete, false);
  assert.equal(first.items[0]?.pending, true);
  assert.equal(first.items[1]?.pathName, 'guitar');
  assert.equal(first.items[1]?.endedAt, entry('first').endedAt);
  assert.ok(first.next);
  assert.deepEqual((await local.page(first.next)).items.map(value => value.id), ['second']);
});

test('partial pagination and concurrent local commands cannot replace a retained snapshot', async () => {
  const env = setup();
  await env.tracking.retainHistory([{ id: 'saved', owner: 'alice', pathId: 'path', pathName: 'guitar', startedAt: new Date(now - 120000).toISOString(), endedAt: new Date(now - 60000).toISOString(), timeZone: utcTimeZone }], 0);
  let mode = 'partial';
  const remote: HistoryRepository = { page: async value => {
    if (value && mode === 'partial') throw new TypeError('network');
    if (value) await env.tracking.retainPaths([{ id: 'path', name: 'guitar', timeZone: utcTimeZone }]);
    return { items: [entry(value ? 'second' : 'first')], next: value ? null : cursor };
  } };
  const history = durableHistoryRepository(remote, env.runtime, failure => failure instanceof TypeError, 25, () => now);
  assert.equal(await history.refresh(), false);
  assert.equal((await env.tracking.snapshot()).history[0]?.id, 'saved');
  mode = 'changed';
  assert.equal(await history.refresh(), false);
  assert.equal((await env.tracking.snapshot()).history[0]?.id, 'saved');
});

test('permanent denial never falls back to retained entries and foreign cursors are rejected', async () => {
  const env = setup();
  const denied = new Error('denied');
  const history = durableHistoryRepository({ page: async () => { throw denied; } }, env.runtime, failure => failure instanceof TypeError, 25, () => now);
  await assert.rejects(history.page(null), error => error === denied);
  await assert.rejects(history.page({ participantId: 'bob', streams: [], retained: true }), /owner/);
});
