export type SettingsJourneyIntent =
  | Readonly<{ kind: 'settings'; routeKey: 'settings:root' }>
  | Readonly<{ kind: 'account'; routeKey: 'settings:account' }>
  | Readonly<{ kind: 'time-zone'; routeKey: 'settings:time-zone' }>
  | Readonly<{ kind: 'interactions'; routeKey: 'settings:interactions' }>
  | Readonly<{ kind: 'blocked-accounts'; routeKey: 'settings:blocked-accounts' }>;

let pendingBootstrap: Readonly<{ intent: SettingsJourneyIntent; sessionKey?: string }> | null = null;

export function settingsJourneyIntentFromPathname(pathname: string): SettingsJourneyIntent | null {
  const normalized = `/${pathname.split('/').filter(Boolean).join('/')}`;
  if (normalized === '/settings') return { kind: 'settings', routeKey: 'settings:root' };
  if (normalized === '/settings/account') return { kind: 'account', routeKey: 'settings:account' };
  if (normalized === '/settings/time-zone') return { kind: 'time-zone', routeKey: 'settings:time-zone' };
  if (normalized === '/settings/interactions') return { kind: 'interactions', routeKey: 'settings:interactions' };
  if (normalized === '/settings/blocked-accounts') {
    return { kind: 'blocked-accounts', routeKey: 'settings:blocked-accounts' };
  }
  return null;
}

export function settingsJourneyHref(intent: SettingsJourneyIntent): string {
  if (intent.kind === 'settings') return '/settings';
  return `/settings/${intent.kind}`;
}

export function queueSettingsJourneyBootstrap(intent: SettingsJourneyIntent, sessionKey?: string) {
  pendingBootstrap = { intent, sessionKey };
}

export function takeSettingsJourneyBootstrap(currentSessionKey?: string): SettingsJourneyIntent | null {
  const pending = pendingBootstrap;
  pendingBootstrap = null;
  if (!pending || (pending.sessionKey !== undefined && pending.sessionKey !== currentSessionKey)) return null;
  return pending.intent;
}

export function scheduleSettingsJourneyBootstrap(
  intent: SettingsJourneyIntent,
  navigateHome: () => void,
  sessionKey?: string,
) {
  const timeout = setTimeout(() => {
    queueSettingsJourneyBootstrap(intent, sessionKey);
    navigateHome();
  }, 50);
  return () => clearTimeout(timeout);
}
