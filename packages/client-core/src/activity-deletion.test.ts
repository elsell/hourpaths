import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyActivityDeletionResult,
  createActivityDeletionOperationOwner,
} from './index.js';

const identity = {
  activityId: 'activity-1',
  ownerId: 'profile-1',
  pathId: 'path-1',
  sessionToken: 'session-1',
};

test('activity deletion requires confirmation and retries with one idempotency key', async () => {
  const keys = ['activity-delete-key-0001', 'unused-key-0001'];
  const owner = createActivityDeletionOperationOwner(() => keys.shift()!);
  const calls: string[] = [];

  assert.deepEqual(await owner.submit(identity, false, async () => {
    throw new Error('must not request');
  }), { kind: 'cancelled' });

  const request = async (key: string) => {
    calls.push(key);
    if (calls.length === 1) throw new Error('offline');
    return {
      accumulatedSeconds: 120,
      intervalProgress: { accumulatedSeconds: 30, targetSeconds: 60 },
      removedFeedEventIds: ['practice:activity-1', 'achievement:interval-1'],
      sessionCount: 2,
      unreadNotificationCount: 7,
    };
  };
  assert.equal((await owner.submit(identity, true, request)).kind, 'failed');
  assert.equal((await owner.submit(identity, true, request)).kind, 'applied');
  assert.deepEqual(calls, ['activity-delete-key-0001', 'activity-delete-key-0001']);
});

test('activity deletion rejects double submit and stale completion across every identity boundary', async () => {
  let keyNumber = 0;
  const owner = createActivityDeletionOperationOwner(() => `activity-delete-key-${String(++keyNumber).padStart(4, '0')}`);
  let resolve!: (value: unknown) => void;
  const first = owner.submit(identity, true, async () => new Promise((done) => { resolve = done; }));

  assert.deepEqual(await owner.submit(identity, true, async () => ({ accumulatedSeconds: 0 })), { kind: 'busy' });
  owner.invalidate();
  resolve({ accumulatedSeconds: 120 });
  assert.deepEqual(await first, { kind: 'superseded' });

  let previousKey = owner.intent(identity).idempotencyKey;
  for (const changed of [
    { ...identity, sessionToken: 'session-2' },
    { ...identity, ownerId: 'profile-2' },
    { ...identity, pathId: 'path-2' },
    { ...identity, activityId: 'activity-2' },
  ]) {
    const changedKey = owner.intent(changed).idempotencyKey;
    assert.notEqual(changedKey, previousKey);
    previousKey = changedKey;
  }
});

test('only an authoritative applied result removes the exact activity and updates progress', () => {
  const activity1 = { activity: { id: 'activity-1' } };
  const activity2 = { activity: { id: 'activity-2' } };
  const timer = { running: false, accumulatedSeconds: 240 };
  const state = { activities: [activity1, activity2], timer };

  const unchanged = applyActivityDeletionResult(state, { kind: 'failed', cause: new Error('offline') }, 'activity-1');
  assert.strictEqual(unchanged, state);

  const next = applyActivityDeletionResult(state, {
    kind: 'applied',
    result: {
      accumulatedSeconds: 120,
      intervalProgress: { accumulatedSeconds: 30, targetSeconds: 60 },
      removedFeedEventIds: ['practice:activity-1'],
      sessionCount: 2,
      unreadNotificationCount: 7,
    },
  }, 'activity-1');
  assert.deepEqual(next.activities, [activity2]);
  assert.deepEqual(next.timer, {
    running: false,
    accumulatedSeconds: 120,
    intervalProgress: { accumulatedSeconds: 30, targetSeconds: 60 },
  });
  assert.deepEqual(state.activities, [activity1, activity2]);
});

test('activity deletion accepts only exact nonempty unique removed feed event IDs', async () => {
  const invalidResults = [
    { accumulatedSeconds: 0, sessionCount: 0, unreadNotificationCount: 0 },
    { accumulatedSeconds: 0, removedFeedEventIds: [], sessionCount: 0, unreadNotificationCount: 0 },
    { accumulatedSeconds: 0, removedFeedEventIds: [''], sessionCount: 0, unreadNotificationCount: 0 },
    { accumulatedSeconds: 0, removedFeedEventIds: [' event-1'], sessionCount: 0, unreadNotificationCount: 0 },
    { accumulatedSeconds: 0, removedFeedEventIds: ['event-1', 'event-1'], sessionCount: 0, unreadNotificationCount: 0 },
    { accumulatedSeconds: 0, removedFeedEventIds: ['practice:activity-2'], sessionCount: 0, unreadNotificationCount: 0 },
    { accumulatedSeconds: 0, removedFeedEventIds: ['practice:activity-1', 'reaction:other'], sessionCount: 0, unreadNotificationCount: 0 },
    { accumulatedSeconds: 0, removedFeedEventIds: ['event-1'], sessionCount: -1, unreadNotificationCount: 0 },
    { accumulatedSeconds: 0, removedFeedEventIds: ['event-1'], sessionCount: 0, unreadNotificationCount: -1 },
    { accumulatedSeconds: 0, removedFeedEventIds: ['event-1'], sessionCount: 0, unreadNotificationCount: 0, unexpected: true },
  ];

  for (const [index, result] of invalidResults.entries()) {
    const owner = createActivityDeletionOperationOwner(() => `activity-delete-invalid-${index}`);
    assert.equal((await owner.submit(identity, true, async () => result)).kind, 'failed');
  }

  const owner = createActivityDeletionOperationOwner(() => 'activity-delete-valid-0001');
  const applied = await owner.submit(identity, true, async () => ({
    accumulatedSeconds: 0,
    removedFeedEventIds: ['practice:activity-1', 'achievement:interval-1'],
    sessionCount: 0,
    unreadNotificationCount: 0,
  }));
  assert.deepEqual(applied, {
    kind: 'applied',
    result: {
      accumulatedSeconds: 0,
      removedFeedEventIds: ['practice:activity-1', 'achievement:interval-1'],
      sessionCount: 0,
      unreadNotificationCount: 0,
    },
  });
});
