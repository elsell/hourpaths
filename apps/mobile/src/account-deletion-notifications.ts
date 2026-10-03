import * as Notifications from 'expo-notifications';

export async function captureDeletionNotifications(): Promise<string[]> {
  const [presented, scheduled] = await Promise.all([
    Notifications.getPresentedNotificationsAsync(), Notifications.getAllScheduledNotificationsAsync(),
  ]);
  return [...new Set([...presented.map(item => item.request.identifier), ...scheduled.map(item => item.identifier)])];
}

/** Captured identifiers survive an account switch; never dismiss every account's notifications. */
export async function clearDeletionNotifications(identifiers: readonly string[]): Promise<void> {
  for (const id of identifiers) {
    await Notifications.cancelScheduledNotificationAsync(id);
    await Notifications.dismissNotificationAsync(id);
  }
  const response = Notifications.getLastNotificationResponse();
  if (response && identifiers.includes(response.notification.request.identifier)) Notifications.clearLastNotificationResponse();
}
