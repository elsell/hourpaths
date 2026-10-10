import assert from 'node:assert/strict';
import test from 'node:test';
import { statsCalendarGroups, statsContributionWeeks } from './stats-presentation';

test('calendar regrouping conserves recorded seconds across year and preferred week boundaries', () => {
  const days = [{ date: '2025-12-31', seconds: 1800 }, { date: '2026-01-01', seconds: 3600 }, { date: '2026-01-04', seconds: 900 }];
  assert.deepEqual(statsCalendarGroups(days, 'week', 7), [{ key: '2025-12-28', seconds: 5400 }, { key: '2026-01-04', seconds: 900 }]);
  for (const unit of ['day', 'week', 'month', 'year'] as const) assert.equal(statsCalendarGroups(days, unit).reduce((sum, bucket) => sum + bucket.seconds, 0), 6300);
});


test('contribution grid preserves historical dates, inserts zero days, and aligns the preferred week start', () => {
  const weeks = statsContributionWeeks([{ date: '2026-03-07', seconds: 60 }, { date: '2026-03-09', seconds: 120 }], 7);
  assert.equal(weeks[0]![6]!.date, '2026-03-07');
  assert.deepEqual(weeks[1]![0], { date: '2026-03-08', seconds: 0 });
  assert.deepEqual(weeks[1]![1], { date: '2026-03-09', seconds: 120 });
  assert.equal(weeks.flat().reduce((sum, day) => sum + (day?.seconds ?? 0), 0), 180);
});


test('calendar retains both range edges and an entirely empty range', () => {
  const range = { startDate: '2026-02-27', endDate: '2026-03-03' };
  const expected = ['2026-02-27', '2026-02-28', '2026-03-01', '2026-03-02', '2026-03-03'];
  for (const days of [[], [{ date: '2026-03-01', seconds: 60 }]]) {
    const cells = statsContributionWeeks(days, 1, range).flat().filter(day => day !== null);
    assert.deepEqual(cells.map(day => day.date), expected);
    assert.equal(cells[0].seconds, 0);
    assert.equal(cells[4].seconds, 0);
    assert.equal(cells.reduce((sum, day) => sum + day.seconds, 0), days.length ? 60 : 0);
  }
});
