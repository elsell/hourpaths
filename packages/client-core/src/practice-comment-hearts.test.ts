import assert from 'node:assert/strict';
import test from 'node:test';

import {
  applyPracticeCommentHeartState,
  createPracticeCommentHeartOperationOwner,
  invalidatePracticeCommentHeartRoster,
  mergePracticeCommentHeartRosterPage,
  practiceCommentHeartRosterPageFromAPI,
  practiceCommentHeartStateFromAPI,
  rollbackPracticeCommentHeart,
  updatePracticeCommentHeartOptimistically,
} from './practice-comment-hearts.js';
import { practiceCommentPageFromAPI } from './practice-comments.js';
import { mergePracticeCommentPage } from './practice-comments.js';

const alice = {
  id: 'alice', username: 'alice', displayName: 'Alice', description: 'Practices daily',
  followerCount: 2, followingCount: 3, relationship: 'following',
};
const bob = {
  id: 'bob', username: 'bob', displayName: 'Bob', followerCount: 0, followingCount: 1,
  relationship: 'none', profilePictureUrl: 'https://images.example.test/bob.jpg',
};

function commentPage(heartCount = 0, heartedByViewer = false) {
  return practiceCommentPageFromAPI({
    data: {
      items: [{
        author: alice,
        heartCount,
        heartedByViewer,
        comment: {
          id: 'comment-1', eventId: 'practice:activity-1', authorUserId: 'alice', text: 'Nice work',
          version: 1, createdAt: '2026-07-28T12:01:00Z', updatedAt: '2026-07-28T12:01:00Z',
          edited: false,
        },
      }],
    },
    meta: {},
  });
}

test('heart mutation state is exact, authoritative, and comment-bound', () => {
  assert.deepEqual(practiceCommentHeartStateFromAPI({
    commentId: 'comment-1', heartCount: 3, heartedByViewer: true,
  }), { commentId: 'comment-1', heartCount: 3, heartedByViewer: true });
  for (const malformed of [
    { commentId: 'comment-1', heartCount: -1, heartedByViewer: true },
    { commentId: 'comment-1', heartCount: 1.5, heartedByViewer: true },
    { commentId: 'comment-1', heartCount: 0, heartedByViewer: true },
    { commentId: 'comment-1', heartCount: 1, heartedByViewer: true, privateActorId: 'secret' },
  ]) assert.throws(() => practiceCommentHeartStateFromAPI(malformed), /invalid practice comment heart/);

  const initial = commentPage();
  const applied = applyPracticeCommentHeartState(initial, {
    commentId: 'comment-1', heartCount: 4, heartedByViewer: true,
  });
  assert.deepEqual({
    count: applied.items[0]?.heartCount,
    selected: applied.items[0]?.heartedByViewer,
    pending: applied.items[0]?.heartPending,
  }, { count: 4, selected: true, pending: false });
  assert.throws(() => applyPracticeCommentHeartState(initial, {
    commentId: 'missing', heartCount: 1, heartedByViewer: true,
  }), /unknown practice comment/);
});

test('optimistic heart changes and exact rollback preserve newer intent and unrelated rows', () => {
  const initial = commentPage(2, false);
  const optimistic = updatePracticeCommentHeartOptimistically(initial, 'comment-1', true);
  assert.deepEqual({
    count: optimistic.items[0]?.heartCount,
    selected: optimistic.items[0]?.heartedByViewer,
    pending: optimistic.items[0]?.heartPending,
  }, { count: 3, selected: true, pending: true });

  const newerIntent = updatePracticeCommentHeartOptimistically(optimistic, 'comment-1', false);
  assert.strictEqual(rollbackPracticeCommentHeart(newerIntent, initial.items[0]!, true), newerIntent);
  const secondIntentFailed = rollbackPracticeCommentHeart(newerIntent, optimistic.items[0]!, false);
  assert.deepEqual({
    count: secondIntentFailed.items[0]?.heartCount,
    selected: secondIntentFailed.items[0]?.heartedByViewer,
    pending: secondIntentFailed.items[0]?.heartPending,
  }, { count: 3, selected: true, pending: false });
  const rolledBack = rollbackPracticeCommentHeart(optimistic, initial.items[0]!, true);
  assert.deepEqual({
    count: rolledBack.items[0]?.heartCount,
    selected: rolledBack.items[0]?.heartedByViewer,
    pending: rolledBack.items[0]?.heartPending,
  }, { count: 2, selected: false, pending: false });
  assert.strictEqual(updatePracticeCommentHeartOptimistically(optimistic, 'comment-1', true), optimistic);
});

test('an older comment-page refresh cannot overwrite a newer optimistic heart intent', () => {
  const initial = commentPage(2, false);
  const optimistic = updatePracticeCommentHeartOptimistically(initial, 'comment-1', true);
  const olderRefresh = commentPage(2, false);
  const merged = mergePracticeCommentPage(optimistic, olderRefresh, '');

  assert.deepEqual({
    count: merged.items[0]?.heartCount,
    selected: merged.items[0]?.heartedByViewer,
    pending: merged.items[0]?.heartPending,
  }, { count: 3, selected: true, pending: true });
});

test('heart owner replays one intent key and supersedes older response completion', async () => {
  const keys = ['comment-heart-key-0001', 'comment-heart-key-0002'];
  const owner = createPracticeCommentHeartOperationOwner(() => keys.shift() ?? 'unexpected-key');
  const requests: Array<{
    hearted: boolean;
    key: string;
    resolve: (state: { commentId: string; heartCount: number; heartedByViewer: boolean }) => void;
    reject: (cause: unknown) => void;
  }> = [];
  const request = (commentId: string, hearted: boolean, key: string) => new Promise<{
    commentId: string; heartCount: number; heartedByViewer: boolean;
  }>((resolve, reject) => requests.push({ hearted, key, resolve, reject }));

  const stale = owner.submit('comment-1', true, request);
  await Promise.resolve();
  const current = owner.submit('comment-1', false, request);
  assert.equal(requests.length, 1);
  requests[0]!.resolve({ commentId: 'comment-1', heartCount: 1, heartedByViewer: true });
  assert.deepEqual(await stale, { kind: 'superseded' });
  await Promise.resolve();
  assert.equal(requests.length, 2);
  requests[1]!.reject(new Error('offline'));
  assert.equal((await current).kind, 'failed');

  const retry = owner.submit('comment-1', false, request);
  await Promise.resolve();
  assert.equal(requests[0]!.key, 'comment-heart-key-0001');
  assert.equal(requests[1]!.key, 'comment-heart-key-0002');
  assert.equal(requests[2]!.key, 'comment-heart-key-0002');
  requests[2]!.resolve({ commentId: 'comment-1', heartCount: 0, heartedByViewer: false });
  assert.deepEqual(await retry, {
    kind: 'applied',
    state: { commentId: 'comment-1', heartCount: 0, heartedByViewer: false },
  });
});

test('serialized retaps leave server state at the newest intent regardless of response timing', async () => {
  const owner = createPracticeCommentHeartOperationOwner(() => 'serialized-key-0001');
  let serverHearted = false;
  let releaseFirst!: () => void;
  const firstGate = new Promise<void>((resolve) => { releaseFirst = resolve; });
  const arrivals: boolean[] = [];
  const request = async (commentId: string, hearted: boolean) => {
    arrivals.push(hearted);
    if (arrivals.length === 1) await firstGate;
    serverHearted = hearted;
    return { commentId, heartCount: hearted ? 1 : 0, heartedByViewer: hearted };
  };

  const add = owner.submit('comment-1', true, request);
  await Promise.resolve();
  const remove = owner.submit('comment-1', false, request);
  await Promise.resolve();
  assert.deepEqual(arrivals, [true]);

  releaseFirst();
  assert.deepEqual(await add, { kind: 'superseded' });
  assert.deepEqual(await remove, {
    kind: 'applied',
    state: { commentId: 'comment-1', heartCount: 0, heartedByViewer: false },
  });
  assert.deepEqual(arrivals, [true, false]);
  assert.equal(serverHearted, false);
});

test('heart owner rejects mismatched acknowledgements and invalid replay keys', async () => {
  const owner = createPracticeCommentHeartOperationOwner(() => 'valid-heart-key-0001');
  const mismatch = await owner.submit('comment-1', true, async () => ({
    commentId: 'comment-2', heartCount: 1, heartedByViewer: true,
  }));
  assert.equal(mismatch.kind, 'failed');

  const invalid = createPracticeCommentHeartOperationOwner(() => 'short');
  await assert.rejects(() => invalid.submit('comment-1', true, async () => ({
    commentId: 'comment-1', heartCount: 1, heartedByViewer: true,
  })), /invalid comment heart idempotency key/);
});

test('heart roster exposes only public identity, pages once, and cannot merge after invalidation', () => {
  const first = practiceCommentHeartRosterPageFromAPI({
    $schema: 'https://api.example.test/schemas/PracticeCommentHeartRosterOutputBody.json',
    data: { items: [alice] }, meta: { nextCursor: 'signed-next' },
  }, 'comment-1');
  const second = practiceCommentHeartRosterPageFromAPI({
    data: { items: [alice, bob] }, meta: {},
  }, 'comment-1');
  const merged = mergePracticeCommentHeartRosterPage(first, second, 'signed-next', first.revision);
  assert.deepEqual(merged.items, [
    { userId: 'alice', username: 'alice', displayName: 'Alice' },
    { userId: 'bob', username: 'bob', displayName: 'Bob', profilePictureURL: 'https://images.example.test/bob.jpg' },
  ]);
  assert.throws(() => mergePracticeCommentHeartRosterPage(first, second, 'wrong', first.revision), /stale heart roster cursor/);

  const invalidated = invalidatePracticeCommentHeartRoster(first, 'alice');
  assert.deepEqual(invalidated.items, []);
  assert.throws(
    () => mergePracticeCommentHeartRosterPage(invalidated, second, 'signed-next', first.revision),
    /stale heart roster revision/,
  );
  assert.throws(() => practiceCommentHeartRosterPageFromAPI({
    data: { items: [{ ...alice, email: 'private@example.test' }] }, meta: {},
  }, 'comment-1'), /invalid heart roster/);
  assert.throws(() => practiceCommentHeartRosterPageFromAPI({
    $schema: 42, data: { items: [alice] }, meta: {},
  }, 'comment-1'), /invalid heart roster/);
});
