import assert from 'node:assert/strict';
import test from 'node:test';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from './offline-tracking';

class Ledger implements TrackingStore {
  rows = new Map<string, TrackingSnapshot>();
  async read(owner: string) { return structuredClone(this.rows.get(owner) ?? null); }
  async commit(owner: string, revision: number, next: TrackingSnapshot) {
    if ((this.rows.get(owner)?.revision ?? 0) !== revision) return false;
    this.rows.set(owner, structuredClone(next)); return true;
  }
}

test('offline manual creation and editing survive restart as one entry and preserve causal commands', async () => {
  const store = new Ledger(); let sequence = 0;
  const clock = () => Date.parse('2026-10-02T12:00:00Z');
  const open = (owner = 'alice') => new OfflineTracking(store, owner, clock, () => `operation-${++sequence}`);
  const first = open();
  await first.retainHome([{ id: 'guitar', name: 'Guitar', timeZone: 'UTC', goal: null }],
    [{ pathId: 'guitar', summary: { savedTotalSeconds: 60, period: null }, timer: null }], 0);
  const created = await first.createManualActivity({ pathId: 'guitar', startedAt: '2026-10-02T11:00:00Z', durationSeconds: 120, note: 'first' });
  first.dispose();
  const second = open();
  await second.editRecordedActivity(created.id, { startedAt: '2026-10-02T11:00:00Z', durationSeconds: 180, note: 'revised' });
  second.dispose();
  const restored = open();
  const history = await restored.localHistory('guitar');
  assert.equal(history.items.length, 1);
  assert.equal(history.items[0].id, created.id);
  assert.equal(history.items[0].note, 'revised');
  assert.equal(history.items[0].pending, true);
  assert.equal((await restored.tracking('guitar'))?.savedTotalSeconds, 240);
  const commands = (await restored.snapshot()).activityOperations!;
  assert.deepEqual(commands.map(value => value.kind), ['create', 'edit']);
  assert.equal(commands[0].activity.id, commands[1].activity.id);
  assert.equal(commands[0].activity.note, 'first');
  assert.notEqual(commands[0].operationId, commands[1].operationId);
  assert.equal((await open('bob').snapshot()).activityOperations?.length ?? 0, 0);
});

test('editing an offline occurrence moves only its overlap into the current goal period and keeps causal time', async () => {
  const store = new Ledger(); let now = Date.parse('2026-10-02T12:00:00Z'), sequence = 0;
  const core = new OfflineTracking(store, 'alice', () => now, () => `id-${++sequence}`);
  await core.retainHome([{ id: 'guitar', name: 'Guitar', timeZone: 'UTC' }], [{ pathId: 'guitar', summary: {
    savedTotalSeconds: 60, period: { savedSeconds: 0, targetSeconds: 600, startsAt: Date.parse('2026-10-02T11:00:00Z'), endsAt: now },
  }, timer: null }], 0);
  const entry = await core.createManualActivity({ pathId: 'guitar', startedAt: '2026-10-02T10:59:00.123456Z', durationSeconds: 120, note: '' });
  assert.equal(entry.endedAt, '2026-10-02T11:01:00.123456Z');
  assert.equal((await core.tracking('guitar'))?.period?.savedSeconds, 60);
  now -= 1000;
  await core.editRecordedActivity(entry.id, { startedAt: '2026-10-02T11:00:00Z', durationSeconds: 180, note: '' });
  assert.equal((await core.tracking('guitar'))?.period?.savedSeconds, 180);
  const commands = (await core.snapshot()).activityOperations!;
  assert.equal(commands[1].stamp.authoredAt, commands[0].stamp.authoredAt);
  assert.ok(commands[1].stamp.counter > commands[0].stamp.counter);
});

test('failed persistence does not publish a manual entry or discard existing timer commands', async () => {
  const ledger = new Ledger(); let fail = false, sequence = 0;
  const store: TrackingStore = { read: owner => ledger.read(owner), commit: (owner, version, state) => {
    if (fail) throw new Error('disk full'); return ledger.commit(owner, version, state);
  } };
  const core = new OfflineTracking(store, 'alice', () => Date.parse('2026-10-02T12:00:00Z'), () => `id-${++sequence}`);
  await core.retainPaths([{ id: 'guitar', name: 'Guitar', timeZone: 'UTC' }]);
  await core.start('guitar');
  fail = true;
  await assert.rejects(core.createManualActivity({ pathId: 'guitar', startedAt: '2026-10-02T11:00:00Z', durationSeconds: 120, note: '' }), /disk full/);
  const state = await core.snapshot();
  assert.equal(state.activityOperations?.length ?? 0, 0);
  assert.equal(state.timers.length, 1);
  assert.equal(state.operations.length, 1);
});

test('a lost acknowledgement cannot double-count a pending manual entry through fresh Home or history', async () => {
  const store = new Ledger(); let sequence = 0;
  const core = new OfflineTracking(store, 'alice', () => Date.parse('2026-10-02T12:00:00Z'), () => `id-${++sequence}`);
  const paths = [{ id: 'guitar', name: 'Guitar', timeZone: 'UTC' }];
  await core.retainHome(paths, [{ pathId: 'guitar', summary: { savedTotalSeconds: 60, period: null }, timer: null }], 0);
  const entry = await core.createManualActivity({ pathId: 'guitar', startedAt: '2026-10-02T11:00:00Z', durationSeconds: 120, note: '' });
  assert.equal(await core.retainTracking('guitar', { savedTotalSeconds: 180, period: null }, null, (await core.snapshot()).revision), false);
  await core.retainHome(paths, [{ pathId: 'guitar', summary: { savedTotalSeconds: 180, period: null }, timer: null }], (await core.snapshot()).revision);
  await core.retainHistory([entry], (await core.snapshot()).revision);
  assert.equal((await core.tracking('guitar'))?.savedTotalSeconds, 180);
  assert.equal((await core.localHistory('guitar')).items.length, 1);
  assert.equal((await core.pendingHistory()).length, 1);
});

test('manual replay retries stable commands after acknowledgement loss and applies edits without double-counting', async () => {
  const ledger = new Ledger(); let sequence = 0, loseAcknowledgement = true;
  const core = new OfflineTracking(ledger, 'alice', () => Date.parse('2026-10-02T12:00:00Z'), () => `id-${++sequence}`);
  await core.retainHome([{ id: 'guitar', name: 'Guitar', timeZone: 'UTC' }], [{ pathId: 'guitar', summary: { savedTotalSeconds: 60, period: null }, timer: null }], 0);
  const created = await core.createManualActivity({ pathId: 'guitar', startedAt: '2026-10-02T11:00:00Z', durationSeconds: 120, note: 'first' });
  await core.editRecordedActivity(created.id, { startedAt: '2026-10-02T11:00:00Z', durationSeconds: 180, note: 'second' });
  const applied = new Map<string, import('./offline-tracking').RetainedActivity>(); const delivered: string[] = [];
  const sync = { send: async () => { throw new Error('unexpected timer'); },
    sendActivity: async (owner: string, operation: import('./offline-recorded-activity').RecordedActivityOperation) => {
      assert.equal(owner, 'alice'); delivered.push(operation.operationId);
      if (!applied.has(operation.operationId)) applied.set(operation.operationId, structuredClone(operation.activity));
      if (loseAcknowledgement) { loseAcknowledgement = false; throw new TypeError('network'); }
      return { kind: 'accepted' as const, activity: applied.get(operation.operationId)! };
    } };
  await assert.rejects(core.replay(sync), /network/);
  assert.equal((await core.snapshot()).activityOperations?.length, 2);
  await core.replay(sync);
  assert.equal(delivered[0], delivered[1]);
  assert.equal(applied.size, 2);
  assert.equal((await core.snapshot()).activityOperations?.length, 0);
  assert.equal((await core.localHistory('guitar')).items[0].note, 'second');
  assert.equal((await core.tracking('guitar'))?.savedTotalSeconds, 240);
  await core.retainTracking('guitar', { savedTotalSeconds: 240, period: null }, null, (await core.snapshot()).revision);
  assert.equal((await core.tracking('guitar'))?.savedTotalSeconds, 240);
});

test('deletion wins over an offline edit while an unrelated manual entry still synchronizes', async () => {
  const ledger = new Ledger(); let sequence = 0;
  const core = new OfflineTracking(ledger, 'alice', () => Date.parse('2026-10-02T12:00:00Z'), () => `id-${++sequence}`);
  await core.retainHome([{ id: 'guitar', name: 'Guitar', timeZone: 'UTC' }], [{ pathId: 'guitar', summary: { savedTotalSeconds: 60, period: null }, timer: null }], 0);
  const original = { id: 'deleted-entry', owner: 'alice', pathId: 'guitar', startedAt: '2026-10-02T10:00:00Z', endedAt: '2026-10-02T10:01:00Z', timeZone: 'UTC', note: 'old' };
  await core.retainHistory([original], (await core.snapshot()).revision);
  await core.editRecordedActivity(original.id, { startedAt: original.startedAt, durationSeconds: 120, note: 'pending' });
  const other = await core.createManualActivity({ pathId: 'guitar', startedAt: '2026-10-02T11:00:00Z', durationSeconds: 180, note: 'unrelated' });
  await core.replay({ send: async () => { throw new Error('unexpected'); }, sendActivity: async (_owner, operation) => operation.activity.id === original.id
    ? { kind: 'rejected', reason: 'deleted', disclosePath: true } : { kind: 'accepted', activity: operation.activity } });
  assert.deepEqual((await core.localHistory('guitar')).items.map(value => value.id), [other.id]);
  assert.equal((await core.tracking('guitar'))?.savedTotalSeconds, 180);
  assert.equal((await core.snapshot()).notices[0].subject, 'activity');
  await assert.rejects(core.editRecordedActivity(original.id, { startedAt: original.startedAt, durationSeconds: 120, note: '' }), /activity_unavailable/);
});

test('an edit of a pending timer result waits for its recorded identity and updates that same entry', async () => {
  const ledger = new Ledger(); let sequence = 0, now = Date.parse('2026-10-02T11:00:00Z');
  const core = new OfflineTracking(ledger, 'alice', () => now, () => `id-${++sequence}`);
  await core.retainHome([{ id: 'guitar', name: 'Guitar', timeZone: 'UTC' }], [{ pathId: 'guitar', summary: { savedTotalSeconds: 0, period: null }, timer: null }], 0);
  const timer = await core.start('guitar'); now += 60000; await core.stop(timer.id);
  const pending = (await core.pendingHistory())[0]; now += 60000;
  await core.editRecordedActivity(pending.id, { startedAt: pending.startedAt, durationSeconds: 120, note: 'edited stop' });
  const delivery: string[] = [];
  await core.replay({ send: async (_owner, operation) => {
    delivery.push(operation.kind);
    return operation.kind === 'start' ? { kind: 'accepted' } : { kind: 'accepted', activity: { ...pending, id: 'server-entry' } };
  }, sendActivity: async (_owner, operation) => {
    delivery.push(operation.kind); assert.equal(operation.activity.id, 'server-entry');
    return { kind: 'accepted', activity: operation.activity };
  } });
  assert.deepEqual(delivery, ['start', 'stop', 'edit']);
  const items = (await core.localHistory('guitar')).items;
  assert.equal(items.length, 1); assert.equal(items[0].id, 'server-entry'); assert.equal(items[0].note, 'edited stop');
  assert.equal((await core.tracking('guitar'))?.savedTotalSeconds, 120);
});
