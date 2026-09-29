const utcTimeZone = 'utc';
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { trackingFromAPI } from './studio/paths/adapters/api-path-repository';

test('rejects a running timer with no identity rather than offering an incorrect Start action', () => {
  assert.throws(() => trackingFromAPI({ running: true, accumulatedSeconds: 30 }), /path_request_failed/);
});
test('maps persisted totals separately from session time and rejects inverted periods', () => {
  const dto = { running: true, accumulatedSeconds: 120, timer: { id: 'session', pathId: 'path', occurrenceTimeZone: utcTimeZone, startedAt: '2026-09-29T12:00:00Z' } };
  const mapped = trackingFromAPI(dto);
  assert.equal(mapped.savedTotalSeconds, 120);
  assert.equal(mapped.activeSession?.startedAt, Date.parse(dto.timer.startedAt));
  assert.throws(() => trackingFromAPI({ ...dto, intervalProgress: {
    accumulatedSeconds: 10, targetSeconds: 60, startedAt: '2026-09-30T00:00:00Z', endedAt: '2026-09-29T00:00:00Z',
  } }), /path_request_failed/);
});

test('appearance saving uses the reviewed revision and surfaces a concurrent edit', async context => {
  const { createServer } = await import('node:http');
  const { once } = await import('node:events');
  const { apiPathRepository, PathRequestError } = await import('./studio/paths/adapters/api-path-repository');
  const received: { method: string | undefined; body: unknown }[] = [];
  const server = createServer(async (request, response) => {
    let body = '';
    for await (const chunk of request) body += chunk;
    received.push({ method: request.method, body: body ? JSON.parse(body) : null });
    response.writeHead(409, { 'Content-Type': 'application/problem+json' });
    response.end(JSON.stringify({ code: 'conflict' }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  context.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const repository = apiPathRepository(`http://127.0.0.1:${address.port}`, () => 'session', () => assert.fail('unexpected_rejection'));
  await assert.rejects(repository.saveAppearance('path', { color: 'gold', emoji: '🎸', revision: 7 }, 'operation-1'), error => error instanceof PathRequestError && error.status === 409);
  assert.deepEqual(received, [{ method: 'PUT', body: { color: 'gold', emoji: '🎸', expectedRevision: 7 } }]);
});
