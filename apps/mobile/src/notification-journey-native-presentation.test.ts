import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  nativeBackNotificationJourneyState,
  normalizeNotificationJourneyRouteState,
} from './notification-journey-route-ancestry';
import {
  notificationJourneyIntentFromPathname,
  queueNotificationJourneyBootstrap,
  takeNotificationJourneyBootstrap,
} from './notification-journey-route-recovery';
import { ownsNotificationSettingsState } from './notification-settings-state';

const source = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');
const rootLayout = source('../app/_layout.tsx') + source('./ui/navigation-theme.ts');
const notificationsRoute = source('../app/notifications.tsx');
const invitationsRoute = source('../app/invitations.tsx');
const notificationSettingsRoute = source('../app/settings/notifications.tsx');
const routePresentation = source('./ui/notification-route-presentation.tsx');
const recoveryView = source('./ui/notification-journey-recovery-view.tsx');
const notificationView = source('./ui/notification-history-view.tsx');
const invitationsView = source('./ui/pending-invitations-view.tsx');
const settingsList = source('./ui/settings-list.tsx');
const nativeHeader = source('./ui/native-header-button.ios.tsx');
const en = JSON.parse(source('../../../packages/i18n/src/locales/en.json')) as Record<string, string>;
const es = JSON.parse(source('../../../packages/i18n/src/locales/es.json')) as Record<string, string>;

test('canonical notification intents admit only exact owned routes', () => {
  assert.equal(notificationJourneyIntentFromPathname('/notifications')?.kind, 'notifications');
  assert.equal(notificationJourneyIntentFromPathname('/invitations')?.kind, 'invitations');
  assert.equal(notificationJourneyIntentFromPathname('/settings/notifications')?.kind, 'notification-settings');
  assert.equal(notificationJourneyIntentFromPathname('/settings/account'), null);
  assert.equal(notificationJourneyIntentFromPathname('/notifications/extra'), null);
});

test('owned queued bootstrap cannot cross replacement while an unowned cold bootstrap is admitted', () => {
  const intent = { kind: 'notifications', routeKey: 'notifications:history' } as const;
  queueNotificationJourneyBootstrap(intent, 'account-a:1');
  assert.equal(takeNotificationJourneyBootstrap('account-b:2'), null);
  assert.equal(takeNotificationJourneyBootstrap('account-a:1'), null);

  queueNotificationJourneyBootstrap(intent);
  assert.deepEqual(takeNotificationJourneyBootstrap('account-b:2'), intent);
});

test('cold invitation ancestry preserves keyed Home state and produces native back chain', () => {
  const tabs = {
    key: 'tabs-key',
    name: '(tabs)',
    state: {
      index: 1,
      routes: [
        { key: 'home-key', name: 'home', state: { retained: 'home' } },
        { key: 'following-key', name: 'following', state: { retained: 'following' } },
      ],
    },
  };
  const notifications = { key: 'notifications-key', name: 'notifications' };
  const invitations = { key: 'invitations-key', name: 'invitations' };
  const normalized = normalizeNotificationJourneyRouteState({
    index: 2,
    key: 'root-key',
    routes: [invitations, tabs, notifications],
  }, { kind: 'invitations', routeKey: 'notifications:invitations' });

  assert.equal(normalized.key, 'root-key');
  assert.deepEqual(normalized.routes.map(({ name }) => name), ['(tabs)', 'notifications', 'invitations']);
  assert.equal(normalized.routes[0]?.key, 'tabs-key');
  assert.equal(normalized.routes[1]?.key, 'notifications-key');
  assert.equal(normalized.routes[2]?.key, 'invitations-key');
  const nested = normalized.routes[0]?.state as typeof tabs.state;
  assert.equal(nested.index, 0);
  assert.equal(nested.routes[0]?.key, 'home-key');
  assert.equal(nested.routes[1]?.key, 'following-key');
  assert.equal(nativeBackNotificationJourneyState(normalized).routes.at(-1)?.name, 'notifications');
  assert.equal(
    nativeBackNotificationJourneyState(nativeBackNotificationJourneyState(normalized)).routes.at(-1)?.name,
    '(tabs)',
  );
});

test('cold notification settings ancestry is Home to Settings to notification settings', () => {
  const normalized = normalizeNotificationJourneyRouteState({
    index: 0,
    routes: [{ key: 'settings-detail', name: 'settings/notifications' }],
  }, { kind: 'notification-settings', routeKey: 'notifications:settings' });
  assert.deepEqual(normalized.routes.map(({ name }) => name), [
    '(tabs)',
    'settings/index',
    'settings/notifications',
  ]);
  assert.equal(normalized.routes[2]?.key, 'settings-detail');
});

test('owned routes never return null and expose visible localized recovery with Retry and Home', () => {
  for (const route of [notificationsRoute, invitationsRoute, notificationSettingsRoute]) {
    assert.match(route, /NotificationJourneyRecoveryView/);
    assert.match(route, /useNotificationJourneyRouteAncestry/);
    assert.doesNotMatch(route, /return null/);
    assert.doesNotMatch(route, /router\.replace\('\/'\)/);
  }
  assert.match(routePresentation, /NotificationJourneyRecoverySource/);
  assert.match(routePresentation, /sessionKey/);
  assert.match(notificationsRoute, /lastRecoverySessionKey/);
  assert.match(invitationsRoute, /lastRecoverySessionKey/);
  assert.match(notificationSettingsRoute, /lastRecoverySessionKey/);
  assert.match(recoveryView, /accessibilityRole="progressbar"/);
  assert.match(recoveryView, /common\.retry/);
  assert.match(recoveryView, /notification\.recoveryHome/);
  for (const catalog of [en, es]) for (const key of [
    'notification.recoveryHome',
    'notification.recoveryOfflineHeading',
    'notification.recoveryOfflineDescription',
    'notification.recoveryUnavailableDescription',
    'notification.updating',
  ]) assert.ok(catalog[key]?.trim(), `${key} must be localized`);
});

test('notification settings fail closed across account-owner replacement', () => {
  assert.equal(ownsNotificationSettingsState('account-a:1', 'account-a:1'), true);
  assert.equal(ownsNotificationSettingsState('account-a:1', 'account-b:2'), false);
  assert.match(notificationSettingsRoute, /setStateOwnerKey\(ownedSessionKey\)/);
  assert.match(notificationSettingsRoute, /setPermission\(null\)/);
  assert.match(notificationSettingsRoute, /setNudgeChannelPreference\(null\)/);
  assert.match(notificationSettingsRoute, /setNudgeChannelLoading\(true\)/);
  assert.match(notificationSettingsRoute, /ownsNotificationSettingsState\(stateOwnerKey, activePresentation\.sessionKey\)/);
  assert.match(notificationSettingsRoute, /activeOwnerKey\.current === ownedSessionKey/);
  assert.match(notificationSettingsRoute, /ownedNudgeChannelLoading = !ownsState \|\| nudgeChannelLoading/);
});

test('notification toolbar action remains stable while busy and announces progress separately', () => {
  assert.match(rootLayout, /disabled=\{notificationPresentation\.busy \|\| !notificationPresentation\.canMarkAllRead\}/);
  assert.doesNotMatch(rootLayout, /canMarkAllRead && !notificationPresentation\.busy/);
  assert.match(nativeHeader, /nativeDisabled\(disabled\)/);
  assert.match(notificationsRoute, /accessibilityLiveRegion="polite"/);
  assert.match(notificationsRoute, /notification\.updating/);
});

test('notification and invitation rows are flat, intrinsic, and accessibility-scalable', () => {
  assert.doesNotMatch(notificationView, /minHeight:\s*68/);
  assert.doesNotMatch(notificationView, /<SettingsSection/);
  assert.doesNotMatch(invitationsView, /<SettingsSection/);
  assert.doesNotMatch(notificationView, /numberOfLines=/);
  assert.doesNotMatch(invitationsView, /numberOfLines=/);
  assert.match(settingsList, /needsCompactVerticalLayout/);
  assert.match(settingsList, /switchRowStacked/);
  assert.match(notificationView, /accessibilityState=\{\{ busy, disabled: !onOpen \|\| busy \}\}/);
});
