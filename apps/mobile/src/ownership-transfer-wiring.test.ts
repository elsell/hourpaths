import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

function sourceSection(start: string, end: string): string {
  const startIndex = page.indexOf(start);
  assert.notEqual(startIndex, -1, `missing source section ${start}`);
  const endIndex = page.indexOf(end, startIndex + start.length);
  assert.notEqual(endIndex, -1, `missing source section terminator ${end}`);
  return page.slice(startIndex, endIndex);
}

test('Path detail exposes server-authoritative ownership transfer and discovers pending transfers without notifications', () => {
  const detail = sourceSection('{selectedPath ? <NativeRouteSource', '{selectedPath && activityHistoryOpen ?');
  assert.match(detail, /selectedCapabilities\.transferOwnership/);
  assert.match(detail, /pendingOwnershipTransfer/);
  assert.match(detail, /<OwnershipTransferSheet/);
  assert.match(detail, /onOpenOwnershipTransfer/);
  assert.match(page, /openOwnershipTransfer\(authoritativePath, true, undefined, true\)/);

  const openPath = sourceSection('function openPathDetail(pathID: string, focusedOwnershipTransferID?: string, navigate = true, loadMembers = true)', 'async function openActivityHistory');
  assert.match(openPath, /openOwnershipTransfer\(path, Boolean\(focusedOwnershipTransferID\), focusedOwnershipTransferID\)/);
  assert.doesNotMatch(openPath, /notification\.id|notificationId|ownershipTransferId/);
});

test('archived Paths keep pending transfer details reachable but suppress every mutation', () => {
  const openPath = sourceSection('function openPathDetail(pathID: string, focusedOwnershipTransferID?: string, navigate = true, loadMembers = true)', 'async function openActivityHistory');
  assert.match(openPath, /currentDestination\.profile\.archivedPaths\.find/);
  const openTransfer = sourceSection('async function openOwnershipTransfer', 'async function refreshAfterOwnershipTransfer');
  assert.doesNotMatch(openTransfer, /path\.archivedAt/);
  const mutation = sourceSection('async function mutatePendingOwnershipTransfer', 'function closeOwnershipTransfer');
  assert.match(mutation, /destination\.profile\.paths\.some\(\(\{ id \}\) => id === pathID\)/);
  const detail = sourceSection('{selectedPath ? <NativeRouteSource', '{selectedPath && activityHistoryOpen ?');
  assert.match(detail, /actionsDisabled=\{Boolean\(selectedPath\.archivedAt\)\}/);
});

test('actionable transfer notifications focus the matching pending sheet while informational notices open Path detail', () => {
  const open = sourceSection('async function openNotificationContext', 'function openPathSharing');
  assert.match(open, /notification\.type === 'path_ownership_transfer_received'/);
  assert.match(open, /openPathDetail\(path\.id, notification\.ownershipTransferId\)/);
  assert.match(open, /notification\.type === 'path_ownership_transfer_received'[\s\S]*return/);
  assert.match(open, /openPathDetail\(path\.id\)/);

  const ownership = sourceSection('async function openOwnershipTransfer', 'async function refreshAfterOwnershipTransfer');
  assert.match(ownership, /!expectedTransferID \|\| loaded\.transfer\.transfer\.id === expectedTransferID/);
});

test('candidate pages are generated-client requests with stable deduplication and stale ownership guards', () => {
  const load = sourceSection('async function loadOwnershipTransferCandidates', 'async function openOwnershipTransfer');
  assert.match(load, /ownershipTransferCandidates\(pathID, cursor \|\| undefined\)/);
  assert.match(load, /ownershipTransferTarget\.current !== target/);
  assert.match(load, /!current\.some\(\(item\) => item\.userID === userID\)/);
  assert.match(load, /result\.data\.meta\.nextCursor/);
});

test('candidate selection obtains and freezes a canonical signed server review', () => {
  const review = sourceSection('async function reviewOwnershipTransferRecipient', 'async function confirmOwnershipTransfer');
  assert.match(review, /reviewOwnershipTransfer\([\s\S]*recipientUserId: recipient\.userID/);
  assert.match(review, /review\.recipient\.displayName/);
  assert.match(review, /review\.recipient\.userId/);
  assert.match(review, /review\.recipient\.username/);
  assert.match(review, /ownershipTransferExpirationPresentation\([\s\S]*review\.reviewedAt,[\s\S]*review\.expiresAt/);
  assert.match(review, /setOwnershipTransferReview\(\{ \.\.\.review, idempotencyKey: Crypto\.randomUUID\(\) \}\)/);
});

test('explicit confirmation submits the frozen reservation and one idempotency key across retries', () => {
  const confirm = sourceSection('async function confirmOwnershipTransfer', 'async function mutatePendingOwnershipTransfer');
  assert.match(confirm, /ownershipTransferSelectedRecipient\?\.userID !== recipient\.userID/);
  assert.match(confirm, /ownershipTransferReview\?\.recipient\.userId !== recipient\.userID/);
  assert.match(confirm, /idempotencyKey = review\.idempotencyKey/);
  assert.match(confirm, /initiateOwnershipTransfer\([\s\S]*reservationToken: review\.reservationToken[\s\S]*idempotencyKey/);
  assert.doesNotMatch(confirm, /Crypto\.randomUUID\(\)/);
});

test('pending accept, decline, and creator cancel refresh authoritative Path and notification state', () => {
  const mutation = sourceSection('async function mutatePendingOwnershipTransfer', 'function closeOwnershipTransfer');
  assert.match(mutation, /pendingOwnershipTransfer\.viewerRole === 'creator'/);
  assert.match(mutation, /acceptOwnershipTransfer\(transferID, idempotencyKey\)/);
  assert.match(mutation, /declineOwnershipTransfer\(transferID, idempotencyKey\)/);
  assert.match(mutation, /cancelOwnershipTransfer\(transferID, idempotencyKey\)/);
  assert.match(mutation, /refreshAfterOwnershipTransfer\(target, ownerID, pathID\)/);

  const refresh = sourceSection('async function refreshAfterOwnershipTransfer', 'async function confirmOwnershipTransfer');
  assert.match(refresh, /loadMobileHomeProfile\(apiURL, currentSession\)/);
  assert.match(refresh, /loadNotificationHistoryPage\(currentSession\)/);
  assert.match(refresh, /currentAdministrationSession\(target, pathID\)/);
  assert.match(refresh, /ownershipTransferTarget\.current !== target/);
  assert.match(refresh, /setDestination\(\(current\) => current\?\.kind === 'home' && current\.profile\.id === ownerID/);
});

test('ownership transfer serializes with same-Path management, timer, manual activity, and archive mutations', () => {
  for (const [start, end] of [
    ['function openPathManagement(path: SessionPath)', 'function updateGoalManagementForm'],
    ['async function confirmPathGoalChanges()', 'function closePathManagement(dirty = false)'],
    ['async function submitPathRename()', 'function reviewPathArchive'],
    ['function reviewPathArchive(path: SessionPath, fromManagement = false)', 'async function confirmPathArchiveChange'],
    ['async function confirmPathArchiveChange()', 'function cancelPathArchiveChange'],
    ['async function toggleTimer(pathID: string)', 'function openPathDetail'],
    ['async function openManualActivity(pathID: string)', 'function changeManualDuration'],
    ['async function submitManualActivity()', 'function closeManualActivity'],
  ] as const) {
    assert.match(sourceSection(start, end), /ownershipTransferPathID/);
  }

  const open = sourceSection('async function openOwnershipTransfer', 'async function refreshAfterOwnershipTransfer');
  assert.match(open, /goalManagementPathID === path\.id/);
  assert.match(open, /manualPathID === path\.id/);
  assert.match(open, /pathArchiveReview\?\.pathId === path\.id/);
  assert.match(open, /pathRenamePathID === path\.id/);
  assert.match(open, /timerBusy\[path\.id\]/);
});

test('review and pending presentation use strictly validated profile-derived time zones for exact and relative expiration', () => {
  const pending = sourceSection('function pendingOwnershipTransferPresentation', 'async function fetchPendingOwnershipTransfer');
  assert.match(pending, /ownershipTransferExpirationPresentation\([\s\S]*transfer\.viewerTimeZone,[\s\S]*i18n/);
  const fetch = sourceSection('async function fetchPendingOwnershipTransfer', 'async function loadOwnershipTransferCandidates');
  assert.doesNotMatch(fetch, /manualActivityDefaults/);
  assert.match(fetch, /validOwnershipTransferProjection\(pendingEnvelope\.data, pathID\)/);
  assert.match(page, /function validReviewedOwnershipTransfer/);
  assert.match(page, /function validOwnershipTransferProjection/);
  assert.match(page, /Object\.keys\(review\)\.sort\(\)\.join/);
  assert.match(page, /Object\.keys\(transfer\)\.sort\(\)\.join/);
});

test('same-owner rotation retains transfer lineage while account switches invalidate stale work', () => {
  assert.match(page, /rotatePathAdministrationTarget\(ownershipTransferTarget\.current, activeBeforeAdoption\.destination\.profile\.id, credential\)/);
  assert.match(page, /ownershipTransferTarget\.current && ownershipTransferTarget\.current\.ownerID !== nextOwnerID\) resetOwnershipTransfer\(\)/);
  assert.match(page, /notificationLifecycleState\.current\.session === currentSession/);
  const reset = sourceSection('function resetOwnershipTransfer()', 'function resetNotifications()');
  assert.match(reset, /ownershipTransferOperations\.invalidate\(\)/);
  assert.match(reset, /ownershipTransferTarget\.current = null/);
});
