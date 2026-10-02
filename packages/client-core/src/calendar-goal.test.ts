import assert from 'node:assert/strict';
import test from 'node:test';
import { currentGoalPeriod, type CalendarGoal } from './calendar-goal';

// These are the product examples also exercised at the API's calendar boundary.
test('offline calendar periods use stored alignments, calendar clamps and the specified offset-transition rules', () => {
  const cases: [CalendarGoal['recurrence'], CalendarGoal['alignment'], string, string, string, string][] = [
    ['hourly', { minute: 15 }, 'UTC', '2026-07-22T14:15:00Z', '2026-07-22T14:15:00Z', '2026-07-22T15:15:00Z'],
    ['daily', { hour: 5 }, 'UTC', '2026-07-22T04:00:00Z', '2026-07-21T05:00:00Z', '2026-07-22T05:00:00Z'],
    ['weekly', { isoWeekday: 1 }, 'Asia/Tokyo', '2026-07-22T12:00:00Z', '2026-07-19T15:00:00Z', '2026-07-26T15:00:00Z'],
    ['monthly', { day: 31 }, 'UTC', '2026-03-30T12:00:00Z', '2026-02-28T00:00:00Z', '2026-03-31T00:00:00Z'],
    ['yearly', { month: 2, day: 29 }, 'UTC', '2026-03-01T12:00:00Z', '2026-02-28T00:00:00Z', '2027-02-28T00:00:00Z'],
    ['daily', { hour: 2 }, 'America/New_York', '2026-03-08T08:00:00Z', '2026-03-08T07:00:00Z', '2026-03-09T06:00:00Z'],
    ['hourly', { minute: 30 }, 'America/New_York', '2026-11-01T06:45:00Z', '2026-11-01T05:30:00Z', '2026-11-01T07:30:00Z'],
    ['hourly', { minute: 30 }, 'America/New_York', '2026-03-08T07:45:00Z', '2026-03-08T07:30:00Z', '2026-03-08T08:30:00Z'],
  ];
  for (const [recurrence, alignment, zone, now, start, end] of cases) {
    assert.deepEqual(currentGoalPeriod({ recurrence, alignment, targetSeconds: 60 }, zone, Date.parse(now)),
      { startsAt: Date.parse(start), endsAt: Date.parse(end), targetSeconds: 60 });
  }
  assert.throws(() => currentGoalPeriod({ recurrence: 'yearly', alignment: { month: 2, day: 30 }, targetSeconds: 60 }, 'UTC', Date.now()), /goal_invalid/);
});
