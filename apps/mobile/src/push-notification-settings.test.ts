import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const appConfig = readFileSync(fileURLToPath(new URL('../app.json', import.meta.url)), 'utf8');
const layout = readFileSync(fileURLToPath(new URL('../app/_layout.tsx', import.meta.url)), 'utf8');
const settings = readFileSync(fileURLToPath(new URL('../app/settings/index.tsx', import.meta.url)), 'utf8');
const notificationSettings = readFileSync(
  fileURLToPath(new URL('../app/settings/notifications.tsx', import.meta.url)),
  'utf8',
);
const nativeAdapter = readFileSync(
  fileURLToPath(new URL('./push-notifications-native.ts', import.meta.url)),
  'utf8',
);
const home = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('Expo notifications are configured as a native plugin and settings route', () => {
  assert.match(appConfig, /"expo-notifications"/);
  assert.match(layout, /settings\/notifications/);
});

test('notification permission control is always discoverable from settings', () => {
  assert.match(settings, /notification\.settings\.openLabel/);
  assert.match(settings, /router\.push\('\/settings\/notifications'\)/);
  assert.match(notificationSettings, /notification\.settings\.permission/);
});

test('permission control requests again or opens system settings as required', () => {
  assert.match(nativeAdapter, /Notifications\.getPermissionsAsync\(\)/);
  assert.match(nativeAdapter, /Notifications\.requestPermissionsAsync\(/);
  assert.match(nativeAdapter, /Notifications\.setNotificationChannelAsync\('hourpaths'/);
  assert.match(nativeAdapter, /Linking\.openSettings\(\)/);
  assert.match(notificationSettings, /ownedPermission\.canAskAgain/);
});

test('native lifecycle presents only unrelated actionable foreground receipts', () => {
  assert.match(nativeAdapter, /nativeForegroundPresentationBeforeDeadline\(/);
  assert.match(nativeAdapter, /setTimeout\(resolve, 2_500\)/);
  assert.match(nativeAdapter, /pushNotificationID\(data\)/);
  assert.match(nativeAdapter, /ports\.foreground\(notificationID\)/);
  assert.match(nativeAdapter, /addNotificationReceivedListener/);
  assert.match(nativeAdapter, /addNotificationResponseReceivedListener/);
  assert.match(nativeAdapter, /handleNotificationTap/);
  assert.match(home, /createForegroundNotificationCoordinator\(/);
  assert.match(home, /foregroundNotificationTargetKey\(pathname, routeParameters\)/);
  assert.match(home, /notificationDestinationTargetKey\(resolved\.destination\)/);
  assert.match(home, /refreshForegroundNotificationTarget\(targetKey, currentSession, ownerID\)/);
  assert.match(home, /response\.status === 404[\s\S]*applyRefreshedMobilePath\(current\.profile, pathID, null\)/);
  assert.match(home, /active\.session\?\.token === currentSession\.token[\s\S]*if \(!isCurrent\(\)\) return;/);
  assert.match(home, /resetPathDetail\(\);[\s\S]*router\.replace\('\/\(tabs\)\/home'\)/);
  assert.match(home, /projected\.archivedAt[\s\S]*admitMobileSessionPaths\(\[\], \[projected\]\)/);
  const foregroundPort = home.slice(home.indexOf('foreground: (notificationID)'), home.indexOf('resolve: (notificationID)'));
  assert.doesNotMatch(foregroundPort, /markPushNotificationRead/);
});

test('native registration uses SecureStore identity and Expo project-scoped tokens', () => {
  assert.match(nativeAdapter, /SecureStore\.getItemAsync/);
  assert.match(nativeAdapter, /SecureStore\.setItemAsync/);
  assert.match(nativeAdapter, /Crypto\.randomUUID\(\)/);
  assert.match(nativeAdapter, /Notifications\.getExpoPushTokenAsync\(\{ projectId \}\)/);
  assert.match(home, /createNativePushRegistrationCoordinator\(/);
  assert.match(home, /\.upsertPushInstallation\(/);
  assert.match(home, /\.deletePushInstallation\(/);
  assert.match(home, /coordinator\.signOut\(ownerID\)/);
});

test('tap wiring refreshes invitation state and performs visible route navigation', () => {
  assert.match(home, /\.getNotification\(notificationID\)/);
  assert.match(home, /await loadPendingInvitationPage\(currentSession\)/);
  assert.match(home, /mergePendingInvitationPage\(/);
  assert.match(home, /openPathDetail\(destination\.pathID\)/);
  assert.match(home, /stageNativePushDeregistration/);
  assert.match(home, /loadNativePushDeregistration/);
  assert.match(home, /retryPendingDeregistration/);
  assert.match(home, /setTimeout\(\(\) => \{ void retryPendingDeregistration\(\); \}, 30_000\)/);
  assert.match(home, /setTimeout\(\(\) => \{ void synchronize\(\); \}, 30_000\)/);
  assert.match(home, /openPathDetail\(path\.id\)/);
  assert.match(home, /openPendingInvitations\(destination\.invitationID\)/);
  assert.match(home, /router\.push\(\{[\s\S]*pathname: '\/invitations'/);
});

test('authoritative unread mutations reconcile the native application badge', () => {
  assert.match(nativeAdapter, /Notifications\.setBadgeCountAsync\(unreadCount\)/);
  assert.match(home, /setNativeNotificationBadge\(envelope\.data\.unreadCount\)/);
  assert.match(home, /setNativeNotificationBadge\(next\.unreadCount\)/);
});
