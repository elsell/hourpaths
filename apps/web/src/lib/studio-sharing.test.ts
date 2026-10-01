import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { apiSharingRepository } from './studio/sharing/adapters/api-sharing-repository';
import { sharingCommands } from './studio/sharing/application/sharing';
test('sharing binds reviewed identity and stable retries; denies stale capability and mismatched list context', async () => {
  const calls: { key?: string; body: unknown }[] = []; let allowed = true; let malformed = false;
  const person = { userId: 'recipient', username: 'person', displayName: 'person' };
  const invitation = { id: 'invite', pathId: 'path', inviterUserId: 'owner', recipientUserId: 'recipient', offeredRole: 'participant', createdAt: '2026-01-01T00:00:00Z' };
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json'); let data: unknown;
    if (request.method === 'POST') { let body = ''; for await (const part of request) body += part; calls.push({ key: request.headers['idempotency-key'] as string, body: JSON.parse(body) }); response.statusCode = calls.length === 1 ? 503 : 201; data = invitation; }
    else if (request.url?.includes('invitation-recipient')) data = person;
    else if (request.url?.includes('invitations')) data = [{ invitation: { ...invitation, pathId: malformed ? 'other' : 'path' }, recipient: person, inviter: { userId: 'owner', username: 'owner', displayName: 'owner' } }];
    else data = { id: 'path', name: 'path', archivedAt: null, capabilities: { inviteMembers: allowed } };
    response.end(JSON.stringify({ data, meta: { nextCursor: '' } }));
  }); server.listen(0, '127.0.0.1'); await once(server, 'listening');
  try {
    const repo = apiSharingRepository(`http://127.0.0.1:${(server.address() as { port: number }).port}`, () => 'credential', () => {});
    assert.equal((await repo.context('path')).name, 'path');
    const command = sharingCommands(repo, () => 'stable-invitation-key');
    const result = await command.review('path', 'person'); assert.equal(result.kind, 'reviewed'); if (result.kind !== 'reviewed') return;
    assert.equal((await command.send(result.review, 'participant')).kind, 'failed');
    assert.equal((await command.send(result.review, 'participant')).kind, 'sent');
    assert.equal(calls[0]?.key, calls[1]?.key); assert.deepEqual(calls[1]?.body, { username: 'person', expectedRecipientUserId: 'recipient', offeredRole: 'participant' });
    assert.equal((await repo.pending('path', '')).items.length, 1);
    malformed = true; await assert.rejects(repo.pending('path', ''));
    allowed = false; await assert.rejects(repo.context('path'));
  } finally { server.close(); server.closeAllConnections(); }
});
