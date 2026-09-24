import type {
  OwnershipTransfer,
} from '@hourpaths/api-client';
import type { PathInvitationNotification as NotificationHistoryItem, PendingPathInvitation } from '@hourpaths/client-core';

export type NotificationSections = Readonly<{
  actionable: readonly NotificationHistoryItem[];
  informational: readonly NotificationHistoryItem[];
}>;

export type NotificationTarget =
  | Readonly<{ kind: 'invitation'; invitationId: string }>
  | Readonly<{ kind: 'follow-request'; requestId: string }>
  | Readonly<{ kind: 'profile'; username: string }>
  | Readonly<{ kind: 'ownership-transfer'; transferId: string }>
  | Readonly<{ kind: 'path'; pathId: string }>;

export type NotificationConvergenceSignal = Readonly<{
  type: 'notification-history-changed';
  ownerId: string;
}>;

export function notificationConvergenceSignal(ownerId: string): NotificationConvergenceSignal {
  return Object.freeze({ type: 'notification-history-changed', ownerId });
}

export function notificationConvergenceOwner(message: unknown, currentOwnerId: string): string | null {
  if (!currentOwnerId || !message || typeof message !== 'object' || Array.isArray(message)) return null;
  const record = message as Record<string, unknown>;
  if (
    Object.keys(record).sort().join(',') !== 'ownerId,type' ||
    record.type !== 'notification-history-changed' ||
    record.ownerId !== currentOwnerId
  ) return null;
  return currentOwnerId;
}

export function notificationSections(
  notifications: readonly NotificationHistoryItem[],
): NotificationSections {
  const newestFirst = (left: NotificationHistoryItem, right: NotificationHistoryItem) =>
    Date.parse(right.createdAt) - Date.parse(left.createdAt);
  return Object.freeze({
    actionable: Object.freeze(
      notifications.filter(({ presentation }) => presentation === 'actionable').sort(newestFirst),
    ),
    informational: Object.freeze(
      notifications.filter(({ presentation }) => presentation === 'informational').sort(newestFirst),
    ),
  });
}

export function notificationTarget(
  notification: NotificationHistoryItem,
  pendingInvitations: readonly PendingPathInvitation[],
  pendingOwnershipTransfers: readonly OwnershipTransfer[],
  accessiblePaths: readonly Readonly<{ id: string }>[],
): NotificationTarget | null {
  if (notification.type === 'path_deleted' || notification.type === 'path_member_removed') return null;
  if (notification.type === 'follow_request_received') {
    return Object.freeze({ kind: 'follow-request', requestId: notification.followRequestId });
  }
  if (notification.type === 'new_follower' || notification.type === 'follow_request_accepted') {
    return Object.freeze({ kind: 'profile', username: notification.actor.username });
  }
  if (notification.type === 'path_invitation_received') {
    return pendingInvitations.some(
      ({ invitation }) =>
        invitation.id === notification.invitationId &&
        invitation.pathId === notification.pathId,
    )
      ? Object.freeze({ kind: 'invitation', invitationId: notification.invitationId })
      : null;
  }

  if (notification.type === 'path_ownership_transfer_received') {
    return pendingOwnershipTransfers.some(
      (transfer) => transfer.id === notification.ownershipTransferId && transfer.pathId === notification.pathId,
    )
      ? Object.freeze({ kind: 'ownership-transfer', transferId: notification.ownershipTransferId })
      : null;
  }

  return accessiblePaths.some(({ id }) => id === notification.pathId)
    ? Object.freeze({ kind: 'path', pathId: notification.pathId })
    : null;
}
