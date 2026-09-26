import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  eligibleProfileSearchQuery,
  profileAccessibilityLabel,
} from './ui/social-profile-presentation';

function source(path: string) {
  const file = fileURLToPath(new URL(path, import.meta.url));
  return existsSync(file) ? readFileSync(file, 'utf8') : '';
}

const tabs = source('../app/(tabs)/_layout.tsx');
const followingLayout = source('../app/(tabs)/following/_layout.tsx') + source('../app/_layout.tsx');
const followingRoute = source('../app/(tabs)/following/index.tsx');
const peopleRoute = source('../app/following/people.tsx');
const rootLayout = source('../app/_layout.tsx');
const profileRoute = source('../app/profile/[username].tsx');
const followRequestsRoute = source('../app/follow-requests.tsx');
const followRequestsView = source('./ui/follow-request-list-view.tsx');
const searchView = source('./ui/social-profile-search-view.tsx');
const profileView = source('./ui/social-profile-detail-view.tsx');
const profileAvatarIOS = source('./ui/social-profile-avatar.ios.tsx');
const profileAvatarFallback = source('./ui/social-profile-avatar.tsx');
const routePresentation = source('./ui/social-profile-route-presentation.tsx');
const homeOrchestration = source('../app/index.tsx');

test('Following is a native primary tab with an independent native stack', () => {
  assert.match(tabs, /NativeTabs\.Trigger name="following"/);
  assert.match(tabs, /i18n\.t\('social\.following'\)/);
  assert.match(tabs, /person\.2/);
  assert.match(followingLayout, /<Stack/);
  assert.match(followingLayout, /headerLargeTitle: true/);
  assert.match(followingLayout, /name="people"/);
  assert.match(peopleRoute, /headerSearchBarOptions/);
  assert.match(peopleRoute, /i18n\.t\('social\.searchPlaceholder'\)/);
  assert.doesNotMatch(followingLayout, /headerStyle|navigationBarColor/);
});

test('profile discovery requires two visible characters before orchestration', () => {
  assert.equal(eligibleProfileSearchQuery(''), undefined);
  assert.equal(eligibleProfileSearchQuery('  a  '), undefined);
  assert.equal(eligibleProfileSearchQuery('  ab  '), 'ab');
  assert.equal(eligibleProfileSearchQuery('\u00e9x'), '\u00e9x');
});

test('search uses compact accessible identity rows and explicit native states', () => {
  assert.match(searchView, /accessibilityRole="button"/);
  assert.match(searchView, /profileAccessibilityLabel/);
  assert.match(searchView, /<SocialProfileAvatar/);
  assert.match(searchView, /styles\.row/);
  assert.match(searchView, /NativeContentUnavailable/);
  assert.match(searchView, /ActivityIndicator/);
  assert.match(searchView, /social\.searchEmptyHeading/);
  assert.match(searchView, /social\.searchUnavailableHeading/);
  assert.match(searchView, /social\.loadMore/);
  assert.match(searchView, /accessibilityRole="alert"/);
  assert.match(searchView, /RefreshControl/);
  assert.doesNotMatch(searchView, /onFollow|followProfile|unfollow|followButton/i);
});

test('result opens a dedicated root-stack profile destination above tabs', () => {
  assert.match(rootLayout, /name="profile\/\[username\]"/);
  assert.match(rootLayout, /i18n\.t\('social\.profileHeading'\)/);
  assert.match(peopleRoute, /pathname: '\/profile\/\[username\]'/);
  assert.match(profileRoute, /useLocalSearchParams/);
  assert.match(profileRoute, /<SocialProfileDetailView/);
  assert.match(profileRoute, /(?:presentation|social)\.profile\.username === username/);
});

test('dedicated profile renders only approved public identity projection', () => {
  assert.match(profileView, /profile\.displayName/);
  assert.match(profileView, /profile\.username/);
  assert.match(profileView, /profile\.description/);
  assert.match(profileView, /profile\.followerCount/);
  assert.match(profileView, /profile\.followingCount/);
  assert.match(profileView, /<SocialProfileAvatar/);
  assert.doesNotMatch(profileView, /profile\.(?:email|provider|privacy|paths?|goals?|activity)/i);
  assert.match(profileView, /<NativePrimaryButton/);
  assert.match(profileView, /profile\.relationship === 'none'/);
  assert.match(profileView, /'cancel-request'/);
  assert.match(profileView, /'unfollow'/);
  assert.match(profileRoute, /Alert\.alert/);
  assert.match(profileRoute, /style: 'destructive'/);
});

test('published route state supports loading, retry, refresh, and pagination without global coupling', () => {
  assert.match(routePresentation, /useSyncExternalStore/);
  assert.match(routePresentation, /SocialProfileRouteSource/);
  assert.match(routePresentation, /searchQuery/);
  assert.match(routePresentation, /loadMore/);
  assert.match(routePresentation, /refreshSearch/);
  assert.match(routePresentation, /retrySearch/);
  assert.match(routePresentation, /loadProfile/);
  assert.match(routePresentation, /refreshProfile/);
  assert.match(routePresentation, /mutateRelationship/);
});

test('follow requests are an independently reachable native destination with compact native decisions', () => {
  assert.match(rootLayout, /name="follow-requests"/);
  assert.match(followingRoute, /router\.push\('\/follow-requests'\)/);
  assert.match(followingRoute, /<FollowingHeaderActions/);
  assert.match(followRequestsRoute, /<NativeRouteScreen/);
  assert.match(followRequestsRoute, /<FollowRequestListView/);
  assert.match(followRequestsView, /<SocialProfileAvatar/);
  assert.match(followRequestsView, /<NativePrimaryButton/);
  assert.match(followRequestsView, /social\.approveRequest/);
  assert.match(followRequestsView, /social\.declineRequest/);
  assert.match(followRequestsView, /NativeContentUnavailable/);
});

test('the published native shell is driven by generated authenticated profile contracts', () => {
  assert.match(homeOrchestration, /createProfileSearchOwner/);
  assert.match(homeOrchestration, /profileSearchPageFromAPI/);
  assert.match(homeOrchestration, /publicProfileFromAPI/);
  assert.match(homeOrchestration, /relationshipMutationResultFromAPI/);
  assert.match(homeOrchestration, /\.searchProfiles\(query, cursor \|\| undefined\)/);
  assert.match(homeOrchestration, /\.profileByUsername\(username\)/);
  assert.match(homeOrchestration, /\.followProfile\(username, idempotencyKey\)/);
  assert.match(homeOrchestration, /\.cancelFollowRequest\(username, idempotencyKey\)/);
  assert.match(homeOrchestration, /\.unfollowProfile\(username, idempotencyKey\)/);
  assert.match(homeOrchestration, /\.followRequests\(cursor \|\| undefined\)/);
  assert.match(homeOrchestration, /\.acceptFollowRequest\(requestID, idempotencyKey\)/);
  assert.match(homeOrchestration, /\.rejectFollowRequest\(requestID, idempotencyKey\)/);
  assert.match(homeOrchestration, /<SocialProfileRouteSource/);
  assert.match(homeOrchestration, /resetSocialProfileDiscovery\(\)/);
});

test('profile accessibility labels identify name and username without invented initials', () => {
  const displayName = ['Alex', 'Rivera'].join(' ');
  const username = ['alex', 'r'].join('.');
  assert.equal(
    profileAccessibilityLabel({ displayName, username }),
    [displayName, `@${username}`].join(', '),
  );
});

test('public profile pictures render through expo-image with a native neutral fallback', () => {
  for (const avatar of [profileAvatarIOS, profileAvatarFallback]) {
    assert.match(avatar, /Image as ExpoImage/);
    assert.match(avatar, /profilePictureURL \?/);
    assert.match(avatar, /source=\{\{ uri: profilePictureURL \}\}/);
    assert.doesNotMatch(avatar, /profile\.(?:email|provider)|initials|charAt|split\(/i);
    assert.doesNotMatch(avatar, /https?:\/\//i);
  }
  assert.match(profileAvatarIOS, /@expo\/ui\/swift-ui/);
  assert.match(profileAvatarIOS, /systemName="person\.crop\.circle\.fill"/);
  assert.match(profileAvatarIOS, /<Host/);
  assert.match(profileAvatarFallback, /styles\.head/);
  assert.match(profileAvatarFallback, /styles\.shoulders/);
});
