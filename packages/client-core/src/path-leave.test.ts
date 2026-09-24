import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyPathLeaveResult,
  createPathLeaveOperationOwner,
  reviewPathLeave,
  type PathLeavePath,
} from './index.js';

const memberPath: PathLeavePath = {
  id: 'path-piano',
  name: 'Piano',
  capabilities: {
    inviteMembers: false,
    leavePath: true,
    manageGoals: false,
    manageLifecycle: false,
    renamePath: false,
    trackTime: true,
    transferOwnership: false,
  },
};

test('leave review defaults to retained activity and records an explicit deletion choice', () => {
  assert.deepEqual(reviewPathLeave(memberPath), {
    pathId: 'path-piano',
    pathName: 'Piano',
    retainActivity: true,
  });
  assert.deepEqual(reviewPathLeave(memberPath, false), {
    pathId: 'path-piano',
    pathName: 'Piano',
    retainActivity: false,
  });
  assert.throws(() => reviewPathLeave({
    ...memberPath,
    capabilities: { ...memberPath.capabilities, leavePath: false },
  }), /Path cannot be left/);
  assert.throws(() => reviewPathLeave({
    ...memberPath,
    capabilities: { ...memberPath.capabilities, trackTime: false },
  }, false), /Path cannot be left/);
});

test('activity deletion owns one exact retry and rejects a mismatched receipt or choice', async () => {
  const owner = createPathLeaveOperationOwner(() => 'leave-delete-key-0001');
  const review = reviewPathLeave(memberPath, false);
  const calls: Array<readonly unknown[]> = [];
  const request = async (...arguments_: Parameters<Parameters<typeof owner.submit>[2]>) => {
    calls.push(arguments_);
    if (calls.length === 1) throw new Error('offline');
    return { pathId: review.pathId, left: true, activityRetained: false };
  };
  assert.equal((await owner.submit(review, true, request)).kind, 'failed');
  await assert.rejects(() => owner.submit(reviewPathLeave(memberPath, true), true, request), /choice changed/);
  assert.deepEqual(await owner.submit(review, true, request), {
    kind: 'applied',
    receipt: { pathId: review.pathId, left: true, activityRetained: false },
  });
  assert.deepEqual(calls, [
    ['path-piano', { confirmed: true, retainActivity: false }, 'leave-delete-key-0001'],
    ['path-piano', { confirmed: true, retainActivity: false }, 'leave-delete-key-0001'],
  ]);

  const mismatch = createPathLeaveOperationOwner(() => 'leave-delete-key-0002');
  assert.equal((await mismatch.submit(review, true, async () => ({ pathId: review.pathId, left: true, activityRetained: true }))).kind, 'failed');
});

test('confirmed leave owns one exact retry and validates the authoritative receipt', async () => {
  const keys = ['leave-request-key-0001', 'leave-request-key-0002'];
  const owner = createPathLeaveOperationOwner(() => keys.shift() ?? 'unexpected');
  const calls: Array<readonly unknown[]> = [];
  const review = reviewPathLeave(memberPath);
  const request = async (...arguments_: Parameters<Parameters<typeof owner.submit>[2]>) => {
    calls.push(arguments_);
    if (calls.length === 1) throw new Error('offline');
    return { pathId: 'path-piano', left: true, activityRetained: true };
  };
  assert.equal((await owner.submit(review, true, request)).kind, 'failed');
  assert.deepEqual(await owner.submit(review, true, request), {
    kind: 'applied',
    receipt: { pathId: 'path-piano', left: true, activityRetained: true },
  });
  assert.deepEqual(calls, [
    ['path-piano', { confirmed: true, retainActivity: true }, 'leave-request-key-0001'],
    ['path-piano', { confirmed: true, retainActivity: true }, 'leave-request-key-0001'],
  ]);
});

test('cancel and stale completion cannot remove a Path', async () => {
  const owner = createPathLeaveOperationOwner(() => 'leave-request-key-0001');
  const review = reviewPathLeave(memberPath);
  assert.deepEqual(await owner.submit(review, false, async () => {
    throw new Error('must not call');
  }), { kind: 'cancelled' });

  let complete!: (value: unknown) => void;
  const pending = owner.submit(review, true, () => new Promise((resolve) => { complete = resolve; }));
  owner.cancel(review.pathId);
  complete({ pathId: review.pathId, left: true, activityRetained: true });
  assert.deepEqual(await pending, { kind: 'superseded' });
});

test('only an applied leave removes active and archived state, timer, and selection', () => {
  const peer = { ...memberPath, id: 'path-reading', name: 'Reading' };
  const state = {
    activePaths: [memberPath, peer],
    archivedPaths: [{ ...memberPath, archivedAt: '2026-07-29T10:00:00Z' }],
    selectedPath: memberPath,
    timerStates: { 'path-piano': { running: false }, 'path-reading': { running: false } },
  };
  const failed = applyPathLeaveResult(state, { kind: 'failed', cause: new Error('offline') });
  assert.strictEqual(failed, state);
  const applied = applyPathLeaveResult(state, {
    kind: 'applied',
    receipt: { pathId: 'path-piano', left: true, activityRetained: true },
  });
  assert.deepEqual(applied.activePaths.map(({ id }) => id), ['path-reading']);
  assert.deepEqual(applied.archivedPaths, []);
  assert.equal(applied.selectedPath, null);
  assert.deepEqual(Object.keys(applied.timerStates), ['path-reading']);
});
