import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { createSessionApiClient } from '@hourpaths/api-client';
import { sharedVisibilityCommands } from './studio/paths/adapters/shared-visibility-commands';

test('visibility reviews enforce current capability/privacy and bind retry and conflict recovery', async () => {
  const ownerName = 'owner';
  let privacy = 'private', allowed = true, archived = false, visibility = 'private', failure = 503, wrong = false;
  const writes: { key: string; body: unknown }[] = [];
  let defer = false, finish!: () => void, started!: () => void;
  const writing = new Promise<void>(resolve => { started = resolve; });
  const path = () => ({ id: wrong ? 'other' : 'path', name: 'Practice', visibility, archivedAt: archived ? '2026-10-01T10:00:00Z' : undefined,
    capabilities: { manageVisibility: allowed, manageGoals: true, trackTime: true, renamePath: true } });
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json'); let data: unknown;
    if (request.method === 'PUT') {
      let body = ''; for await (const part of request) body += part;
      if (defer) { started(); await new Promise<void>(resolve => { finish = resolve; }); }
      const value = JSON.parse(body); writes.push({ key: String(request.headers['idempotency-key']), body: value });
      response.statusCode = failure || 200;
      if (!failure) visibility = value.visibility;
      data = path();
    } else data = request.url === '/v1/me'
      ? { id: 'owner', email: 'owner@example.test', displayName: ownerName, profileVisibility: privacy }
      : path();
    response.end(JSON.stringify({ data }));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  try {
    const port = (server.address() as { port: number }).port;
    const url = `http://127.0.0.1:${port}`; let serial = 0;
    const commands = sharedVisibilityCommands(signal => createSessionApiClient(url, () => 'credential', signal), () => `visibility-operation-${++serial}`);
    let context = await commands.load('path');
    assert.deepEqual(context.options, ['private', 'followers']);
    assert.throws(() => commands.review(context, 'public'));
    assert.equal(commands.review(context, 'private'), null); assert.equal(writes.length, 0);
    const review = commands.review(context, 'followers')!; assert.equal(review.broader, true);
    assert.equal((await commands.submit(review)).kind, 'failed');
    failure = 0; const applied = await commands.submit(review); assert.equal(applied.kind, 'applied');
    assert.deepEqual(writes[0], writes[1]); assert.deepEqual(writes[1].body, { confirmed: true, expectedVisibility: 'private', visibility: 'followers' });
    assert.equal((await commands.submit(review)).kind, 'superseded');
    privacy = 'public'; context = await commands.load('path'); assert.deepEqual(context.options, ['private', 'followers', 'public']);
    const narrowed = commands.review(context, 'private')!; assert.equal(narrowed.broader, false);
    failure = 409; assert.deepEqual(await commands.submit(narrowed), { kind: 'failed', requiresReview: true });
    assert.equal((await commands.submit(narrowed)).kind, 'superseded');
    failure = 0; context = await commands.load('path');
    wrong = true; assert.equal((await commands.submit(commands.review(context, 'private')!)).kind, 'failed'); wrong = false;
    allowed = false; await assert.rejects(commands.load('path')); allowed = true;
    archived = true; await assert.rejects(commands.load('path')); archived = false;
    context = await commands.load('path'); const pending = commands.review(context, 'public')!;
    commands.dispose(); const count = writes.length;
    assert.equal((await commands.submit(pending)).kind, 'superseded'); assert.equal(writes.length, count);
    const replacement = sharedVisibilityCommands(signal => createSessionApiClient(url, () => 'credential', signal), () => 'late-visibility-operation');
    const fresh = await replacement.load('path'); defer = true;
    const late = replacement.submit(replacement.review(fresh, 'public')!); await writing;
    replacement.dispose(); finish(); assert.equal((await late).kind, 'superseded');
  } finally { server.close(); server.closeAllConnections(); }
});
