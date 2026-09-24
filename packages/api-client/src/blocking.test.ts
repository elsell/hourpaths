import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index.js';

test('blocking wrappers preserve safe paths, pagination, and idempotency headers', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  globalThis.fetch = async (input, init) => {
    requests.push(new Request(input, init));
    return new Response(JSON.stringify({ data: {} }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const client = createSessionApiClient('https://api.example.test', () => 'session');
  await client.reviewProfileBlock('alice');
  await client.blockProfile('alice', 'block-request-0001', { version: 1, token: 'signed-review-token', expiresAt: '2026-07-29T10:05:00Z' });
  await client.blockedAccounts('next-page');
  await client.unblockAccount('user-1', 'unblock-request-01');

  assert.deepEqual(requests.map((request) => [request.method, new URL(request.url).pathname]), [
    ['GET', '/v1/profiles/alice/block-review'],
    ['POST', '/v1/profiles/alice/block'],
    ['GET', '/v1/blocked-accounts'],
    ['DELETE', '/v1/blocked-accounts/user-1'],
  ]);
  assert.equal(new URL(requests[2]!.url).searchParams.get('cursor'), 'next-page');
  assert.equal(requests[1]!.headers.get('idempotency-key'), 'block-request-0001');
  assert.deepEqual(await requests[1]!.json(), { acknowledgement: { version: 1, token: 'signed-review-token', expiresAt: '2026-07-29T10:05:00Z' } });
  assert.equal(requests[3]!.headers.get('idempotency-key'), 'unblock-request-01');
});
