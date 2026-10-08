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
  const received: { key: string | undefined; body: unknown; running: string | undefined }[] = [];
  const server = createServer(async (request, response) => {
    assert.equal(request.url, '/v1/paths/guitar/offline-timer');
    assert.equal(request.headers.authorization, 'Bearer account-session');
    let body = '';
    for await (const part of request) body += part;
    received.push({ key: request.headers['idempotency-key'] as string | undefined, body: JSON.parse(body), running: request.headers['x-hourpaths-timer-running'] as string | undefined });
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
  assert.deepEqual(received[0], { key: operation.operationId, running: 'false', body: {
    timerId: 'timer', kind: 'stop', startedAt: operation.startedAt, endedAt: operation.endedAt, occurrenceTimeZone: operation.timeZone,
  } });
  participant = 'bob';
  await assert.rejects(sync.send('alice', operation), /tracking_sync_invalid/);
  status = 200; participant = 'alice';
  await sync.send('alice', { ...operation, kind: 'correct', correctedStartedAt: '2026-10-01T11:59:00Z' });
  assert.deepEqual(received.at(-1)?.body, { timerId: operation.timerId, kind: 'correct', startedAt: operation.startedAt,
    correctedStartedAt: '2026-10-01T11:59:00Z', endedAt: operation.endedAt, occurrenceTimeZone: operation.timeZone });
  await sync.send('alice', { ...operation, kind: 'start', endedAt: undefined }, true);
  assert.equal(received.at(-1)?.running, 'true');
  await sync.send('alice', { ...operation, kind: 'start', endedAt: undefined }, false);
  assert.equal(received.at(-1)?.running, 'false');
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

test('recorded replay binds complete occurrence and maps deletion without losing transient failures', async t => {
  let status = 200, outcome = 'accepted', participant = 'alice', identified = true;
  const received: { key: string | undefined; body: unknown }[] = [];
  const activity = { id: 'entry', participantId: 'alice', pathId: 'guitar', startedAt: '2026-10-01T12:00:00.123456Z', endedAt: '2026-10-01T12:01:00.123456Z', occurrenceTimeZone: 'Asia/Tokyo', durationSeconds: 60, note: 'draft', createdAt: '2026-10-01T13:00:00Z', updatedAt: '2026-10-01T13:00:00Z' };
  const server = createServer(async (request, response) => {
    assert.equal(request.url, '/v1/paths/guitar/offline-activity');
    let body = ''; for await (const part of request) body += part;
    received.push({ key: request.headers['idempotency-key'] as string, body: JSON.parse(body) });
    response.writeHead(status, { 'content-type': 'application/json' });
    response.end(JSON.stringify(status === 200 ? { data: { outcome, ...(outcome === 'accepted' ? { activity: { ...activity, participantId: participant }, order: { authoredAt: '2026-10-01T12:30:00Z', counter: 2, operationId: 'operation-000001' } } : {}) } } : { ...(identified ? { operation: 'synchronize-offline-path-activity' } : {}) }));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  const api = createSessionApiClient(`http://127.0.0.1:${address.port}`, () => 'session');
  const sync = apiTrackingSync(owner => owner === 'alice' ? api : null);
  const command = { kind: 'edit' as const, operationId: 'operation-000001', activity: { id: 'entry', owner: 'alice', pathId: 'guitar', startedAt: activity.startedAt, endedAt: activity.endedAt, timeZone: activity.occurrenceTimeZone, note: 'draft' }, stamp: { authoredAt: '2026-10-01T12:30:00Z', counter: 2 } };
  assert.ok(sync.sendActivity);
  const result = await sync.sendActivity('alice', command);
  assert.equal(result.kind, 'accepted');
  assert.deepEqual(received[0], { key: command.operationId, body: { kind: 'edit', activityId: 'entry', startedAt: activity.startedAt, durationSeconds: 60, occurrenceTimeZone: 'Asia/Tokyo', note: 'draft', authoredAt: command.stamp.authoredAt, counter: 2 } });
  if (result.kind === 'accepted') { assert.equal(result.activity.note, 'draft'); assert.deepEqual(result.activity.editStamp, command.stamp); }
  participant = 'bob'; await assert.rejects(sync.sendActivity('alice', command), /tracking_sync_invalid/);
  outcome = 'deleted'; assert.deepEqual(await sync.sendActivity('alice', command), { kind: 'rejected', reason: 'deleted', disclosePath: true });
  status = 503; await assert.rejects(sync.sendActivity('alice', command), /temporarily_unavailable/);
  status = 404; assert.deepEqual(await sync.sendActivity('alice', command), { kind: 'rejected', reason: 'membership', disclosePath: false });
  identified = false; await assert.rejects(sync.sendActivity('alice', command), /temporarily_unavailable/);
  status = 401; await assert.rejects(sync.sendActivity('alice', command), TrackingReplaySuspended);
});
