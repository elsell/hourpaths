import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyPathArchiveResult,
  createPathArchiveOperationOwner,
  pathArchiveLists,
  reviewPathArchiveChange,
  type PathArchiveMutationResult,
} from './index.js';

type TestPath = {
  id: string;
  name: string;
  archivedAt?: string | null;
  capabilities: {
    trackTime: boolean;
    renamePath: boolean;
    inviteMembers: boolean;
    manageGoals: boolean;
    manageLifecycle: boolean;
    transferOwnership: boolean;
  };
};

type TestTimer = {
  running: boolean;
  accumulatedSeconds: number;
  timer?: { id: string; startedAt: string };
};

const creatorCapabilities = {
  trackTime: true,
  renamePath: true,
  inviteMembers: true,
  manageGoals: true,
  manageLifecycle: true,
  transferOwnership: false,
};
const activePath: TestPath = { id: 'path-1', name: 'Read', capabilities: creatorCapabilities };
const otherActivePath: TestPath = { id: 'path-2', name: 'Piano', archivedAt: null, capabilities: creatorCapabilities };
const archivedPath: TestPath = {
  id: 'path-3',
  name: 'Run',
  archivedAt: '2026-07-23T15:00:00Z',
  capabilities: { ...creatorCapabilities, trackTime: false, inviteMembers: false, manageGoals: false },
};

test('active and archived Paths are projected into distinct lists', () => {
  const paths = [activePath, archivedPath, otherActivePath];

  const lists = pathArchiveLists(paths);

  assert.deepEqual(lists.activePaths, [activePath, otherActivePath]);
  assert.deepEqual(lists.archivedPaths, [archivedPath]);
  assert.notStrictEqual(lists.activePaths, paths);
  assert.notStrictEqual(lists.archivedPaths, paths);
});

test('an authoritative archive moves the Path and stops only its timer', () => {
  const selectedPath = activePath;
  const selectedTimer: TestTimer = {
    running: true,
    accumulatedSeconds: 90,
    timer: { id: 'timer-1', startedAt: '2026-07-23T14:59:00Z' },
  };
  const untouchedTimer: TestTimer = {
    running: true,
    accumulatedSeconds: 20,
    timer: { id: 'timer-2', startedAt: '2026-07-23T14:59:30Z' },
  };
  const state = {
    activePaths: [activePath, otherActivePath],
    archivedPaths: [archivedPath],
    selectedPath,
    timerStates: {
      'path-1': selectedTimer,
      'path-2': untouchedTimer,
    },
    unrelated: { retained: true },
  };
  const result = {
    ...activePath,
    archivedAt: '2026-07-23T15:00:00Z',
  };

  const next = applyPathArchiveResult(state, result);

  assert.deepEqual(next.activePaths, [otherActivePath]);
  assert.deepEqual(next.archivedPaths, [archivedPath, result]);
  assert.strictEqual(next.selectedPath, result);
  assert.deepEqual(next.timerStates['path-1'], {
    running: false,
    accumulatedSeconds: 90,
    timer: undefined,
  });
  assert.strictEqual(next.timerStates['path-2'], untouchedTimer);
  assert.strictEqual(next.unrelated, state.unrelated);
  assert.equal(state.timerStates['path-1'].running, true);
});

test('unarchive returns the Path to active without creating or restarting a timer', () => {
  const stoppedTimer: TestTimer = { running: false, accumulatedSeconds: 120 };
  const state = {
    activePaths: [activePath],
    archivedPaths: [archivedPath],
    selectedPath: archivedPath,
    timerStates: { 'path-3': stoppedTimer },
  };
  const result = { id: 'path-3', name: 'Run', archivedAt: null, capabilities: creatorCapabilities };

  const next = applyPathArchiveResult(state, result);

  assert.deepEqual(next.activePaths, [activePath, result]);
  assert.deepEqual(next.archivedPaths, []);
  assert.strictEqual(next.selectedPath, result);
  assert.strictEqual(next.timerStates['path-3'], stoppedTimer);

  const withoutTimer = applyPathArchiveResult(
    { ...state, timerStates: {} as Record<string, TestTimer> },
    result,
  );
  assert.deepEqual(withoutTimer.timerStates, {});
});

test('archive review captures the expected state and requires explicit confirmation', async () => {
  const owner = createPathArchiveOperationOwner(() => 'archive-key-00001');
  const review = reviewPathArchiveChange(activePath);
  let requests = 0;

  assert.deepEqual(review, {
    pathId: 'path-1',
    expectedArchived: false,
    archived: true,
  });
  assert.equal(Object.isFrozen(review), true);
  assert.deepEqual(
    await owner.submit(review, false, async () => {
      requests += 1;
      return { ...activePath, archivedAt: '2026-07-23T15:00:00Z' };
    }),
    { kind: 'cancelled' },
  );
  assert.equal(requests, 0);
});

test('a failed confirmed change retries with the same idempotency key and body', async () => {
  const keys = ['archive-key-00001', 'next-key-00000001'];
  const owner = createPathArchiveOperationOwner(() => keys.shift()!);
  const review = reviewPathArchiveChange(activePath);
  const calls: Array<{ key: string; body: unknown }> = [];
  const failure = new Error('offline');
  const request = async (
    _pathId: string,
    body: { confirmed: true; expectedArchived: boolean; archived: boolean },
    idempotencyKey: string,
  ): Promise<TestPath> => {
    calls.push({ key: idempotencyKey, body });
    if (calls.length === 1) throw failure;
    return { ...activePath, archivedAt: '2026-07-23T15:00:00Z' };
  };

  assert.deepEqual(await owner.submit(review, true, request), { kind: 'failed', cause: failure });
  const result = await owner.submit(review, true, request);

  assert.equal(result.kind, 'applied');
  assert.deepEqual(calls.map(({ key }) => key), ['archive-key-00001', 'archive-key-00001']);
  assert.strictEqual(calls[0]?.body, calls[1]?.body);
  assert.deepEqual(calls[0]?.body, {
    confirmed: true,
    expectedArchived: false,
    archived: true,
  });

  const unarchive = reviewPathArchiveChange({
    ...activePath,
    archivedAt: '2026-07-23T15:00:00Z',
  });
  await owner.submit(unarchive, true, async (_pathId, body, key) => {
    assert.equal(key, 'next-key-00000001');
    assert.deepEqual(body, { confirmed: true, expectedArchived: true, archived: false });
    return { ...activePath, archivedAt: null };
  });
});

test('new intent supersedes stale completion per Path and cancel causes no operation', async () => {
  const keys = ['archive-key-00001', 'unarchive-key-001', 'other-key-0000001'];
  const owner = createPathArchiveOperationOwner(() => keys.shift()!);
  let resolveArchive!: (path: TestPath) => void;
  const archivePending = new Promise<TestPath>((resolve) => { resolveArchive = resolve; });
  const first = owner.submit(
    reviewPathArchiveChange(activePath),
    true,
    async () => archivePending,
  );
  const secondReview = reviewPathArchiveChange({
    ...activePath,
    archivedAt: '2026-07-23T15:00:00Z',
  });
  const second = owner.submit(secondReview, true, async () => ({ ...activePath, archivedAt: null }));
  const other = owner.submit(
    reviewPathArchiveChange(otherActivePath),
    true,
    async () => ({ ...otherActivePath, archivedAt: '2026-07-23T15:00:00Z' }),
  );

  assert.equal((await second).kind, 'applied');
  assert.equal((await other).kind, 'applied');
  resolveArchive({ ...activePath, archivedAt: '2026-07-23T15:00:00Z' });
  assert.deepEqual(await first, { kind: 'superseded' });

  let cancelledRequest = false;
  owner.cancel('path-1');
  const cancelled: PathArchiveMutationResult<TestPath> = await owner.submit(
    reviewPathArchiveChange(activePath),
    false,
    async () => {
      cancelledRequest = true;
      return activePath;
    },
  );
  assert.deepEqual(cancelled, { kind: 'cancelled' });
  assert.equal(cancelledRequest, false);
});

test('generated archive idempotency keys must be bounded printable ASCII', async () => {
  const owner = createPathArchiveOperationOwner(() => 'bad\nkey');

  await assert.rejects(
    owner.submit(reviewPathArchiveChange(activePath), true, async () => activePath),
    /idempotency/i,
  );
});
