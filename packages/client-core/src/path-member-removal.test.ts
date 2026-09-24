import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyPathMemberRemovalResult,
  createPathMemberRemovalOperationOwner,
  reviewPathMemberRemoval,
  type PathMemberRemovalReview,
} from './index.js';

const participantReview: PathMemberRemovalReview = {
  displayName: 'Avery Chen',
  pathId: 'path-piano',
  role: 'participant',
  runningTimer: true,
  sessionCount: 3,
  totalTrackedSeconds: 7_200,
  userId: 'user-avery',
  username: 'avery',
};

test('member removal review retains only authoritative ordinary-member evidence', () => {
  assert.deepEqual(reviewPathMemberRemoval(participantReview), participantReview);
  for (const role of ['creator', 'administrator'] as const) {
    assert.throws(() => reviewPathMemberRemoval({ ...participantReview, role }), /ordinary Path member/);
  }
  assert.throws(() => reviewPathMemberRemoval({ ...participantReview, sessionCount: -1 }), /removal review/);
  assert.throws(() => reviewPathMemberRemoval({ ...participantReview, totalTrackedSeconds: 1.5 }), /removal review/);
  assert.throws(() => reviewPathMemberRemoval({ ...participantReview, username: ' avery' }), /removal review/);
});

test('confirmed removal owns one role-bound retry and validates the exact receipt', async () => {
  const keys = ['remove-member-key-0001', 'remove-member-key-0002'];
  const owner = createPathMemberRemovalOperationOwner(() => keys.shift() ?? 'unexpected');
  const calls: Array<readonly unknown[]> = [];
  const request = async (...arguments_: Parameters<Parameters<typeof owner.submit>[2]>) => {
    calls.push(arguments_);
    if (calls.length === 1) throw new Error('offline');
    return { activityDeleted: true, pathId: 'path-piano', removed: true, userId: 'user-avery' };
  };

  assert.equal((await owner.submit(participantReview, true, request)).kind, 'failed');
  assert.deepEqual(await owner.submit(participantReview, true, request), {
    kind: 'applied',
    receipt: { activityDeleted: true, pathId: 'path-piano', removed: true, userId: 'user-avery' },
  });
  assert.deepEqual(calls, [
    ['path-piano', 'user-avery', { confirmed: true, expectedRole: 'participant' }, 'remove-member-key-0001'],
    ['path-piano', 'user-avery', { confirmed: true, expectedRole: 'participant' }, 'remove-member-key-0001'],
  ]);
});

test('supporter removal requires an access-only receipt', async () => {
  const review = { ...participantReview, role: 'supporter' as const, runningTimer: false, sessionCount: 0, totalTrackedSeconds: 0 };
  const owner = createPathMemberRemovalOperationOwner(() => 'remove-supporter-key-01');
  assert.deepEqual(await owner.submit(review, true, async (_pathId, _userId, body) => {
    assert.deepEqual(body, { confirmed: true, expectedRole: 'supporter' });
    return { activityDeleted: false, pathId: review.pathId, removed: true, userId: review.userId };
  }), {
    kind: 'applied',
    receipt: { activityDeleted: false, pathId: review.pathId, removed: true, userId: review.userId },
  });
});

test('cancel and stale completion cannot remove a member', async () => {
  const owner = createPathMemberRemovalOperationOwner(() => 'remove-member-key-0001');
  assert.deepEqual(await owner.submit(participantReview, false, async () => {
    throw new Error('must not call');
  }), { kind: 'cancelled' });

  let complete!: (value: unknown) => void;
  const pending = owner.submit(participantReview, true, () => new Promise((resolve) => { complete = resolve; }));
  owner.cancel(participantReview.pathId, participantReview.userId);
  complete({ activityDeleted: true, pathId: participantReview.pathId, removed: true, userId: participantReview.userId });
  assert.deepEqual(await pending, { kind: 'superseded' });
});

test('only an applied exact result removes the selected member row', () => {
  const supporter = {
    ...participantReview,
    displayName: 'Sam Rivera',
    role: 'supporter' as const,
    runningTimer: false,
    sessionCount: 0,
    totalTrackedSeconds: 0,
    userId: 'user-sam',
    username: 'sam',
  };
  const state = { members: [participantReview, supporter], selected: participantReview };
  assert.strictEqual(applyPathMemberRemovalResult(state, { kind: 'failed', cause: new Error('offline') }), state);
  assert.deepEqual(applyPathMemberRemovalResult(state, {
    kind: 'applied',
    receipt: { activityDeleted: true, pathId: participantReview.pathId, removed: true, userId: participantReview.userId },
  }), { members: [supporter], selected: null });
});
