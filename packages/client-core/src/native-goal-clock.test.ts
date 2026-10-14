import assert from 'node:assert/strict';
import test from 'node:test';
import { nativeGoalClock } from './native-goal-clock';

test('goal clock counts saved time plus only the running overlap and stops at the period boundary', () => {
  const period = { accumulatedSeconds: 600, targetSeconds: 3600, startedAt: '2026-10-14T12:00:00Z', endedAt: '2026-10-14T13:00:00Z' };
  const result = nativeGoalClock(period, Date.parse('2026-10-14T11:45:00Z'), Date.parse('2026-10-14T12:15:00Z'))!;
  assert.equal(result.lower, Date.parse('2026-10-14T11:50:00Z'));
  assert.equal(result.upper, Date.parse(period.endedAt));
  assert.equal(result.fraction, 1500 / 3600);
  assert.equal(nativeGoalClock(period, Date.parse('2026-10-14T11:45:00Z'), Date.parse(period.endedAt)), undefined);
});

test('missing, invalid or future bounds never create an invented native timer', () => {
  const now = Date.parse('2026-10-14T12:15:00Z');
  assert.equal(nativeGoalClock({ accumulatedSeconds: 0, targetSeconds: 10 }, now, now), undefined);
  assert.equal(nativeGoalClock({ accumulatedSeconds: 0, targetSeconds: 10, startedAt: 'bad', endedAt: 'bad' }, now, now), undefined);
  assert.equal(nativeGoalClock({ accumulatedSeconds: 0, targetSeconds: 10, startedAt: '2026-10-15T12:00:00Z', endedAt: '2026-10-15T13:00:00Z' }, now, now), undefined);
});
