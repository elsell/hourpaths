import test from 'node:test';
import assert from 'node:assert/strict';
import { AccountExportController, collectAccountExport, type AccountExportRepository, type ExportPath } from './account-export';
import { OfflineTracking, type TrackingSnapshot, type TrackingStore } from './offline-tracking';

const now = Date.parse('2026-10-09T12:00:00Z'), instant = new Date(now).toISOString();
const profile = { id: 'alice', email: 'alice@example.test', username: 'alice', displayName: 'Alice', description: '', visibility: 'private', timeZone: 'Etc/UTC', firstDayOfWeek: 1, createdAt: instant, updatedAt: instant };
const path: ExportPath = { id: 'guitar', name: 'Guitar', visibility: 'private', createdAt: instant, updatedAt: instant, archivedAt: null, intervalGoal: null, overallTargetSeconds: null };
const entry = { id: 'entry', pathId: path.id, participantId: 'alice', startedAt: instant, endedAt: instant, timeZone: 'Etc/UTC', note: 'Own note', createdAt: instant, updatedAt: instant, version: 1 };
function repository(overrides: Partial<AccountExportRepository> = {}): AccountExportRepository {
  return { profile: async () => profile, paths: async archived => ({ items: archived ? [] : [path], nextCursor: '' }), activities: async () => ({ items: [entry], nextCursor: '' }), ...overrides };
}

test('export follows all pages, includes archived Paths and durable pending work without exposing unavailable Paths', async () => {
  let snapshot: TrackingSnapshot | null = null;
  const store: TrackingStore = { read: async () => structuredClone(snapshot), commit: async (_owner, revision, value) => {
    if ((snapshot?.revision ?? 0) !== revision) return false; snapshot = structuredClone(value); return true;
  } };
  let id = 0;
  const local = new OfflineTracking(store, 'alice', () => now, () => 'local-' + ++id);
  await local.retainPaths([{ id: 'guitar', name: 'Guitar', timeZone: 'Etc/UTC' }, { id: 'lost', name: 'Private lost Path', timeZone: 'Etc/UTC' }]);
  await local.start('guitar'); await local.start('lost');
  const requests: string[] = [];
  const repo = repository({
    paths: async (archived, cursor) => {
      requests.push(String(archived) + ':' + cursor);
      return archived ? { items: [{ ...path, id: 'archived', archivedAt: instant }], nextCursor: '' }
        : cursor ? { items: [], nextCursor: '' } : { items: [path], nextCursor: 'next' };
    },
    activities: async pathId => ({ items: [{ ...entry, id: pathId + '-entry', pathId }], nextCursor: '' }),
  });
  const result = await collectAccountExport({ owner: 'alice', repository: repo, current: () => true, now: () => now, device: () => store.read('alice') });
  assert.deepEqual(requests, ['false:', 'false:next', 'true:']);
  assert.equal(result.paths.length, 2); assert.equal(result.activities.length, 2);
  assert.equal(result.device.runningTimers.length, 1); assert.equal(result.device.queuedTimers.length, 1);
  assert.equal(result.device.excludedItemCount, 2);
  assert.equal(JSON.stringify(result).includes('Private lost Path'), false);
  assert.equal((await store.read('alice'))?.timers.length, 2);
});

test('cross-account results and cursor loops abort export rather than producing a partial file', async () => {
  const input = { owner: 'alice', current: () => true, now: () => now, device: async () => null };
  await assert.rejects(collectAccountExport({ ...input, repository: repository({ activities: async () => ({ items: [{ ...entry, participantId: 'bob' }], nextCursor: '' }) }) }), /account_changed/);
  await assert.rejects(collectAccountExport({ ...input, repository: repository({ paths: async () => ({ items: [], nextCursor: 'repeated' }) }) }), /invalid/);
});

test('account replacement during export cannot send an old account document to the save adapter', async () => {
  let owner = 'alice', finish!: (value: typeof profile) => void, saves = 0;
  const controller = new AccountExportController({ owner: () => owner, current: () => true, now: () => now,
    device: async () => null, repository: repository({ profile: () => new Promise(resolve => { finish = resolve; }) }),
    sink: { save: async () => { saves++; return 'saved'; } },
  });
  const work = controller.save();
  owner = 'bob'; finish(profile); await work;
  assert.equal(saves, 0);
  controller.dispose();
});
