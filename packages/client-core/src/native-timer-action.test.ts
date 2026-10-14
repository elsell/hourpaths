import assert from 'node:assert/strict';
import test from 'node:test';
import type { TrackingSnapshot } from './offline-tracking';
import { applyNativeTimerAction } from './native-timer-action';
const at = Date.parse('2026-10-14T12:00:00Z');
function snapshot(): TrackingSnapshot {
  return { owner: 'alice', revision: 4, paths: [{ id: 'guitar', name: 'Guitar', timeZone: 'UTC' }],
    history: [], historyRetainedAt: null, summaries: {}, unreflected: [], timers: [], operations: [], corrections: [], notices: [] };
}
const start = { owner: 'alice', revision: 4, pathId: 'guitar', kind: 'start' as const, timerId: 'session', operationId: 'start-command' };
test('native start and stop preserve offline operation identity and elapsed time without mutating inputs', () => {
  const original = snapshot();
  const started = applyNativeTimerAction(original, start, at);
  assert.equal(original.timers.length, 0);
  assert.equal(started.revision, 5);
  const stopped = applyNativeTimerAction(started, { ...start, kind: 'stop', revision: 5, operationId: 'stop-command' }, at + 60_000);
  assert.equal(stopped.timers.length, 0);
  assert.deepEqual(stopped.operations.map(value => [value.kind, value.timerId, value.startedAt, value.endedAt]), [
    ['start', 'session', new Date(at).toISOString(), undefined],
    ['stop', 'session', new Date(at).toISOString(), new Date(at + 60_000).toISOString()],
  ]);
});
test('old-account and repeated/stale native actions cannot mutate current tracking', () => {
  assert.throws(() => applyNativeTimerAction(snapshot(), { ...start, owner: 'bob' }, at), /owner/);
  const started = applyNativeTimerAction(snapshot(), start, at);
  assert.throws(() => applyNativeTimerAction(started, start, at), /stale/);
  assert.throws(() => applyNativeTimerAction(started, { ...start, revision: 5, kind: 'stop', pathId: 'other' }, at), /unavailable/);
  assert.throws(() => applyNativeTimerAction(started, { ...start, revision: 5, kind: 'stop', timerId: 'older-session' }, at), /unavailable/);
  assert.equal(started.timers.length, 1);
});
test('native stop with a reversed device clock retains correction instead of losing the session', () => {
  const started = applyNativeTimerAction(snapshot(), start, at);
  const stopped = applyNativeTimerAction(started, { ...start, revision: 5, kind: 'stop', operationId: 'stop' }, at - 1000);
  assert.equal(stopped.corrections[0]?.timer.id, 'session');
  assert.equal(stopped.operations.length, 1);
});

test('committed Stop offers Start with the new revision, then restores Stop after restart', async () => {
  const { refreshNativeTimerPresentation } = await import('./native-timer-action');
  const active = applyNativeTimerAction(snapshot(), start, at);
  const stopped = applyNativeTimerAction(active, { ...start, revision: 5, kind: 'stop', operationId: 'stop' }, at + 60000);
  const presentation = { owner: 'alice', accountLease: 'lease', startLabel: 'Start', stopLabel: 'Stop', titles: ['Ready', 'Running'], timers: [{ pathId: 'guitar', id: 'session', startedAt: at }] };
  const ready = refreshNativeTimerPresentation(presentation, stopped, at + 60000, ['next-session', 'next-start']);
  const action = JSON.parse(ready.timers[0].action.target).action;
  assert.equal(action.kind, 'start'); assert.equal(action.revision, stopped.revision);
  assert.equal(ready.timers[0].stoppedAt, at + 60000);
  assert.equal(ready.title, 'Ready');
  const restarted = applyNativeTimerAction(stopped, action, at + 120000);
  const running = refreshNativeTimerPresentation(ready, restarted, at + 120000, ['unused', 'next-stop']);
  assert.equal(JSON.parse(running.timers[0].action.target).action.kind, 'stop');
  assert.equal(running.timers[0].stoppedAt, undefined);
  assert.equal(running.timers[0].startedAt, at + 120000);
});
