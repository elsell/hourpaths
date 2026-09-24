import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const rootLayout = readFileSync(fileURLToPath(new URL('../app/_layout.tsx', import.meta.url)), 'utf8');
const notificationRoute = readFileSync(fileURLToPath(new URL('../app/notifications.tsx', import.meta.url)), 'utf8');
const notificationView = readFileSync(fileURLToPath(new URL('./ui/notification-history-view.tsx', import.meta.url)), 'utf8');
const nativeRoute = readFileSync(fileURLToPath(new URL('./ui/native-route-presentation.tsx', import.meta.url)), 'utf8');
const routePresentation = readFileSync(fileURLToPath(new URL('./ui/notification-route-presentation.tsx', import.meta.url)), 'utf8');

test('mobile notification bell loads and paginates recipient-scoped generated history', () => {
  assert.match(page, /\.notifications\(cursor \|\| undefined\)/);
  assert.match(page, /mergeNotificationHistoryPage\(empty, page, ''\)/);
  assert.match(page, /mergeNotificationHistoryPage\(current, page, cursor\)/);
  assert.match(page, /cursor !== notificationHistory\.nextCursor/);
  assert.match(page, /ownsCurrentNotificationOperation\(notificationTarget\.current, ownerID, intentKey, currentSession\)/);
  assert.match(page, /record\.meta\.unreadCount/);
  assert.match(page, /setNotificationUnreadCount\(history\.unreadCount\)/);
  assert.doesNotMatch(page, /history\.items\.filter\(\(item\) => !item\.read\)\.length/);
});

test('native notification refresh replaces page one without clearing useful state on failure', () => {
  assert.match(page, /async function refreshNotifications\(\)/);
  const refresh = page.slice(
    page.indexOf('async function refreshNotifications()'),
    page.indexOf('async function openPendingInvitations'),
  );
  assert.doesNotMatch(refresh, /setNotificationHistory\(empty\)/);
  assert.match(refresh, /mergeNotificationHistoryPage\([\s\S]*\{ items: \[\], nextCursor: '', unreadCount:/);
  assert.match(refresh, /notificationRefreshOperations\.issue\(\)/);
  assert.match(refresh, /ticket\.current\(\)/);
  assert.match(refresh, /setNotificationsErrorKey\('notification\.error'\)/);
  assert.match(page, /onRetry=\{\(\) => void refreshNotifications\(\)\}/);
  const pushRefresh = page.slice(
    page.indexOf('async function refreshPushNotificationHistory'),
    page.indexOf('async function resolvePushNotification'),
  );
  assert.match(pushRefresh, /notificationRefreshOperations\.issue\(\)/);
  assert.match(pushRefresh, /ticket\.current\(\)/);
});

test('every notification load keeps stale cleanup from clearing a newer owner', () => {
  for (const [start, end] of [
    ['async function openNotifications', 'async function refreshNotifications'],
    ['async function refreshNotifications', 'async function openPendingInvitations'],
    ['async function loadMoreNotifications', 'async function mutateNotification'],
  ] as const) {
    const startIndex = page.indexOf(start);
    const operation = page.slice(startIndex, page.indexOf(end, startIndex + start.length));
    assert.match(operation, /const ticket = notificationRefreshOperations\.issue\(\)/);
    assert.match(operation, /finally \{[\s\S]*ticket\.current\(\)[\s\S]*notificationTarget\.current = null/);
  }
  const mutation = page.slice(
    page.indexOf('async function mutateNotification'),
    page.indexOf('async function openNotification', page.indexOf('async function mutateNotification') + 1),
  );
  assert.match(mutation, /notificationMutationOperations\.issue\(\)/);
  assert.match(mutation, /finally \{[\s\S]*notificationMutationTarget\.current = null/);
});

test('notification route uses native pull refresh and refreshes while active on foreground resume', () => {
  assert.match(nativeRoute, /RefreshControl/);
  assert.match(nativeRoute, /refreshControl=\{onRefresh \? <RefreshControl/);
  assert.match(nativeRoute, /alwaysBounceVertical=\{Boolean\(onRefresh\)\}/);
  assert.match(routePresentation, /refresh: \(\) => void/);
  assert.match(notificationRoute, /refreshing=\{presentation\.refreshing\}/);
  assert.match(notificationRoute, /onRefresh=\{presentation\.refresh\}/);
  assert.match(notificationRoute, /AppState\.addEventListener\('change'/);
  assert.match(notificationRoute, /state === 'active'/);
  assert.match(notificationRoute, /refreshRef\.current\?\.\(\)/);
  assert.match(notificationRoute, /useFocusEffect/);
  assert.match(notificationRoute, /hasFocused\.current/);
});

test('mobile notification history keeps actionable and informational presentation distinct', () => {
  assert.match(notificationView, /notification\.actionableHeading/);
  assert.match(notificationView, /notification\.informationalHeading/);
  assert.match(notificationView, /item\.presentation === 'actionable'/);
  assert.match(notificationView, /item\.presentation === 'informational'/);
  assert.match(notificationView, /notificationPresentationMessageKey\(item\)/);
});

test('mobile notification context authoritatively resolves pending invitations or accessible Paths', () => {
  assert.match(page, /loadPendingInvitationsThroughTarget\([\s\S]*notification\.invitationId/);
  assert.match(page, /notification\.type === 'path_invitation_received'[\s\S]*\? true/);
  assert.match(page, /destination\.profile\.paths\.find\(\(item\) => item\.id === notification\.pathId\)/);
  assert.match(page, /destination\.profile\.archivedPaths\.find\(\(item\) => item\.id === notification\.pathId\)/);
  assert.match(page, /openPendingInvitations\(notification\.invitationId\)/);
});

test('social notification taps open dedicated request and profile destinations', () => {
  const context = page.slice(
    page.indexOf('async function openNotificationContext'),
    page.indexOf('function openPathSharing'),
  );
  assert.match(context, /notification\.type === 'follow_request_received'[\s\S]*loadSocialFollowRequests\(true\)[\s\S]*router\.push\('\/follow-requests'\)/);
  assert.match(context, /notification\.type === 'new_follower'[\s\S]*notification\.type === 'follow_request_accepted'[\s\S]*loadSocialProfile\(notification\.actor\.username\)/);
  assert.match(context, /pathname: '\/profile\/\[username\]'[\s\S]*username: notification\.actor\.username/);
});

test('practice-reaction history and push taps use the existing authorized Path fallback', () => {
  const context = page.slice(
    page.indexOf('async function openNotificationContext'),
    page.indexOf('function openPathSharing'),
  );
  const push = page.slice(
    page.indexOf('async function resolvePushNotification'),
    page.indexOf('async function markPushNotificationRead'),
  );
  const canOpen = page.slice(
    page.indexOf('canOpen={(notification)'),
    page.indexOf('errorKey={notificationsErrorKey'),
  );

  assert.match(notificationView, /notificationPresentationMessageKey\(item\)/);
  assert.match(context, /destination\.profile\.paths\.find[\s\S]*destination\.profile\.archivedPaths\.find[\s\S]*openPathDetail\(path\.id\)/);
  assert.match(push, /current\.destination\.profile\.paths\.some[\s\S]*current\.destination\.profile\.archivedPaths\.some[\s\S]*kind: 'path'/);
  assert.match(canOpen, /ownedHomeDestination\.profile\.paths\.some[\s\S]*ownedHomeDestination\.profile\.archivedPaths\.some/);
  assert.match(context, /notification\.type === 'practice_comment'[\s\S]*openPracticeComments\(notification\.socialFeedEventId/);
  assert.match(push, /notification\.type === 'practice_comment'[\s\S]*kind: 'comments'[\s\S]*notification\.socialFeedEventId/);
  assert.match(push, /interactionDisabledDestinationFromAPI\(envelope\?\.data\)/);
  assert.match(page, /destination\.kind === 'interaction-disabled'[\s\S]*interactionDisabledComments[\s\S]*interactionDisabledReactions/);
  assert.match(page, /interactionNoticeKey: destination\.interaction/);
  assert.match(page, /interactionNoticeEventID: destination\.eventID/);
  const disabledResolution = push.slice(
    push.indexOf('if (interactionDisabled)'),
    push.indexOf('if (!notification)'),
  );
  assert.doesNotMatch(disabledResolution, /profile\.paths|archivedPaths/);
  assert.match(page, /\.getPracticeFeedEvent\(destination\.eventID\)/);
  assert.match(page, /socialFeedEventFromAPI\(envelope\.data\)/);
  assert.match(page, /interactionDisabledEventOperations\.issue\(\)/);
  assert.match(page, /current\.session === currentSession[\s\S]*current\.destination\.profile\.id === ownerID/);
  assert.match(page, /items: \[focused, \.\.\.socialFeedPage\.current\.items\.filter/);
  assert.match(page, /response\.status === 404/);
  assert.doesNotMatch(page.slice(
    page.indexOf("if (destination.kind === 'interaction-disabled')"),
    page.indexOf("if (destination.kind === 'comments')"),
  ), /loadSocialFeed/);
});

test('mobile notification state is discarded across session ownership boundaries', () => {
  assert.match(page, /notificationTarget\.current = null/);
  assert.match(page, /resetNotifications\(\)/);
  assert.match(page, /notificationTarget\.current && notificationTarget\.current\.ownerID !== nextOwnerID/);
});

test('an unavailable old push uses owned generic accessible feedback', () => {
  const lifecycle = page.slice(
    page.indexOf('const dispose = installNativeNotificationLifecycle'),
    page.indexOf('return () => {', page.indexOf('const dispose = installNativeNotificationLifecycle')),
  );
  assert.match(lifecycle, /unavailable: \(\) => \{[\s\S]*current[\s\S]*notification\.itemUnavailable/);
  assert.doesNotMatch(lifecycle, /pathName|actor|displayName|username/);
  assert.match(page, /notificationTapFeedbackKey[\s\S]*<StatusBanner[\s\S]*notificationTapFeedbackKey/);
  assert.match(page, /resetNotifications\(\)[\s\S]*setNotificationTapFeedbackKey\(null\)/);
  const cleanup = page.slice(
    page.indexOf('return () => {', page.indexOf('const dispose = installNativeNotificationLifecycle')),
    page.indexOf('}, [session?.token', page.indexOf('const dispose = installNativeNotificationLifecycle')),
  );
  assert.match(cleanup, /current = false;[\s\S]*setNotificationTapFeedbackKey\(null\)/);
});

test('mobile notification mutations use authoritative unread counts and owned responses', () => {
  assert.match(page, /\.markNotificationRead\(mutation\.notificationId\)/);
  assert.match(page, /\.deleteNotification\(mutation\.notificationId\)/);
  assert.match(page, /\.markAllNotificationsRead\(\)/);
  assert.match(page, /applyNotificationMutation\(/);
  assert.match(page, /ownsCurrentNotificationOperation\(notificationMutationTarget\.current, ownerID, intentKey, currentSession\)/);
  assert.match(page, /setNotificationUnreadCount\(next\.unreadCount\)/);
  assert.match(rootLayout, /notification\.markAllRead/);
  assert.match(notificationView, /notification\.delete/);
  assert.match(page, /notification\.mutationError/);
});

test('mobile opens notification context only after its read acknowledgement', () => {
  assert.match(page, /await mutateNotification\(\{ kind: 'read', notificationId: notification\.id \}, notification\)/);
  assert.match(page, /openNotificationContext\(notification\)/);
});

test('notifications are a dedicated native destination rather than embedded on Home', () => {
  const home = page.slice(
    page.indexOf("destination?.kind === 'home' ? <ScrollView"),
    page.indexOf('{selectedPath ? <NativeRouteSource'),
  );
  assert.doesNotMatch(home, /notification\.actionableHeading|pathInvitation\.pendingHeading/);
  assert.match(page, /router\.push\('\/notifications'\)/);
  assert.match(page, /<NotificationRouteSource/);
  assert.match(notificationRoute, /useNotificationRoutePresentation/);
  assert.match(notificationRoute, /<NativeRouteScreen[\s\S]*grouped[\s\S]*onRefresh=\{presentation\.refresh\}/);
  assert.match(rootLayout, /headerRight:[\s\S]*NativeHeaderButton/);
  assert.match(rootLayout, /notification\.markAllRead/);
  assert.match(notificationRoute, /pathInvitation\.pendingHeading/);
  assert.match(page, /prepareNotificationRoute\(\)[\s\S]*router\.push\('\/notifications'\)/);
});

test('notification screen uses compact native flat rows and native empty state', () => {
  assert.doesNotMatch(notificationView, /<SettingsSection/);
  assert.match(notificationView, /accessibilityRole="header"/);
  assert.match(notificationView, /<NativeContentUnavailable/);
  assert.match(notificationView, /<NativeActionMenu/);
  assert.match(notificationView, /accessibilityState=\{\{ busy, disabled: !onOpen \|\| busy \}\}/);
  assert.match(notificationView, /notification\.rowAccessibility/);
  assert.match(notificationView, /item\.read \? 'notification\.read' : 'notification\.unread'/);
});

test('notification and invitation routes expose explicit recovery instead of staying blank', () => {
  assert.match(notificationRoute, /<NotificationJourneyRecoveryView/);
  assert.doesNotMatch(notificationRoute, /return null/);
  assert.match(page, /<NotificationJourneyRecoverySource/);
});

test('notification mutations preserve concurrently refreshed history', () => {
  assert.match(page, /setNotificationHistory\(\(current\) => \{[\s\S]*applyNotificationMutation\(current, mutation, envelope\.data\)[\s\S]*return next\.history/);
});
