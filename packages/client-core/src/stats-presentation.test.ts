import assert from 'node:assert/strict';
import test from 'node:test';
import { statsCalendarGroups } from './stats-presentation';

test('calendar regrouping conserves recorded seconds across year and preferred week boundaries', () => {
  const days = [{ date: '2025-12-31', seconds: 1800 }, { date: '2026-01-01', seconds: 3600 }, { date: '2026-01-04', seconds: 900 }];
  assert.deepEqual(statsCalendarGroups(days, 'week', 7), [{ key: '2025-12-28', seconds: 5400 }, { key: '2026-01-04', seconds: 900 }]);
  for (const unit of ['day', 'week', 'month', 'year'] as const) assert.equal(statsCalendarGroups(days, unit).reduce((sum, bucket) => sum + bucket.seconds, 0), 6300);
});
