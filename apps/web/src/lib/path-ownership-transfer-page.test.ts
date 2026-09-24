import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('creator ownership flow is effective-capability gated and cursor paged', () => {
  assert.match(page, /effectivePathCapabilities\(selectedPath\)\.transferOwnership/);
  assert.match(page, /\.ownershipTransferCandidates\(pathID, cursor \|\| undefined\)/);
  assert.match(page, /mergeOwnershipTransferCandidates/);
  assert.match(page, /ownershipCandidateNextCursor/);
});

test('review identifies the recipient, both role effects, delay, and exact plus relative expiration', () => {
  assert.match(page, /pathOwnership\.reviewRecipient/);
  assert.match(page, /ownershipSelectedCandidate\.displayName/);
  assert.match(page, /ownershipSelectedCandidate\.username/);
  assert.match(page, /requestOwnershipTransferReview\(current, pathID, candidate\.userId\)/);
  assert.match(page, /decodeOwnershipTransferReview/);
  assert.match(page, /pathOwnership\.recipientBecomesCreator/);
  assert.match(page, /pathOwnership\.creatorBecomesAdministrator/);
  assert.match(page, /pathOwnership\.noChangeUntilAccepted/);
  assert.match(page, /ownershipReviewExpiration\.relative/);
  assert.match(page, /ownershipReviewExpiration\.exact/);
  assert.match(page, /ownershipTransferExpiration\(review\.reviewedAt, review\.expiresAt, review\.viewerTimeZone, i18n\)/);
  assert.match(page, /const review = ownershipReview/);
  assert.match(page, /reservationToken: review\.reservationToken/);
});

test('pending and mutation results retain the authoritative viewer timezone', () => {
  assert.match(page, /ownershipTransferFromResult/);
  assert.match(page, /validViewerTimeZone\(result\.viewerTimeZone\)/);
  assert.match(page, /viewerTimeZone: result\.viewerTimeZone/);
  assert.match(page, /transfer\.viewerTimeZone \? ownershipTransferExpiration\(transfer\.createdAt, transfer\.expiresAt, transfer\.viewerTimeZone, i18n\)/);
  assert.doesNotMatch(page, /Intl\.DateTimeFormat\(\)\.resolvedOptions\(\)\.timeZone/);
});

test('pending requests load from the authoritative endpoint independently of notification state', () => {
  assert.match(page, /\.pendingOwnershipTransfer\(path\.id\)/);
  assert.match(page, /pendingOwnershipTransfers/);
  assert.match(page, /pathOwnership\.creatorPendingExplanation/);
  assert.match(page, /pathOwnership\.recipientPendingExplanation/);
  const loadProfile = page.slice(page.indexOf('async function loadProfile'), page.indexOf('function handleFailure'));
  assert.match(loadProfile, /pendingOwnershipTransfer\(path\.id\)/);
  assert.doesNotMatch(loadProfile, /notificationHistory/);
});

test('creator cancel and recipient accept or decline retain idempotency keys across retry', () => {
  assert.match(page, /\.cancelOwnershipTransfer\(transfer\.id, idempotencyKey\)/);
  assert.match(page, /\.acceptOwnershipTransfer\(transfer\.id, idempotencyKey\)/);
  assert.match(page, /\.declineOwnershipTransfer\(transfer\.id, idempotencyKey\)/);
  assert.match(page, /ownershipMutationKeys\[key\] \?\? crypto\.randomUUID\(\)/);
  assert.match(page, /await refreshOwnershipState\(current/);
});

test('ownership transfer uses compact accessible rows and destructive action semantics', () => {
  assert.match(page, /class="ownership-row"/);
  assert.match(page, /aria-label=\{i18n\.t\('pathOwnership\.candidateAccessibility'/);
  assert.match(page, /class="ownership-action destructive"/);
  assert.match(page, /role="status"/);
  assert.match(page, /role="alert"/);
});
