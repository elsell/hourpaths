import assert from 'node:assert/strict';
import test from 'node:test';
import { projectTrackingState, type TrackingSnapshot } from './offline-tracking';
import { nativeGoalPeriodSchedule } from './native-goal-period';

test('native refresh reprojects a new hour from retained work without carrying over the old total or changing timer identity', () => {
  const at = (time: string) => Date.parse(`2026-10-14T${time}:00Z`);
  const state: TrackingSnapshot = {
    owner: 'alice', revision: 7,
    paths: [{ id: 'guitar', name: 'guitar', timeZone: 'UTC', goal: { targetSeconds: 600, recurrence: 'hourly', alignment: { minute: 0 } } }],
    summaries: { guitar: { savedTotalSeconds: 1020, period: { savedSeconds: 1020, targetSeconds: 600, startsAt: at('10:00'), endsAt: at('11:00') } } },
    history: [], historyRetainedAt: new Date(at('10:50')).toISOString(), unreflected: [],
    timers: [{ id: 'new-session', pathId: 'guitar', startedAt: new Date(at('11:03')).toISOString(), timeZone: 'UTC' }],
    operations: [{ id: 'old-session', timerId: 'old-session', operationId: 'save-old', kind: 'stop', pathId: 'guitar',
      startedAt: new Date(at('10:58')).toISOString(), endedAt: new Date(at('11:02')).toISOString(), timeZone: 'UTC' }],
    corrections: [], notices: [],
  };
  const before = JSON.stringify(state);
  const view = projectTrackingState(state, 'guitar', at('11:05'));
  assert.deepEqual(view?.period, { startsAt: at('11:00'), endsAt: at('12:00'), targetSeconds: 600, savedSeconds: 120 });
  assert.equal(view?.activeSession?.id, 'new-session');
  assert.equal(view?.pending, true);
  assert.equal(JSON.stringify(state), before);
});


test('period schedule clips saved work at each boundary and never carries it into later hours', () => {
  const at = (time: string) => Date.parse(`2026-10-14T${time}:00Z`);
  const state: TrackingSnapshot = {
    owner: 'alice', revision: 8,
    paths: [{ id: 'guitar', name: 'guitar', timeZone: 'UTC', goal: { targetSeconds: 600, recurrence: 'hourly', alignment: { minute: 0 } } }],
    summaries: { guitar: { savedTotalSeconds: 600, period: { savedSeconds: 600, targetSeconds: 600, startsAt: at('10:00'), endsAt: at('11:00') } } },
    history: [], historyRetainedAt: null, unreflected: [], timers: [], corrections: [], notices: [],
    operations: [{ id: 'old', timerId: 'old', operationId: 'stop', kind: 'stop', pathId: 'guitar',
      startedAt: new Date(at('10:58')).toISOString(), endedAt: new Date(at('11:02')).toISOString(), timeZone: 'UTC' }],
  };
  const periods = nativeGoalPeriodSchedule(state, 'alice', 'guitar', at('11:05'), at('13:00'));
  assert.deepEqual(periods, [
    { startsAt: at('11:00'), endsAt: at('12:00'), targetSeconds: 600, savedSeconds: 120 },
    { startsAt: at('12:00'), endsAt: at('13:00'), targetSeconds: 600, savedSeconds: 0 },
  ]);
  assert.deepEqual(nativeGoalPeriodSchedule(state, 'alice', 'guitar', at('11:05'), at('11:05')), []);
  assert.deepEqual(nativeGoalPeriodSchedule({ ...state, unavailablePaths: ['guitar'] }, 'alice', 'guitar', at('11:05'), at('13:00')), []);
  assert.throws(() => nativeGoalPeriodSchedule(state, 'bob', 'guitar', at('11:05'), at('13:00')), /owner/);
});
