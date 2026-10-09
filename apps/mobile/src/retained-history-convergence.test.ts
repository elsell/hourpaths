import assert from 'node:assert/strict';
import test from 'node:test';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from '@hourpaths/client-core';
import { retainedHistoryFromSnapshot } from './offline/retained-history-presentation';

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
  await tracking.retainPaths([{ id: 'guitar', name: 'Guitar', timeZone: 'UTC' }]);
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
