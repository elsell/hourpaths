import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createSignOutTimerResolutionCoordinator,
  type RunningTimerSnapshot,
} from './index.js';

const timers: readonly RunningTimerSnapshot[] = [
  {
    pathId: 'path-reading',
    startedAt: '2026-07-28T12:00:00.000Z',
    accumulatedSeconds: 45,
  },
  {
    pathId: 'path-piano',
    startedAt: '2026-07-28T12:05:00.000Z',
    accumulatedSeconds: 12,
  },
];

test('captures an immutable authoritative snapshot without retaining caller-owned data', () => {
  const source = timers.map((timer) => ({ ...timer }));
  const coordinator = createSignOutTimerResolutionCoordinator();
  const resolution = coordinator.begin(source);

  source[0]!.pathId = 'changed';

  assert.deepEqual(resolution.state(), {
    kind: 'awaiting_choice',
    timers,
    authorizeSignOut: false,
  });
  assert.equal(Object.isFrozen(resolution.state()), true);
  assert.equal(Object.isFrozen(resolution.state().timers), true);
  assert.equal(Object.isFrozen(resolution.state().timers[0]), true);
});

test('stop-and-save authorizes sign-out only after every timer succeeds', async () => {
  const coordinator = createSignOutTimerResolutionCoordinator();
  const resolution = coordinator.begin(timers);
  const requested: string[] = [];

  const result = await resolution.stopAndSave(async (timer) => {
    requested.push(timer.pathId);
    return { pathId: timer.pathId, running: false };
  });

  assert.deepEqual(requested, ['path-reading', 'path-piano']);
  assert.deepEqual(result, {
    kind: 'ready_to_sign_out',
    resolution: 'stopped_and_saved',
    timers,
    authorizeSignOut: true,
  });
  assert.equal(result.authorizeSignOut, true);
  assert.deepEqual(resolution.state(), result);
});

test('partial stop-and-save failure retains only unresolved timers for retry', async () => {
  const coordinator = createSignOutTimerResolutionCoordinator();
  const resolution = coordinator.begin(timers);
  const failure = new Error('offline');
  const firstRequests: string[] = [];

  const first = await resolution.stopAndSave(async (timer) => {
    firstRequests.push(timer.pathId);
    if (timer.pathId === 'path-piano') throw failure;
    return { pathId: timer.pathId, running: false };
  });

  assert.deepEqual(firstRequests, ['path-reading', 'path-piano']);
  assert.deepEqual(first, {
    kind: 'stop_failed',
    timers,
    unresolvedTimers: [timers[1]],
    failures: [{ timer: timers[1], cause: failure }],
    authorizeSignOut: false,
  });
  assert.equal(first.authorizeSignOut, false);

  const retryRequests: string[] = [];
  const retry = await resolution.stopAndSave(async (timer) => {
    retryRequests.push(timer.pathId);
    return { pathId: timer.pathId, running: false };
  });

  assert.deepEqual(retryRequests, ['path-piano']);
  assert.equal(retry.kind, 'ready_to_sign_out');
  assert.equal(retry.authorizeSignOut, true);
});

test('malformed or mismatched stop acknowledgements fail closed and remain retryable', async () => {
  const coordinator = createSignOutTimerResolutionCoordinator();
  const resolution = coordinator.begin([timers[0]!]);

  const result = await resolution.stopAndSave(async () => ({
    pathId: 'another-path',
    running: false,
  }));

  assert.equal(result.kind, 'stop_failed');
  assert.equal(result.authorizeSignOut, false);
  assert.deepEqual(result.unresolvedTimers, [timers[0]]);
  assert.match(String(result.failures[0]?.cause), /invalid timer stop acknowledgement/);
});

test('keep-running explicitly authorizes sign-out without stopping timers', () => {
  const coordinator = createSignOutTimerResolutionCoordinator();
  const resolution = coordinator.begin(timers);

  assert.deepEqual(resolution.keepRunning(), {
    kind: 'ready_to_sign_out',
    resolution: 'kept_running',
    timers,
    authorizeSignOut: true,
  });
});

test('cancel keeps every timer running and does not authorize sign-out', () => {
  const coordinator = createSignOutTimerResolutionCoordinator();
  const resolution = coordinator.begin(timers);

  assert.deepEqual(resolution.cancel(), {
    kind: 'cancelled',
    timers,
    authorizeSignOut: false,
  });
});

test('new ownership supersedes pending work so stale completion cannot authorize sign-out', async () => {
  const coordinator = createSignOutTimerResolutionCoordinator();
  const staleResolution = coordinator.begin(timers);
  let complete!: () => void;
  const gate = new Promise<void>((resolve) => { complete = resolve; });
  const pending = staleResolution.stopAndSave(async (timer) => {
    await gate;
    return { pathId: timer.pathId, running: false };
  });

  const currentResolution = coordinator.begin([timers[0]!]);
  assert.equal(currentResolution.keepRunning().authorizeSignOut, true);

  complete();
  const stale = await pending;
  assert.deepEqual(stale, {
    kind: 'superseded',
    timers,
    authorizeSignOut: false,
  });
  assert.equal(staleResolution.keepRunning().authorizeSignOut, false);
});

test('explicit coordinator invalidation supersedes a resolution', () => {
  const coordinator = createSignOutTimerResolutionCoordinator();
  const resolution = coordinator.begin(timers);

  coordinator.invalidate();

  assert.equal(resolution.keepRunning().kind, 'superseded');
  assert.equal(resolution.cancel().authorizeSignOut, false);
});

test('invalid authoritative timer snapshots are rejected', () => {
  const coordinator = createSignOutTimerResolutionCoordinator();

  assert.throws(() => coordinator.begin([]), /running timer snapshot/);
  assert.throws(() => coordinator.begin([timers[0]!, timers[0]!]), /running timer snapshot/);
  assert.throws(() => coordinator.begin([{ ...timers[0]!, startedAt: 'invalid' }]), /running timer snapshot/);
  assert.throws(() => coordinator.begin([{ ...timers[0]!, accumulatedSeconds: -1 }]), /running timer snapshot/);
});
