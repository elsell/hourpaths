import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
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

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('Home notification history is session-owned, paginated, localized, and exposes unread state', () => {
  assert.match(page, /const notificationOperations = createSessionOperationOwner\(\)/);
  assert.match(page, /const notificationMutationOperations = createSessionOperationOwner\(\)/);
  assert.match(page, /\.notifications\(cursor \|\| undefined\)/);
  assert.match(page, /mergeNotificationHistoryPage\(/);
  assert.match(page, /unreadCount: envelope\.meta\.unreadCount/);
  assert.match(page, /notificationUnreadCount = notificationHistory\.unreadCount/);
  assert.doesNotMatch(page, /notificationHistory\.items\.filter\(\(\{ read \}\) => !read\)\.length/);
  assert.match(page, /session !== current \|\| profile\?\.id !== ownerID \|\| !ticket\.current\(\)/);
  assert.match(page, /notificationPresentationMessageKey\(notification\)/);
  assert.match(page, /notification\.actionableHeading/);
  assert.match(page, /notification\.informationalHeading/);
  assert.match(page, /notification\.loadMore/);
  assert.match(page, /notification\.unreadCount/);
  assert.match(page, /aria-live="polite"/);
});

test('opening notification marks unread state before navigation and fails closed without browser capabilities', () => {
  assert.match(page, /notificationOperations\.invalidate\(\);\s+notificationsBusy = false;\s+if \(!notification\.read && !await mutateNotification/);
  assert.match(page, /notificationTarget\(notification, pendingInvitations\.items, visiblePendingOwnershipTransfers, accessiblePaths\)/);
  assert.match(page, /await mutateNotification\(\{ kind: 'read', notificationId: notification\.id \}\)/);
  assert.doesNotMatch(page, /document\.getElementById|scrollIntoView/);
  assert.match(page, /if \(target\.kind === 'invitation'\) \{\s+return;/);
  assert.match(page, /target\.kind === 'ownership-transfer'/);
  assert.match(page, /ownershipTransferFocusTarget = target\.transferId/);
  assert.match(page, /const path = accessiblePaths\.find\(\(candidate\) => candidate\.id === target\.pathId\)/);
  assert.match(page, /if \(!path\) return;/);
});

test('notification mutations use generated routes, global busy state, stale guards, and localized retry', () => {
  assert.match(page, /\.markNotificationRead\(mutation\.notificationId\)/);
  assert.match(page, /\.deleteNotification\(mutation\.notificationId\)/);
  assert.match(page, /\.markAllNotificationsRead\(\)/);
  assert.match(page, /applyNotificationMutation\(\s*notificationHistory,\s*mutation,\s*envelope\.data/);
  assert.match(page, /session !== current \|\| profile\?\.id !== ownerID \|\| !ticket\.current\(\)/);
  assert.match(page, /notificationMutationBusy/);
  assert.match(page, /notification\.markAllRead/);
  assert.match(page, /notification\.delete/);
  assert.match(page, /notification\.mutationError/);
  assert.match(page, /retryNotificationMutation/);
  assert.match(page, /common\.retry/);
  assert.match(page, /notificationConvergenceBrowser\?\.publish\(notificationConvergenceSignal\(ownerID\)\);\s+void notificationRefreshLatch\.request\(ownerID\);\s+return true;/);
});

test('web notification state converges from server truth across tabs, focus, and visibility', () => {
  assert.match(page, /openNotificationConvergenceBrowser\(/);
  assert.match(page, /notificationConvergenceOwner\(message, profile\?\.id \?\? ''\)/);
  assert.match(page, /notificationConvergenceBrowser\?\.close\(\)/);
  assert.match(page, /notificationRefreshLatch\.dispose\(\)/);
});

test('web notification convergence replaces page one only after success and preserves retry', () => {
  assert.match(page, /mergeNotificationHistoryPage\(\s*replace \? \{ items: \[\], nextCursor: '', unreadCount: notificationHistory\.unreadCount \} : notificationHistory,\s*page,\s*cursor,\s*\)/);
  assert.match(page, /if \(session !== current \|\| profile\?\.id !== ownerID \|\| !ticket\.current\(\)\) return;/);
  assert.match(page, /notificationConvergenceBrowser\?\.publish\(notificationConvergenceSignal\(ownerID\)\)/);
  assert.match(page, /if \(profile\) void notificationRefreshLatch\.request\(profile\.id\)/);
  assert.match(page, /retryNotificationHistory/);
  assert.match(page, /notificationErrorKey.*common\.retry/s);
});
