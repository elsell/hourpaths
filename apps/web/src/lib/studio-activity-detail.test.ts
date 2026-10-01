import assert from 'node:assert/strict';
import { test } from 'node:test';
import { reviewActivityDeletion, deleteReviewedActivity } from './studio/history/application/activity-deletion';
import type { ActivityDetail } from './studio/history/domain/detail';
const zone = 'utc';
const detail: ActivityDetail = { id: 'entry', pathId: 'path', participantId: 'owner', pathName: 'practice-path', owned: true, startedAt: 1000, endedAt: 2000, seconds: 1, timeZone: zone, version: 1, note: null };
test('deletion requires owner review and retains identity and key across uncertain retries', async () => {
  assert.throws(() => reviewActivityDeletion({ ...detail, owned: false }, 'key'));
  const review = reviewActivityDeletion(detail, 'same-key');
  const requests: unknown[] = [];
  const repository = { async detail() { return detail; }, async revisions() { return { items: [], next: null }; }, async remove(value: typeof review) { requests.push(value); if (requests.length === 1) throw new Error('connection_lost'); return { accumulatedSeconds: 0, sessionCount: 0, unreadNotificationCount: 0, removedFeedEventIds: [], period: null }; } };
  await assert.rejects(deleteReviewedActivity(repository, review));
  await deleteReviewedActivity(repository, review);
  assert.deepEqual(requests, [review, review]);
  assert.equal(Object.isFrozen(review), true);
});
import { createServer } from 'node:http';
import { once } from 'node:events';
import { apiActivityRepository } from './studio/history/adapters/api-activity-repository';
test('HTTP detail isolates identity, redacts nonowner notes, and rejects repeated revision cursors', async () => {
  let owner = 'owner';
  let activityId = 'entry';
  const server = createServer((request, response) => {
    response.setHeader('Content-Type', 'application/json');
    const activity = { id: activityId, pathId: 'path', participantId: owner, startedAt: '2026-01-01T00:00:00Z', endedAt: '2026-01-01T00:01:00Z', durationSeconds: 60, occurrenceTimeZone: zone, note: 'private-note' };
    const url = new URL(request.url!, 'http://localhost');
    const body = url.pathname === '/v1/me' ? { data: { id: 'owner' } } : url.pathname === '/v1/paths/path' ? { data: { id: 'path', name: 'practice-path' } } : url.pathname.endsWith('/revisions') ? { data: [{ ...activity, version: 1, replacedAt: '2026-01-02T00:00:00Z' }], meta: { nextCursor: 'again' } } : { data: { activity, version: 2 } };
    response.end(JSON.stringify(body));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  const address = server.address() as { port: number };
  const repository = apiActivityRepository(`http://127.0.0.1:${address.port}`, () => 'credential', () => {});
  try {
    assert.equal((await repository.detail('path', 'entry')).note, 'private-note');
    owner = 'someone-else';
    assert.equal((await repository.detail('path', 'entry')).owned, false);
    assert.equal((await repository.detail('path', 'entry')).note, null);
    assert.equal((await repository.revisions('path', 'entry', null)).items[0]?.note, null);
    await assert.rejects(repository.revisions('path', 'entry', 'again'));
    activityId = 'unrelated-entry';
    await assert.rejects(repository.detail('path', 'entry'));
  } finally { server.close(); server.closeAllConnections(); }
});
import { QueryClient } from '@tanstack/react-query';
import { applyActivityDeletion } from './studio/presentation/activity-cache';
test('deletion applies authoritative totals and removes exact projections without discarding unrelated data', () => {
  const client = new QueryClient();
  const cursor = { participantId: 'owner', streams: [{ pathId: 'path', pathName: 'practice-path', remaining: [{ id: 'entry' }, { id: 'other' }], cursor: null, loaded: true }] };
  client.setQueryData(['account', 'history'], { pages: [{ items: [{ id: 'entry' }, { id: 'other' }], next: cursor }], pageParams: [cursor] });
  client.setQueryData(['account', 'social', 'feed', null], { pages: [{ items: [{ id: 'removed' }, { id: 'retained' }], next: null }], pageParams: [null] });
  client.setQueryData(['account', 'tracking', 'path'], { savedTotalSeconds: 100, activeSession: { id: 'running', startedAt: 1 }, period: null });
  client.setQueryData(['account', 'appearance', 'path'], { emoji: '✨' });
  applyActivityDeletion(client, 'account', reviewActivityDeletion(detail, 'key'), { accumulatedSeconds: 99, sessionCount: 1, unreadNotificationCount: 0, removedFeedEventIds: ['removed'], period: null });
  const history = client.getQueryData<any>(['account', 'history']);
  assert.deepEqual(history.pages[0].items, [{ id: 'other' }]);
  assert.deepEqual(history.pageParams[0].streams[0].remaining, [{ id: 'other' }]);
  assert.deepEqual(client.getQueryData<any>(['account', 'social', 'feed', null]).pages[0].items, [{ id: 'retained' }]);
  assert.equal(client.getQueryData<any>(['account', 'tracking', 'path']).savedTotalSeconds, 99);
  assert.equal(client.getQueryData<any>(['account', 'tracking', 'path']).activeSession.id, 'running');
  assert.deepEqual(client.getQueryData(['account', 'appearance', 'path']), { emoji: '✨' });
  client.clear();
});
