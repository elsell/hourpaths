import assert from 'node:assert/strict';
import test from 'node:test';
import { liveTimerProgress } from './live-timer-progress';

test('live totals clip running time to the authoritative period and replace it after Stop', () => {
  const state = { running: true, accumulatedSeconds: 600, timer: { startedAt: '2026-09-28T23:50:00Z' }, intervalProgress: { accumulatedSeconds: 30, targetSeconds: 3600, startedAt: '2026-09-29T00:00:00Z', endedAt: '2026-09-30T00:00:00Z' } };
  const live = liveTimerProgress(state, Date.parse('2026-09-29T00:20:00Z'));
  assert.equal(live.accumulatedSeconds, 2400);
  assert.equal(live.intervalProgress?.accumulatedSeconds, 1230);
  assert.equal(state.accumulatedSeconds, 600);
  assert.equal(liveTimerProgress({ ...state, running: false, accumulatedSeconds: 2400, intervalProgress: { ...state.intervalProgress, accumulatedSeconds: 1230 } }, Date.parse('2026-09-29T00:21:00Z')).intervalProgress?.accumulatedSeconds, 1230);
  assert.equal(liveTimerProgress(state, Date.parse('2026-09-30T00:00:00Z')).intervalProgress, undefined);
});
