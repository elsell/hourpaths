const utcTimeZone = 'utc';
import assert from 'node:assert/strict';
import test from 'node:test';
import { createServer } from 'node:http';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from '@hourpaths/client-core';
import { apiPathRepository, PathRequestError } from './studio/paths/adapters/api-path-repository';
import { durablePathRepository } from './studio/offline/adapters/durable-path-repository';
import type { Path } from './studio/paths/domain/path';

test('retained Home readers expose rate limiting immediately instead of waiting through network retries', async context => {
  let reads = 0;
  const server = createServer((_request, response) => {
    reads++;
    response.writeHead(429, { 'Content-Type': 'application/problem+json', 'Retry-After': '30' });
    response.end(JSON.stringify({ code: 'rate_limited' }));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  context.after(() => new Promise<void>(resolve => { server.closeAllConnections(); server.close(() => resolve()); }));
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const reader = apiPathRepository(`http://127.0.0.1:${address.port}`, () => 'credential', () => undefined, false);
  await assert.rejects(reader.list(false, AbortSignal.timeout(1000)), error => error instanceof PathRequestError && error.status === 429);
  assert.equal(reads, 1);
});

test('Studio offline cache cannot expose tracking for a confirmed inaccessible Path', async context => {
  const server = createServer((_request, response) => {
    response.writeHead(503, { 'Content-Type': 'application/problem+json' });
    response.end(JSON.stringify({ code: 'temporarily_unavailable' }));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  context.after(() => new Promise<void>(resolve => { server.closeAllConnections(); server.close(() => resolve()); }));
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  let row: TrackingSnapshot | null = null;
  const store: TrackingStore = {
    read: async () => structuredClone(row),
    commit: async (owner, revision, next) => {
      if (owner !== 'alice' || (row?.revision ?? 0) !== revision) return false;
      row = structuredClone(next); return true;
    },
  };
  let sequence = 0;
  const tracking = new OfflineTracking(store, 'alice', () => Date.parse('2026-10-01T12:00:00Z'), () => `web-${++sequence}`);
  const path: Path = { id: 'guitar', name: 'Guitar', visibility: 'private', archived: false, pinned: false,
    position: null, pinnedPosition: null, recentActivityAt: 0, goal: null, overallTarget: null,
    canTrack: true, canEdit: true, canManageGoals: true, canManageLifecycle: true,
    canManageVisibility: true, canTransferOwnership: true, canLeave: true, canInvite: true };
  await tracking.retainPaths([{ id: path.id, name: path.name, timeZone: utcTimeZone }]);
  await tracking.start(path.id);
  await tracking.replay({ send: async () => ({ kind: 'rejected', reason: 'deleted', disclosePath: false }) });
  const repository = durablePathRepository(apiPathRepository(`http://127.0.0.1:${address.port}`, () => 'session', () => {}), {
    readHome: async () => ({ owner: 'alice', paths: [path], appearances: {} }), saveHome: async () => {}, savePaths: async () => {}, saveAppearance: async () => {},
  }, async () => ({ owner: 'alice', timeZone: utcTimeZone, tracking, assertCurrent: () => {}, wake: () => {} }),
  error => error instanceof PathRequestError && error.status === 503);
  assert.deepEqual(await repository.list(false), []);
  await assert.rejects(repository.read(path.id));
  await assert.rejects(repository.start(path.id, 'new-start'));
  assert.equal((await tracking.snapshot()).operations.length, 0);
});

test('Studio Home hydrates hidden Paths before offline restart and preserves its complete cache when one read fails', async context => {
  let offline = false, failSecond = false, sequence = 0, interruptHydration = false, savedSeconds = 60;
  const requests: string[] = [];
  const availability: boolean[] = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url!, 'http://localhost').pathname; requests.push(url);
    if (offline || failSecond && url === '/v1/paths/reading/timer') { response.writeHead(503); response.end('{}'); return; }
    if (interruptHydration && url === '/v1/paths/reading/timer' && row) {
      row = { ...row, revision: row.revision + 1 }; interruptHydration = false;
    }
    let body: unknown;
    if (url === '/v1/paths') body = { data: ['guitar', 'reading'].map((id, position) => ({ id, name: id, visibility: 'private', home: { manualPosition: position },
      intervalGoal: { targetSeconds: 1800, recurrence: 'daily', alignment: { hour: 0 } },
      capabilities: { trackTime: true, renamePath: true, manageGoals: true, manageLifecycle: true } })),
      meta: { homePreferences: { orderMethod: 'manual' } } };
    else if (url.endsWith('/timer')) body = { data: { accumulatedSeconds: savedSeconds, running: false, intervalProgress: { accumulatedSeconds: savedSeconds, targetSeconds: 1800,
      startedAt: '2026-10-01T00:00:00Z', endedAt: '2026-10-02T00:00:00Z' } } };
    else if (url.startsWith('/v1/me/path-appearances/')) body = { data: { revision: 2, color: 'mint', emoji: '🎸' } };
    else { response.writeHead(404); response.end('{}'); return; }
    response.writeHead(200, { 'Content-Type': 'application/json' }); response.end(JSON.stringify(body));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  context.after(() => new Promise<void>(resolve => { server.closeAllConnections(); server.close(() => resolve()); }));
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  let row: TrackingSnapshot | null = null;
  let home: import('./studio/offline/ports/home-cache').CachedHome | null = null;
  const store: TrackingStore = { read: async () => structuredClone(row), commit: async (_owner, revision, next) => {
    if ((row?.revision ?? 0) !== revision) return false; row = structuredClone(next); return true;
  } };
  const cache: import('./studio/offline/ports/home-cache').HomeCache = {
    readHome: async () => structuredClone(home),
    saveHome: async (owner, paths, appearances) => { home = structuredClone({ owner, paths, appearances }); },
    savePaths: async (owner, paths) => { home = structuredClone({ owner, paths, appearances: home?.appearances ?? {} }); },
    saveAppearance: async (owner, path, appearance) => { home = structuredClone({ owner, paths: home?.paths ?? [], appearances: { ...home?.appearances, [path]: appearance } }); },
  };
  const create = () => {
    const tracking = new OfflineTracking(store, 'alice', () => Date.parse('2026-10-01T12:00:00Z'), () => `home-${++sequence}`);
    return durablePathRepository(apiPathRepository(`http://127.0.0.1:${address.port}`, () => 'session', () => {}), cache,
      async () => ({ owner: 'alice', timeZone: utcTimeZone, tracking, assertCurrent: () => {}, wake: () => {}, connected: () => !offline, reportNetwork: value => { availability.push(value); } }),
      error => error instanceof PathRequestError && error.status === 503);
  };
  const first = create(); await first.list(false);
  assert.equal((await store.read('alice'))?.summaries.reading.savedTotalSeconds, 60);
  await first.tracking('reading');
  savedSeconds = 75;
  assert.equal((await first.tracking('reading')).savedTotalSeconds, 75);
  const retained = await cache.readHome('alice'); assert.equal(retained?.appearances.reading.emoji, '🎸');
  interruptHydration = true; savedSeconds = 90; await first.list(false);
  assert.equal((await store.read('alice'))?.summaries.reading.savedTotalSeconds, 90);
  failSecond = true; await first.list(false); assert.deepEqual(await cache.readHome('alice'), retained);
  assert.equal(availability.at(-1), false);
  failSecond = false; await first.list(false); assert.equal(availability.at(-1), true);
  offline = true; const restored = create(); await restored.list(false);
  const reading = await restored.start('reading', 'start-hidden'); assert.ok(reading.activeSession);
  assert.equal((await restored.appearance('reading')).color, 'mint');
  assert.ok(requests.includes('/v1/paths/reading/timer'));
});

test('cancelled empty Home hydration cannot commit after its timezone read completes', async context => {
  const server = createServer((_request, response) => {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ data: [], meta: { homePreferences: { orderMethod: 'manual' } } }));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  context.after(() => new Promise<void>(resolve => { server.closeAllConnections(); server.close(() => resolve()); }));
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  let row: TrackingSnapshot | null = null, savedHome = false;
  const tracking = new OfflineTracking({ read: async () => row, commit: async (_owner, _revision, value) => { row = value; return true; } },
    'alice', () => Date.parse('2026-10-01T12:00:00Z'), () => 'unused');
  const cancellation = new AbortController();
  const repository = durablePathRepository(apiPathRepository(`http://127.0.0.1:${address.port}`, () => 'session', () => {}), {
    readHome: async () => null, saveHome: async () => { savedHome = true; }, savePaths: async () => {}, saveAppearance: async () => {},
  }, async () => ({ owner: 'alice', timeZone: utcTimeZone, tracking, assertCurrent: () => {}, wake: () => {},
    refreshTimeZone: async () => { cancellation.abort(); return utcTimeZone; } }), () => false);
  await assert.rejects(repository.list(false, cancellation.signal), cancellation.signal.reason);
  assert.equal(row, null);
  assert.equal(savedHome, false);
});
