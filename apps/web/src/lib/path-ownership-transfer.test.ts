import assert from 'node:assert/strict';
import test from 'node:test';
import type { OwnershipTransfer, OwnershipTransferCandidate } from '@hourpaths/api-client';
import { createTranslator } from '@hourpaths/i18n';
import {
  currentPendingOwnershipTransfers,
  decodeOwnershipTransferReview,
  mergeOwnershipTransferCandidates,
  ownershipTransferExpiration,
} from './path-ownership-transfer';

const first: OwnershipTransferCandidate = {
  administrator: false,
  displayName: 'Ari Reed',
  userId: 'user-1',
  username: 'ari',
};
const secondDisplayName = ['Bo', 'Lane'].join(' ');
const invalidTimeZone = ['Not', 'AZone'].join('/');

test('candidate pagination preserves order and de-duplicates replayed boundaries', () => {
  const second = { ...first, displayName: secondDisplayName, userId: 'user-2', username: 'bo' };
  assert.deepEqual(mergeOwnershipTransferCandidates([first], [first, second], false), [first, second]);
  assert.deepEqual(mergeOwnershipTransferCandidates([first], [second], true), [second]);
});

test('signed review accepts only the requested Path and canonical recipient', () => {
  const review = {
    reservationToken: 'signed-review',
    expiresAt: '2026-07-30T16:00:00Z',
    recipient: { displayName: first.displayName, userId: first.userId, username: first.username },
    reviewedAt: '2026-07-27T16:00:00Z',
    viewerTimeZone: 'America/New_York',
  };
  assert.deepEqual(decodeOwnershipTransferReview(review, 'path-1', 'user-1'), review);
  assert.equal(decodeOwnershipTransferReview(review, '', 'user-1'), undefined);
  assert.equal(decodeOwnershipTransferReview({ ...review, recipient: { ...first, userId: 'other' } }, 'path-1', 'user-1'), undefined);
  assert.equal(decodeOwnershipTransferReview({ ...review, reservationToken: '' }, 'path-1', 'user-1'), undefined);
  assert.equal(decodeOwnershipTransferReview({ ...review, viewerTimeZone: invalidTimeZone }, 'path-1', 'user-1'), undefined);
});

test('expiration uses every configured duration unit without rounding and the viewer time zone', () => {
  const translator = createTranslator(['en']);
  const result = ownershipTransferExpiration(
    '2026-07-27T16:00:00Z',
    '2026-07-28T17:30:00Z',
    'America/New_York',
    translator,
  );
  assert.equal(result?.relative, '1 day 1 hour 30 minutes');
  assert.match(result?.exact ?? '', /Jul 28, 2026/);
  assert.match(result?.exact ?? '', /1:30 PM/);
  assert.match(result?.summary ?? '', /1 day 1 hour 30 minutes/);
});

test('expiration rejects malformed, sub-minute, and invalid-zone presentations', () => {
  const translator = createTranslator(['en']);
  assert.equal(ownershipTransferExpiration('bad', '2026-07-27T17:00:00Z', 'UTC', translator), undefined);
  assert.equal(ownershipTransferExpiration('2026-07-27T16:00:00Z', '2026-07-27T16:00:30Z', 'UTC', translator), undefined);
  assert.equal(ownershipTransferExpiration('2026-07-27T16:00:00Z', '2026-07-27T17:00:00Z', invalidTimeZone, translator), undefined);
});

test('independent pending projection hides terminal, expired, and duplicate Path requests', () => {
  const pending = (id: string, pathId: string, expiresAt = '2026-07-30T16:00:00Z'): OwnershipTransfer => ({
    id,
    pathId,
    creatorUserId: 'creator',
    recipientUserId: 'recipient',
    createdAt: '2026-07-27T16:00:00Z',
    reviewedAt: '2026-07-27T15:59:00Z',
    expiresAt,
    state: 'pending',
  });
  assert.deepEqual(currentPendingOwnershipTransfers([
    pending('kept', 'path-1'),
    pending('duplicate', 'path-1'),
    { ...pending('declined', 'path-2'), state: 'declined', declinedAt: '2026-07-27T17:00:00Z' },
    pending('expired', 'path-3', '2026-07-27T15:59:59Z'),
  ], Date.parse('2026-07-27T16:00:00Z')).map(({ id }) => id), ['kept']);
});
