import assert from 'node:assert/strict';
import test from 'node:test';
import {
  blockReviewFromAPI,
  blockedAccountPageFromAPI,
  blockResultFromAPI,
  mergeBlockedAccountPage,
  unblockResultFromAPI,
} from './user-blocking';

const alex = {
  userId: 'user-alex',
  username: 'alex.r',
  displayName: 'Alex Rivera',
};

test('block review admits only the target identity and compact shared Path summaries', () => {
  assert.deepEqual(blockReviewFromAPI({
    data: {
      target: alex,
      sharedPaths: [
        { id: 'path-piano', name: 'Piano' },
        { id: 'path-reading', name: 'Reading' },
      ],
      acknowledgement: { version: 1, token: 'signed-review-token', expiresAt: '2026-07-29T10:05:00Z' },
    },
  }), {
    target: { userId: 'user-alex', username: 'alex.r', displayName: 'Alex Rivera' },
    sharedPaths: [
      { id: 'path-piano', name: 'Piano' },
      { id: 'path-reading', name: 'Reading' },
    ],
    acknowledgement: { version: 1, token: 'signed-review-token', expiresAt: '2026-07-29T10:05:00Z' },
  });
  assert.throws(() => blockReviewFromAPI({
    data: { target: { ...alex, email: 'private@example.com' }, sharedPaths: [], acknowledgement: { version: 1, token: 'signed-review-token', expiresAt: '2026-07-29T10:05:00Z' } },
  }), /invalid block review/);
  assert.throws(() => blockReviewFromAPI({ data: { target: alex, sharedPaths: [] } }), /invalid block review/);
});

test('block and unblock results bind the mutation response to its exact target', () => {
  assert.deepEqual(blockResultFromAPI({ data: { target: alex, blocked: true } }), {
    target: { userId: 'user-alex', username: 'alex.r', displayName: 'Alex Rivera' },
    blocked: true,
  });
  assert.deepEqual(unblockResultFromAPI({ data: { target: alex, blocked: false } }), {
    target: { userId: 'user-alex', username: 'alex.r', displayName: 'Alex Rivera' },
    blocked: false,
  });
  assert.throws(() => unblockResultFromAPI({ data: { target: alex, blocked: true } }), /invalid unblock result/);
});

test('blocked-account pages validate identity, time, cursor, and duplicate safety', () => {
  const page = blockedAccountPageFromAPI({
    data: [{ ...alex, blockedAt: '2026-07-29T10:00:00Z' }],
    meta: { nextCursor: 'cursor-2' },
  });
  assert.equal(page.items[0]?.identity.username, 'alex.r');
  assert.equal(page.items[0]?.blockedAt, '2026-07-29T10:00:00Z');
  assert.equal(page.nextCursor, 'cursor-2');
  assert.throws(() => blockedAccountPageFromAPI({
    data: [
      { ...alex, blockedAt: '2026-07-29T10:00:00Z' },
      { ...alex, blockedAt: '2026-07-29T10:01:00Z' },
    ],
    meta: {},
  }), /invalid blocked accounts/);
});

test('pagination replaces refreshed identities, appends new accounts, and rejects stale cursors', () => {
  const initial = blockedAccountPageFromAPI({
    data: [{ ...alex, blockedAt: '2026-07-29T10:00:00Z' }],
    meta: { nextCursor: 'cursor-2' },
  });
  const next = blockedAccountPageFromAPI({
    data: [
      { ...alex, displayName: 'Alex R.', blockedAt: '2026-07-29T10:00:00Z' },
      { userId: 'user-sam', username: 'sam.reader', displayName: 'Sam', blockedAt: '2026-07-29T09:00:00Z' },
    ],
    meta: {},
  });
  const merged = mergeBlockedAccountPage(initial, next, 'cursor-2');
  assert.deepEqual(merged.items.map(({ identity }) => identity.displayName), ['Alex R.', 'Sam']);
  assert.throws(() => mergeBlockedAccountPage(initial, next, 'stale'), /stale blocked-account cursor/);
});
