import assert from 'node:assert/strict';
import test from 'node:test';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { createSessionApiClient } from '@hourpaths/api-client';
import { apiTrackingHistory } from './adapters/api-tracking-history';

test('retained history reads every own-account page and rejects incomplete or foreign snapshots', async t => {
  let mode: 'valid' | 'failure' | 'foreign' | 'repeat' = 'valid';
  const requested: string[] = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url!, 'http://localhost'); requested.push(request.url!);
    assert.equal(request.headers.authorization, 'Bearer session');
    let body: unknown;
    if (url.pathname === '/v1/paths') body = { data: [{ id: 'guitar', name: 'Guitar' }], meta: {} };
    else if (url.pathname === '/v1/paths/archived') body = { data: [], meta: {} };
    else {
      assert.equal(url.pathname, '/v1/paths/guitar/activities');
      assert.equal(url.searchParams.get('participantId'), 'alice');
      const second = url.searchParams.has('cursor');
      if (second && mode === 'failure') { response.writeHead(503); response.end('{}'); return; }
      body = { data: [{ version: 2, activity: { id: second ? 'older' : 'newer', pathId: 'guitar', participantId: mode === 'foreign' ? 'bob' : 'alice',
        startedAt: '2026-10-01T12:00:00Z', endedAt: '2026-10-01T12:01:00Z', durationSeconds: 60, occurrenceTimeZone: 'UTC',
        createdAt: '2026-10-01T12:01:00Z', updatedAt: '2026-10-01T12:02:00Z', note: 'Practice' } }],
        meta: { nextCursor: !second || mode === 'repeat' ? 'next-page' : '' } };
    }
    response.writeHead(200, { 'Content-Type': 'application/json' }); response.end(JSON.stringify(body));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  const api = createSessionApiClient(`http://127.0.0.1:${address.port}`, () => 'session');
  const load = apiTrackingHistory(owner => owner === 'alice' ? api : null, () => Date.parse('2026-10-01T12:00:00Z'));
  const entries = await load('alice');
  assert.deepEqual(entries.map(entry => entry.id), ['newer', 'older']);
  assert.equal(entries[0].note, 'Practice'); assert.equal(entries[0].version, 2);
  mode = 'failure'; await assert.rejects(load('alice'), /unavailable/);
  mode = 'foreign'; await assert.rejects(load('alice'), /invalid/);
  mode = 'repeat'; await assert.rejects(load('alice'), /cursor_repeated/);
  const before = requested.length; await assert.rejects(load('bob'), /suspended/); assert.equal(requested.length, before);
});
