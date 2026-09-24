import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index.js';

test('follow relationship operations use generated authenticated contracts and idempotency keys', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    return Response.json({ data: {} });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const client = createSessionApiClient('https://api.example.test', () => 'application-session');
  await client.followProfile('alice', 'follow-request-key');
  await client.cancelFollowRequest('alice', 'cancel-request-key');
  await client.unfollowProfile('alice', 'unfollow-request-key');
  await client.followRequests('signed+/=');
  await client.acceptFollowRequest('request-1', 'accept-request-key');
  await client.rejectFollowRequest('request-2', 'reject-request-key');

  assert.deepEqual(captured.map(({ method }) => method), ['POST', 'DELETE', 'DELETE', 'GET', 'POST', 'POST']);
  assert.deepEqual(captured.map(({ url }) => url), [
    'https://api.example.test/v1/profiles/alice/follow',
    'https://api.example.test/v1/profiles/alice/follow-request',
    'https://api.example.test/v1/profiles/alice/follow',
    'https://api.example.test/v1/follow-requests?cursor=signed%2B%2F%3D&limit=25',
    'https://api.example.test/v1/follow-requests/request-1/accept',
    'https://api.example.test/v1/follow-requests/request-2/reject',
  ]);
  assert.deepEqual(captured.map((request) => request.headers.get('authorization')), Array(6).fill('Bearer application-session'));
  assert.deepEqual(captured.map((request) => request.headers.get('idempotency-key')), [
    'follow-request-key', 'cancel-request-key', 'unfollow-request-key', null, 'accept-request-key', 'reject-request-key',
  ]);
});
