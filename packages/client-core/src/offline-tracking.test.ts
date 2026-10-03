import assert from 'node:assert/strict';
import test from 'node:test';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from './offline-tracking';

class DurableFake implements TrackingStore {
  rows = new Map<string, TrackingSnapshot>();
  failWrites = false;
  async read(owner: string) { return structuredClone(this.rows.get(owner) ?? null); }
  async commit(owner: string, revision: number, value: TrackingSnapshot) {
    if (this.failWrites) throw new Error('disk full');
    if ((this.rows.get(owner)?.revision ?? 0) !== revision) return false;
    this.rows.set(owner, structuredClone(value));
    return true;
  }
}
const path = { id: 'guitar', name: 'Guitar', timeZone: 'America/New_York' };
function client(store: DurableFake, owner: string, instant: string, prefix: string) {
  let id = 0;
  return new OfflineTracking(store, owner, () => new Date(instant).getTime(), () => `${prefix}-${++id}`);
}

test('offline timer survives orchestrator restart and queues original start before stop', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]);
  const started = await first.start(path.id);
  const restored = client(store, 'alice', '2026-10-01T12:05:00Z', 'b');
  assert.equal((await restored.snapshot()).timers[0].id, started.id);
  await restored.stop(started.id);
  const state = await restored.snapshot();
  assert.equal(state.timers.length, 0);
  assert.deepEqual(state.operations.map(x => x.kind), ['start', 'stop']);
  assert.equal(state.operations[0].startedAt, '2026-10-01T12:00:00.000Z');
  assert.equal(state.operations[1].endedAt, '2026-10-01T12:05:00.000Z');
  assert.equal(state.operations[1].timerId, started.id);
  assert.equal(state.operations[1].timeZone, path.timeZone);
  assert.equal((await client(store, 'bob', '2026-10-01T12:06:00Z', 'c').snapshot()).operations.length, 0);
});

test('failed durable stop retains running timer and cannot partially enqueue', async () => {
  const store = new DurableFake();
  const current = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await current.retainPaths([path]);
  const timer = await current.start(path.id);
  store.failWrites = true;
  await assert.rejects(current.stop(timer.id), /disk full/);
  const state = await current.snapshot();
  assert.equal(state.timers.length, 1);
  assert.equal(state.operations.length, 1);
});

test('backward device clock retains a correction instead of creating replayable stop', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]);
  const timer = await first.start(path.id);
  await client(store, 'alice', '2026-10-01T11:59:00Z', 'b').stop(timer.id);
  const state = await first.snapshot();
  assert.equal(state.corrections[0].timer.id, timer.id);
  assert.equal(state.operations.filter(x => x.kind === 'stop').length, 0);
});

test('lost acknowledgement reuses the durable operation and preserves causal start/stop delivery', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]);
  const timer = await first.start(path.id);
  const later = client(store, 'alice', '2026-10-01T12:05:00Z', 'b');
  await later.stop(timer.id);
  const received = new Map<string, string>();
  const attempts: string[] = [];
  let loseAcknowledgement = true;
  const server = { async send(owner: string, operation: import('./offline-tracking').TrackingOperation) {
    assert.equal(owner, 'alice');
    attempts.push(operation.operationId);
    const payload = JSON.stringify(operation);
    if (received.has(operation.operationId)) assert.equal(received.get(operation.operationId), payload);
    received.set(operation.operationId, payload);
    if (loseAcknowledgement) { loseAcknowledgement = false; throw new Error('connection lost after commit'); }
    return { kind: 'accepted' as const };
  } };
  await assert.rejects(later.replay(server), /connection lost/);
  assert.equal((await later.snapshot()).operations.length, 2);
  await client(store, 'alice', '2026-10-01T12:06:00Z', 'c').replay(server);
  assert.equal(attempts[0], attempts[1]);
  assert.equal(received.size, 2);
  assert.equal((await later.snapshot()).operations.length, 0);
});

test('permanent rejection settles one timer and retains a notice while independent work synchronizes', async () => {
  const store = new DurableFake();
  const current = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await current.retainPaths([path, { ...path, id: 'reading' }]);
  await current.start(path.id);
  await current.start('reading');
  await current.replay({ async send(_owner, operation) {
    return operation.pathId === path.id
      ? { kind: 'rejected' as const, reason: 'membership' as const, disclosePath: false }
      : { kind: 'accepted' as const };
  } });
  const state = await current.snapshot();
  assert.equal(state.operations.length, 0);
  assert.deepEqual(state.timers.map(x => x.pathId), ['reading']);
  assert.equal(state.notices[0].reason, 'membership');
  assert.equal(state.notices[0].pathId, undefined);
  await current.dismissNotice(state.notices[0].id);
  assert.equal((await current.snapshot()).notices.length, 0);
});

test('history hydration cannot overwrite a concurrent local stop and retains only the own-account 90-day window', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]);
  const timer = await first.start(path.id);
  const revision = (await first.snapshot()).revision;
  const later = client(store, 'alice', '2026-10-01T12:05:00Z', 'b');
  await later.stop(timer.id);
  const entry = { id: 'server-entry', owner: 'alice', pathId: path.id, startedAt: '2026-09-30T12:00:00Z', endedAt: '2026-09-30T12:05:00Z', timeZone: path.timeZone };
  assert.equal(await later.retainHistory([entry], revision), false);
  const current = await later.snapshot();
  assert.equal(await later.retainHistory([entry, { ...entry, id: 'old-entry', startedAt: '2026-01-01T12:00:00Z', endedAt: '2026-01-01T12:05:00Z' }], current.revision), true);
  const restored = client(store, 'alice', '2026-10-01T12:06:00Z', 'c');
  assert.deepEqual((await restored.snapshot()).history.map(value => value.id), ['server-entry']);
  assert.equal((await restored.pendingHistory())[0].endedAt, '2026-10-01T12:05:00.000Z');
  await assert.rejects(restored.retainHistory([{ ...entry, owner: 'bob' }], (await restored.snapshot()).revision), /tracking_history_invalid/);
  assert.equal((await client(store, 'bob', '2026-10-01T12:06:00Z', 'd').snapshot()).history.length, 0);
});

test('partial archive acknowledgement replaces pending history with saved interval and durable discarded-time notice', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]);
  const timer = await first.start(path.id);
  const later = client(store, 'alice', '2026-10-01T12:05:00Z', 'b');
  await later.stop(timer.id);
  await later.replay({ async send(_owner, operation) {
    if (operation.kind === 'start') return { kind: 'accepted' as const };
    return { kind: 'rejected' as const, reason: 'archived' as const, disclosePath: true, savedSeconds: 120, discardedSeconds: 180,
      activity: { id: 'saved-archive-entry', owner: 'alice', pathId: path.id, startedAt: operation.startedAt, endedAt: '2026-10-01T12:02:00Z', timeZone: path.timeZone } };
  } });
  const state = await later.snapshot();
  assert.equal(state.history[0].id, 'saved-archive-entry');
  assert.equal(state.notices[0].discardedSeconds, 180);
  assert.equal(state.notices[0].savedSeconds, 120);
  assert.equal((await later.pendingHistory()).length, 0);
});

test('adopted online timer stops offline with its existing identity and refresh cannot erase a pending start', async () => {
  const store = new DurableFake();
  const current = client(store, 'alice', '2026-10-01T12:05:00Z', 'a');
  await current.retainPaths([path]);
  const online = { id: 'server-timer', pathId: path.id, startedAt: '2026-10-01T12:00:00Z', timeZone: path.timeZone };
  assert.equal(await current.adoptTimers([online], (await current.snapshot()).revision), true);
  await current.stop(online.id);
  assert.equal((await current.snapshot()).operations[0].timerId, online.id);
  assert.equal((await current.pendingHistory()).length, 1);
  assert.equal(await current.adoptTimers([online], (await current.snapshot()).revision), true);
  assert.equal((await current.snapshot()).timers.length, 0);
  const next = await current.start(path.id);
  assert.equal(await current.adoptTimers([], (await current.snapshot()).revision), true);
  assert.equal((await current.snapshot()).timers[0].id, next.id);
});

test('archive response automatically ends a still-running local timer and replays its original interval', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]);
  await first.start(path.id);
  const restored = client(store, 'alice', '2026-10-01T12:05:00Z', 'b');
  const sent: string[] = [];
  await restored.replay({ async send(_owner, operation) {
    sent.push(operation.kind);
    if (operation.kind === 'start') return { kind: 'accepted' as const, mustStop: true };
    assert.equal(operation.endedAt, '2026-10-01T12:05:00.000Z');
    return { kind: 'rejected' as const, reason: 'archived' as const, disclosePath: true, savedSeconds: 120, discardedSeconds: 180 };
  } });
  assert.deepEqual(sent, ['start', 'stop']);
  assert.equal((await restored.snapshot()).timers.length, 0);
});

test('a terminal start acknowledgement removes local timer and its queued stop', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]);
  const timer = await first.start(path.id);
  const later = client(store, 'alice', '2026-10-01T12:05:00Z', 'b');
  await later.stop(timer.id);
  let calls = 0;
  await later.replay({ async send() { calls++; return { kind: 'accepted' as const, terminal: true }; } });
  assert.equal(calls, 1);
  assert.equal((await later.snapshot()).operations.length, 0);
});

test('offline Home totals include pending time exactly once before and after acknowledgement and refresh', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]);
  const base = { savedTotalSeconds: 600, period: { savedSeconds: 60, targetSeconds: 3600, startsAt: Date.parse('2026-10-01T00:00:00Z'), endsAt: Date.parse('2026-10-02T00:00:00Z') } };
  await first.retainTracking(path.id, base, null, (await first.snapshot()).revision);
  const timer = await first.start(path.id);
  const later = client(store, 'alice', '2026-10-01T12:05:00Z', 'b');
  await later.stop(timer.id);
  assert.equal((await later.tracking(path.id))?.savedTotalSeconds, 900);
  assert.equal((await later.tracking(path.id))?.period?.savedSeconds, 360);
  assert.equal(await later.retainTracking(path.id, base, null, (await later.snapshot()).revision), false);
  await later.replay({ async send(_owner, operation) {
    return { kind: 'accepted' as const, ...(operation.kind === 'stop' ? { activity: {
      id: 'confirmed-entry', owner: 'alice', pathId: path.id, startedAt: operation.startedAt, endedAt: operation.endedAt!, timeZone: operation.timeZone,
    } } : {}) };
  } });
  assert.equal((await later.tracking(path.id))?.savedTotalSeconds, 900);
  await later.retainTracking(path.id, { ...base, savedTotalSeconds: 900, period: { ...base.period, savedSeconds: 360 } }, null, (await later.snapshot()).revision);
  assert.equal((await later.tracking(path.id))?.savedTotalSeconds, 900);
  assert.equal((await later.tracking(path.id))?.period?.savedSeconds, 360);
});

test('complete Home hydration commits all paths together and fences a concurrent command', async () => {
  const store = new DurableFake();
  const tracking = client(store, 'alice', '2026-10-01T12:00:00Z', 'home');
  const summary = { savedTotalSeconds: 5, period: null };
  const second = { ...path, id: 'second' };
  assert.equal(await tracking.retainHome([path, second], [
    { pathId: path.id, summary, timer: null }, { pathId: second.id, summary, timer: null },
  ], 0), true);
  const fetchedAt = (await tracking.snapshot()).revision;
  const timer = await tracking.start(path.id);
  assert.equal(await tracking.retainHome([path, second], [
    { pathId: path.id, summary, timer: null }, { pathId: second.id, summary, timer: null },
  ], fetchedAt), false);
  assert.equal((await tracking.snapshot()).timers[0]?.id, timer.id);
  assert.equal((await tracking.tracking(second.id))?.savedTotalSeconds, 5);
  const before = await tracking.snapshot();
  await assert.rejects(tracking.retainHome([path, second], [{ pathId: path.id, summary, timer: null }], before.revision));
  assert.deepEqual(await tracking.snapshot(), before);
});


test('confirmed membership loss survives restart and a stale refresh without discarding another Path', async () => {
  const store = new DurableFake();
  const current = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await current.retainPaths([path, { ...path, id: 'reading' }]);
  const timer = await current.start(path.id);
  await client(store, 'alice', '2026-10-01T12:01:00Z', 'b').stop(timer.id);
  await current.start(path.id);
  await current.start('reading');
  const before = (await current.snapshot()).revision;
  const attempted: string[] = [];
  await assert.rejects(current.replay({ async send(_owner, operation) {
    attempted.push(operation.pathId);
    if (operation.pathId === 'reading') throw new Error('offline');
    return { kind: 'rejected', reason: 'membership', disclosePath: false };
  } }), /offline/);
  assert.deepEqual(attempted, [path.id, 'reading']);
  const restored = client(store, 'alice', '2026-10-01T12:02:00Z', 'c');
  assert.equal(await restored.retainPaths([path], before), false);
  assert.equal(await restored.retainTracking(path.id, { savedTotalSeconds: 0, period: null }, timer, (await restored.snapshot()).revision), false);
  await restored.dismissNotice((await restored.snapshot()).notices[0].id);
  await assert.rejects(restored.start(path.id), /tracking_path_unavailable/);
  assert.deepEqual((await restored.snapshot()).operations.map(value => value.pathId), ['reading']);
  assert.equal(await restored.retainPaths([path], (await restored.snapshot()).revision), true);
  await restored.start(path.id);
});

test('clock correction atomically queues reviewed timing while retaining original identity across restart and retries', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]); const timer = await first.start(path.id);
  const corrected = client(store, 'alice', '2026-10-01T11:59:00Z', 'b');
  await corrected.stop(timer.id);
  await assert.rejects(corrected.correct(timer.id, '2026-10-01T12:00:00Z', '2026-10-01T12:01:00Z'), /correction_invalid/);
  store.failWrites = true;
  await assert.rejects(corrected.correct(timer.id, '2026-10-01T11:57:00Z', '2026-10-01T11:59:00Z'), /disk full/);
  assert.equal((await corrected.snapshot()).corrections.length, 1);
  store.failWrites = false;
  await corrected.correct(timer.id, '2026-10-01T11:57:00Z', '2026-10-01T11:59:00Z');
  const restored = client(store, 'alice', '2026-10-01T12:01:00Z', 'c');
  const state = await restored.snapshot();
  assert.equal(state.corrections.length, 0);
  assert.deepEqual(state.operations.map(value => value.kind), ['correct']);
  assert.equal(state.operations[0].startedAt, timer.startedAt);
  assert.equal(state.operations[0].correctedStartedAt, '2026-10-01T11:57:00.000Z');
  assert.equal((await restored.pendingHistory())[0].startedAt, '2026-10-01T11:57:00.000Z');
  const sent: unknown[] = [];
  await assert.rejects(restored.replay({ send: async (_owner, operation) => { sent.push(operation); throw new Error('ack lost'); } }), /ack lost/);
  await restored.replay({ send: async (_owner, operation) => { sent.push(operation); return { kind: 'accepted', terminal: true }; } });
  assert.deepEqual(sent[0], sent[1]);
  assert.equal((await restored.snapshot()).operations.length, 0);
});

test('rejected reviewed timing returns to correction without resurrecting a server timer', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'a');
  await first.retainPaths([path]); const timer = await first.start(path.id);
  const current = client(store, 'alice', '2026-10-01T11:59:00Z', 'b');
  await current.stop(timer.id);
  await current.correct(timer.id, '2026-10-01T11:57:00Z', '2026-10-01T11:59:00Z');
  await current.replay({ send: async () => ({ kind: 'rejected', reason: 'validation', disclosePath: true }) });
  const state = await current.snapshot();
  assert.equal(state.operations.length, 0);
  assert.equal(state.corrections[0].reviewedStartedAt, '2026-10-01T11:57:00.000Z');
  assert.equal(await current.retainTracking(path.id, { savedTotalSeconds: 0, period: null }, timer, state.revision), false);
  assert.equal((await current.snapshot()).timers.length, 0);
});

test('offline daily rollover keeps lifetime time once and moves only the overlap into the new goal period', async () => {
  const store = new DurableFake();
  const current = client(store, 'alice', '2026-10-01T23:50:00Z', 'a');
  await current.retainPaths([{ ...path, timeZone: 'UTC', goal: { recurrence: 'daily', alignment: { hour: 0 }, targetSeconds: 1800 } }]);
  await current.retainTracking(path.id, { savedTotalSeconds: 90, period: { savedSeconds: 90, targetSeconds: 1800,
    startsAt: Date.parse('2026-10-01T00:00:00Z'), endsAt: Date.parse('2026-10-02T00:00:00Z') } }, null, (await current.snapshot()).revision);
  const timer = await current.start(path.id);
  const restored = client(store, 'alice', '2026-10-02T00:20:00Z', 'b');
  assert.equal((await restored.tracking(path.id))?.period?.startsAt, Date.parse('2026-10-02T00:00:00Z'));
  assert.equal((await restored.tracking(path.id))?.period?.savedSeconds, 0);
  await restored.stop(timer.id);
  const summary = await restored.tracking(path.id);
  assert.equal(summary?.period?.savedSeconds, 1200);
  assert.equal(summary?.savedTotalSeconds, 1890);
  const nextDay = client(store, 'alice', '2026-10-03T00:00:00Z', 'c');
  assert.equal((await nextDay.tracking(path.id))?.period?.savedSeconds, 0);
  assert.equal((await nextDay.tracking(path.id))?.savedTotalSeconds, 1890);
});

test('a positive subsecond stop retains a quiet notice across restart without dropping its sync commands', async () => {
  const store = new DurableFake();
  const first = client(store, 'alice', '2026-10-01T12:00:00Z', 'short');
  await first.retainPaths([path]);
  const timer = await first.start(path.id);
  const restored = client(store, 'alice', '2026-10-01T12:00:00.500Z', 'stop');
  await restored.stop(timer.id);
  const state = await client(store, 'alice', '2026-10-01T12:00:02Z', 'read').snapshot();
  assert.equal(state.timers.length, 0);
  assert.deepEqual(state.operations.map(value => value.kind), ['start', 'stop']);
  assert.equal(state.notices[0]?.reason, 'subsecond');
  assert.equal(state.notices[0]?.pathId, path.id);
});


test('Home retains a fetched snapshot through history-only writes without dropping newer history', async () => {
  const store = new DurableFake();
  const tracking = client(store, 'alice', '2026-10-01T12:00:00Z', 'home');
  await tracking.retainPaths([path]);
  const fetchedAt = (await tracking.snapshot()).revision;
  const entry = { id: 'history', owner: 'alice', pathId: path.id, startedAt: '2026-10-01T10:00:00Z', endedAt: '2026-10-01T10:01:00Z', timeZone: path.timeZone };
  assert.equal(await tracking.retainHistory([entry], fetchedAt), true);
  await tracking.retainActivity({ ...entry, id: 'detail', note: 'Retained detail' });
  assert.equal(await tracking.retainHome([path], [{ pathId: path.id, summary: { savedTotalSeconds: 120, period: null }, timer: null }], fetchedAt), true);
  const state = await tracking.snapshot();
  assert.deepEqual(state.history.map(value => value.id), ['history', 'detail']);
  assert.equal((await tracking.tracking(path.id))?.savedTotalSeconds, 120);
});

test('history refreshes cannot hide a command or an older-client write from the Home fence', async () => {
  for (const legacy of [false, true]) {
    const store = new DurableFake();
    const tracking = client(store, 'alice', '2026-10-01T12:00:00Z', 'home');
    await tracking.retainPaths([path]);
    const fetchedAt = (await tracking.snapshot()).revision;
    await tracking.retainHistory([], fetchedAt);
    if (legacy) {
      const state = await tracking.snapshot();
      const revision = state.revision++;
      state.timers.push({ id: 'older-client-timer', pathId: path.id, startedAt: '2026-10-01T12:00:00Z', timeZone: path.timeZone });
      assert.equal(await store.commit('alice', revision, state), true);
    } else await tracking.start(path.id);
    await tracking.retainHistory([], (await tracking.snapshot()).revision);
    const before = await tracking.snapshot();
    assert.equal(await tracking.retainHome([path], [{ pathId: path.id, summary: { savedTotalSeconds: 0, period: null }, timer: null }], fetchedAt), false);
    assert.deepEqual(await tracking.snapshot(), before);
  }
});


test('Home retries a competing history commit locally and retains its result', async () => {
  class CompetingHistoryStore extends DurableFake {
    beforeCommit?: () => Promise<void>;
    override async commit(owner: string, revision: number, state: TrackingSnapshot) {
      const compete = this.beforeCommit;
      this.beforeCommit = undefined;
      if (compete) await compete();
      return super.commit(owner, revision, state);
    }
  }
  const store = new CompetingHistoryStore();
  const home = client(store, 'alice', '2026-10-01T12:00:00Z', 'home');
  const history = client(store, 'alice', '2026-10-01T12:00:00Z', 'history');
  await home.retainPaths([path]);
  const fetchedAt = (await home.snapshot()).revision;
  const entry = { id: 'racing-detail', owner: 'alice', pathId: path.id, startedAt: '2026-10-01T10:00:00Z', endedAt: '2026-10-01T10:01:00Z', timeZone: path.timeZone };
  store.beforeCommit = () => history.retainActivity(entry);
  assert.equal(await home.retainHome([path], [{ pathId: path.id, summary: { savedTotalSeconds: 60, period: null }, timer: null }], fetchedAt), true);
  assert.deepEqual((await home.snapshot()).history, [entry]);
  assert.equal((await home.tracking(path.id))?.savedTotalSeconds, 60);
  assert.equal(await home.retainHome([path], [{ pathId: path.id, summary: { savedTotalSeconds: 0, period: null }, timer: null }], Number.NaN), false);
});
