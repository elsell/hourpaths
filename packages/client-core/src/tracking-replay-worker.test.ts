import assert from 'node:assert/strict';
import test from 'node:test';
import { TrackingReplayWorker } from './tracking-replay-worker';

test('automatic retry backs off without losing wakeups and stops scheduling after disposal', async () => {
  const scheduled: { delay: number; work: () => void; cancelled: boolean }[] = [];
  const schedule = (work: () => void, delay: number) => {
    const task = { delay, work, cancelled: false }; scheduled.push(task);
    return () => { task.cancelled = true; };
  };
  let attempts = 0, refreshed = 0;
  const worker = new TrackingReplayWorker(async () => {
    attempts++;
    if (attempts < 3) throw new Error('temporarily offline');
  }, schedule, () => { refreshed++; });
  await worker.wake();
  assert.equal(scheduled[0].delay, 1000);
  scheduled[0].work();
  await worker.idle();
  assert.equal(scheduled[1].delay, 2000);
  await worker.wake(); // Connectivity restoration does not wait for the backoff.
  assert.equal(scheduled[1].cancelled, true);
  assert.equal(attempts, 3);
  assert.equal(refreshed, 1);
  worker.dispose();
  await worker.wake();
  assert.equal(attempts, 3);
});

test('a session pause prevents replay until sign-in and account disposal suppresses late UI refresh', async () => {
  let release: (() => void) | undefined;
  let attempts = 0, refreshed = 0;
  const worker = new TrackingReplayWorker(async () => {
    attempts++;
    await new Promise<void>(resolve => { release = resolve; });
  }, () => () => {}, () => { refreshed++; });
  worker.setPaused(true);
  await worker.wake();
  assert.equal(attempts, 0);
  worker.setPaused(false);
  const work = worker.wake();
  assert.equal(attempts, 1);
  worker.dispose();
  release!();
  await work;
  assert.equal(refreshed, 0);
});
