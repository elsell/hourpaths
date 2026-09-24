import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index.js';

test('deletePath sends the reviewed name, confirmation, and stable idempotency key', async () => {
  const originalFetch = globalThis.fetch;
  let request: Request | undefined;
  globalThis.fetch = async (input) => {
    request = input instanceof Request ? input : new Request(input);
    return new Response(JSON.stringify({
      data: { pathId: 'path-1', deleted: true },
    }), { status: 200, headers: { 'content-type': 'application/json' } });
  };
  try {
    const response = await createSessionApiClient('https://api.example.test', () => 'session')
      .deletePath('path-1', { confirmed: true, expectedName: 'Guitar' }, 'delete-path-key-0001');
    assert.deepEqual(response.data?.data, { pathId: 'path-1', deleted: true });
    assert.equal(request?.method, 'DELETE');
    assert.equal(new URL(request!.url).pathname, '/v1/paths/path-1');
    assert.equal(request?.headers.get('Idempotency-Key'), 'delete-path-key-0001');
    assert.deepEqual(await request?.json(), { confirmed: true, expectedName: 'Guitar' });
  } finally {
    globalThis.fetch = originalFetch;
  }
});
