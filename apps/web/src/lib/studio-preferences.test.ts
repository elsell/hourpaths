import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { apiPreferencesRepository } from './studio/preferences/adapters/api-preferences-repository';
import { PreferenceFailure } from './studio/preferences/domain/preferences';

test('preference transport preserves reviewed zone, retry identity, revision and session classification', async context => {
  const london = 'europe/london';
  const newYork = 'america/new_york';
  const requests: { path?: string; key?: string; body: unknown }[] = [];
  let status = 200, rejected = 0;
  const server = createServer(async (request, response) => {
    let body = ''; for await (const chunk of request) body += chunk;
    requests.push({ path: request.url, key: request.headers['idempotency-key'] as string, body: body ? JSON.parse(body) : null });
    response.writeHead(status, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ data: request.url?.includes('time-zone') ? { timeZone: london, effectiveAt: '2026-09-29T12:00:00Z', changed: true } : { channel: 'nudges', enabled: false, revision: 4 } }));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  context.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address(); assert.ok(address && typeof address !== 'string');
  const repository = apiPreferencesRepository(`http://127.0.0.1:${address.port}`, () => 'session', () => { rejected++; });
  const change = { reviewed: newYork, proposed: london };
  await repository.changeTimeZone(change, 'same-operation');
  await repository.changeTimeZone(change, 'same-operation');
  assert.deepEqual(requests[0], requests[1]);
  assert.deepEqual(requests[0].body, { reviewedTimeZone: change.reviewed, proposedTimeZone: change.proposed, confirmed: true });
  await repository.saveNudges({ enabled: false, revision: 3 }, 'nudge-operation');
  assert.deepEqual(requests[2].body, { enabled: false, expectedRevision: 3 });
  status = 409;
  await assert.rejects(repository.changeTimeZone(change, 'same-operation'), error => error instanceof PreferenceFailure && error.kind === 'conflict');
  status = 503;
  await assert.rejects(repository.timeZone()); assert.equal(rejected, 0);
  status = 401;
  await assert.rejects(repository.timeZone()); assert.equal(rejected, 1);
});
