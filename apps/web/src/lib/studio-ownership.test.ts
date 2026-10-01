import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { apiOwnershipRepository } from './studio/ownership/adapters/api-ownership-repository';
import { ownershipCommands } from './studio/ownership/application/commands';
test('ownership review and receipts bind canonical recipient and immutable expiration; unresolved retry preserves identity', async () => {
  const review = { recipient: { userId: 'recipient', username: 'recipient', displayName: 'recipient' }, reservationToken: 'reservation', reviewedAt: '2026-10-01T12:00:00Z', expiresAt: '2026-10-02T13:30:00Z', viewerTimeZone: 'UTC' };
  const transfer = { id: 'transfer', pathId: 'path', creatorUserId: 'owner', recipientUserId: 'recipient', createdAt: '2026-10-01T12:01:00Z', reviewedAt: review.reviewedAt, expiresAt: review.expiresAt, state: 'pending' };
  const calls: { key: string; body: unknown }[] = [];
  let wrongReceipt = false, wrongReview = false;
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json'); let data: unknown;
    if (request.url?.endsWith('/review')) data = { ...review, recipient: { ...review.recipient, userId: wrongReview ? 'other' : 'recipient' } };
    else {
      let body = ''; for await (const part of request) body += part;
      calls.push({ key: request.headers['idempotency-key'] as string, body: JSON.parse(body) });
      response.statusCode = calls.length === 1 ? 503 : 200;
      data = { transfer: { ...transfer, expiresAt: wrongReceipt ? '2026-10-04T13:30:00Z' : transfer.expiresAt }, counterpart: review.recipient, counterpartRole: 'recipient', replayed: calls.length > 1, viewerTimeZone: 'UTC' };
    }
    response.end(JSON.stringify({ data }));
  }); server.listen(0, '127.0.0.1'); await once(server, 'listening');
  try {
    const repo = apiOwnershipRepository(`http://127.0.0.1:${(server.address() as { port: number }).port}`, () => 'credential', () => {});
    const reviewed = await repo.review('path', 'recipient');
    let keys = 0;
    const commands = ownershipCommands(repo, () => `key-${++keys}`);
    const action = { kind: 'initiate' as const, pathId: 'path', review: reviewed };
    await assert.rejects(commands.submit(action));
    assert.equal((await commands.submit(action))?.state, 'pending');
    assert.deepEqual(calls[0], calls[1]);
    assert.equal(keys, 1);
    assert.deepEqual(calls[1]?.body, { reservationToken: 'reservation' });
    wrongReceipt = true;
    await assert.rejects(commands.submit(action));
    wrongReview = true; await assert.rejects(repo.review('path', 'recipient'));
    commands.dispose(); const count = calls.length;
    assert.equal(await commands.submit(action), null);
    assert.equal(calls.length, count);
  } finally { server.close(); server.closeAllConnections(); }
});
