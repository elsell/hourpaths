import test from 'node:test';
import assert from 'node:assert/strict';
import { contributionWeeks, calendarGroups } from './studio/analytics/domain/statistics';
import { statisticsFromAPI } from './studio/analytics/adapters/api-statistics-repository';

test('calendar preserves historical labels, fills missing days, and honors Sunday week start', () => {
  const days = [{ date: '2024-02-28', seconds: 10 }, { date: '2024-03-02', seconds: 20 }];
  const weeks = contributionWeeks(days, 7);
  assert.equal(weeks[0].length, 7);
  assert.deepEqual(weeks[0].slice(0, 3), [null, null, null]);
  assert.deepEqual(weeks[0][4], { date: '2024-02-29', seconds: 0 });
  assert.deepEqual(calendarGroups(days, 'month', 7), [{ key: '2024-02', seconds: 10 }, { key: '2024-03', seconds: 20 }]);
  assert.deepEqual(calendarGroups(days, 'week', 7), [{ key: '2024-02-25', seconds: 30 }]);
});

test('statistics adapter rejects malformed dates and owns the mapped arrays', () => {
  const dto = { range: 'week', anchor: '2026-09-29', startDate: '2026-09-28', endDate: '2026-10-04', weekStartsOn: 1, bucketUnit: 'day', totalSeconds: 60,
    availablePaths: [{ id: 'p', name: 'guitar', archived: false }], distribution: [{ pathId: 'p', name: 'guitar', seconds: 60 }], buckets: [{ key: '2026-09-29', seconds: 60 }], calendar: [{ date: '2026-09-29', seconds: 60 }] };
  const value = statisticsFromAPI(dto);
  dto.calendar[0].seconds = 99;
  assert.equal(value.days[0].seconds, 60);
  assert.equal('calendar' in value, false);
  assert.throws(() => statisticsFromAPI({ ...dto, startDate: '2026-02-30' }));
  assert.throws(() => statisticsFromAPI({ ...dto, totalSeconds: -1 }));
});

test('temporary Stats failure retains cached-view eligibility while access denial does not', async context => {
  const { createServer } = await import('node:http');
  const { apiStatisticsRepository } = await import('./studio/analytics/adapters/api-statistics-repository');
  const { StatisticsUnavailable } = await import('./studio/analytics/domain/statistics');
  let status = 503;
  let rejected = 0;
  const server = createServer((_request, response) => { response.writeHead(status, { 'Content-Type': 'application/problem+json' }); response.end('{}'); });
  await new Promise<void>((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  context.after(() => { server.closeAllConnections(); server.close(); });
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const repository = apiStatisticsRepository(`http://127.0.0.1:${address.port}`, () => 'test-session', () => { rejected++; });
  const selection = { range: 'week' as const, pathIds: [] };
  await assert.rejects(repository.load(selection), error => error instanceof StatisticsUnavailable && error.retryable);
  status = 403;
  await assert.rejects(repository.load(selection), error => error instanceof StatisticsUnavailable && !error.retryable);
  assert.equal(rejected, 0);
  status = 401;
  await assert.rejects(repository.load(selection), error => error instanceof StatisticsUnavailable && !error.retryable);
  assert.equal(rejected, 1);
});
