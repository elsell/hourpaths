import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyPathMemberRoleChangeResult,
  createPathMemberRoleChangeOperationOwner,
  reviewPathMemberRoleChange,
} from './path-member-role-change';

const participant = {
  displayName: 'Avery', pathId: 'path-piano', role: 'participant' as const,
  runningTimer: true, sessionCount: 3, totalTrackedSeconds: 7200,
  userId: 'user-avery', username: 'avery',
};

test('only supported Path member role transitions can be reviewed', () => {
  assert.equal(reviewPathMemberRoleChange(participant, 'supporter').role, 'supporter');
  assert.equal(reviewPathMemberRoleChange(participant, 'administrator').role, 'administrator');
  assert.equal(reviewPathMemberRoleChange({ ...participant, role: 'administrator' }, 'participant').role, 'participant');
  assert.throws(() => reviewPathMemberRoleChange(participant, 'participant'), /different role/);
  assert.throws(() => reviewPathMemberRoleChange({ ...participant, role: 'supporter' }, 'administrator'), /supported Path member role/);
  assert.throws(() => reviewPathMemberRoleChange({ ...participant, role: 'administrator' }, 'supporter'), /supported Path member role/);
  assert.throws(() => reviewPathMemberRoleChange({ ...participant, role: 'creator' }, 'supporter'), /supported Path member role/);
});

test('administrator transitions preserve tracked activity in the authoritative receipt', async () => {
  const owner = createPathMemberRoleChangeOperationOwner(() => 'role-change-key-0002');
  const review = reviewPathMemberRoleChange(participant, 'administrator');
  const result = await owner.submit(review, true, async () => ({
    activityDeleted: false,
    pathId: participant.pathId,
    role: 'administrator',
    userId: participant.userId,
  }));
  assert.deepEqual(result, {
    kind: 'applied',
    receipt: { activityDeleted: false, pathId: participant.pathId, role: 'administrator', userId: participant.userId },
  });
});

test('role change freezes expected and target roles under one retry key', async () => {
  const requests: unknown[][] = [];
  const owner = createPathMemberRoleChangeOperationOwner(() => 'role-change-key-0001');
  const review = reviewPathMemberRoleChange(participant, 'supporter');
  const request = async (...args: Parameters<Parameters<typeof owner.submit>[2]>) => {
    requests.push(args);
    if (requests.length === 1) throw new Error('offline');
    return { activityDeleted: true, pathId: participant.pathId, role: 'supporter', userId: participant.userId };
  };
  assert.equal((await owner.submit(review, true, request)).kind, 'failed');
  assert.equal((await owner.submit(review, true, request)).kind, 'applied');
  assert.deepEqual(requests, [
    [participant.pathId, participant.userId, { confirmed: true, expectedRole: 'participant', role: 'supporter' }, 'role-change-key-0001'],
    [participant.pathId, participant.userId, { confirmed: true, expectedRole: 'participant', role: 'supporter' }, 'role-change-key-0001'],
  ]);
});

test('cancel and stale completion cannot apply a destructive role change', async () => {
  const owner = createPathMemberRoleChangeOperationOwner(() => 'role-change-key-0001');
  const review = reviewPathMemberRoleChange(participant, 'supporter');
  assert.equal((await owner.submit(review, false, async () => ({}))).kind, 'cancelled');
  let complete!: (value: unknown) => void;
  const pending = owner.submit(review, true, () => new Promise((resolve) => { complete = resolve; }));
  owner.cancel(participant.pathId, participant.userId);
  complete({ activityDeleted: true, pathId: participant.pathId, role: 'supporter', userId: participant.userId });
  assert.equal((await pending).kind, 'superseded');
});

test('authoritative receipt changes only the exact roster member and clears deleted aggregates', () => {
  const result = {
    kind: 'applied' as const,
    receipt: { activityDeleted: true, pathId: participant.pathId, role: 'supporter' as const, userId: participant.userId },
  };
  assert.deepEqual(applyPathMemberRoleChangeResult({ members: [
    { ...participant, canChangeRole: true },
    { ...participant, userId: 'other' },
  ] }, result), { members: [
    { ...participant, canChangeRole: true, role: 'supporter', runningTimer: false, sessionCount: 0, totalTrackedSeconds: 0 },
    { ...participant, userId: 'other' },
  ] });
});
