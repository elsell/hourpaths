import assert from 'node:assert/strict';
import { test } from 'node:test';
import { adjacentLiveActivity, projectLiveActivity } from './live-activity-pages';

test('manual navigation follows every path before the next person and reverses across people', () => {
  const pages = [{ timerId: 'maya-guitar' }, { timerId: 'maya-reading' }, { timerId: 'alex-walking' }];
  assert.equal(adjacentLiveActivity(pages, 'maya-guitar', 1), 'maya-reading');
  assert.equal(adjacentLiveActivity(pages, 'maya-reading', 1), 'alex-walking');
  assert.equal(adjacentLiveActivity(pages, 'alex-walking', -1), 'maya-reading');
  assert.equal(adjacentLiveActivity(pages, 'maya-guitar', -1), 'maya-guitar');
  assert.equal(adjacentLiveActivity(pages, 'alex-walking', 1), null);
  assert.equal(adjacentLiveActivity(pages, 'removed', 1), null);
});

test('goal projection counts only the live interval overlap and freezes at rollover pending refresh', () => {
  const page = { startedAt: '2026-09-27T09:50:00Z', goal: { recordedSeconds: 120, targetSeconds: 3600, intervalStart: '2026-09-27T10:00:00Z', intervalEnd: '2026-09-27T11:00:00Z', label: 'hour' } };
  assert.deepEqual(projectLiveActivity(page, Date.parse('2026-09-27T10:10:00Z')), { sessionSeconds: 1200, goalSeconds: 720, goalExpired: false });
  assert.deepEqual(projectLiveActivity(page, Date.parse('2026-09-27T11:10:00Z')), { sessionSeconds: 4800, goalSeconds: 3720, goalExpired: true });
});
