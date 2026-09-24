import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index.js';
import type { PathInvitationNotification, PracticeCommentEdit, PracticeCommentInput } from './index.js';

const commentNotification: PathInvitationNotification = {
  id: 'notification-comment-1',
  type: 'practice_comment',
  presentation: 'informational',
  read: false,
  createdAt: '2026-07-28T12:00:00Z',
  actor: {
    userId: 'commenter',
    username: 'reader',
    displayName: 'Reader',
  },
  pathId: 'path-1',
  pathName: 'Piano',
  socialFeedEventId: 'practice:activity-1',
  commentId: 'comment-1',
};

test('practice comment notifications expose typed comment navigation context', () => {
  assert.deepEqual(
    {
      type: commentNotification.type,
      eventId: commentNotification.socialFeedEventId,
      commentId: commentNotification.commentId,
    },
    {
      type: 'practice_comment',
      eventId: 'practice:activity-1',
      commentId: 'comment-1',
    },
  );
});

test('practice comment operations use authenticated generated pagination, mutation, and history contracts', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    if (request.method === 'DELETE') return new Response(null, { status: 204 });
    return Response.json({ data: { items: [] }, meta: {} });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const create: PracticeCommentInput = { text: 'Strong work!' };
  const edit: PracticeCommentEdit = { text: 'Excellent work!', expectedVersion: 1 };
  const client = createSessionApiClient('https://api.example.test', () => 'application-session');

  await client.socialFeedComments('practice:activity-1', 'comments+/=');
  await client.createSocialFeedComment('practice:activity-1', create, 'create-comment-key');
  await client.editSocialFeedComment('practice:activity-1', 'comment/1', edit, 'edit-comment-key');
  await client.deleteSocialFeedComment('practice:activity-1', 'comment/1', 'delete-comment-key');
  await client.socialFeedCommentHistory('practice:activity-1', 'comment/1', 'history+/=');

  assert.deepEqual(captured.map((request) => [
    request.method,
    request.url,
    request.headers.get('idempotency-key'),
  ]), [
    ['GET', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/comments?cursor=comments%2B%2F%3D&limit=25', null],
    ['POST', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/comments', 'create-comment-key'],
    ['PATCH', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/comments/comment%2F1', 'edit-comment-key'],
    ['DELETE', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/comments/comment%2F1', 'delete-comment-key'],
    ['GET', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/comments/comment%2F1/history?cursor=history%2B%2F%3D&limit=25', null],
  ]);
  assert.deepEqual(await captured[1]?.json(), create);
  assert.deepEqual(await captured[2]?.json(), edit);
  assert.equal(captured[0]?.body, null);
  assert.equal(captured[3]?.body, null);
  assert.equal(captured[4]?.body, null);
  for (const request of captured) assert.equal(request.headers.get('authorization'), 'Bearer application-session');
});

test('practice comment hearts use authenticated idempotent mutations and paginated roster contracts', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    return Response.json(request.method === 'GET'
      ? { data: { items: [] }, meta: {} }
      : { data: { commentId: 'comment/1', heartCount: request.method === 'PUT' ? 1 : 0, heartedByViewer: request.method === 'PUT' } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const client = createSessionApiClient('https://api.example.test', () => 'application-session');
  await client.setPracticeCommentHeart('practice:activity-1', 'comment/1', 'set-comment-heart-key');
  await client.removePracticeCommentHeart('practice:activity-1', 'comment/1', 'remove-comment-heart-key');
  await client.practiceCommentHearts('practice:activity-1', 'comment/1', 'hearts+/=');

  assert.deepEqual(captured.map((request) => [
    request.method,
    request.url,
    request.headers.get('idempotency-key'),
  ]), [
    ['PUT', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/comments/comment%2F1/heart', 'set-comment-heart-key'],
    ['DELETE', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/comments/comment%2F1/heart', 'remove-comment-heart-key'],
    ['GET', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/comments/comment%2F1/hearts?cursor=hearts%2B%2F%3D&limit=25', null],
  ]);
  for (const request of captured) {
    assert.equal(request.headers.get('authorization'), 'Bearer application-session');
    assert.equal(request.body, null);
  }
});
