import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index';

test('ownership-transfer client uses generated paged and idempotent routes', async (context) => {
  const calls: Array<{ authorization: string | null; body?: unknown; method: string; path: string }> = [];
  const originalFetch = globalThis.fetch;
  context.after(() => { globalThis.fetch = originalFetch; });
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    calls.push({
      authorization: request.headers.get('authorization'),
      body: request.method === 'POST' && request.headers.get('content-type')?.includes('json') ? await request.clone().json() : undefined,
      method: request.method,
      path: new URL(request.url).pathname + new URL(request.url).search,
    });
    return Response.json({ data: { transfer: {
      id: 'transfer-1', pathId: 'path-1', creatorUserId: 'creator', recipientUserId: 'recipient',
      createdAt: '2026-07-27T16:00:00Z', expiresAt: '2026-08-03T16:00:00Z', state: 'pending',
    }, replayed: false }, meta: {} });
  };

  const api = createSessionApiClient('https://api.example', () => 'active-token');
  await api.ownershipTransferCandidates('path-1', 'signed-page');
  await api.pendingOwnershipTransfer('path-1');
  await api.reviewOwnershipTransfer('path-1', { recipientUserId: 'recipient' });
  await api.initiateOwnershipTransfer('path-1', { reservationToken: 'signed-review' }, 'initiate-transfer1');
  await api.acceptOwnershipTransfer('transfer-1', 'accept-transfer-1');
  await api.declineOwnershipTransfer('transfer-1', 'decline-transfer1');
  await api.cancelOwnershipTransfer('transfer-1', 'cancel-transfer-1');

  assert.deepEqual(calls.map(({ method, path }) => [method, path]), [
    ['GET', '/v1/paths/path-1/ownership-transfer-candidates?cursor=signed-page&limit=25'],
    ['GET', '/v1/paths/path-1/ownership-transfer'],
    ['POST', '/v1/paths/path-1/ownership-transfer/review'],
    ['POST', '/v1/paths/path-1/ownership-transfers'],
    ['POST', '/v1/path-ownership-transfers/transfer-1/accept'],
    ['POST', '/v1/path-ownership-transfers/transfer-1/decline'],
    ['POST', '/v1/path-ownership-transfers/transfer-1/cancel'],
  ]);
  assert.ok(calls.every((call) => call.authorization === 'Bearer active-token'));
  assert.deepEqual(calls[2]?.body, { recipientUserId: 'recipient' });
  assert.deepEqual(calls[3]?.body, { reservationToken: 'signed-review' });
});
