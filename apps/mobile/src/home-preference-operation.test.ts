import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHomePreferenceOperationOwner,
  homePreferencesFromAPI,
  type HomePreferenceSnapshot,
} from './home-preference-operation';

const current: HomePreferenceSnapshot = {
  manualPathIds: ['path-a', 'path-b'],
  orderMethod: 'recent',
  pinnedPathIds: [],
  revision: 2,
  updatedAt: '2026-08-04T18:00:00Z',
};

test('Home preference admission rejects extra, malformed, and contradictory fields', () => {
  assert.deepEqual(homePreferencesFromAPI(current), current);
  assert.deepEqual(homePreferencesFromAPI({
    manualPathIds: current.manualPathIds,
    orderMethod: 'recent',
    pinnedPathIds: [],
    revision: 0,
  }), {
    manualPathIds: current.manualPathIds,
    orderMethod: 'recent',
    pinnedPathIds: [],
    revision: 0,
  });
  assert.throws(() => homePreferencesFromAPI({ ...current, extra: true }));
  assert.throws(() => homePreferencesFromAPI({ ...current, revision: -1 }));
  assert.throws(() => homePreferencesFromAPI({ ...current, pinnedPathIds: ['path-c'] }));
  assert.throws(() => homePreferencesFromAPI({ ...current, manualPathIds: ['path-a', 'path-a'] }));
  assert.throws(() => homePreferencesFromAPI({ ...current, updatedAt: 'today' }));
});

test('only the newest preference mutation may replace Home and retries retain their key', async () => {
  const keys = ['preference-key-0001', 'preference-key-0002', 'preference-key-0003'];
  const owner = createHomePreferenceOperationOwner(() => keys.shift()!);
  let resolveFirst!: (value: HomePreferenceSnapshot) => void;
  const calls: Array<[number, string]> = [];
  const first = owner.submit(current, { ...current, orderMethod: 'alphabetical' }, (body, key) => {
    calls.push([body.expectedRevision, key]);
    return new Promise((resolve) => { resolveFirst = resolve; });
  });
  const second = await owner.submit(current, { ...current, pinnedPathIds: ['path-a'] }, async (body, key) => {
    calls.push([body.expectedRevision, key]);
    return { ...current, pinnedPathIds: ['path-a'], revision: 3 };
  });
  resolveFirst({ ...current, orderMethod: 'alphabetical', revision: 3 });

  assert.equal((await first).kind, 'superseded');
  assert.deepEqual(second, { kind: 'applied', preferences: { ...current, pinnedPathIds: ['path-a'], revision: 3 } });
  assert.deepEqual(calls, [[2, 'preference-key-0001'], [2, 'preference-key-0002']]);

  const failed = await owner.submit(second.preferences, { ...second.preferences, orderMethod: 'manual' }, async () => {
    throw new Error('offline');
  });
  assert.equal(failed.kind, 'failed');
  const retriedKeys: string[] = [];
  await owner.submit(second.preferences, { ...second.preferences, orderMethod: 'manual' }, async (_body, key) => {
    retriedKeys.push(key);
    return { ...second.preferences, orderMethod: 'manual', revision: 4 };
  });
  assert.deepEqual(retriedKeys, ['preference-key-0003']);
});
