import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import { socialRouteTargetKey } from './ui/social-route-recovery-presentation';

function source(path: string) {
  const file = fileURLToPath(new URL(path, import.meta.url));
  return existsSync(file) ? readFileSync(file, 'utf8') : '';
}

const followingLayout = source('../app/(tabs)/following/_layout.tsx');
const followingRoute = source('../app/(tabs)/following/index.tsx');
const peopleRoute = source('../app/(tabs)/following/people.tsx');
const profileRoute = source('../app/profile/[username].tsx');
const requestsRoute = source('../app/follow-requests.tsx');
const activityRoute = source('../app/(tabs)/following/activity/[pathID]/[activityID].tsx');
const commentsRoute = source('../app/(tabs)/following/comments/[eventID].tsx');
const heartsRoute = source('../app/(tabs)/following/comments/[eventID]/hearts/[commentID].tsx');
const feedView = source('./ui/social-feed-view.tsx');
const searchView = source('./ui/social-profile-search-view.tsx');
const profileView = source('./ui/social-profile-detail-view.tsx');
const requestsView = source('./ui/follow-request-list-view.tsx');
const headerIOS = source('./ui/following-header-actions.ios.tsx');
const recoveryView = source('./ui/social-route-recovery-view.tsx');

test('Following uses system navigation material and a compact standard toolbar hierarchy', () => {
  assert.doesNotMatch(followingLayout, /headerStyle|navigationBarColor/);
  assert.match(followingRoute, /<FollowingHeaderActions/);
  assert.doesNotMatch(followingRoute, /headerRight|NativeHeaderButton/);
  assert.match(headerIOS, /<Stack\.Toolbar placement="right">/);
  assert.match(headerIOS, /<Stack\.Toolbar\.Button/);
  assert.equal((headerIOS.match(/<Stack\.Toolbar\.Button/g) ?? []).length, 2);
  assert.match(headerIOS, /person\.2/);
  assert.match(headerIOS, /person\.2\.badge\.plus/);
});

test('social list presentations are flat, separated, and intrinsically scalable', () => {
  for (const view of [feedView, searchView, requestsView]) {
    assert.match(view, /styles\.separator/);
    assert.doesNotMatch(view, /borderRadius:\s*mobileTheme\.radii\.md/);
    assert.doesNotMatch(view, /minHeight:\s*(?:68|76)/);
  }
  assert.doesNotMatch(feedView, /numberOfLines=\{1\}/);
  assert.doesNotMatch(searchView, /numberOfLines=\{1\}/);
  assert.match(requestsView, /flexWrap:\s*'wrap'/);
  assert.match(profileView, /flexWrap:\s*'wrap'/);
  assert.doesNotMatch(profileView, /size=\{112\}/);
  assert.doesNotMatch(profileView, /borderRadius:\s*mobileTheme\.radii\.md/);
});

test('owned social routes render visible recovery instead of null bodies', () => {
  for (const route of [followingRoute, peopleRoute, profileRoute, requestsRoute, activityRoute, commentsRoute, heartsRoute]) {
    assert.match(route, /SocialRouteRecoveryView/);
    assert.doesNotMatch(route, /return null/);
    assert.match(route, /scheduleSocialRouteBootstrap/);
  }
  assert.match(recoveryView, /NativeContentUnavailable/);
  assert.match(recoveryView, /accessibilityRole="progressbar"/);
  assert.match(recoveryView, /onRetry/);
  assert.match(recoveryView, /onGoFollowing/);
  assert.match(recoveryView, /onGoHome/);
});

test('recovery targets distinguish profile identity from shared destinations', () => {
  assert.equal(socialRouteTargetKey({ kind: 'following', routeKey: 'social:following' }), 'social:following');
  assert.equal(socialRouteTargetKey({ kind: 'people', routeKey: 'social:people' }), 'social:people');
  assert.equal(socialRouteTargetKey({ kind: 'follow-requests', routeKey: 'social:follow-requests' }), 'social:follow-requests');
  const username = ['Alex', 'R'].join('.');
  assert.equal(socialRouteTargetKey({ kind: 'profile', routeKey: 'social:profile:alex.r', username }), 'social:profile:alex.r');
});
