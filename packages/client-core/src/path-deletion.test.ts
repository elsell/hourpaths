import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyPathDeletionResult,
  createPathDeletionOperationOwner,
  reviewPathDeletion,
  type PathDeletionMutationResult,
} from './index.js';

type TestPath = {
  id: string;
  name: string;
  archivedAt?: string;
  capabilities: {
    trackTime: boolean;
    renamePath: boolean;
    inviteMembers: boolean;
    manageGoals: boolean;
    manageLifecycle: boolean;
    transferOwnership: boolean;
  };
};

const creatorCapabilities = {
  trackTime: true,
  renamePath: true,
  inviteMembers: true,
  manageGoals: true,
  manageLifecycle: true,
  transferOwnership: true,
};
const activePath: TestPath = { id: 'path-1', name: 'Read', capabilities: creatorCapabilities };
const archivedPath: TestPath = {
  id: 'path-2',
  name: 'Run',
  archivedAt: '2026-07-27T12:00:00Z',
  capabilities: { ...creatorCapabilities, trackTime: false, inviteMembers: false, manageGoals: false },
};
const otherPath: TestPath = { id: 'path-3', name: 'Piano', capabilities: creatorCapabilities };

test('deletion review is frozen and bound to the Path id and canonical name', () => {
  const review = reviewPathDeletion(activePath);

  assert.deepEqual(review, { pathId: 'path-1', expectedName: 'Read' });
  assert.equal(Object.isFrozen(review), true);
  assert.throws(() => reviewPathDeletion({ ...activePath, id: ' path-1' }), /invalid Path deletion review/);
  assert.throws(() => reviewPathDeletion({ ...activePath, name: ' Read ' }), /invalid Path deletion review/);
});

test('deletion requires explicit confirmation and cancellation issues no request', async () => {
  const owner = createPathDeletionOperationOwner(() => 'delete-key-000001');
  let requested = false;

  const result = await owner.submit(reviewPathDeletion(activePath), false, async () => {
    requested = true;
    return { pathId: activePath.id, deleted: true };
  });

  assert.deepEqual(result, { kind: 'cancelled' });
  assert.equal(requested, false);
});

test('a failed confirmed deletion retries with the same frozen body and idempotency key', async () => {
  const keys = ['delete-key-000001', 'unused-key-000001'];
  const owner = createPathDeletionOperationOwner(() => keys.shift()!);
  const calls: Array<{ body: unknown; key: string }> = [];
  const failure = new Error('offline');
  const request = async (_pathId: string, body: unknown, key: string) => {
    calls.push({ body, key });
    if (calls.length === 1) throw failure;
    return { pathId: activePath.id, deleted: true as const };
  };

  assert.deepEqual(await owner.submit(reviewPathDeletion(activePath), true, request), {
    kind: 'failed',
    cause: failure,
  });
  assert.deepEqual(await owner.submit(reviewPathDeletion(activePath), true, request), {
    kind: 'applied',
    deletion: { pathId: 'path-1', deleted: true },
  });
  assert.deepEqual(calls.map(({ key }) => key), ['delete-key-000001', 'delete-key-000001']);
  assert.strictEqual(calls[0]?.body, calls[1]?.body);
  assert.equal(Object.isFrozen(calls[0]?.body), true);
  assert.deepEqual(calls[0]?.body, { confirmed: true, expectedName: 'Read' });
});

test('cancellation and newer deletion intent supersede stale completion', async () => {
  const owner = createPathDeletionOperationOwner(() => 'delete-key-000001');
  let resolveFirst!: (value: unknown) => void;
  const pending = new Promise<unknown>((resolve) => { resolveFirst = resolve; });
  const first = owner.submit(reviewPathDeletion(activePath), true, async () => pending);

  owner.cancel(activePath.id);
  resolveFirst({ pathId: activePath.id, deleted: true });
  assert.deepEqual(await first, { kind: 'superseded' });

  let resolveStale!: (value: unknown) => void;
  const stale = owner.submit(reviewPathDeletion(activePath), true, async () =>
    new Promise((resolve) => { resolveStale = resolve; }));
  const current = owner.submit(reviewPathDeletion(activePath), true, async () => ({
    pathId: activePath.id,
    deleted: true,
  }));
  assert.equal((await current).kind, 'applied');
  resolveStale({ pathId: activePath.id, deleted: true });
  assert.deepEqual(await stale, { kind: 'superseded' });
});

test('malformed or mismatched authoritative deletion results fail closed', async () => {
  const results: unknown[] = [
    { pathId: 'other-path', deleted: true },
    { pathId: activePath.id, deleted: false },
    { pathId: activePath.id, deleted: true, unexpected: true },
    null,
  ];

  for (const authoritative of results) {
    const owner = createPathDeletionOperationOwner(() => 'delete-key-000001');
    const result = await owner.submit(
      reviewPathDeletion(activePath),
      true,
      async () => authoritative,
    );
    assert.equal(result.kind, 'failed');
    assert.match(String(result.kind === 'failed' ? result.cause : ''), /invalid Path deletion result/);
  }
});

test('only an applied deletion removes its Path, timer, and selection from pure client state', () => {
  const timer = { running: true, accumulatedSeconds: 90 };
  const otherTimer = { running: true, accumulatedSeconds: 30 };
  const state = {
    activePaths: [activePath, otherPath],
    archivedPaths: [archivedPath],
    selectedPath: activePath,
    timerStates: { 'path-1': timer, 'path-3': otherTimer },
    unrelated: { retained: true },
  };
  const failed: PathDeletionMutationResult = { kind: 'failed', cause: new Error('offline') };

  assert.strictEqual(applyPathDeletionResult(state, failed), state);

  const next = applyPathDeletionResult(state, {
    kind: 'applied',
    deletion: { pathId: activePath.id, deleted: true },
  });
  assert.deepEqual(next.activePaths, [otherPath]);
  assert.deepEqual(next.archivedPaths, [archivedPath]);
  assert.equal(next.selectedPath, null);
  assert.deepEqual(next.timerStates, { 'path-3': otherTimer });
  assert.strictEqual(next.unrelated, state.unrelated);
  assert.deepEqual(state.activePaths, [activePath, otherPath]);
  assert.strictEqual(state.timerStates['path-1'], timer);

  const archivedState = { ...state, activePaths: [otherPath], selectedPath: archivedPath };
  const withoutArchived = applyPathDeletionResult(archivedState, {
    kind: 'applied',
    deletion: { pathId: archivedPath.id, deleted: true },
  });
  assert.deepEqual(withoutArchived.archivedPaths, []);
  assert.equal(withoutArchived.selectedPath, null);
  assert.strictEqual(withoutArchived.timerStates, archivedState.timerStates);
});
