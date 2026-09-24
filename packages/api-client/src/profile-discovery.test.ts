import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index.js';

test('profile discovery uses authenticated generated GET contracts without private parameters', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    const data = request.url.endsWith('/profiles/alex')
      ? { id: 'user-alex', username: 'alex', displayName: 'Alex Rivera', followerCount: 3, followingCount: 5 }
      : [{ id: 'user-alex', username: 'alex', displayName: 'Alex Rivera', followerCount: 3, followingCount: 5 }];
    return Response.json(
      request.url.endsWith('/profiles/alex') ? { data } : { data, meta: { nextCursor: 'signed+/=' } },
    );
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const client = createSessionApiClient('https://api.example.test', () => 'application-session');
  await client.searchProfiles('ál ex', 'signed+/=');
  await client.profileByUsername('alex');

  assert.equal(captured.length, 2);
  assert.equal(captured[0]?.method, 'GET');
  assert.equal(captured[0]?.headers.get('authorization'), 'Bearer application-session');
  assert.equal(captured[0]?.url, 'https://api.example.test/v1/profiles?query=%C3%A1l%20ex&cursor=signed%2B%2F%3D&limit=25');
  assert.equal(captured[1]?.url, 'https://api.example.test/v1/profiles/alex');
  for (const request of captured) {
    assert.equal(request.url.includes('email'), false);
    assert.equal(request.url.includes('visibility'), false);
    assert.equal(request.body, null);
  }
});
