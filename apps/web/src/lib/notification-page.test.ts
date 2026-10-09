import assert from 'node:assert/strict';
import test from 'node:test';
import type { PendingPathInvitation, PathInvitationNotification } from '@hourpaths/client-core';
import {
  notificationConvergenceOwner,
  notificationConvergenceSignal,
  notificationSections,
  notificationTarget,
} from './notification-page.js';

const actionable: PathInvitationNotification = {
  id: 'notification-actionable',
  type: 'path_invitation_received',
  presentation: 'actionable',
  read: false,
  createdAt: '2026-07-23T12:00:00Z',
  actor: { userId: 'inviter', username: 'book.owner', displayName: 'book-owner-display-name' },
  pathId: 'path-reading',
  pathName: 'Morning Reading',
  invitationId: 'invitation-pending',
  offeredRole: 'participant',
};

const informational: PathInvitationNotification = {
  ...actionable,
  id: 'notification-informational',
  type: 'path_invitation_accepted',
  presentation: 'informational',
  createdAt: '2026-07-23T13:00:00Z',
};

const pendingInvitation: PendingPathInvitation = {
  invitation: {
    id: 'invitation-pending',
    pathId: 'path-reading',
    inviterUserId: 'inviter',
    recipientUserId: 'recipient',
    offeredRole: 'participant',
    createdAt: '2026-07-23T11:00:00Z',
  },
  pathName: 'Morning Reading',
  inviter: actionable.actor,
};

test('notification sections remain distinct and newest first', () => {
  const olderActionable = {
    ...actionable,
    id: 'notification-older',
    createdAt: '2026-07-23T10:00:00Z',
  };
  const sections = notificationSections([olderActionable, informational, actionable]);

  assert.deepEqual(sections.actionable.map(({ id }) => id), [
    'notification-actionable',
    'notification-older',
  ]);
  assert.deepEqual(sections.informational.map(({ id }) => id), [
    'notification-informational',
  ]);
});

test('notification targets resolve only current invitation and accessible Path context', () => {
  assert.deepEqual(
    notificationTarget(actionable, [pendingInvitation], [], [{ id: 'path-reading' }]),
    { kind: 'invitation', invitationId: 'invitation-pending' },
  );
  assert.deepEqual(
    notificationTarget(informational, [pendingInvitation], [], [{ id: 'path-reading' }]),
    { kind: 'path', pathId: 'path-reading' },
  );

  assert.equal(notificationTarget(actionable, [], [], [{ id: 'path-reading' }]), null);
  assert.equal(notificationTarget(informational, [pendingInvitation], [], []), null);
});

test('actionable ownership transfer targets only its independently pending request', () => {
  const notification: PathInvitationNotification = {
    id: 'transfer-notification',
    type: 'path_ownership_transfer_received',
    presentation: 'actionable',
    read: false,
    createdAt: '2026-07-23T14:00:00Z',
    actor: actionable.actor,
    pathId: 'path-reading',
    pathName: 'Morning Reading',
    ownershipTransferId: 'transfer-1',
  };
  const transfer = {
    id: 'transfer-1', pathId: 'path-reading', creatorUserId: 'inviter', recipientUserId: 'recipient',
    reviewedAt: '2026-07-23T12:59:00Z', createdAt: '2026-07-23T13:00:00Z', expiresAt: '2026-07-30T13:00:00Z', state: 'pending' as const,
  };
  assert.deepEqual(notificationTarget(notification, [], [transfer], [{ id: 'path-reading' }]), {
    kind: 'ownership-transfer', transferId: 'transfer-1',
  });
  assert.equal(notificationTarget(notification, [], [], [{ id: 'path-reading' }]), null);
});

test('standalone Path-deletion notices never resolve a navigation target', () => {
  const deleted: PathInvitationNotification = {
    id: 'path-deleted-notice',
    type: 'path_deleted',
    presentation: 'informational',
    read: false,
    createdAt: '2026-07-23T15:00:00Z',
    actor: actionable.actor,
    pathName: 'Morning Reading',
  };
  assert.equal(notificationTarget(deleted, [pendingInvitation], [], [{ id: 'path-reading' }]), null);
});

test('removed-member notices stay informational while role changes can reopen an accessible Path', () => {
  const changed: PathInvitationNotification = {
    id: 'role-change-notice', type: 'path_member_role_changed', presentation: 'informational', read: false,
    createdAt: '2026-07-23T15:00:00Z', actor: actionable.actor, pathId: 'path-reading',
    pathName: 'Morning Reading', offeredRole: 'participant',
  };
  const removed: PathInvitationNotification = { ...changed, id: 'removal-notice', type: 'path_member_removed', offeredRole: 'participant' };
  assert.deepEqual(notificationTarget(changed, [], [], [{ id: 'path-reading' }]), { kind: 'path', pathId: 'path-reading' });
  assert.equal(notificationTarget(removed, [], [], [{ id: 'path-reading' }]), null);
});

test('social notices resolve dedicated follow-request and profile destinations', () => {
  const request: PathInvitationNotification = {
    id: 'follow-request-notice', type: 'follow_request_received', presentation: 'actionable', read: false,
    createdAt: '2026-07-23T15:00:00Z', actor: actionable.actor, followRequestId: 'request-1',
  };
  const accepted: PathInvitationNotification = {
    ...request, id: 'follow-accepted-notice', type: 'follow_request_accepted', presentation: 'informational',
  };
  assert.deepEqual(notificationTarget(request, [], [], []), { kind: 'follow-request', requestId: 'request-1' });
  assert.deepEqual(notificationTarget(accepted, [], [], []), { kind: 'profile', username: 'book.owner' });
});

test('notification convergence signals carry no notification state and are same-user owned', () => {
  const signal = notificationConvergenceSignal('user-a');
  assert.deepEqual(signal, { type: 'notification-history-changed', ownerId: 'user-a' });
  assert.equal(notificationConvergenceOwner(signal, 'user-a'), 'user-a');
  assert.equal(notificationConvergenceOwner(signal, 'user-b'), null);
  assert.equal(notificationConvergenceOwner({ ...signal, notificationId: 'secret' }, 'user-a'), null);
  assert.equal(notificationConvergenceOwner({ type: signal.type }, 'user-a'), null);
  assert.equal(notificationConvergenceOwner(null, 'user-a'), null);
});
