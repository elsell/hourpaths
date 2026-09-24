import assert from 'node:assert/strict';
import test from 'node:test';
import { applyRefreshedMobilePath, type MobileHomeProfile } from './session-destination';

const active = { id: 'path-1', name: 'path-one' } as MobileHomeProfile['paths'][number];
const archived = { archivedAt: '2026-08-08T00:00:00Z', id: 'path-2', name: 'path-two' } as MobileHomeProfile['paths'][number];
const profile = {
  archivedPaths: [archived],
  paths: [active],
  timers: { 'path-1': { running: false }, 'path-2': { running: false } },
} as unknown as MobileHomeProfile;

test('authoritative refresh moves Paths between active and archived projections', () => {
  const nowArchived = { ...active, archivedAt: '2026-08-08T01:00:00Z' };
  const moved = applyRefreshedMobilePath(profile, 'path-1', nowArchived);
  assert.deepEqual(moved.paths, []);
  assert.deepEqual(moved.archivedPaths, [archived, nowArchived]);
  assert.deepEqual(moved.timers, { 'path-2': { running: false } });
});

test('authoritative not-found evicts stale private Path data and timer state', () => {
  const removed = applyRefreshedMobilePath(profile, 'path-1', null);
  assert.deepEqual(removed.paths, []);
  assert.deepEqual(removed.archivedPaths, [archived]);
  assert.deepEqual(removed.timers, { 'path-2': { running: false } });
});
