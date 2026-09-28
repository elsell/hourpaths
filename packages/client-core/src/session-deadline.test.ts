import assert from 'node:assert/strict';
import test from 'node:test';
import { scheduleSessionDeadline } from './session-deadline.js';

test('30-day session scheduling never overflows or expires early and remains cancellable', () => {
  let now = 0;
  let pending: (() => void) | undefined;
  let delay = 0;
  let fired = 0;
  const clock = {
    now: () => now,
    schedule: (callback: () => void, milliseconds: number) => {
      assert.ok(milliseconds >= 0 && milliseconds <= 2_147_483_647);
      pending = callback;
      delay = milliseconds;
      return () => { pending = undefined; };
    },
  };
  const deadline = 30 * 24 * 60 * 60 * 1000;
  const cancel = scheduleSessionDeadline(() => fired++, deadline, clock);
  assert.ok(delay < deadline);
  now += delay;
  pending!();
  assert.equal(fired, 0);
  now = deadline;
  pending!();
  assert.equal(fired, 1);
  cancel();
  assert.equal(pending, undefined);
  const cancelAgain = scheduleSessionDeadline(() => fired++, deadline + 1000, clock);
  cancelAgain();
  assert.equal(pending, undefined);
  assert.equal(fired, 1);
});
