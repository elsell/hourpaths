import assert from 'node:assert/strict';
import test from 'node:test';
import {
  foregroundNotificationTargetKey,
  notificationDestinationTargetKey,
} from './foreground-notification-routing';

test('visible native routes map to stable notification target keys', () => {
  assert.equal(foregroundNotificationTargetKey('/following/comments/event-1', { eventID: 'event-1' }), 'comments:event-1');
  assert.equal(foregroundNotificationTargetKey('/following/comments/event-1/hearts/comment-1', { eventID: 'event-1' }), 'comments:event-1');
  assert.equal(foregroundNotificationTargetKey('/path/path-1', { pathID: 'path-1' }), 'path:path-1');
  assert.equal(foregroundNotificationTargetKey('/path/path-1/nudge-settings', { pathID: 'path-1' }), 'path:path-1');
  assert.equal(foregroundNotificationTargetKey('/invitations', {}), 'invitations');
  assert.equal(foregroundNotificationTargetKey('/follow-requests', {}), 'follow-requests');
  assert.equal(foregroundNotificationTargetKey('/profile/user_one', { username: 'user_one' }), 'profile:user_one');
  assert.equal(foregroundNotificationTargetKey('/notifications', {}), null);
  assert.equal(foregroundNotificationTargetKey('/path/path-1', { pathID: ['path-1', 'path-2'] }), null);
});

test('authorized notification destinations use the same target identity', () => {
  assert.equal(notificationDestinationTargetKey({ kind: 'comments', eventID: 'event-1' }), 'comments:event-1');
  assert.equal(notificationDestinationTargetKey({ kind: 'interaction-disabled', eventID: 'event-1', interaction: 'comments', pathID: 'path-1' }), 'comments:event-1');
  assert.equal(notificationDestinationTargetKey({ kind: 'invitation', invitationID: 'invitation-1' }), 'invitations');
  assert.equal(notificationDestinationTargetKey({ kind: 'follow-request', requestID: 'request-1' }), 'follow-requests');
  assert.equal(notificationDestinationTargetKey({ kind: 'profile', username: 'user_one' }), 'profile:user_one');
  assert.equal(notificationDestinationTargetKey({ kind: 'path', pathID: 'path-1' }), 'path:path-1');
});
