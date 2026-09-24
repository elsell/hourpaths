import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../routes/+page.svelte', import.meta.url)), 'utf8');
const access = readFileSync(fileURLToPath(new URL('./path-member-access.svelte', import.meta.url)), 'utf8');
const comparison = readFileSync(fileURLToPath(new URL('./path-member-comparison.svelte', import.meta.url)), 'utf8');

test('web uses a dedicated Path-member access destination and generated contracts', () => {
  assert.match(page, /i18n\.t\('pathMembers\.heading'\)/);
  assert.match(page, /<PathMemberAccess/);
  assert.match(page, /\.pathMembers\(pathID, cursor \|\| undefined\)/);
  assert.match(page, /\.reviewPathMemberRemoval\(pathID, member\.userId\)/);
  assert.match(page, /pathMemberRemovalOperations\.submit\(review, true/);
  assert.match(page, /\.removePathMember\(requestedPathID, userID, body, idempotencyKey\)/);
  assert.match(page, /applyPathMemberRemovalResult/);
});

test('web review presents exact participant consequences and supporter access-only copy', () => {
  assert.match(access, /pathMembers\.participantActivityWarning/);
  assert.match(access, /pathMembers\.participantSocialWarning/);
  assert.match(access, /pathMembers\.offlineWarning/);
  assert.match(access, /pathMembers\.timerLabel[\s\S]*review\.runningTimer \? 'pathMembers\.timerRunning' : 'pathMembers\.timerNotRunning'/);
  assert.match(access, /pathMembers\.runningTimerWarning/);
  assert.match(access, /pathMembers\.reinviteWarning/);
  assert.match(access, /pathMembers\.removeParticipantAndData/);
  assert.match(access, /pathMembers\.supporterWarning/);
  assert.match(access, /pathMembers\.removeSupporter/);
  assert.doesNotMatch(access, /dialog|modal/i);
  assert.match(page, /result\.kind === 'applied' && pathMembersOpen/);
});

test('archived roster remains readable and server capabilities remove mutation affordances', () => {
  assert.match(access, /selected\.canRemove/);
  assert.match(access, /\{#if archived\}<p class="archived-note">/);
});

test('web offers compact role selection with an explicit destructive downgrade confirmation', () => {
  assert.match(access, /<select[\s\S]*pathMembers\.changeRole/);
  assert.match(access, /selected\.canChangeRole/);
  assert.match(access, /pathMembers\.roleChangeWarning/);
  assert.match(access, /pathMembers\.confirmSupporter/);
  assert.match(access, /roleChangeBusy/);
  assert.match(page, /\.changePathMemberRole\(/);
  assert.match(page, /createPathMemberRoleChangeOperationOwner/);
  assert.match(page, /reviewPathMemberRoleChange\(roleChangeReviewSource\(member, pathID\), role\)/);
  assert.match(page, /applyPathMemberRoleChangeResult/);
  assert.match(page, /result\.receipt\.activityDeleted[\s\S]*pathMemberActivities = \[\]/);
});

test('web offers explicit administrator grant, revoke, and self-step-down confirmations from People detail', () => {
  assert.match(access, /selected\.canGrantAdministrator[\s\S]*pathMembers\.confirmGrantAdministrator/);
  assert.match(access, /selected\.canRevokeAdministrator[\s\S]*pathMembers\.confirmRevokeAdministrator/);
  assert.match(access, /selected\.canStepDownAdministrator[\s\S]*pathMembers\.confirmStepDownAdministrator/);
  assert.match(access, /pathMembers\.grantAdministratorWarning/);
  assert.match(access, /pathMembers\.revokeAdministratorWarning/);
  assert.match(access, /pathMembers\.stepDownAdministratorWarning/);
  assert.match(page, /reviewPathMemberRoleChange\(roleChangeReviewSource\(member, pathID\), role\)/);
  assert.match(page, /canGrantAdministrator: false[\s\S]*openPathMembers\('', true, false\)/);
});

test('web People detail presents server-authored participant goal progress', () => {
  assert.match(access, /selected\.intervalProgress/);
  assert.match(access, /selected\.overallProgress/);
  assert.match(access, /pathMembers\.intervalProgressLabel/);
  assert.match(access, /pathMembers\.overallProgressLabel/);
  assert.match(access, /<progress/);
  assert.match(access, /class="progress-comparison"/);
});

test('web Path Details loads and presents the paged participant comparison', () => {
  assert.match(page, /openPathMembers\('', true, false\)/);
  assert.match(page, /<PathMemberComparison/);
  assert.match(comparison, /id="path-participant-comparison-heading"/);
  assert.match(comparison, /pathMembers\.intervalProgressLabel/);
  assert.match(comparison, /pathMembers\.overallProgressLabel/);
  assert.match(comparison, /members\.filter\(\(member\) => member\.role !== 'supporter'\)/);
  assert.match(page, /openPathMembers\(pathMembersNextCursor, false, false\)/);
});
