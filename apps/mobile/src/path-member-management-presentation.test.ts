import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const view = readFileSync(fileURLToPath(new URL('./ui/path-member-management-view.tsx', import.meta.url)), 'utf8');
const childRoute = readFileSync(fileURLToPath(new URL('./ui/native-child-route-presentation.tsx', import.meta.url)), 'utf8');
const membersRoute = readFileSync(fileURLToPath(new URL('../app/path/[pathID]/members/index.tsx', import.meta.url)), 'utf8');
const removalRoute = readFileSync(fileURLToPath(new URL('../app/path/[pathID]/members/[userID].tsx', import.meta.url)), 'utf8');
const memberPage = readFileSync(fileURLToPath(new URL('./path-member-page.ts', import.meta.url)), 'utf8');
const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('member list and destructive review are distinct native stack routes', () => {
  assert.match(childRoute, /pathMembersRouteKey/);
  assert.match(childRoute, /pathMemberRemovalRouteKey/);
  assert.match(membersRoute, /NativeRouteScreen/);
  assert.match(removalRoute, /NativeRouteScreen/);
  assert.doesNotMatch(membersRoute + removalRoute, /Modal|NativeSheet|Alert/);
  assert.doesNotMatch(membersRoute + removalRoute, /return null/);
  assert.match(membersRoute, /usePathRouteAncestry\(\{ kind: 'members'/);
  assert.match(removalRoute, /usePathRouteAncestry\(\{ kind: 'member'/);
  assert.match(membersRoute + removalRoute, /NativeRouteRecoveryView/);
});

test('cold People and member routes recover authoritative state without a blank screen', () => {
  assert.match(page, /intent\.kind === 'members'[\s\S]*openPathMembers\(intent\.pathID, undefined, false, true\)/);
  assert.match(page, /intent\.kind === 'member'[\s\S]*recoverPathMember\(intent\.pathID, intent\.userID\)/);
  assert.match(page, /async function recoverPathMember[\s\S]*seenCursors[\s\S]*inspectPathMember\(member, false\)/);
  assert.match(page, /currentPathRouteIntent\?\.kind === 'member'[\s\S]*NativeRouteRecoveryView/);
  assert.match(page, /rotatePathMemberTarget/);
  assert.match(page, /ownsActivePathMember/);
});

test('member management is a compact refreshable native list with complete states', () => {
  assert.match(childRoute, /onRefresh/);
  assert.match(childRoute, /refreshing/);
  assert.match(childRoute, /grouped/);
  assert.match(view, /NativeContentUnavailable/);
  assert.match(view, /pathMembers\.unavailableHeading/);
  assert.match(view, /pathMembers\.emptyHeading/);
  assert.match(view, /accessibilityRole="button"/);
  assert.match(view, /onOpen\(member\)/);
  assert.match(view, /roleLabels\[member\.role\]/);
});

test('participant removal review is a dedicated, readable destructive surface', () => {
  assert.match(view, /export function PathMemberRemovalReviewView/);
  assert.match(view, /pathMembers\.participantActivityWarning/);
  assert.match(view, /pathMembers\.participantSocialWarning/);
  assert.match(view, /pathMembers\.offlineWarning/);
  assert.match(view, /pathMembers\.timerLabel[\s\S]*review\.runningTimer \? 'pathMembers\.timerRunning' : 'pathMembers\.timerNotRunning'/);
  assert.match(view, /pathMembers\.runningTimerWarning/);
  assert.match(view, /pathMembers\.reinviteWarning/);
  assert.match(view, /pathMembers\.removeParticipantAndData/);
  assert.match(view, /pathMembers\.supporterWarning/);
  assert.match(view, /pathMembers\.removeSupporter/);
  assert.doesNotMatch(view, /Modal|NativeSheet|Alert/);
});

test('ordinary member role changes use a compact native picker and explicit destructive confirmation', () => {
  assert.match(view, /<NativeChoicePicker/);
  assert.match(view, /pathMembers\.changeRole/);
  assert.match(view, /pathMembers\.roleParticipantEffect/);
  assert.match(view, /pathMembers\.roleSupporterEffect/);
  assert.match(view, /pathMembers\.roleChangeWarning/);
  assert.match(view, /pathMembers\.confirmSupporter/);
  assert.match(view, /member\.canChangeRole/);
  assert.match(page, /\.changePathMemberRole\(/);
  assert.match(page, /createPathMemberRoleChangeOperationOwner/);
  assert.match(page, /reviewPathMemberRoleChange\(pathMemberRemovalReview \?\?/);
  assert.match(page, /applyPathMemberRoleChangeResult/);
  assert.match(page, /result\.receipt\.activityDeleted[\s\S]*setPathMemberActivities\(\[\]\)/);
  assert.match(page, /dismissible=\{!pathMemberRemovalBusy && !pathMemberUnblockBusy && !pathMemberRoleChangeBusy && !nudgeSendBusy\}/);
  assert.match(page, /allowNativeChildRouteDismissal\(pathMemberRemovalRouteKey/);
});

test('administrator lifecycle uses server-authored capabilities and deliberate compact native confirmations', () => {
  assert.match(view, /member\.canGrantAdministrator/);
  assert.match(view, /member\.canRevokeAdministrator/);
  assert.match(view, /member\.canStepDownAdministrator/);
  assert.match(view, /pathMembers\.confirmGrantAdministrator/);
  assert.match(view, /pathMembers\.confirmRevokeAdministrator/);
  assert.match(view, /pathMembers\.confirmStepDownAdministrator/);
  assert.match(view, /pendingRole === 'administrator'/);
  assert.match(view, /pendingRole === 'participant'/);
  assert.match(view, /onConfirmRoleChange/);
  assert.match(view, /onCancelRoleChange/);
  assert.match(memberPage, /typeof member\.canGrantAdministrator !== 'boolean'/);
  assert.match(memberPage, /typeof member\.canRevokeAdministrator !== 'boolean'/);
  assert.match(memberPage, /typeof member\.canStepDownAdministrator !== 'boolean'/);
  assert.match(page, /role === 'administrator'/);
  assert.match(page, /selectedPathMember\.canStepDownAdministrator/);
  assert.match(page, /result\.receipt\.role === 'administrator'/);
  assert.match(page, /await openPathMembers\(member\.pathId, undefined, false\)/);
  assert.match(page, /inspectPathMember\(refreshedMember, false\)/);
});

test('server-authored capability wires roster, review, removal, and exact reconciliation', () => {
  assert.match(page, /label: i18n\.t\('pathMembers\.heading'\)/);
  assert.match(page, /\.pathMembers\(pathID, cursor\)/);
  assert.match(page, /data: result\.data \? \{ data: pathMemberPageFromAPI\(pathID, ownerID, result\.data\) \} : undefined/);
  assert.match(page, /\.reviewPathMemberRemoval\(member\.pathId, member\.userId\)/);
  assert.match(page, /pathMemberRemovalOperations\.submit\(review, true/);
  assert.match(page, /\.removePathMember\(pathID, userID, body, idempotencyKey\)/);
  assert.match(page, /applyPathMemberRemovalResult/);
  assert.match(page, /!ownsActivePathMember\(review\.pathId, review\.userId\)/);
  assert.match(page, /result\.kind === 'applied' && currentTarget\?\.ownerID === ownerID && currentTarget\.pathID === review\.pathId/);
  assert.match(page, /member\.canRemove/);
  assert.match(page, /PathMemberRemovalReviewView/);
});

test('People presents each participant’s server-authored interval and overall goal progress', () => {
  assert.match(memberPage, /intervalProgress:\s*pathMemberProgressFromAPI\(typed\.intervalProgress\)/);
  assert.match(memberPage, /overallProgress:\s*pathMemberProgressFromAPI\(typed\.overallProgress\)/);
  assert.match(view, /member\.intervalProgress\s*\?[\s\S]*?<PathMemberProgressRow/);
  assert.match(view, /member\.overallProgress\s*\?[\s\S]*?<PathMemberProgressRow/);
  assert.match(view, /function MemberRow[\s\S]*member\.intervalProgress[\s\S]*member\.overallProgress/);
  assert.match(view, /pathMembers\.intervalProgressLabel/);
  assert.match(view, /pathMembers\.overallProgressLabel/);
  assert.match(view, /<ProgressIndicator/);
});

test('Path Details loads the participant comparison and puts it first for supporters', () => {
  assert.match(page, /openPathMembers\(pathID, undefined, false\)/);
  assert.match(page, /participantComparison=\{<PathMemberManagementView/);
  assert.match(page, /comparisonFirst=\{!selectedCapabilities\.trackTime\}/);
  assert.match(page, /items: pathMembers\.filter\(\(member\) => member\.role !== 'supporter'\)/);
  assert.match(page, /onLoadMore=\{\(\) => \{ if \(pathMembersCursor\) void openPathMembers\(selectedPath\.id, pathMembersCursor, false\); \}\}/);
});
