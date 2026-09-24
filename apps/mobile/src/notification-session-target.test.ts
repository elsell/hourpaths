import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createNotificationSessionTarget,
  currentNotificationSessionCredential,
  ownsNotificationSessionTarget,
  rotateNotificationSessionTarget,
} from './notification-session-target';

test('notification targets retain arbitrary same-owner credential rotations', () => {
  const sessionA = { token: 'a' };
  const sessionB = { token: 'b' };
  const sessionC = { token: 'c' };
  const initial = createNotificationSessionTarget('owner-a', sessionA, 'notifications:history');
  const rotatedB = rotateNotificationSessionTarget(initial, 'owner-a', sessionB);
  const rotatedC = rotateNotificationSessionTarget(rotatedB, 'owner-a', sessionC);

  assert.deepEqual(rotatedC.sessionTokens, ['a', 'b', 'c']);
  assert.equal(ownsNotificationSessionTarget(rotatedC, 'owner-a', 'a', 'notifications:history'), true);
  assert.equal(ownsNotificationSessionTarget(rotatedC, 'owner-a', 'c', 'notifications:history'), true);
  assert.equal(currentNotificationSessionCredential(rotatedC, 'owner-a', 'notifications:history', sessionC), sessionC);
});

test('notification targets reject replacement owners and mismatched intents', () => {
  const target = createNotificationSessionTarget('owner-a', { token: 'a' }, 'notifications:history');
  assert.strictEqual(rotateNotificationSessionTarget(target, 'owner-b', { token: 'b' }), target);
  assert.equal(ownsNotificationSessionTarget(target, 'owner-b', 'a', 'notifications:history'), false);
  assert.equal(ownsNotificationSessionTarget(target, 'owner-a', 'a', 'notifications:invitations'), false);
});
