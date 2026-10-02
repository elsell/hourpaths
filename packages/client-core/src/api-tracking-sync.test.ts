import assert from 'node:assert/strict';
import test from 'node:test';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { createSessionApiClient } from '@hourpaths/api-client';
import { apiTrackingSync } from './adapters/api-tracking-sync';
import { TrackingReplaySuspended } from './offline-tracking';

test('real HTTP replay retains identity and maps archive, authorization and foreign-account responses safely', async t => {
  let status = 200;
  let participant = 'alice';
  let identified = true;
  const received: { key: string | undefined; body: unknown }[] = [];
  const server = createServer(async (request, response) => {
    assert.equal(request.url, '/v1/paths/guitar/offline-timer');
    assert.equal(request.headers.authorization, 'Bearer account-session');
    let body = '';
    for await (const part of request) body += part;
    received.push({ key: request.headers['idempotency-key'] as string | undefined, body: JSON.parse(body) });
    response.writeHead(status, { 'content-type': 'application/json' });
    response.end(JSON.stringify(status === 200 ? { data: {
      outcome: 'archived', terminal: true, mustStop: false, savedSeconds: 120, discardedSeconds: 180,
      activity: { id: 'entry', participantId: participant, pathId: 'guitar', startedAt: '2026-10-01T12:00:00Z',
        endedAt: '2026-10-01T12:02:00Z', occurrenceTimeZone: 'America/New_York', durationSeconds: 120,
        createdAt: '2026-10-01T12:10:00Z', updatedAt: '2026-10-01T12:10:00Z' },
    } } : { code: 'request_failed', ...(identified ? { operation: 'synchronize-offline-path-timer' } : {}) }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const api = createSessionApiClient(`http://127.0.0.1:${address.port}`, () => 'account-session');
  const sync = apiTrackingSync(owner => owner === 'alice' ? api : null);
  const operation = { id: 'timer', timerId: 'timer', operationId: 'durable-operation-01', kind: 'stop' as const,
    pathId: 'guitar', startedAt: '2026-10-01T12:00:00Z', endedAt: '2026-10-01T12:05:00Z', timeZone: 'America/New_York' };
  const result = await sync.send('alice', operation);
  assert.equal(result.kind, 'rejected');
  assert.equal(result.activity?.owner, 'alice');
  assert.deepEqual(received[0], { key: operation.operationId, body: {
    timerId: 'timer', kind: 'stop', startedAt: operation.startedAt, endedAt: operation.endedAt, occurrenceTimeZone: operation.timeZone,
  } });
  participant = 'bob';
  await assert.rejects(sync.send('alice', operation), /tracking_sync_invalid/);
  status = 200; participant = 'alice';
  await sync.send('alice', { ...operation, kind: 'correct', correctedStartedAt: '2026-10-01T11:59:00Z' });
  assert.deepEqual(received.at(-1)?.body, { timerId: operation.timerId, kind: 'correct', startedAt: operation.startedAt,
    correctedStartedAt: '2026-10-01T11:59:00Z', endedAt: operation.endedAt, occurrenceTimeZone: operation.timeZone });
  status = 503;
  await assert.rejects(sync.send('alice', operation), /temporarily_unavailable/);
  status = 404;
  assert.deepEqual(await sync.send('alice', operation), { kind: 'rejected', reason: 'membership', disclosePath: false });
  identified = false;
  await assert.rejects(sync.send('alice', operation), /temporarily_unavailable/);
  status = 401;
  await assert.rejects(sync.send('alice', operation), TrackingReplaySuspended);
  const before = received.length;
  await assert.rejects(sync.send('bob', operation), TrackingReplaySuspended);
  assert.equal(received.length, before);
});
