import { createSessionApiClient } from '@hourpaths/api-client';
import { createNotificationHistoryOwner, notificationPresentationMessageKey, type NotificationHistoryItem, type NotificationHistorySnapshot } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { openNotificationConvergenceBrowser, type NotificationConvergenceBrowser } from '../../../notification-convergence-browser';
import { notificationConvergenceOwner, notificationConvergenceSignal } from '../../../notification-page';
import type { Notification, NotificationsSnapshot } from '../domain/notification';
import type { Notifications, NotificationOperations } from '../ports/notifications';
function required<T>(result: { response: Response; data?: { data: T } }): T {
  if (!result.response.ok || !result.data) throw Error('notification_request_failed');
  return result.data.data;
}
function target(item: NotificationHistoryItem): Notification['target'] {
  switch (item.type) {
    case 'goal_practice_reminder': return { kind: 'reminder', paths: item.reminder.paths };
    case 'path_deleted': case 'path_member_removed': return null;
    case 'path_invitation_received': return { kind: 'invitations' };
    case 'follow_request_received': return { kind: 'people' };
    case 'new_follower': case 'follow_request_accepted': return { kind: 'profile', username: item.actor.username };
    case 'path_ownership_transfer_received': return { kind: 'ownership', pathId: item.pathId };
    default: return { kind: 'path', pathId: item.pathId };
  }
}
export function apiNotifications(baseURL: string, token: () => string | null, rejected: (token: string | null) => void, i18n: Translator): Notifications {
  const client = (signal: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  let operations: NotificationOperations | null = null;
  const run = <T>(operation: (signal: AbortSignal) => Promise<T>) => { if (!operations) return Promise.reject(Error('notification_not_started')); return operations.run(operation); };
  let ownerId = '', disposed = false, channel: NotificationConvergenceBrowser | null = null;
  const owner = createNotificationHistoryOwner({
    page: cursor => run(async signal => {
      if (!ownerId) {
        const identity = required(await client(signal).profile());
        if (disposed) throw Error('notification_owner_disposed');
        if (typeof identity.id !== 'string' || !identity.id) throw Error('notification_owner_invalid');
        ownerId = identity.id;
        channel = openNotificationConvergenceBrowser(message => { if (notificationConvergenceOwner(message, ownerId)) void owner.refresh(); }, () => { void owner.refresh(); });
      }
      const result = await client(signal).notifications(cursor || undefined);
      const items = required(result);
      return { items, nextCursor: result.data?.meta.nextCursor ?? '', unreadCount: result.data?.meta.unreadCount };
    }),
    mutate: change => run(async signal => {
      const api = client(signal);
      const result = change.kind === 'read-all' ? await api.markAllNotificationsRead() : change.kind === 'read' ? await api.markNotificationRead(change.notificationId) : await api.deleteNotification(change.notificationId);
      return required(result);
    }),
    cancel() { operations?.cancel(); },
    changed() { channel?.publish(notificationConvergenceSignal(ownerId)); },
  });
  let previous: NotificationHistorySnapshot | null = null;
  let snapshot: NotificationsSnapshot;
  return {
    start(value) { if (disposed || operations) return; operations = value; void owner.refresh(); },
    snapshot() {
      const value = owner.snapshot();
      if (value !== previous) {
        previous = value;
        snapshot = { items: value.history.items.map(item => ({ id: item.id, ...(item.type === 'nudge_received' ? { reportTarget: { kind: 'nudge' as const, id: item.id } } : {}), createdAt: Date.parse(item.createdAt), read: item.read, presentation: item.presentation, target: target(item), message: i18n.t(notificationPresentationMessageKey(item), {
          displayName: item.actor.displayName, username: item.actor.username, pathName: 'pathName' in item ? item.pathName : '', emoji: item.type === 'practice_reaction' ? item.reaction : '',
          pathVisibility: item.type === 'path_visibility_changed' ? i18n.t(`pathVisibility.option.${item.pathVisibility}`) : '',
        }) })), unreadCount: value.history.unreadCount, nextCursor: value.history.nextCursor, loaded: value.loaded, loading: value.loading, mutating: value.mutating, error: value.error };
      }
      return snapshot;
    },
    invalidate: owner.invalidate,
    subscribe: owner.subscribe, refresh: () => operations ? owner.refresh() : Promise.resolve(), loadMore: owner.loadMore, mutate: owner.mutate,
    dispose() { disposed = true; channel?.close(); channel = null; owner.dispose(); },
  };
}
