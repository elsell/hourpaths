import assert from 'node:assert/strict';
import test from 'node:test';
import { pathMemberProgressFromAPI } from './path-member-progress';

test('Path People progress accepts absent goals and exact nonnegative projections', () => {
  assert.equal(pathMemberProgressFromAPI(undefined), undefined);
  assert.deepEqual(pathMemberProgressFromAPI({ accumulatedSeconds: 90, targetSeconds: 120 }), {
    accumulatedSeconds: 90,
    targetSeconds: 120,
  });
});

test('Path People progress rejects malformed, unsafe, or widened projections', () => {
  for (const value of [
    null,
    { accumulatedSeconds: -1, targetSeconds: 120 },
    { accumulatedSeconds: 0, targetSeconds: 0 },
    { accumulatedSeconds: 0.5, targetSeconds: 120 },
    { accumulatedSeconds: 0, targetSeconds: 120, unrelatedProfileData: true },
  ]) assert.throws(() => pathMemberProgressFromAPI(value), /invalid Path member progress/);
});
