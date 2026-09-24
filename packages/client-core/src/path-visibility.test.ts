import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyPathVisibilityResult,
  comparePathVisibility,
  createPathVisibilityOperationOwner,
  pathVisibilityOptions,
  pathVisibilityFromAPI,
  reviewPathVisibilityChange,
  type PathVisibility,
} from './index.js';

const capabilities = Object.freeze({ manageVisibility: true });
type TestProjection = Readonly<{ id: string; name: string; visibility: PathVisibility; capabilities: object; archivedAt?: string }>;
const reading: TestProjection = Object.freeze({ id: 'path-1', name: 'Reading', visibility: 'private', capabilities });

test('visibility ordering and profile constraints expose only approved audiences', () => {
  assert.equal(pathVisibilityFromAPI('followers'), 'followers');
  assert.throws(() => pathVisibilityFromAPI('members'), /invalid Path visibility/);
  assert.deepEqual(pathVisibilityOptions('private'), ['private', 'followers']);
  assert.deepEqual(pathVisibilityOptions('public'), ['private', 'followers', 'public']);
  assert.ok(comparePathVisibility('private', 'followers') < 0);
  assert.ok(comparePathVisibility('followers', 'public') < 0);
  assert.equal(comparePathVisibility('public', 'public'), 0);
});

test('review distinguishes unchanged, forbidden, broader, and narrowing transitions', () => {
  assert.deepEqual(reviewPathVisibilityChange(reading, 'private', 'public'), {
    kind: 'unchanged', pathId: 'path-1', pathName: 'Reading', current: 'private', proposed: 'private',
  });
  assert.deepEqual(reviewPathVisibilityChange(reading, 'public', 'private'), {
    kind: 'not-permitted', pathId: 'path-1', pathName: 'Reading', current: 'private', proposed: 'public',
  });
  assert.deepEqual(reviewPathVisibilityChange(reading, 'followers', 'private'), {
    kind: 'ready', pathId: 'path-1', pathName: 'Reading', current: 'private', proposed: 'followers', broader: true,
  });
  assert.deepEqual(reviewPathVisibilityChange({ ...reading, visibility: 'public' }, 'private', 'public'), {
    kind: 'ready', pathId: 'path-1', pathName: 'Reading', current: 'public', proposed: 'private', broader: false,
  });
});

test('operation owner requires explicit submission and retains one key across identical retry', async () => {
  const keys = ['visibility-key-0001', 'visibility-key-0002'];
  const owner = createPathVisibilityOperationOwner(() => keys.shift()!);
  const review = reviewPathVisibilityChange(reading, 'followers', 'public');
  assert.equal(review.kind, 'ready');
  const calls: Array<{ body: unknown; key: string; pathId: string }> = [];
  const request = async (pathId: string, body: unknown, key: string) => {
    calls.push({ pathId, body, key });
    if (calls.length === 1) throw new Error('offline');
    return { ...reading, visibility: 'followers' as const };
  };

  assert.deepEqual(await owner.submit(review, false, request), { kind: 'cancelled' });
  assert.equal(calls.length, 0);
  assert.equal((await owner.submit(review, true, request)).kind, 'failed');
  const applied = await owner.submit(review, true, request);
  assert.equal(applied.kind, 'applied');
  assert.deepEqual(calls, [
    {
      pathId: 'path-1',
      body: { confirmed: true, expectedVisibility: 'private', visibility: 'followers' },
      key: 'visibility-key-0001',
    },
    {
      pathId: 'path-1',
      body: { confirmed: true, expectedVisibility: 'private', visibility: 'followers' },
      key: 'visibility-key-0001',
    },
  ]);
  assert.equal(Object.isFrozen(calls[0]!.body), true);
});

test('changed intent gets a new key and invalid or stale projections never apply', async () => {
  const keys = ['visibility-key-0001', 'visibility-key-0002', 'visibility-key-0003'];
  const owner = createPathVisibilityOperationOwner(() => keys.shift()!);
  const first = reviewPathVisibilityChange(reading, 'followers', 'public');
  const second = reviewPathVisibilityChange(reading, 'public', 'public');
  assert.equal(first.kind, 'ready');
  assert.equal(second.kind, 'ready');
  assert.equal((await owner.submit(first, true, async (): Promise<TestProjection> => { throw new Error('offline'); })).kind, 'failed');
  let secondKey = '';
  const mismatch = await owner.submit(second, true, async (_path, _body, key) => {
    secondKey = key;
    return { ...reading, id: 'path-other', visibility: 'public' as const };
  });
  assert.equal(secondKey, 'visibility-key-0002');
  assert.equal(mismatch.kind, 'failed');

  let resolve!: (value: TestProjection) => void;
  const pending = owner.submit(first, true, async () => new Promise<TestProjection>((next) => { resolve = next; }));
  owner.cancel('path-1');
  resolve({ ...reading, visibility: 'followers' });
  assert.deepEqual(await pending, { kind: 'superseded' });
});

test('authoritative result replaces the full useful projection in every local Path reference', () => {
  const other = { id: 'path-2', name: 'Piano', visibility: 'followers' as const, capabilities };
  const authoritative = {
    ...reading,
    name: 'Reading together',
    visibility: 'followers' as const,
    overallTarget: { targetSeconds: 3600 },
  };
  const applied = applyPathVisibilityResult<TestProjection>(
    { activePaths: [reading, other], archivedPaths: [], selectedPath: reading },
    authoritative,
  );
  assert.deepEqual(applied.activePaths, [authoritative, other]);
  assert.strictEqual(applied.selectedPath, authoritative);
  assert.throws(() => applyPathVisibilityResult<TestProjection>(
    { activePaths: [reading], archivedPaths: [], selectedPath: reading },
    { ...authoritative, id: 'missing' },
  ), /authoritative Path/);
  assert.throws(() => applyPathVisibilityResult<TestProjection>(
    { activePaths: [], archivedPaths: [{ ...reading, archivedAt: '2026-08-08T20:00:00Z' }], selectedPath: null },
    { ...authoritative, archivedAt: '2026-08-08T20:00:00Z' },
  ), /active Path/);
});
