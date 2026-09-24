import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index.js';

test('archive state uses the dedicated generated route with reviewed state and replay key', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    if (request.method === 'GET') {
      return Response.json({ data: [], meta: {} });
    }
    return Response.json({ data: { id: 'path-1', name: 'Read', visibility: 'private', archivedAt: '2026-07-22T12:00:00Z' } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const api = createSessionApiClient('https://api.example.test', () => 'current-token');
  await api.setPathArchiveState('path-1', {
    confirmed: true,
    expectedArchived: false,
    archived: true,
  }, 'path-archive-key-0001');
  await api.archivedPaths('archived-cursor');

  assert.equal(captured[0]?.method, 'PUT');
  assert.equal(new URL(captured[0]!.url).pathname, '/v1/paths/path-1/archive-state');
  assert.equal(captured[0]?.headers.get('authorization'), 'Bearer current-token');
  assert.equal(captured[0]?.headers.get('idempotency-key'), 'path-archive-key-0001');
  assert.deepEqual(await captured[0]?.json(), { confirmed: true, expectedArchived: false, archived: true });
  assert.equal(captured[1]?.method, 'GET');
  assert.equal(new URL(captured[1]!.url).search, '?archived=true&cursor=archived-cursor&limit=25');
});
