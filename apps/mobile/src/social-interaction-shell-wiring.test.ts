import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const source = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('same-owner credential rotation retains comment presentation and target lineage', () => {
  const rotation = source.slice(
    source.indexOf('const deepSocialTargets = ['),
    source.indexOf('setSessionRenewable(canRenew)', source.indexOf('const deepSocialTargets = [')),
  );
  assert.match(rotation, /rotateSocialSessionTarget\(practiceCommentTarget\.current, ownerID, credential\)/);
  assert.match(rotation, /rotateSocialSessionTarget\(practiceCommentHistoryTarget\.current, ownerID, credential\)/);
  assert.match(rotation, /rotateSocialSessionTarget\(practiceCommentHeartRosterTarget\.current, ownerID, credential\)/);
  assert.doesNotMatch(rotation, /setPracticeComments\(null\)/);
  assert.doesNotMatch(rotation, /practiceCommentPage\.current = \{ items: \[\], nextCursor: '' \}/);
});

test('reaction and comment mutations synchronously reject duplicate admission', () => {
  assert.match(source, /socialFeedReactionAdmissions\.has\(event\.id\)/);
  assert.match(source, /socialFeedReactionAdmissions\.set\(event\.id, admission\)/);
  assert.match(source, /socialFeedReactionAdmissions\.get\(event\.id\) === admission/);
  assert.match(source, /practiceCommentCreateAdmission\.current\) throw new Error\('comment_create_in_progress'\)/);
  assert.match(source, /practiceCommentCreateAdmission\.current = admission/);
  assert.match(source, /practiceCommentCreateAdmission\.current === admission/);
  assert.match(source, /practiceCommentMutationAdmissions\.has\(retryScope\)/);
  assert.match(source, /practiceCommentMutationAdmissions\.set\(retryScope, admission\)/);
});

test('late create failure after replacement cannot ask a disposed view to restore its draft', () => {
  const create = source.slice(
    source.indexOf('async function createPracticeComment'),
    source.indexOf('async function editPracticeComment'),
  );
  assert.match(create, /catch \(cause\) \{[\s\S]*if \(!ticket\.current\(\) \|\| !currentPracticeCommentTarget\(target\)\) return;/);
  assert.match(source, /resetSocialProfileDiscovery\(\) \{[\s\S]*clearAllPracticeCommentDrafts\(\)/);
});

test('retryable interaction mutations retain their frozen idempotency key', () => {
  assert.match(source, /previousRetry\?\.reaction === reaction[\s\S]*previousRetry\.key[\s\S]*Crypto\.randomUUID\(\)/);
  assert.match(source, /practiceCommentMutationKey\(retryScope, text\)/);
  assert.match(source, /practiceCommentMutationKey\(retryScope, `\$\{comment\.version\}\\u0000\$\{text\}`\)/);
  assert.match(source, /practiceCommentMutationKey\(retryScope, String\(comment\.version\)\)/);
});

test('rotated old-token failures retain the active credential and surface mutation failure', () => {
  const reaction = source.slice(
    source.indexOf('async function mutateSocialFeedReaction'),
    source.indexOf('async function getConfiguredTimeZone'),
  );
  assert.match(reaction, /socialFeedReactionTargets\.set\([\s\S]*createSocialSessionTarget\(ownerID, currentSession, reactionIntentKey\)/);
  assert.match(reaction, /ownsCurrentSocialOperation\([\s\S]*socialFeedReactionTargets\.get\(event\.id\)[\s\S]*reactionIntentKey[\s\S]*currentSession/);
  assert.doesNotMatch(reaction, /ownsCurrentSocialOperation\(\s*socialFeedTarget\.current/);
  assert.match(reaction, /handleSocialOperationFailure\(cause, currentSession, currentTarget\)/);
  assert.match(reaction, /if \(currentTarget\(\)\) throw cause/);
  assert.match(source, /notificationLifecycleState\.current\.session\?\.token !== requestSession\.token\) return/);
});

test('same-owner rotation extends every admitted reaction target independently of feed refreshes', () => {
  assert.match(source, /for \(const \[eventID, target\] of socialFeedReactionTargets\)[\s\S]*rotateSocialSessionTarget\(target, ownerID, credential\)/);
  assert.match(source, /socialFeedReactionTargets\.clear\(\)/);
});

test('cold heart roster maps failed comment resolution to retryable unavailable', () => {
  assert.match(source, /currentSocialRouteIntent\?\.kind === 'comment-hearts'[\s\S]*practiceComments\?\.eventID === currentDeepSocialEvent\.id && practiceComments\.status === 'error'/);
});
