import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { apiNudgesRepository } from './studio/nudges/adapters/api-nudges-repository';
test('encouragement retry retains its intent, rejects cross-recipient data, and disposal prevents writes', async () => {
  const calls: { key: string; body: unknown }[] = []; let wrong = false;
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json');
    let data: unknown = { eligible: true, pathId: 'path', recipientUserId: wrong ? 'other' : 'person' };
    if (request.method === 'POST') {
      let body = ''; for await (const part of request) body += part;
      calls.push({ key: String(request.headers['idempotency-key']), body: JSON.parse(body) });
      response.statusCode = calls.length === 1 ? 503 : 200;
      data = { id: 'receipt', pathId: 'path', recipientUserId: wrong ? 'other' : 'person', senderUserId: 'sender', sentAt: '2026-10-01T10:00:00Z', content: { kind: 'preset', preset: 'keep_it_going' } };
    }
    response.end(JSON.stringify({ data }));
  }); server.listen(0, '127.0.0.1'); await once(server, 'listening');
  try {
    const repo = apiNudgesRepository(`http://127.0.0.1:${(server.address() as { port: number }).port}`, () => 'credential', () => {});
    assert.equal((await repo.eligibility('path', 'person')).eligible, true);
    wrong = true; await assert.rejects(repo.eligibility('path', 'person')); wrong = false;
    const commands = repo.commands(() => 'stable-nudge-key-value');
    assert.equal((await commands.send('path', 'person', 'keep_it_going')).kind, 'failed');
    assert.equal((await commands.send('path', 'person', 'keep_it_going')).kind, 'applied');
    assert.deepEqual(calls[0], calls[1]);
    wrong = true; assert.equal((await commands.send('path', 'person', 'keep_it_going')).kind, 'failed');
    commands.dispose(); const before = calls.length;
    assert.equal((await commands.send('path', 'person', 'keep_it_going')).kind, 'superseded'); assert.equal(calls.length, before);
  } finally { server.close(); server.closeAllConnections(); }
});
test('audience saves retain reviewed revision and reject cross-user preference receipts', async () => {
  let wrong = false; const writes: unknown[] = [];
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json');
    if (request.method === 'PUT') { let body=''; for await(const part of request) body+=part; writes.push(JSON.parse(body)); }
    const updating = request.method === 'PUT';
    response.end(JSON.stringify({ data: { pathId:'path',userId:wrong?'other':'person',audience:updating?'nobody':'path_members',revision:updating?2:1 } }));
  }); server.listen(0,'127.0.0.1'); await once(server,'listening');
  try {
    const repo=apiNudgesRepository(`http://127.0.0.1:${(server.address() as {port:number}).port}`,()=> 'credential',()=>{});
    const preference=await repo.audience('path'), commands=repo.commands(()=> 'stable-audience-key');
    wrong=true; assert.equal((await commands.save(preference,'nobody')).kind,'failed');
    wrong=false; assert.equal((await commands.save(preference,'nobody')).kind,'applied');
    assert.deepEqual(writes,[{audience:'nobody',expectedRevision:1},{audience:'nobody',expectedRevision:1}]);
  } finally { server.close(); server.closeAllConnections(); }
});
