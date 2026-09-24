import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index.js';

test('renamePath sends reviewed and proposed names through the dedicated generated route', async () => {
  const originalFetch = globalThis.fetch;
  let request: Request | undefined;
  globalThis.fetch = async (input) => {
    request = input instanceof Request ? input : new Request(input);
    return new Response(JSON.stringify({
      data: {
        id: 'path-1',
        name: 'After',
        visibility: 'private',
        capabilities: {
          trackTime: true,
          renamePath: true,
          inviteMembers: true,
          manageGoals: true,
          manageLifecycle: true,
          manageVisibility: true,
        },
      },
    }), { status: 200, headers: { 'content-type': 'application/json' } });
  };
  try {
    const response = await createSessionApiClient('https://api.example.test', () => 'session')
      .renamePath('path-1', { expectedName: 'Before', name: 'After' }, 'rename-path-key-0001');
    assert.equal(response.data?.data.name, 'After');
    assert.equal(request?.method, 'PUT');
    assert.equal(new URL(request!.url).pathname, '/v1/paths/path-1/name');
    assert.equal(request?.headers.get('Idempotency-Key'), 'rename-path-key-0001');
    assert.deepEqual(await request?.json(), { expectedName: 'Before', name: 'After' });
  } finally {
    globalThis.fetch = originalFetch;
  }
});
