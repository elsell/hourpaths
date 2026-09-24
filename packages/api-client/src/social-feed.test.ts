import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index.js';

test('social feed uses the authenticated generated paginated contract', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    return Response.json({ data: { items: [] }, meta: { nextCursor: 'next-signed' } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const client = createSessionApiClient('https://api.example.test', () => 'application-session');
  await client.socialFeed('signed+/=');

  assert.equal(captured.length, 1);
  assert.equal(captured[0]?.method, 'GET');
  assert.equal(captured[0]?.headers.get('authorization'), 'Bearer application-session');
  assert.equal(captured[0]?.url, 'https://api.example.test/v1/social/feed?cursor=signed%2B%2F%3D&limit=25');
  assert.equal(captured[0]?.body, null);
});

test('active following uses the authenticated generated participant-page contract', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    return Response.json({ data: { items: [] }, meta: { nextCursor: 'next-signed' } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const client = createSessionApiClient('https://api.example.test', () => 'application-session');
  await client.socialActiveFollowing('active+/=');

  assert.equal(captured.length, 1);
  assert.equal(captured[0]?.method, 'GET');
  assert.equal(captured[0]?.headers.get('authorization'), 'Bearer application-session');
  assert.equal(captured[0]?.url, 'https://api.example.test/v1/social/feed/active?cursor=active%2B%2F%3D&limit=25');
  assert.equal(captured[0]?.body, null);
});

test('practice reactions use authenticated idempotent generated mutation contracts', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    return Response.json({
      data: {
        reactions: { applause: 0, celebrate: 0, fire: 1, heart: 2, strong: 0 },
        viewerReaction: request.method === 'PUT' ? 'fire' : null,
      },
    });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const client = createSessionApiClient('https://api.example.test', () => 'application-session');
  await client.setSocialFeedReaction('practice:activity-1', 'fire', 'set-reaction-key');
  await client.removeSocialFeedReaction('practice:activity-1', 'remove-reaction-key');

  assert.deepEqual(captured.map((request) => [request.method, request.url, request.headers.get('idempotency-key')]), [
    ['PUT', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/reaction', 'set-reaction-key'],
    ['DELETE', 'https://api.example.test/v1/social/feed/practice%3Aactivity-1/reaction', 'remove-reaction-key'],
  ]);
  assert.deepEqual(await captured[0]?.json(), { reaction: 'fire' });
  assert.equal(captured[1]?.body, null);
  for (const request of captured) assert.equal(request.headers.get('authorization'), 'Bearer application-session');
});
