import assert from 'node:assert/strict';
import test from 'node:test';
import { commitLatestStateUpdate } from './use-latest-state';

test('unrelated rows published while deletion is pending enter the synchronous projection snapshot before render', () => {
  const activities = { current: ['activity-target'] };
  const members = { current: ['member-target'] };
  const notifications = { current: ['notification-target'] };

  commitLatestStateUpdate(activities, (current) => [...current, 'activity-unrelated']);
  commitLatestStateUpdate(members, (current) => [...current, 'member-unrelated']);
  commitLatestStateUpdate(notifications, (current) => [...current, 'notification-unrelated']);

  assert.deepEqual(activities.current, ['activity-target', 'activity-unrelated']);
  assert.deepEqual(members.current, ['member-target', 'member-unrelated']);
  assert.deepEqual(notifications.current, ['notification-target', 'notification-unrelated']);
});
