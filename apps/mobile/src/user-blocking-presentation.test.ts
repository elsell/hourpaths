import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

function source(path: string) {
  const file = fileURLToPath(new URL(path, import.meta.url));
  return existsSync(file) ? readFileSync(file, 'utf8') : '';
}

const rootLayout = source('../app/_layout.tsx');
const profileRoute = source('../app/profile/[username].tsx');
const profileView = source('./ui/social-profile-detail-view.tsx');
const settingsRoute = source('../app/settings/index.tsx');
const blockedRoute = source('../app/settings/blocked-accounts.tsx');
const blockedView = source('./ui/blocked-account-list-view.tsx');
const presentation = source('./ui/user-blocking-route-presentation.tsx');
const port = source('./user-blocking-port.ts');
const orchestration = source('../app/index.tsx');

test('profile exposes an accessible native destructive overflow Block action', () => {
  assert.match(profileRoute, /<PathHeaderMenu/);
  assert.match(profileRoute, /blocking\.blockAction/);
  assert.match(profileRoute, /destructive: true/);
  assert.match(profileRoute, /systemImage: 'hand\.raised'/);
  assert.match(profileRoute, /profile\.profile\?\.relationship !== 'self'/);
});

test('block preflight communicates progress and separates shared-Path visibility from leaving', () => {
  assert.match(profileRoute, /const ownedPresentation = presentation/);
  assert.match(profileRoute, /await ownedPresentation\.reviewBlock\(username\)/);
  assert.match(profileView, /blockingStatus/);
  assert.match(profileView, /accessibilityRole="progressbar"/);
  assert.match(profileRoute, /blocking\.sharedPathsWarning/);
  assert.match(profileRoute, /blocking\.leavePathsSeparately/);
  assert.match(profileRoute, /style: 'destructive'/);
  assert.match(profileRoute, /const idempotencyKey = Crypto\.randomUUID\(\)/);
  assert.match(profileRoute, /await ownedPresentation\.blockUser\(expectedUsername, idempotencyKey, acknowledgement\)/);
  assert.match(profileRoute, /canBlockCurrentSocialProfile\(expectedUsername\)/);
  assert.match(profileRoute, /blockSubmitting\.current/);
  assert.match(profileRoute, /router\.replace\('\/\(tabs\)\/following'\)/);
});

test('Settings opens a dedicated native-stack Blocked Accounts destination', () => {
  assert.match(settingsRoute, /settings\/blocked-accounts/);
  assert.match(settingsRoute, /blocking\.settingsHeading/);
  assert.match(rootLayout, /name="settings\/blocked-accounts"/);
  assert.match(blockedRoute, /<BlockedAccountListView/);
  assert.match(blockedRoute, /Alert\.alert/);
  assert.match(blockedRoute, /ownedPresentation\.unblockUser/);
});

test('blocked accounts use compact accessible identity rows and complete collection states', () => {
  assert.match(blockedView, /<SocialProfileAvatar/);
  assert.match(blockedView, /profileAccessibilityLabel/);
  assert.match(blockedView, /<NativeSheetAction/);
  assert.match(blockedView, /NativeContentUnavailable/);
  assert.match(blockedView, /ActivityIndicator/);
  assert.match(blockedView, /RefreshControl/);
  assert.match(blockedView, /blocking\.emptyHeading/);
  assert.match(blockedView, /blocking\.unavailableHeading/);
  assert.match(blockedView, /blocking\.loadMore/);
  assert.match(blockedView, /accessibilityRole="alert"/);
});

test('typed port owns the interim authenticated transport boundary without editing generated clients', () => {
  assert.match(port, /satisfies UserBlockingPort/);
  assert.match(port, /reviewProfileBlock\(username\)/);
  assert.match(port, /blockProfile\(username, idempotencyKey, acknowledgement\)/);
  assert.match(port, /blockedAccounts\(cursor\)/);
  assert.match(port, /unblockAccount\(userId, idempotencyKey\)/);
  assert.doesNotMatch(port, /\bfetch\b|Authorization|\/v1\//);
  assert.match(orchestration, /<UserBlockingRouteSource/);
  assert.match(orchestration, /hideBlockedUserFromSocialSurfaces/);
  assert.match(presentation, /onBlocked/);
  assert.match(presentation, /useSyncExternalStore/);
});
