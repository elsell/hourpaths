import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index';

test('leavePath sends the explicit retained-activity contract and idempotency key', async () => {
  const originalFetch = globalThis.fetch;
  let request: Request | undefined;
  globalThis.fetch = async (input) => {
    request = input instanceof Request ? input : new Request(input);
    return new Response(JSON.stringify({ data: { pathId: 'path-1', left: true, activityRetained: true } }), { status: 200, headers: { 'content-type': 'application/json' } });
  };
  try {
    const result = await createSessionApiClient('https://api.example.test', () => 'session').leavePath('path-1', { confirmed: true, retainActivity: true }, 'leave-path-key-0001');
    assert.equal(result.response.status, 200);
    assert.equal(new URL(request!.url).pathname, '/v1/paths/path-1/membership');
    assert.equal(request!.method, 'DELETE');
    assert.equal(request!.headers.get('Idempotency-Key'), 'leave-path-key-0001');
    assert.deepEqual(await request!.json(), { confirmed: true, retainActivity: true });
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test('leavePath sends an explicit permanent activity deletion choice', async () => {
  const originalFetch = globalThis.fetch;
  let request: Request | undefined;
  globalThis.fetch = async (input) => {
    request = input instanceof Request ? input : new Request(input);
    return new Response(JSON.stringify({ data: { pathId: 'path-1', left: true, activityRetained: false } }), { status: 200, headers: { 'content-type': 'application/json' } });
  };
  try {
    const result = await createSessionApiClient('https://api.example.test', () => 'session').leavePath('path-1', { confirmed: true, retainActivity: false }, 'leave-path-delete-0001');
    assert.equal(result.data?.data.activityRetained, false);
    assert.deepEqual(await request!.json(), { confirmed: true, retainActivity: false });
  } finally {
    globalThis.fetch = originalFetch;
  }
});
