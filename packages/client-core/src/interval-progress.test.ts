import assert from 'node:assert/strict';
import test from 'node:test';
import { intervalProgress } from './index.js';

test('current interval progress is omitted when the API projection is absent', () => {
  assert.equal(intervalProgress(), undefined);
});

test('current interval progress preserves zero, partial, and uncapped completed seconds', () => {
  assert.deepEqual(intervalProgress({ accumulatedSeconds: 0, targetSeconds: 120 }), {
    accumulatedSeconds: 0,
    targetSeconds: 120,
    visualSeconds: 0,
    completed: false,
  });
  assert.deepEqual(intervalProgress({ accumulatedSeconds: 60, targetSeconds: 120 }), {
    accumulatedSeconds: 60,
    targetSeconds: 120,
    visualSeconds: 60,
    completed: false,
  });
  assert.deepEqual(intervalProgress({ accumulatedSeconds: 150, targetSeconds: 120 }), {
    accumulatedSeconds: 150,
    targetSeconds: 120,
    visualSeconds: 120,
    completed: true,
  });
});
