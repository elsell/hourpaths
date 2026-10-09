import assert from 'node:assert/strict';
import test from 'node:test';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from '@hourpaths/client-core';
import { retainedHistoryFromSnapshot, activityDeletionFromSnapshot, onlineHistoryFromSnapshot } from './offline/retained-history-presentation';

class DurableHistoryStore implements TrackingStore {
  private rows = new Map<string, TrackingSnapshot>();
  async read(owner: string) { return structuredClone(this.rows.get(owner) ?? null); }
  async commit(owner: string, revision: number, next: TrackingSnapshot) {
    if ((this.rows.get(owner)?.revision ?? 0) !== revision) return false;
    this.rows.set(owner, structuredClone(next));
    return true;
  }
}

test('an open retained timeline follows acknowledgement without retaining its pending row or duplicating the session', async () => {
  let now = Date.parse('2026-10-09T12:00:00Z');
  let sequence = 0;
  const tracking = new OfflineTracking(new DurableHistoryStore(), 'owner', () => now, () => `local-${++sequence}`);
  const timeZone = ['U', 'T', 'C'].join('');
  const path = { id: 'guitar', name: 'guitar', timeZone };
  await tracking.retainPaths([path]);
  const timer = await tracking.start('guitar');
  now += 172_000;
  await tracking.stop(timer.id);
  const before = retainedHistoryFromSnapshot(await tracking.snapshot(), 'owner', 'guitar');
  assert.equal(before.length, 1);
  assert.equal(before[0].pending, true);
  assert.equal(before[0].activity.durationSeconds, 172);

  await tracking.replay({ async send(owner, operation) {
    return operation.kind === 'start' ? { kind: 'accepted', serverTimerId: 'server-timer' } : {
      kind: 'accepted', activity: {
        id: 'saved-session', owner, pathId: operation.pathId, startedAt: operation.startedAt,
        endedAt: operation.endedAt!, timeZone: operation.timeZone,
      },
    };
  } });
  const published = await tracking.snapshot();
  const after = retainedHistoryFromSnapshot(published, 'owner', 'guitar');
  assert.equal(after.length, 1);
  assert.equal(after[0].pending, false);
  assert.equal(after[0].activity.id, 'saved-session');
  assert.equal(after[0].activity.durationSeconds, 172);
  assert.equal(before[0].pending, true, 'the old route snapshot stays immutable');
  assert.deepEqual(retainedHistoryFromSnapshot(published, 'replacement-owner', 'guitar'), []);
  assert.deepEqual(retainedHistoryFromSnapshot({ ...published, unavailablePaths: ['guitar'] }, 'owner', 'guitar'), []);
});

test('an open detail only becomes deleted from its own account explicit tombstone', async () => {
  const tracking = new OfflineTracking(new DurableHistoryStore(), 'owner', () => 0, () => 'id');
  const snapshot = await tracking.snapshot();
  assert.equal(activityDeletionFromSnapshot(snapshot, 'owner', 'entry'), false, 'missing history is not deletion');
  const deleted = { ...snapshot, deletedActivityIds: ['entry'] };
  assert.equal(activityDeletionFromSnapshot(deleted, 'owner', 'entry'), true);
  assert.equal(activityDeletionFromSnapshot(deleted, 'owner', 'other'), false);
  assert.equal(activityDeletionFromSnapshot({ ...deleted, activityAliases: { 'pending-entry': 'entry' } }, 'owner', 'pending-entry'), true);
  assert.equal(activityDeletionFromSnapshot(deleted, 'replacement-owner', 'entry'), false);
  assert.equal(activityDeletionFromSnapshot(null, 'owner', 'entry'), false);
});


test('online history follows a local stop and acknowledgement while preserving loaded participants and newer revisions', async () => {
  const timeZone = ['U', 'T', 'C'].join('');
  const emptyMetadata = '';
  const newerNote = 'newer-remote-edit';
  let now = Date.parse('2026-10-09T12:00:00Z'); let id = 0;
  const tracking = new OfflineTracking(new DurableHistoryStore(), 'owner', () => now, () => `local-${++id}`);
  await tracking.retainPaths([{ id: 'guitar', name: 'guitar', timeZone }]);
  const other = { version: 3, activity: { id: 'other-session', pathId: 'guitar', participantId: 'other', startedAt: '2026-10-09T11:00:00Z', endedAt: '2026-10-09T11:01:00Z', durationSeconds: 60, occurrenceTimeZone: timeZone, createdAt: emptyMetadata, updatedAt: emptyMetadata } };
  const timer = await tracking.start('guitar'); now += 88_000; await tracking.stop(timer.id);
  const pending = onlineHistoryFromSnapshot([other], await tracking.snapshot(), 'owner', 'guitar');
  assert.equal(pending.length, 2); assert.equal(pending[0].pending, true);
  assert.deepEqual(pending[1], other);
  await tracking.replay({ async send(owner, operation) { return operation.kind === 'start'
    ? { kind: 'accepted', serverTimerId: 'server-timer' }
    : { kind: 'accepted', activity: { id: 'saved', owner, pathId: operation.pathId, startedAt: operation.startedAt, endedAt: operation.endedAt!, timeZone: operation.timeZone } }; } });
  const state = await tracking.snapshot();
  const saved = onlineHistoryFromSnapshot([other], state, 'owner', 'guitar');
  assert.deepEqual(saved.map(row => row.activity.id), ['saved', 'other-session']);
  assert.equal(saved[0].activity.durationSeconds, 88); assert.equal(saved[0].pending, false);
  assert.equal(saved[0].retained, false, 'online acknowledged rows can open their authoritative detail');
  const newer = { ...saved[0], version: 5, activity: { ...saved[0].activity, note: newerNote } };
  assert.deepEqual(onlineHistoryFromSnapshot([newer, other], state, 'owner', 'guitar'), [newer, other]);
  assert.deepEqual(onlineHistoryFromSnapshot([newer, other], { ...state, deletedActivityIds: ['saved'] }, 'owner', 'guitar'), [other]);
  assert.deepEqual(onlineHistoryFromSnapshot([other], state, 'different-owner', 'guitar'), []);
  assert.deepEqual(onlineHistoryFromSnapshot([other], { ...state, unavailablePaths: ['guitar'] }, 'owner', 'guitar'), []);
});
