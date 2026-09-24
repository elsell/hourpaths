import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const appSource = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const profileSource = readFileSync(fileURLToPath(new URL('../app/profile/[username].tsx', import.meta.url)), 'utf8');
const blockingSource = readFileSync(fileURLToPath(new URL('./ui/user-blocking-route-presentation.tsx', import.meta.url)), 'utf8');
const profilePresentationSource = readFileSync(fileURLToPath(new URL('./ui/social-profile-route-presentation.tsx', import.meta.url)), 'utf8');

test('the authenticated shell publishes visible recovery for every admitted social route', () => {
  assert.match(appSource, /currentSocialRouteIntent \? <SocialRouteRecoverySource/);
  assert.match(appSource, /target=\{currentSocialRouteIntent\}/);
  assert.match(appSource, /onGoFollowing=\{\(\) => router\.replace\('\/\(tabs\)\/following'\)\}/);
  assert.match(appSource, /onGoHome=\{\(\) => router\.replace\('\/\(tabs\)\/home'\)\}/);
  assert.match(appSource, /case 'profile':[\s\S]*loadSocialProfile\(currentSocialRouteIntent\.username, true\)/);
});

test('social operations rotate only within the same owner and keep independent targets', () => {
  for (const target of [
    'socialProfileSearchTarget',
    'socialProfileDetailTarget',
    'socialFollowRequestTarget',
    'socialActiveFollowingTarget',
    'socialFeedTarget',
    'socialDeepEventTarget',
  ]) {
    assert.match(appSource, new RegExp(`\\b${target},`));
  }
  assert.match(appSource, /target\.current = rotateSocialSessionTarget\(target\.current, ownerID, credential\)/);
  assert.match(appSource, /handleSocialOperationFailure\([\s\S]*notificationLifecycleState\.current\.session\?\.token !== requestSession\.token/);
  assert.match(appSource, /const deepSocialTargets = \[[\s\S]*practiceCommentTarget[\s\S]*practiceCommentHeartRosterTarget[\s\S]*socialFeedActivityTarget/);
  assert.match(appSource, /rotateSocialSessionTarget\(practiceCommentTarget\.current, ownerID, credential\)/);
  assert.match(appSource, /const latest = notificationLifecycleState\.current;[\s\S]*socialFollowRequestSubmitting\.current/);
});

test('deep comment routes resolve their exact event and rotations preserve drafts while retaining lineage', () => {
  assert.match(appSource, /async function loadSocialDeepEvent\(eventID: string\)/);
  assert.match(appSource, /\.getPracticeFeedEvent\(eventID\)/);
  assert.match(appSource, /currentSocialRouteIntent\.kind === 'comments' \|\| currentSocialRouteIntent\.kind === 'comment-hearts'[\s\S]*loadSocialDeepEvent\(currentSocialRouteIntent\.eventID\)/);
  assert.match(appSource, /rotateSocialSessionTarget\(practiceCommentTarget\.current, ownerID, credential\)/);
  assert.match(appSource, /clearAllPracticeCommentDrafts\(\)/);
  assert.doesNotMatch(appSource, /case 'comments':[\s\S]{0,500}void loadSocialFeed\('', true\)/);
});

test('delayed profile confirmations retain their admitted credential and target', () => {
  assert.match(appSource, /const admittedToken = session\?\.token \?\? null/);
  assert.doesNotMatch(appSource, /userBlockingToken\.current/);
  assert.match(blockingSource, /if \(!isCurrent\(\)\) throw new Error\('blocking_superseded'\)/);
  assert.match(blockingSource, /createOwnedUserBlockingPort\(\(\) => portRef\.current/);
  assert.match(blockingSource, /const result = await owned\(\(\) => currentPort\(\)\.blockUser/);
  assert.match(blockingSource, /if \(!isCurrent\(\)\) throw new Error\('blocking_superseded'\);[\s\S]*onBlocked\(result\.target\)/);
  assert.match(profileSource, /const ownedPresentation = presentation/);
  assert.match(profileSource, /completeBlock\([\s\S]*ownedPresentation/);
  assert.match(profileSource, /canBlockCurrentSocialProfile\(expectedUsername\)/);
  assert.match(profileSource, /ownedPresentation\.blockUser\(expectedUsername, idempotencyKey, acknowledgement\)/);
  assert.match(profilePresentationSource, /currentPresentation\?\.sessionKey === sessionKey/);
  assert.match(profilePresentationSource, /props\.isCurrent\(\)/);
  assert.match(appSource, /sessionKey=\{socialPresentationKey\}/);
  assert.match(appSource, /socialPresentationGeneration\.current \+= 1/);
});
