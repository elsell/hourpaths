import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

function source(path: string): string {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');
}

const orchestration = source('../app/index.tsx');
const people = source('./ui/path-member-management-view.tsx');
const memberRoute = source('../app/path/[pathID]/members/[userID].tsx');

test('every current Path member can open People independently of management authority', () => {
  assert.match(
    orchestration,
    /label:\s*i18n\.t\('pathMembers\.heading'\)[\s\S]*onPress:\s*\(\)\s*=>\s*void openPathMembers\(selectedPath\.id\)/,
  );
  assert.doesNotMatch(
    orchestration,
    /\(selectedPath\.capabilities\.manageMembers[\s\S]{0,240}label:\s*i18n\.t\('pathMembers\.heading'\)/,
  );
  assert.doesNotMatch(
    orchestration,
    /function openPathMembers[\s\S]{0,500}capabilities\.manageMembers/,
  );
});

test('People remains a compact native list and every row opens a Path-scoped member detail', () => {
  assert.match(people, /member\.displayName/);
  assert.match(people, /@\{member\.username\}/);
  assert.match(people, /roleLabels\[member\.role\]/);
  assert.match(people, /minHeight:\s*(?:60|mobileTheme\.sizes\.minimumTouchTarget)/);
  assert.match(people, /onOpen\(member\)/);
  assert.doesNotMatch(people, /if \(protectedRole\) return <View/);
  assert.match(memberRoute, /useLocalSearchParams<\{ pathID: string; userID: string \}>/);
  assert.match(memberRoute, /NativeRouteScreen/);
  assert.doesNotMatch(memberRoute + people, /\/profile\/|profile\/[\[]username|loadProfile|SocialProfile/);
});

test('Path member detail exposes only complete, server-authorized shared-Path actions', () => {
  assert.match(people, /blockedByViewer/);
  assert.match(people, /blocking\.unblock/);
  assert.match(people, /onUnblock/);
  assert.match(people, /canRemove/);
  assert.match(people, /onRemove/);
  assert.doesNotMatch(people, /onLeave|pathLeave\.action/);
  assert.doesNotMatch(people, /blocksViewer|blockedByTarget/);
});

test('shared-Path actions reuse authoritative unblock and removal operations with separate feedback', () => {
  assert.match(orchestration, /userBlockingPort\.unblockUser\(member\.userId,\s*Crypto\.randomUUID\(\)\)/);
  assert.match(orchestration, /removeSelectedPathMember/);
  assert.match(orchestration, /setPathMemberUnblockBusy\(true\)/);
  assert.match(orchestration, /setPathMemberUnblockErrorKey\('blocking\.unblockUnavailable'\)/);
  assert.match(orchestration, /blockedByViewer[\s\S]*unblock/);
  assert.match(people, /disabled=\{busy \|\| unblockBusy\}/);
  assert.match(orchestration, /dismissible=\{!pathMemberRemovalBusy && !pathMemberUnblockBusy && !pathMemberRoleChangeBusy && !nudgeSendBusy\}/);
  assert.match(memberRoute, /gestureEnabled:\s*presentation\.dismissible/);
  assert.match(memberRoute, /headerBackVisible:\s*presentation\.dismissible/);
  assert.match(memberRoute, /addListener\('beforeRemove'[\s\S]{0,180}presentation\?\.dismissible === false[\s\S]{0,100}preventDefault\(\)/);
  assert.match(orchestration, /allowNativeChildRouteDismissal\(pathMemberRemovalRouteKey\(review\.pathId, review\.userId\)\)[\s\S]{0,80}router\.back\(\)/);
});

test('returning from a participant session releases the hidden history route', () => {
  assert.match(orchestration, /closeActivityDetailRoute[\s\S]{0,1400}pathMembersOpen && selectedPathMember[\s\S]{0,120}setActivityHistoryOpen\(false\)/);
});
