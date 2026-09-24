import assert from 'node:assert/strict';
import test from 'node:test';
import {
  appendOptimisticPracticeComment,
  mergePracticeCommentPage,
  practiceCommentFromAPI,
  practiceCommentMutationFromAPI,
  practiceCommentHistoryPageFromAPI,
  practiceCommentPageFromAPI,
  removePracticeComment,
  restoreDeletedPracticeComment,
  rollbackPracticeCommentEdit,
  mergePracticeCommentHistoryPage,
  replacePracticeComment,
  updatePracticeCommentOptimistically,
} from './practice-comments.js';

const alice = { id: 'alice', username: 'alice', displayName: 'Alice', description: 'Practices daily', followerCount: 2, followingCount: 3, relationship: 'following' };
const bob = { id: 'bob', username: 'bob', displayName: 'Bob', followerCount: 0, followingCount: 1, relationship: 'none', profilePictureUrl: 'https://images.example.test/bob.jpg' };

function apiItem(id: string, author: typeof alice | typeof bob = alice, version = 1, text = id) {
  return {
    author,
    heartCount: 0,
    heartedByViewer: false,
    comment: {
      id,
      eventId: 'practice:activity-1',
      authorUserId: author.id,
      text,
      version,
      createdAt: `2026-07-28T12:0${version}:00Z`,
      updatedAt: `2026-07-28T12:0${version}:00Z`,
      edited: version > 1,
    },
  };
}

test('comment pages validate public authors and merge once in chronological order', () => {
  const first = practiceCommentPageFromAPI({ $schema: 'https://api.example.test/schemas/PracticeCommentPage.json', data: { items: [apiItem('one')] }, meta: { nextCursor: 'next' } });
  const second = practiceCommentPageFromAPI({ data: { items: [apiItem('one'), apiItem('two', bob)] }, meta: {} });
  const merged = mergePracticeCommentPage(first, second, 'next');

  assert.deepEqual(merged.items.map(({ id }) => id), ['one', 'two']);
  assert.deepEqual(merged.items[1]?.author, { userId: 'bob', username: 'bob', displayName: 'Bob', profilePictureURL: 'https://images.example.test/bob.jpg' });
  assert.equal(merged.nextCursor, '');
  assert.throws(() => mergePracticeCommentPage(first, second, 'stale'), /stale comment cursor/);
  assert.throws(() => practiceCommentPageFromAPI({ data: { items: [apiItem('two', bob, 2), apiItem('one')] }, meta: {} }), /chronological/);
  assert.throws(() => practiceCommentPageFromAPI({ $schema: 42, data: { items: [apiItem('one')] }, meta: {} }), /invalid practice comment page/);
  assert.throws(() => practiceCommentPageFromAPI({ data: { items: [{ ...apiItem('one'), heartCount: 0, heartedByViewer: true }] }, meta: {} }), /invalid practice comment/);
});

test('failed overlapping mutations roll back only their exact optimistic row', () => {
  const initial = practiceCommentPageFromAPI({ data: { items: [apiItem('one'), apiItem('two', bob)] }, meta: {} });
  const optimistic = updatePracticeCommentOptimistically(initial, 'one', 1, 'Pending edit');
  const concurrentlyUpdated = replacePracticeComment(optimistic, 'two', practiceCommentFromAPI(apiItem('two', bob, 2, 'Newer peer')));
  const rolledBack = rollbackPracticeCommentEdit(concurrentlyUpdated, initial.items[0]!, 'Pending edit');
  assert.deepEqual(rolledBack.items.map(({ id, text, version }) => [id, text, version]), [
    ['one', 'one', 1], ['two', 'Newer peer', 2],
  ]);

  const sameRowWon = replacePracticeComment(optimistic, 'one', practiceCommentFromAPI(apiItem('one', alice, 2, 'Authoritative')));
  assert.equal(rollbackPracticeCommentEdit(sameRowWon, initial.items[0]!, 'Pending edit'), sameRowWon);

  const deleted = removePracticeComment(initial, 'one');
  const peerWon = replacePracticeComment(deleted, 'two', practiceCommentFromAPI(apiItem('two', bob, 2, 'Newer peer')));
  assert.deepEqual(restoreDeletedPracticeComment(peerWon, initial.items[0]!).items.map(({ id, text }) => [id, text]), [
    ['one', 'one'], ['two', 'Newer peer'],
  ]);
});

test('history pages are strict, comment-bound, chronological, and cursor-owned', () => {
  const first = practiceCommentHistoryPageFromAPI({
    data: { versions: [{ commentId: 'one', text: 'Original', version: 1, createdAt: '2026-07-28T12:01:00Z' }] },
    meta: { nextCursor: 'signed-next' },
  }, 'one');
  const second = practiceCommentHistoryPageFromAPI({
    data: { versions: [{ commentId: 'one', text: 'Edited', version: 2, createdAt: '2026-07-28T12:02:00Z' }] },
    meta: {},
  }, 'one');
  const merged = mergePracticeCommentHistoryPage(first, second, 'signed-next');
  assert.deepEqual(merged.versions.map(({ version }) => version), [1, 2]);
  assert.throws(() => mergePracticeCommentHistoryPage(first, second, 'crossed'), /stale comment history cursor/);
  const crossedPage = practiceCommentHistoryPageFromAPI({
    data: { versions: [{ commentId: 'one', text: 'Crossed page', version: 1, createdAt: '2026-07-28T12:01:30Z' }] },
    meta: {},
  }, 'one');
  assert.throws(() => mergePracticeCommentHistoryPage(first, crossedPage, 'signed-next'), /stale comment history page/);
  assert.throws(() => practiceCommentHistoryPageFromAPI({ data: { versions: [{ commentId: 'two', text: 'Wrong', version: 1, createdAt: '2026-07-28T12:01:00Z' }] }, meta: {} }, 'one'), /invalid comment history/);
  assert.throws(() => practiceCommentHistoryPageFromAPI({ data: { versions: [{ commentId: 'one', text: 'Original', version: 1, createdAt: '2026-07-28T12:01:00Z', private: true }] }, meta: {} }, 'one'), /invalid comment history/);
});

test('mutation acknowledgements combine an exact bare comment with the known author projection', () => {
  const item = apiItem('two', bob, 2, 'Revised');
  const author = { userId: 'bob', username: 'bob', displayName: 'Bob', profilePictureURL: 'https://images.example.test/bob.jpg' };
  assert.deepEqual(practiceCommentMutationFromAPI(item.comment, author).author, author);
  assert.deepEqual(
    practiceCommentMutationFromAPI(item.comment, author, { heartCount: 3, heartedByViewer: true }),
    { ...practiceCommentMutationFromAPI(item.comment, author), heartCount: 3, heartedByViewer: true },
  );
  assert.throws(() => practiceCommentMutationFromAPI({ ...item.comment, relationship: 'following' }, author), /invalid practice comment/);
});

test('optimistic create, edit, authoritative replacement, and removal retain stable identities', () => {
  const initial = practiceCommentPageFromAPI({ data: { items: [apiItem('one')] }, meta: {} });
  const created = appendOptimisticPracticeComment(initial, {
    author: { userId: 'bob', username: 'bob', displayName: 'Bob', profilePictureURL: 'https://images.example.test/bob.jpg' },
    createdAt: '2026-07-28T12:03:00Z',
    eventId: 'practice:activity-1',
    temporaryId: 'pending:create-1',
    text: 'Draft',
  });
  assert.equal(created.items[1]?.pending, true);

  const authoritative = practiceCommentFromAPI(apiItem('two', bob, 1, 'Draft'));
  const settled = replacePracticeComment(created, 'pending:create-1', authoritative);
  assert.equal(settled.items[1]?.id, 'two');
  assert.equal(settled.items[1]?.pending, false);

  const editing = updatePracticeCommentOptimistically(settled, 'two', 1, 'Revised');
  assert.equal(editing.items[1]?.text, 'Revised');
  assert.equal(editing.items[1]?.version, 1);
  assert.equal(editing.items[1]?.pending, true);

  const edited = replacePracticeComment(editing, 'two', practiceCommentFromAPI(apiItem('two', bob, 2, 'Revised')));
  assert.deepEqual({ text: edited.items[1]?.text, version: edited.items[1]?.version, pending: edited.items[1]?.pending }, {
    text: 'Revised', version: 2, pending: false,
  });
  assert.deepEqual(removePracticeComment(edited, 'two').items.map(({ id }) => id), ['one']);
  assert.throws(() => updatePracticeCommentOptimistically(settled, 'two', 2, 'stale'), /stale comment version/);
});
