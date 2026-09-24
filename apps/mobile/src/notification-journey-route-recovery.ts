export type NotificationJourneyIntent =
  | Readonly<{ kind: 'notifications'; routeKey: 'notifications:history' }>
  | Readonly<{ kind: 'invitations'; routeKey: 'notifications:invitations' }>
  | Readonly<{ kind: 'notification-settings'; routeKey: 'notifications:settings' }>;

let pendingBootstrap: Readonly<{
  intent: NotificationJourneyIntent;
  sessionKey?: string;
}> | null = null;

export function notificationJourneyIntentFromPathname(pathname: string): NotificationJourneyIntent | null {
  const normalized = `/${pathname.split('/').filter(Boolean).join('/')}`;
  if (normalized === '/notifications') return { kind: 'notifications', routeKey: 'notifications:history' };
  if (normalized === '/invitations') return { kind: 'invitations', routeKey: 'notifications:invitations' };
  if (normalized === '/settings/notifications') {
    return { kind: 'notification-settings', routeKey: 'notifications:settings' };
  }
  return null;
}

export function notificationJourneyHref(intent: NotificationJourneyIntent): string {
  if (intent.kind === 'notifications') return '/notifications';
  if (intent.kind === 'invitations') return '/invitations';
  return '/settings/notifications';
}

export function queueNotificationJourneyBootstrap(intent: NotificationJourneyIntent, sessionKey?: string) {
  pendingBootstrap = { intent, sessionKey };
}

export function takeNotificationJourneyBootstrap(currentSessionKey?: string) {
  const pending = pendingBootstrap;
  pendingBootstrap = null;
  if (!pending || (pending.sessionKey !== undefined && pending.sessionKey !== currentSessionKey)) return null;
  return pending.intent;
}

export function scheduleNotificationJourneyBootstrap(
  intent: NotificationJourneyIntent,
  navigateHome: () => void,
  sessionKey?: string,
) {
  const timeout = setTimeout(() => {
    queueNotificationJourneyBootstrap(intent, sessionKey);
    navigateHome();
  }, 50);
  return () => clearTimeout(timeout);
}
