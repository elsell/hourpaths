import assert from 'node:assert/strict';
import test from 'node:test';
import { createTimerOperationOwner, elapsedTimerSeconds, formatTimerDuration, timerMutationPresentation, type TimerState } from './timer-control.js';

const running = (startedAt = '2026-07-21T12:00:00Z'): TimerState => ({
  running: true,
  accumulatedSeconds: 90,
  timer: { id: 'timer-1', pathId: 'path-1', startedAt, occurrenceTimeZone: 'America/New_York' },
});

test('elapsed duration adds server history to the live running interval', () => {
  assert.equal(elapsedTimerSeconds(running(), Date.parse('2026-07-21T12:01:01.900Z')), 151);
  assert.equal(elapsedTimerSeconds({ running: false, accumulatedSeconds: 90 }, Date.now()), 90);
  assert.equal(formatTimerDuration(3661), '1:01:01');
  assert.equal(formatTimerDuration(-10), '0:00');
});

test('a failed start retries with the same key and a successful stop gets a new key', async () => {
  const keys = ['start-key-0000001', 'stop-key-00000001'];
  const owner = createTimerOperationOwner(() => keys.shift()!);
  const starts: string[] = [];
  const failure = new Error('offline');
  const start = async (key: string) => {
    starts.push(key);
    if (starts.length === 1) throw failure;
    return running();
  };

  assert.deepEqual(await owner.start('path-1', start), { kind: 'failed', cause: failure });
  assert.equal((await owner.start('path-1', start)).kind, 'applied');
  const stop = await owner.stop('path-1', 'timer-1', async (key) => {
    assert.equal(key, 'stop-key-00000001');
    return { running: false, accumulatedSeconds: 151, saved: true, subsecond: false };
  });
  assert.deepEqual(stop, { kind: 'applied', state: { running: false, accumulatedSeconds: 151, saved: true, subsecond: false } });
  assert.deepEqual(starts, ['start-key-0000001', 'start-key-0000001']);
});

test('shared operation owner preserves a subsecond unsaved stop result', async () => {
  const owner = createTimerOperationOwner(() => 'stop-key-00000001');
  const subsecond = { running: false, accumulatedSeconds: 47, saved: false, subsecond: true } as const;

  const result = await owner.stop('path-1', 'timer-1', async () => subsecond);

  assert.deepEqual(result, { kind: 'applied', state: subsecond });
  if (result.kind === 'applied') {
    assert.strictEqual(result.state, subsecond);
    assert.deepEqual(timerMutationPresentation(result.state), { state: subsecond, notice: 'subsecond', controlMessage: 'timer.start' });
  }
  assert.deepEqual(timerMutationPresentation(running()), { state: running(), notice: null, controlMessage: 'timer.stop' });
  assert.deepEqual(timerMutationPresentation({ ...subsecond, saved: true, subsecond: false }), {
    state: { ...subsecond, saved: true, subsecond: false }, notice: null, controlMessage: 'timer.start',
  });
});

test('an existing timer surfaced by a distinct start presents the Stop control without resetting its state', () => {
  const existing = running('2026-07-21T11:00:00Z');
  const presentation = timerMutationPresentation(existing);

  assert.strictEqual(presentation.state, existing);
  assert.equal(presentation.controlMessage, 'timer.stop');
});

test('newer per-Path intent supersedes stale completion without affecting another Path', async () => {
  const keys = ['start-key-0000001', 'stop-key-00000001', 'other-key-0000001'];
  const owner = createTimerOperationOwner(() => keys.shift()!);
  let resolveStart!: (state: TimerState) => void;
  const pendingStart = new Promise<TimerState>((resolve) => { resolveStart = resolve; });
  const first = owner.start('path-1', () => pendingStart);
  const second = owner.stop('path-1', 'timer-old', async () => ({ running: false, accumulatedSeconds: 10, saved: true, subsecond: false }));
  const other = owner.start('path-2', async () => ({ ...running(), timer: { ...running().timer!, id: 'timer-2', pathId: 'path-2' } }));

  assert.equal((await second).kind, 'applied');
  assert.equal((await other).kind, 'applied');
  resolveStart(running());
  assert.deepEqual(await first, { kind: 'superseded' });
});

test('cancel supersedes pending operations and generated keys are bounded printable ASCII', async () => {
  const owner = createTimerOperationOwner(() => 'bad\nkey');
  await assert.rejects(owner.start('path-1', async () => running()), /idempotency/i);

  const valid = createTimerOperationOwner(() => 'start-key-0000001');
  let resolve!: (state: TimerState) => void;
  const pending = new Promise<TimerState>((accept) => { resolve = accept; });
  const result = valid.start('path-1', () => pending);
  valid.cancel();
  resolve(running());
  assert.deepEqual(await result, { kind: 'superseded' });
});
