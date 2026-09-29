import assert from 'node:assert/strict';
import { test } from 'node:test';
import { currentProgress } from './studio/paths/domain/progress';

test('an active session adds only its overlap with the current goal period', () => {
  const snapshot = {
    savedTotalSeconds: 3600,
    activeSession: { id: 'timer', startedAt: 0 },
    period: { savedSeconds: 600, targetSeconds: 1800, startsAt: 600_000, endsAt: 3_600_000 },
  };
  const result = currentProgress(snapshot, 900_000);
  assert.equal(result.totalSeconds, 4500);
  assert.equal(result.periodSeconds, 900);
  assert.equal(result.sessionSeconds, 900);
  assert.equal(snapshot.savedTotalSeconds, 3600);
});
test('a crossed period boundary requests refresh rather than displaying last period as current', () => {
  const result = currentProgress({
    savedTotalSeconds: 0,
    activeSession: { id: 'timer', startedAt: 0 },
    period: { savedSeconds: 100, targetSeconds: 1800, startsAt: 0, endsAt: 60_000 },
  }, 61_000);
  assert.equal(result.periodSeconds, null);
  assert.equal(result.needsPeriodRefresh, true);
  assert.equal(result.totalSeconds, 61);
});
