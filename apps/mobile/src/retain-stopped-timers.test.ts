import assert from 'node:assert/strict';
import test from 'node:test';
import { retainStoppedTimers } from './timers/retain-stopped-timers';

test('background reconciliation retains restart controls but removes inaccessible and restarted rows', () => {
  const stopped = { id: 'old', pathId: 'guitar', name: 'guitar', startedAt: 1000, stoppedAt: 2000 };
  assert.deepEqual(retainStoppedTimers([], [stopped], ['guitar']), [stopped]);
  assert.deepEqual(retainStoppedTimers([], [stopped], []), []);
  const restarted = { id: 'new', pathId: 'guitar', name: 'guitar', startedAt: 3000 };
  assert.deepEqual(retainStoppedTimers([restarted], [stopped], ['guitar']), [restarted]);
  assert.deepEqual(retainStoppedTimers([], [restarted], ['guitar']), []);
});
