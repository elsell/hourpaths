import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { apiSharingRepository } from './studio/sharing/adapters/api-sharing-repository';
test('inbox binds visibility acknowledgement and acceptance receipt to reviewed invitation; rejects mismatched public identity', async () => {
  const calls: { key: string; body: unknown }[] = [];
  let malformed = false, wrongRole = false;
  const invitation = { id: 'invite', pathId: 'path', inviterUserId: 'owner', recipientUserId: 'recipient', offeredRole: 'participant', createdAt: '2026-01-01T00:00:00Z' };
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json'); let data: unknown;
    if (request.method === 'POST') {
      let body = ''; for await (const part of request) body += part;
      calls.push({ key: request.headers['idempotency-key'] as string, body: body ? JSON.parse(body) : undefined });
      response.statusCode = calls.length === 1 ? 503 : 200;
      data = { ...invitation, offeredRole: wrongRole ? 'supporter' : 'participant', acceptedAt: '2026-01-02T00:00:00Z' };
    } else data = [{ invitation, pathName: 'path', inviter: { userId: malformed ? 'other' : 'owner', username: 'owner', displayName: 'owner' }, warning: { pathVisibility: 'public', hasRetainedActivity: true } }];
    response.end(JSON.stringify({ data, meta: { nextCursor: '' } }));
  }); server.listen(0, '127.0.0.1'); await once(server, 'listening');
  try {
    const repo = apiSharingRepository(`http://127.0.0.1:${(server.address() as { port: number }).port}`, () => 'credential', () => {});
    const item = (await repo.inbox('')).items[0]!;
    const commands = repo.inboxCommands(() => 'stable-acceptance-key');
    assert.equal((await commands.respond(item, 'accept')).kind, 'failed');
    assert.equal((await commands.respond(item, 'accept')).kind, 'accepted');
    assert.deepEqual(calls[0], calls[1]);
    assert.deepEqual(calls[1]?.body, { visibilityWarningAcknowledgement: { pathVisibility: 'public' } });
    wrongRole = true; commands.clear();
    assert.equal((await commands.respond(item, 'accept')).kind, 'failed');
    malformed = true; await assert.rejects(repo.inbox(''));
    commands.dispose(); const count = calls.length;
    assert.equal((await commands.respond(item, 'accept')).kind, 'superseded');
    assert.equal(calls.length, count);
  } finally { server.close(); server.closeAllConnections(); }
});
