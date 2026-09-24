import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const share = readFileSync(
  fileURLToPath(new URL('./ui/path-share-sheet.tsx', import.meta.url)),
  'utf8',
);

function sourceSection(start: string, end: string): string {
  const startIndex = page.indexOf(start);
  assert.notEqual(startIndex, -1, `missing source section ${start}`);
  const endIndex = page.indexOf(end, startIndex + start.length);
  assert.notEqual(endIndex, -1, `missing source section terminator ${end}`);
  return page.slice(startIndex, endIndex);
}

test('Share exposes compact native visibility only to visibility managers', () => {
  assert.match(page, /selectedCapabilities\.manageVisibility/);
  assert.match(page, /capabilities\.manageVisibility/);
  assert.match(page, /onVisibilityChange=\{selectedCapabilities\.manageVisibility \? changePathVisibility : undefined\}/);
  assert.match(share, /<NativeChoicePicker/);
  assert.match(share, /pathVisibility\.heading/);
  assert.match(share, /pathVisibility\.option\.\$\{props\.effectiveVisibility\}/);
  assert.match(share, /visibilityOptions/);
  assert.match(share, /disabled=\{props\.busy\}/);
});

test('visibility choices are constrained by profile privacy and require explicit review', () => {
  const open = sourceSection('function openPathSharing(path: SessionPath)', 'function closePathSharing');
  assert.match(open, /setPathVisibilityDraft\(path\.visibility\)/);
  assert.match(page, /pathVisibilityOptions\(destination\.profile\.profileVisibility\)/);

  const review = sourceSection('function reviewPathVisibility(visibility = pathVisibilityDraft)', 'function cancelPathVisibilityReview()');
  assert.match(review, /reviewPathVisibilityChange\(/);
  assert.match(review, /destination\.profile\.profileVisibility/);
  assert.match(review, /not-permitted/);
  assert.match(review, /presentNativeDestructiveConfirmation\(\{/);
  assert.match(review, /pathVisibility\.confirmation\.transition/);
  assert.match(review, /pathVisibility\.confirmation\.historyExposure/);
  assert.match(review, /pathVisibility\.confirmation\.unchangedScope/);
  assert.doesNotMatch(review, /\.setPathVisibility\(/);

  assert.match(review, /pathVisibility\.confirmation\.narrowingAccess/);
  assert.match(review, /pathVisibility\.confirmation\.narrowingHeading/);
  assert.match(share, /onVisibilityChange/);
});

test('cancel makes no request while confirmation retains its key and serializes by session and Path', () => {
  const cancel = sourceSection('function cancelPathVisibilityReview()', 'async function reloadPathVisibilityConflict(');
  assert.doesNotMatch(cancel, /\.setPathVisibility\(/);
  assert.doesNotMatch(cancel, /setDestination\(/);

  const confirm = sourceSection('async function confirmPathVisibility(', 'function closePathManagement(dirty = false)');
  assert.match(page, /createPathVisibilityOperationOwner/);
  assert.match(confirm, /\.submit\(review, true/);
  assert.match(confirm, /\.setPathVisibility\(requestedPathID, body, idempotencyKey\)/);
  assert.match(page, /pathVisibilityConfirmationTarget\.current = confirmationTarget/);
  assert.match(confirm, /pathVisibilityConfirmationTarget\.current !== admittedTarget/);
  assert.match(confirm, /pathVisibilityTarget\.current = admittedTarget/);
  assert.match(confirm, /ownsPathAdministrationTarget\(pathVisibilityTarget\.current, ownerID, pathID, currentSession\.token\)/);
  assert.match(confirm, /result\.kind === 'failed'/);
  assert.match(confirm, /setPathVisibilityErrorKey/);
});

test('success replaces the authoritative Path across active, archived, selected, and management state', () => {
  const confirm = sourceSection('async function confirmPathVisibility(', 'function closePathManagement(dirty = false)');
  assert.match(confirm, /applyPathVisibilityResult(?:<SessionPath>)?\(\{/);
  assert.match(confirm, /activePaths: current\.profile\.paths/);
  assert.match(confirm, /archivedPaths: current\.profile\.archivedPaths/);
  assert.match(confirm, /selectedPath:/);
  assert.match(confirm, /const activePaths: HomePath\[\] = applied\.activePaths\.map/);
  assert.match(confirm, /paths: activePaths/);
  assert.match(confirm, /archivedPaths: applied\.archivedPaths/);
  assert.doesNotMatch(confirm, /setGoalManagementCurrent\(result\.path\)/);
  assert.match(confirm, /setPathVisibilityDraft\(result\.path\.visibility\)/);
  assert.match(confirm, /setPathVisibilityReview\(null\)/);
  assert.match(confirm, /setPathVisibilitySaved\(true\)/);

  const reset = sourceSection('function resetGoalManagement()', 'function resetManualActivity()');
  assert.match(reset, /pathVisibilityOperations\.cancel\(\)/);
  assert.match(reset, /pathVisibilityTarget\.current = null/);
});

test('a visibility conflict discards the stale retry and reloads active and archived Paths before another submit', () => {
  const reload = sourceSection('async function reloadPathVisibilityConflict(', 'async function confirmPathVisibility(');
  const confirm = sourceSection('async function confirmPathVisibility(', 'function closePathManagement(dirty = false)');
  const review = sourceSection('function reviewPathVisibility(visibility = pathVisibilityDraft)', 'function cancelPathVisibilityReview()');

  assert.match(confirm, /failure\.kind === 'http' && failure\.status === 409/);
  assert.match(confirm, /pathVisibilityOperations\.cancel\(pathID\)/);
  assert.match(confirm, /pathVisibilityConflict\.current = pathVisibilityTarget\.current/);
  assert.match(confirm, /reloadPathVisibilityConflict\(recoverySession, ownerID, pathID\)/);
  assert.match(reload, /loadMobileHomeProfile\(apiURL, recoverySession\)/);
  assert.match(reload, /currentAdministrationSession\(conflict, pathID\)/);
  assert.match(reload, /profile\.paths\.find/);
  assert.match(reload, /profile\.archivedPaths\.find/);
  assert.match(reload, /setDestination/);
  assert.match(reload, /resetInvitationShare\(\)/);
  assert.match(reload, /setSelectedPathID\(null\)/);
  assert.match(review, /pathVisibilityConflict\.current/);
  assert.match(review, /reloadPathVisibilityConflict/);
});
