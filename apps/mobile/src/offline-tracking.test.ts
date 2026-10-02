const utcTimeZone = 'utc';
import assert from 'node:assert/strict';
import test from 'node:test';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from '@hourpaths/client-core';
import { mobileTrackingAdapter } from './offline/mobile-tracking-adapter';
import type { MobileHomeProfile } from './session-destination';

function fixture() {
  const data = new Map<string, TrackingSnapshot>();
  const store: TrackingStore = {
    read: async owner => structuredClone(data.get(owner) ?? null),
    commit: async (owner, revision, value) => {
      if ((data.get(owner)?.revision ?? 0) !== revision) return false;
      data.set(owner, structuredClone(value)); return true;
    },
  };
  let now = Date.parse('2026-10-01T12:00:00Z');
  let sequence = 0;
  const core = () => new OfflineTracking(store, 'alice', () => now, () => `native-${++sequence}`);
  let live = true;
  const create = (tracking = core()) => mobileTrackingAdapter({ owner: 'alice', tracking,
    sync: { send: async () => { throw new TypeError('network'); } }, schedule: () => () => {},
    current: () => live, changed: () => {},
  });
  return { create, core, store, tick: () => { now += 60000; }, leave: () => { live = false; } };
}
const home: MobileHomeProfile = {
  id: 'alice', email: 'alice@example.test', displayName: 'alice', profileVisibility: 'private',
  homePreferences: { orderMethod: 'manual', pinnedPathIds: [], manualPathIds: ['guitar'], revision: 1 },
  paths: [{ id: 'guitar', name: 'guitar', visibility: 'private', capabilities: { trackTime: true },
    home: { classification: 'solo', pinned: false } } as MobileHomeProfile['paths'][number]],
  archivedPaths: [], pendingInvitations: { items: [], nextCursor: '' },
  timers: { guitar: { running: false, accumulatedSeconds: 60 } },
};

test('native adapter restores a running timer then stops durably without a network acknowledgement', async () => {
  const env = fixture();
  const first = env.create();
  assert.equal(await first.retain(home, utcTimeZone, await first.beginHydration()), true);
  const started = await first.start('guitar');
  assert.ok(started.timer);
  first.dispose(); env.tick();
  const restored = env.create();
  const running = await restored.timer('guitar');
  assert.equal(running.timer?.startedAt, started.timer.startedAt);
  const stopped = await restored.stop('guitar', running.timer!.id);
  assert.equal(stopped.running, false);
  assert.equal(stopped.pending, true);
  assert.equal(stopped.accumulatedSeconds, 120);
  assert.deepEqual((await env.core().snapshot()).operations.map(value => value.kind), ['start', 'stop']);
  restored.dispose();
});

test('native hydration keeps server timestamp precision and rejects stale or foreign account data', async () => {
  const env = fixture(); const adapter = env.create();
  const precise = '2026-10-01T11:59:00.123456Z';
  const profile = { ...home, timers: { guitar: { running: true, accumulatedSeconds: 60, timer: {
    id: 'server-timer', pathId: 'guitar', startedAt: precise, occurrenceTimeZone: utcTimeZone,
  } } } };
  assert.equal(await adapter.retain(profile, utcTimeZone, await adapter.beginHydration()), true);
  assert.equal((await adapter.timer('guitar')).timer?.startedAt, precise);
  const revision = await adapter.beginHydration();
  await adapter.stop('guitar', 'server-timer');
  assert.equal(await adapter.retain(profile, utcTimeZone, revision), false);
  await assert.rejects(adapter.retain({ ...profile, id: 'bob' }, utcTimeZone, await adapter.beginHydration()), /owner/);
  env.leave();
  await assert.rejects(adapter.start('guitar'), /superseded/);
  adapter.dispose();
});

test('native cold Home restoration publishes durable timers before making a network request', async () => {
  const { mobileOfflineHome } = await import('./offline/mobile-offline-home');
  const rows = new Map<string, TrackingSnapshot>();
  const store: TrackingStore = {
    read: async owner => structuredClone(rows.get(owner) ?? null),
    commit: async (owner, revision, next) => {
      if ((rows.get(owner)?.revision ?? 0) !== revision) return false;
      rows.set(owner, structuredClone(next)); return true;
    },
  };
  const homes = new Map<string, import('./offline/mobile-home-cache').RetainedMobileHome>();
  let requests = 0, sequence = 0, now = Date.parse('2026-10-01T12:00:00Z');
  const credential = { token: 'session', nextAction: 'home' as const, ownerId: 'alice', expiresAt: '2026-10-02T12:00:00Z' };
  const create = () => mobileOfflineHome({
    store, home: { readHome: async owner => homes.get(owner) ?? null, saveHome: async value => { homes.set(value.owner, structuredClone(value)); } },
    now: () => now, newId: () => `home-${++sequence}`, schedule: () => () => {},
    owner: async () => { requests++; return 'alice'; }, bind: async (value, owner) => { value.ownerId = owner; },
    remote: async () => { requests++; return structuredClone(home); }, timeZone: async () => utcTimeZone,
    sync: () => ({ send: async () => { throw new TypeError('network'); } }), publish: () => {},
  });
  const first = create();
  assert.equal((await first.load(credential, () => true)).retained, false);
  const started = await first.start('guitar');
  first.dispose(); now += 60000;
  const restored = create(); const before = requests;
  const loaded = await restored.load(credential, () => true);
  assert.equal(requests, before);
  assert.equal(loaded.retained, true);
  assert.equal(loaded.profile.timers.guitar.timer?.id, started.timer?.id);
  await restored.stop('guitar', started.timer!.id);
  assert.deepEqual(rows.get('alice')?.operations.map(value => value.kind), ['start', 'stop']);
  await assert.rejects(restored.load({ ...credential, ownerId: 'bob' }, () => true), /owner/);
  restored.dispose();
});

test('retained-account recovery stops existing timers after restart without enabling starts or replay', async () => {
  const { mobileOfflineHome } = await import('./offline/mobile-offline-home');
  const rows = new Map<string, TrackingSnapshot>();
  const homes = new Map<string, import('./offline/mobile-home-cache').RetainedMobileHome>();
  const store: TrackingStore = {
    read: async owner => structuredClone(rows.get(owner) ?? null),
    commit: async (owner, revision, next) => {
      if ((rows.get(owner)?.revision ?? 0) !== revision) return false;
      rows.set(owner, structuredClone(next)); return true;
    },
  };
  let instant = Date.parse('2026-10-01T12:00:00Z'), id = 0, requests = 0;
  const service = () => mobileOfflineHome({
    store, home: { readHome: async owner => homes.get(owner) ?? null, saveHome: async value => { homes.set(value.owner, value); } },
    now: () => instant, newId: () => `retained-${++id}`, schedule: () => () => {},
    owner: async () => 'alice', bind: async (credential, owner) => { credential.ownerId = owner; },
    remote: async () => home, timeZone: async () => utcTimeZone,
    sync: () => ({ send: async () => { requests++; throw new TypeError('network'); } }), publish: () => {},
  });
  const active = service();
  await active.load({ token: 'live', ownerId: 'alice', expiresAt: '2026-10-02T12:00:00Z', nextAction: 'home' }, () => true);
  const started = await active.start('guitar');
  active.pause(); active.dispose();
  const paused = service(); instant += 60000;
  assert.equal((await paused.restoreRetained('alice'))?.state.timers[0]?.id, started.timer?.id);
  const before = requests;
  await assert.rejects(paused.start('guitar'), /session_unavailable/);
  paused.wake();
  const stopped = await paused.stop('guitar', started.timer!.id);
  assert.equal(stopped.running, false);
  assert.equal(stopped.pending, true);
  assert.equal(requests, before);
  assert.equal((await paused.retained())?.state.operations.at(-1)?.kind, 'stop');
  paused.dispose();
  const different = service();
  assert.equal(await different.restoreRetained('bob'), null);
  await assert.rejects(different.stop('guitar', started.timer!.id), /session_unavailable/);
  different.dispose();
});


test('native cached Home hides confirmed lost membership after restart', async () => {
  const { mobileOfflineHome } = await import('./offline/mobile-offline-home');
  const env = fixture();
  const adapter = env.create();
  await adapter.retain(home, utcTimeZone, await adapter.beginHydration());
  await adapter.start('guitar');
  adapter.dispose();
  const core = env.core();
  await core.replay({ send: async () => ({ kind: 'rejected', reason: 'membership', disclosePath: false }) });
  const service = mobileOfflineHome({
    store: env.store,
    home: { readHome: async () => ({ owner: 'alice', timeZone: utcTimeZone, profile: home }), saveHome: async () => {} },
    now: () => Date.parse('2026-10-01T12:01:00Z'), newId: () => 'unused', schedule: () => () => {},
    owner: async () => { throw new Error('unexpected_network'); }, bind: async () => {},
    remote: async () => { throw new Error('unexpected_network'); }, timeZone: async () => utcTimeZone,
    sync: () => ({ send: async () => { throw new Error('unexpected_replay'); } }), publish: () => {},
  });
  const loaded = await service.load({ token: 'valid', ownerId: 'alice', expiresAt: '2026-10-02T12:00:00Z', nextAction: 'home' }, () => true);
  assert.deepEqual(loaded.profile.paths, []);
  assert.equal(loaded.profile.timers.guitar, undefined);
  await assert.rejects(service.start('guitar'), /tracking_snapshot_unavailable/);
  assert.equal((await core.snapshot()).operations.length, 0);
  service.dispose();
});

test('native retained history survives failed refresh and restart, includes pending stops, and fences concurrent commands', async () => {
  const { mobileOfflineHome } = await import('./offline/mobile-offline-home');
  const { retainedHistoryDetails } = await import('./offline/retained-history-presentation');
  const env = fixture();
  const homes = new Map<string, import('./offline/mobile-home-cache').RetainedMobileHome>();
  let now = Date.parse('2026-10-01T12:00:00Z'), sequence = 0;
  const entry = { id: 'recorded', owner: 'alice', pathId: 'guitar', pathName: 'Guitar', startedAt: '2026-09-30T12:00:00Z',
    endedAt: '2026-09-30T12:01:00Z', timeZone: utcTimeZone, version: 2, note: 'Practice' };
  let readHistory = async () => [entry, { ...entry, id: 'old', startedAt: '2026-01-01T12:00:00Z', endedAt: '2026-01-01T12:01:00Z' }];
  const create = () => mobileOfflineHome({
    store: env.store, home: { readHome: async owner => homes.get(owner) ?? null, saveHome: async value => { homes.set(value.owner, value); } },
    now: () => now, newId: () => `history-${++sequence}`, schedule: () => () => {},
    owner: async () => 'alice', bind: async () => {}, remote: async () => home, timeZone: async () => utcTimeZone,
    history: () => readHistory(), sync: () => ({ send: async () => { throw new Error('offline'); } }), publish: () => {},
  });
  const credential = { token: 'session', ownerId: 'alice', expiresAt: '2026-10-02T12:00:00Z', nextAction: 'home' as const };
  const first = create(); await first.load(credential, () => true); await first.refreshHistory();
  assert.deepEqual((await first.history('guitar')).items.map(item => item.id), ['recorded']);
  const timer = await first.start('guitar'); now += 60000; await first.stop('guitar', timer.timer!.id);
  readHistory = async () => { throw new Error('page_failed'); };
  await assert.rejects(first.refreshHistory(), /page_failed/);
  first.dispose();
  const restored = create(); await restored.load(credential, () => true);
  const local = await restored.history('guitar');
  assert.equal(local.incomplete, false);
  assert.deepEqual(local.items.map(item => item.pending), [true, false]);
  assert.equal(local.items[1].note, 'Practice');
  assert.equal(retainedHistoryDetails(local.items)[0].activity.durationSeconds, 60);
  assert.equal(retainedHistoryDetails(local.items)[1].retained, true);
  let resolve!: (entries: typeof entry[]) => void;
  let entered!: () => void; const reading = new Promise<void>(done => { entered = done; });
  readHistory = () => new Promise(done => { resolve = done; entered(); });
  const hydration = restored.refreshHistory(); await reading;
  await restored.start('guitar'); resolve([]);
  await assert.rejects(hydration, /hydration_changed/);
  assert.equal((await restored.history('guitar')).items.length, 2);
  restored.pause(); await assert.rejects(restored.history('guitar'), /session_unavailable/);
  restored.dispose();
});

test('native Home publishes a new goal period before an unavailable refresh and retains the full timer once', async () => {
  const { mobileOfflineHome } = await import('./offline/mobile-offline-home');
  const { liveTimerProgress } = await import('@hourpaths/client-core');
  const env = fixture();
  const profile: MobileHomeProfile = { ...home, paths: home.paths.map(path => ({ ...path, intervalGoal: {
    targetSeconds: 1800, recurrence: 'daily', alignment: { hour: 0 },
  } })), timers: { guitar: { accumulatedSeconds: 60, running: false, intervalProgress: { accumulatedSeconds: 60, targetSeconds: 1800,
    startedAt: '2026-10-01T00:00:00Z', endedAt: '2026-10-02T00:00:00Z' } } } };
  let now = Date.parse('2026-10-01T23:50:00Z'), id = 0, online = true;
  const published: MobileHomeProfile[] = [];
  const service = mobileOfflineHome({
    store: env.store, home: { readHome: async () => null, saveHome: async () => {} }, now: () => now, newId: () => `period-${++id}`,
    schedule: () => () => {}, owner: async () => 'alice', bind: async () => {}, timeZone: async () => utcTimeZone,
    remote: async () => { if (!online) throw new Error('offline'); return profile; },
    sync: () => ({ send: async () => { throw new Error('offline'); } }), publish: (_owner, value) => { published.push(value); },
  });
  await service.load({ token: 'valid', ownerId: 'alice', expiresAt: '2026-10-03T12:00:00Z', nextAction: 'home' }, () => true);
  const running = await service.start('guitar'); now = Date.parse('2026-10-02T00:20:00Z'); online = false;
  await service.present(); await assert.rejects(service.refresh(), /offline/);
  const displayed = liveTimerProgress(published.at(-1)!.timers.guitar, now);
  assert.equal(displayed.intervalProgress?.accumulatedSeconds, 1200);
  assert.equal(displayed.accumulatedSeconds, 1860);
  const stopped = await service.stop('guitar', running.timer!.id);
  assert.equal(stopped.intervalProgress?.accumulatedSeconds, 1200);
  assert.equal(stopped.accumulatedSeconds, 1860);
  service.dispose();
});
