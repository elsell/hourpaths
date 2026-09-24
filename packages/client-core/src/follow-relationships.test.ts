import assert from 'node:assert/strict';
import test from 'node:test';
import {
  followRequestPageFromAPI,
  followRequestReviewResultFromAPI,
  mergeFollowRequestPage,
  relationshipMutationResultFromAPI,
  removeResolvedFollowRequest,
} from './index.js';

const profile = { id: 'user-alice', username: 'alice', displayName: 'Alice', followerCount: 2, followingCount: 3, relationship: 'none' };

test('relationship mutation retains only the authoritative profile state and optional request id', () => {
  assert.deepEqual(relationshipMutationResultFromAPI({ data: { profile: { ...profile, relationship: 'following' } } }), {
    profile: { userId: 'user-alice', username: 'alice', displayName: 'Alice', followerCount: 2, followingCount: 3, relationship: 'following' },
  });
  assert.equal(relationshipMutationResultFromAPI({ data: { profile: { ...profile, relationship: 'requested' }, requestId: 'request-1' } }).requestId, 'request-1');
  assert.throws(() => relationshipMutationResultFromAPI({ data: { profile: { ...profile, relationship: 'requested' } } }), /invalid relationship response/);
  assert.throws(() => relationshipMutationResultFromAPI({ data: { profile, requestId: 'request-1' } }), /invalid relationship response/);
});

test('request review requires an authoritative matching safe request and decision', () => {
  const reviewed = followRequestReviewResultFromAPI({ data: { decision: 'accepted', request: { id: 'request-1', requester: profile, createdAt: '2026-07-27T12:00:00Z' } } });
  assert.equal(reviewed.request.id, 'request-1');
  assert.equal(reviewed.decision, 'accepted');
  assert.throws(() => followRequestReviewResultFromAPI({ data: { decision: 'removed', request: reviewed.request } }), /invalid follow request review/);
});

test('incoming requests validate safe profiles, paginate once, and resolve locally only after acknowledgement', () => {
  const first = followRequestPageFromAPI({ data: [{ id: 'request-1', requester: profile, createdAt: '2026-07-27T12:00:00Z' }], meta: { nextCursor: 'signed' } });
  const second = followRequestPageFromAPI({ data: [{ id: 'request-2', requester: { ...profile, id: 'user-bob', username: 'bob', displayName: 'Bob' }, createdAt: '2026-07-27T11:00:00Z' }], meta: {} });
  const merged = mergeFollowRequestPage(first, second, 'signed');
  assert.deepEqual(merged.items.map(({ id }) => id), ['request-1', 'request-2']);
  assert.deepEqual(removeResolvedFollowRequest(merged, 'request-1').items.map(({ id }) => id), ['request-2']);
  assert.throws(() => mergeFollowRequestPage(first, second, 'stale'), /stale follow request cursor/);
  assert.throws(() => removeResolvedFollowRequest(merged, 'missing'), /unknown follow request/);
});

test('incoming requests reject recipient metadata, duplicate ids, and self projections', () => {
  for (const data of [
    [{ id: 'request-1', requester: profile, recipientUserId: 'viewer', createdAt: '2026-07-27T12:00:00Z' }],
    [{ id: 'request-1', requester: profile, createdAt: '2026-07-27T12:00:00Z' }, { id: 'request-1', requester: profile, createdAt: '2026-07-27T11:00:00Z' }],
    [{ id: 'request-1', requester: { ...profile, relationship: 'self' }, createdAt: '2026-07-27T12:00:00Z' }],
  ]) assert.throws(() => followRequestPageFromAPI({ data, meta: {} }), /invalid follow request response/);
});
