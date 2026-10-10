import assert from 'node:assert/strict';
import test from 'node:test';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { createSessionApiClient } from '@hourpaths/api-client';
import { apiTrackingHistory } from './adapters/api-tracking-history';

test('retained history reads every own-account page and rejects incomplete or foreign snapshots', async t => {
  let mode: 'valid' | 'failure' | 'foreign' | 'repeat' | 'empty' = 'valid';
  const requested: string[] = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url!, 'http://localhost'); requested.push(request.url!);
    assert.equal(request.headers.authorization, 'Bearer session');
    let body: unknown;
    {
      assert.equal(url.pathname, '/v1/me/activity-history');
      assert.equal(url.searchParams.get('window'), 'retained');
      const second = !!url.searchParams.get('cursor');
      if (second && mode === 'failure') { response.writeHead(503); response.end('{}'); return; }
      body = { data: [{ pathName: 'Guitar', version: 2, activity: { id: second ? 'older' : 'newer', pathId: 'guitar', participantId: mode === 'foreign' ? 'bob' : 'alice',
        startedAt: '2026-10-01T12:00:00Z', endedAt: '2026-10-01T12:01:00Z', durationSeconds: 60, occurrenceTimeZone: 'UTC',
        createdAt: '2026-10-01T12:01:00Z', updatedAt: '2026-10-01T12:02:00Z', note: 'Practice' } }],
        meta: { nextCursor: !second || mode === 'repeat' ? 'next-page' : '' } };
    }
    if (mode === 'empty' && !url.searchParams.get('cursor')) body = { data: [], meta: { nextCursor: 'next-page' } };
    response.writeHead(200, { 'Content-Type': 'application/json' }); response.end(JSON.stringify(body));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  const api = createSessionApiClient(`http://127.0.0.1:${address.port}`, () => 'session');
  const load = apiTrackingHistory(owner => owner === 'alice' ? api : null, () => Date.parse('2026-10-01T12:00:00Z'));
  const entries = await load('alice');
  assert.deepEqual(entries.map(entry => entry.id), ['newer', 'older']);
  assert.equal(requested.length, 2);
  assert.equal(entries[0].note, 'Practice'); assert.equal(entries[0].version, 2);
  mode = 'empty'; assert.deepEqual((await load('alice')).map(entry => entry.id), ['older']);
  mode = 'failure'; await assert.rejects(load('alice'), /unavailable/);
  mode = 'foreign'; await assert.rejects(load('alice'), /invalid/);
  mode = 'repeat'; await assert.rejects(load('alice'), /cursor_repeated/);
  const before = requested.length; await assert.rejects(load('bob'), /suspended/); assert.equal(requested.length, before);
});
