import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const detail = readFileSync(fileURLToPath(new URL('./ui/activity-detail-view.tsx', import.meta.url)), 'utf8');
const detailRoute = readFileSync(fileURLToPath(new URL('../app/path/[pathID]/history/[activityID].tsx', import.meta.url)), 'utf8');

test('owned active activity detail exposes a compact native destructive action', () => {
  assert.match(detail, /!archived && ownsActivity[\s\S]*SettingsSection[\s\S]*SettingsActionRow/);
  assert.match(detail, /label=\{i18n\.t\('pathDetails\.delete'\)\}/);
  assert.match(detail, /disabled=\{deletionBusy \|\| manualBusy\}/);
  assert.match(detail, /deletionErrorText[\s\S]*deletionRetryable[\s\S]*common\.retry/);
});

test('pending deletion prevents native back and gesture dismissal until programmatic success', () => {
  assert.match(page, /dismissible=\{!activityDeletionBusy\}[\s\S]*activityDetailRouteKey/);
  assert.match(detailRoute, /addListener\('beforeRemove'[\s\S]*presentation\?\.dismissible === false[\s\S]*preventDefault\(\)/);
  assert.match(detailRoute, /gestureEnabled:\s*presentation\.dismissible/);
  assert.match(detailRoute, /headerBackVisible:\s*presentation\.dismissible/);
  assert.match(page, /allowNativeChildRouteDismissal\(activityDetailRouteKey\(pathID, activityID\)\)[\s\S]*announceForAccessibility\(i18n\.t\('pathDetails\.deleted'\)\)[\s\S]*router\.back\(\)/);
});

test('only transient deletion failures retain a retry action and all failures use deletion-specific copy', () => {
  assert.match(page, /setActivityDeletionRetryable\(classifySessionFailure\(failure\)\.retryable\)/);
  assert.match(page, /setActivityDeletionErrorKey\(classifySessionFailure\(failure\)\.retryable[\s\S]*pathDetails\.deleteRetryableError[\s\S]*pathDetails\.deleteFailed/);
  assert.match(detail, /deletionRetryable\s*\? <ActionButton/);
});

test('deletion is confirmed natively before the generated client mutation runs', () => {
  const begin = page.slice(page.indexOf('function beginActivityDeletion'), page.indexOf('async function submitActivityDeletion'));
  const submit = page.slice(page.indexOf('async function submitActivityDeletion'), page.indexOf('async function openActivityHistory'));
  assert.match(begin, /presentNativeDestructiveConfirmation\(\{/);
  assert.doesNotMatch(begin, /\.deleteActivity\(/);
  assert.match(submit, /createSessionApiClient\(apiURL,[\s\S]*\.deleteActivity\(pathID, activityID, idempotencyKey\)/);
  assert.match(page, /createActivityDeletionOperationOwner\(\(\) => Crypto\.randomUUID\(\)\)/);
});

test('successful deletion removes exact history state, closes detail, and applies authoritative progress', () => {
  assert.match(page, /applyLoadedActivityDeletionResult\(/);
  assert.match(page, /setActivityHistory\(\[\.\.\.next\.activities\]\)/);
  assert.match(page, /accumulatedSeconds: next\.timer\.accumulatedSeconds/);
  assert.match(page, /intervalProgress: next\.timer\.intervalProgress/);
  assert.match(page, /socialFeedPage\.current = next\.feed/);
  assert.match(page, /practiceCommentPage\.current = next\.comments/);
  assert.match(page, /setNotificationHistory\(next\.notifications\)/);
  assert.match(page, /const nextUnreadCount = next\.notifications\.unreadCount/);
  assert.doesNotMatch(page, /notificationUnreadCount - next\.removedUnreadCount/);
  assert.match(page, /setNativeNotificationBadge\(nextUnreadCount\)/);
  assert.match(page, /setPathMembers\(\[\.\.\.next\.members\]\)/);
  assert.match(page, /socialFeedOperations\.invalidate\(\)[\s\S]*socialFeedPage\.current = next\.feed/);
  assert.match(page, /notificationRefreshOperations\.invalidate\(\)[\s\S]*setNotificationHistory\(next\.notifications\)/);
  assert.match(page, /pathMemberListOperations\.invalidate\(\)[\s\S]*pathMemberActivityOperations\.invalidate\(\)[\s\S]*pathDetailOperations\.invalidate\(\)[\s\S]*setActivityHistory/);
  assert.match(page, /practiceCommentMutationOperations\.clear\(\)[\s\S]*practiceCommentPage\.current = next\.comments/);
  assert.match(page, /new Set\(next\.removedFeedEventIds\)[\s\S]*for \(const eventID of removedFeedEventIDs\)/);
  assert.match(page, /removedFeedEventIDs\.has\(practiceCommentTarget\.current\.eventID\)/);
  assert.match(page, /removedFeedEventIDs\.has\(practiceCommentHistoryTarget\.current\.eventID\)/);
  assert.match(page, /removedFeedEventIDs\.has\(practiceCommentHeartRosterTarget\.current\.eventID\)/);
  assert.match(page, /removedFeedEventIDs\.has\(socialFeedActivity\.event\.id\)/);
  assert.match(page, /router\.back\(\)/);
  assert.match(page, /onDismiss=\{\(\) => closeActivityDetailRoute\(activeActivityID\)\}/);
});

test('delayed deletion completion projects from the latest unrelated activity, member, and notification rows', () => {
  assert.match(page, /\[activityHistory, setActivityHistory, activityHistoryRef\] = useLatestState/);
  assert.match(page, /\[pathMembers, setPathMembers, pathMembersRef\] = useLatestState/);
  assert.match(page, /\[pathMemberActivities, setPathMemberActivities, pathMemberActivitiesRef\] = useLatestState/);
  assert.match(page, /\[selectedPathMember, setSelectedPathMember, selectedPathMemberRef\] = useLatestState/);
  assert.match(page, /\[notificationHistory, setNotificationHistory, notificationHistoryRef\][\s\S]*useLatestState/);
  assert.match(page, /activities: activityHistoryRef\.current/);
  assert.match(page, /members: pathMembersRef\.current/);
  assert.match(page, /notifications: notificationHistoryRef\.current/);
  assert.match(page, /pathMemberActivities: pathMemberActivitiesRef\.current/);
  assert.match(page, /selectedMember: selectedPathMemberRef\.current/);
  assert.match(page, /latestDestination = notificationLifecycleState\.current\.destination[\s\S]*timer = latestDestination\.profile\.timers\[pathID\]/);
});
