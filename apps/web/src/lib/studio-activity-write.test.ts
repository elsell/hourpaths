import { test } from 'node:test';
import assert from 'node:assert/strict';
import { reviewActivityWrite, saveReviewedActivity } from './studio/history/application/activity-write';
test('activity command preserves exact seconds, normalizes private note, and retries the same command', async () => {
  const note = ['', 'e\u0301', '🎸', ''].join(' ');
  const input = { localDate: '2026-01-01', localTime: '01:30:41', seconds: 14021, note };
  const review = reviewActivityWrite('path', 'entry', input, 'stable-key');
  assert.equal(review.input.seconds, 14021); assert.equal(review.input.note, ' é 🎸 ');
  const received: unknown[] = [];
  const repository = { async save(value: typeof review) { received.push(value); if (received.length === 1) throw new Error('connection_lost'); return { id: 'entry', version: 2 }; } };
  await assert.rejects(saveReviewedActivity(repository, review)); await saveReviewedActivity(repository, review);
  assert.deepEqual(received, [review, review]);
  for (const note of ['x'.repeat(2001), 'private\u0001note']) assert.throws(() => reviewActivityWrite('path', null, { ...input, note }, 'key'));
});
import { createServer } from 'node:http';
import { once } from 'node:events';
import { apiActivityRepository } from './studio/history/adapters/api-activity-repository';
test('generated transport sends exact reviewed occurrence and note with the stable edit key', async () => {
  const requests: unknown[] = [];
  const server = createServer(async (request, response) => {
    let body = ''; for await (const chunk of request) body += chunk;
    requests.push({ method: request.method, url: request.url, key: request.headers['idempotency-key'], body: JSON.parse(body) });
    response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify({ data: { activity: { id: 'entry', pathId: 'path' }, version: 2 } }));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  const address = server.address() as { port: number };
  try {
    const repository = apiActivityRepository(`http://127.0.0.1:${address.port}`, () => 'credential', () => {});
    const input = { localDate: '2026-01-01', localTime: '01:30:41', seconds: 14021, note: 'private-note' };
    assert.deepEqual(await repository.save(reviewActivityWrite('path', 'entry', input, 'same-key')), { id: 'entry', version: 2 });
    assert.deepEqual(requests, [{ method: 'PUT', url: '/v1/paths/path/activities/entry', key: 'same-key', body: { localDate: input.localDate, localStartTime: input.localTime, durationSeconds: 14021, note: input.note } }]);
  } finally {server.close();server.closeAllConnections();}
});
