import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index';

test('member access methods use roster, authoritative review, and role-bound removal contracts', async () => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  globalThis.fetch = async (input) => {
    const request = input instanceof Request ? input : new Request(input);
    requests.push(request);
    return new Response(JSON.stringify({ data: {} }), { status: 200, headers: { 'content-type': 'application/json' } });
  };
  try {
    const client = createSessionApiClient('https://api.example.test', () => 'session');
    await client.pathMembers('path-1', 'next');
    await client.reviewPathMemberRemoval('path-1', 'target');
    await client.removePathMember('path-1', 'target', { confirmed: true, expectedRole: 'participant' }, 'remove-member-key-0001');
	await client.changePathMemberRole('path-1', 'target', { confirmed: true, expectedRole: 'participant', role: 'supporter' }, 'change-member-role-0001');
    assert.equal(new URL(requests[0].url).pathname, '/v1/paths/path-1/members');
    assert.equal(new URL(requests[0].url).searchParams.get('cursor'), 'next');
    assert.equal(new URL(requests[1].url).pathname, '/v1/paths/path-1/members/target/removal-review');
    assert.equal(requests[2].method, 'DELETE');
    assert.equal(requests[2].headers.get('Idempotency-Key'), 'remove-member-key-0001');
    assert.deepEqual(await requests[2].json(), { confirmed: true, expectedRole: 'participant' });
    assert.equal(requests[3].method, 'PATCH');
    assert.equal(requests[3].headers.get('Idempotency-Key'), 'change-member-role-0001');
    assert.deepEqual(await requests[3].json(), { confirmed: true, expectedRole: 'participant', role: 'supporter' });
  } finally {
    globalThis.fetch = originalFetch;
  }
});
