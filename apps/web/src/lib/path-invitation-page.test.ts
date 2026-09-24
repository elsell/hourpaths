import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

function section(start: string, end: string): string {
  const startIndex = page.indexOf(start);
  assert.notEqual(startIndex, -1, `missing ${start}`);
  const endIndex = page.indexOf(end, startIndex + start.length);
  assert.notEqual(endIndex, -1, `missing ${end}`);
  return page.slice(startIndex, endIndex);
}

test('active Home loads recipient-scoped pending invitations and retains signed pagination', () => {
  assert.match(page, /\.pendingPathInvitations\(cursor \|\| undefined\)/);
  assert.match(page, /mergePendingInvitationPage\(pendingInvitations, page, cursor\)/);
  assert.match(page, /pathInvitation\.pendingHeading/);
  assert.match(page, /pathInvitation\.pendingContext/);
  const pending = section('{#each pendingInvitations.items as pending', '{/each}');
  assert.match(pending, /pending\.inviter\.displayName/);
  assert.match(pending, /pending\.inviter\.username/);
  assert.match(pending, /pending\.pathName/);
  assert.doesNotMatch(pending, /\.email|profileVisibility/);
});

test('Share is capability gated and exact-username review renders only canonical public identity', () => {
  assert.match(page, /\{#if selectedCapabilities\.inviteMembers\}/);
  const share = section('{#if sharingPath && selectedCapabilities.inviteMembers}', '{#if pathDetailBusy');
  assert.match(share, /pathInvitation\.usernameLabel/);
  assert.match(share, /pathInvitation\.usernameHint/);
  assert.match(share, /pathInvitation\.reviewedIdentity/);
  assert.doesNotMatch(share, /\.email|providerEmail/);
});

test('review, send, and accept use generated routes through owned retry-safe operations', () => {
  assert.match(page, /invitationReviewOwner\.review\(/);
  assert.match(page, /\.reviewPathInvitationRecipient\(pathID, exactUsername\)/);
  assert.match(page, /invitationSendOwner\.submit\(review, invitationRole, true/);
  assert.match(page, /\.sendPathInvitation\(pathID, body, idempotencyKey\)/);
  assert.match(page, /reviewPendingPathInvitationAcceptance\(pending\)/);
  assert.match(page, /invitationAcceptOwner\.submit\(review, confirmed/);
  assert.match(page, /\.acceptPathInvitation\(id, idempotencyKey, body\)/);
  assert.equal(
    page.match(/pathInvitationOutputData\(await invitationResponse/g)?.length,
    3,
    'every singleton invitation output must be unwrapped before domain validation',
  );
});

test('roles, confirmation, warning retention, and supporter read-only effects are explicit', () => {
  assert.match(page, /pathInvitation\.role\.participantEffect/);
  assert.match(page, /pathInvitation\.role\.supporterEffect/);
  assert.match(page, /pathInvitation\.confirmHeading/);
  assert.match(page, /pathInvitation\.confirmSend/);
  assert.match(page, /pathInvitation\.visibilityWarning\.heading/);
  assert.match(page, /pathInvitation\.supporterReadOnly/);
  assert.match(page, /result\.kind === 'failed'[\s\S]*pendingInvitationErrors =/);
});

test('invitation handlers recheck session ownership and current server capability', () => {
  assert.match(page, /effectivePathCapabilities\(path\)\.inviteMembers/);
  assert.match(page, /session !== current/);
  assert.match(page, /invitationOwnerID !== ownerID/);
  assert.match(page, /invitationReviewOwner\.cancel\(\)/);
  assert.match(page, /invitationSendOwner\.cancel\(/);
  assert.match(page, /invitationAcceptOwner\.cancel\(\)/);
});

test('warning acceptance is reviewed before any generated API call and no-warning acceptance remains one tap', () => {
  const acceptance = section(
    'function acceptPendingInvitation(invitationID: string)',
    'function reviewArchiveChange()',
  );
  const reviewIndex = acceptance.indexOf('reviewPendingPathInvitationAcceptance(pending)');
  const confirmationIndex = acceptance.indexOf("review.kind === 'confirmation-required'");
  const submitIndex = acceptance.indexOf('invitationAcceptOwner.submit(review, confirmed');
  const apiIndex = acceptance.indexOf('.acceptPathInvitation(id, idempotencyKey, body)');
  assert.ok(reviewIndex >= 0);
  assert.ok(confirmationIndex > reviewIndex);
  assert.ok(submitIndex > confirmationIndex);
  assert.ok(apiIndex > submitIndex);
  assert.match(
    acceptance.slice(confirmationIndex, submitIndex),
    /pendingInvitationReview = review[\s\S]*return/,
    'the warning branch must stop before submission',
  );
  assert.match(
    acceptance,
    /if \(review\.kind === 'ready'\) void submitPendingInvitationAcceptance\(review, true\)/,
  );
});

test('inline accessible warning has complete localized copy in required reading order and named actions', () => {
  const pending = section('{#each pendingInvitations.items as pending', '{/each}');
  assert.match(pending, /role="alertdialog"/);
  assert.match(pending, /aria-labelledby="path-invitation-warning-heading-\{invitation\.id\}"/);
  const keys = [
    'pathInvitation.visibilityWarning.heading',
    'pathInvitation.visibilityWarning.audience.',
    'pathInvitation.visibilityWarning.exposure',
    'pathInvitation.visibilityWarning.privacyScope',
    'pathInvitation.visibilityWarning.retainedActivity',
    'pathInvitation.visibilityWarning.confirm',
    'pathInvitation.visibilityWarning.cancel',
  ];
  let previous = -1;
  for (const key of keys) {
    const index = pending.indexOf(key);
    assert.ok(index > previous, `${key} must follow the preceding warning content`);
    previous = index;
  }
  assert.match(pending, /\{#if pendingInvitationReview\.warning\.hasRetainedActivity\}/);
  assert.match(pending, /confirmPendingInvitation\(invitation\.id\)/);
  assert.match(pending, /cancelPendingInvitationReview\(invitation\.id\)/);
  assert.match(
    pending,
    /use:focusPendingInvitationConfirm=\{invitation\.id\}/,
  );
  assert.match(
    pending,
    /use:focusPendingInvitationAccept=\{invitation\.id\}/,
  );
  assert.match(
    page,
    /pendingInvitationFocusTarget = \{ kind: 'confirm', invitationID \};\s*pendingInvitationReview = review/,
  );
  assert.match(
    page,
    /pendingInvitationFocusTarget = \{ kind: 'accept', invitationID \};\s*pendingInvitationReview = null/,
  );
});

test('cancel, stale warning failure, owner changes, and accepted removal all retain safe state', () => {
  const acceptance = section(
    'function acceptPendingInvitation(invitationID: string)',
    'function reviewArchiveChange()',
  );
  const cancelStart = acceptance.indexOf('function cancelPendingInvitationReview');
  const cancelEnd = acceptance.indexOf('async function submitPendingInvitationAcceptance', cancelStart);
  const cancel = acceptance.slice(cancelStart, cancelEnd);
  assert.match(
    cancel,
    /invitationAcceptOwner\.cancel\(invitationID\)[\s\S]*pendingInvitationReview = null/,
  );
  assert.doesNotMatch(cancel, /pendingInvitations\s*=/, 'cancel must leave the invitation pending');
  const failureStart = acceptance.indexOf("if (result.kind === 'failed')");
  const failureEnd = acceptance.indexOf("if (result.kind !== 'accepted')", failureStart);
  const failure = acceptance.slice(failureStart, failureEnd);
  assert.match(
    failure,
    /pathInvitationFailureMessageKey\(result\.failure\)[\s\S]*await loadPendingInvitations\('', true\)[\s\S]*return/,
  );
  assert.doesNotMatch(failure, /pendingInvitations\s*=/, 'server warning failures must stay pending');
  assert.match(
    acceptance,
    /items: pendingInvitations\.items\.filter[\s\S]*pendingInvitationReview = null/,
  );
  assert.match(page, /function resetInvitations\(\)[\s\S]*pendingInvitationReview = null/);
  assert.match(
    page,
    /if \(profile && profile\.id !== home\.profile\.id\) \{ resetManualActivity\(\); resetPathDetails\(\); resetInvitations\(\); \}/,
    'owner replacement must clear the reviewed warning',
  );
  const refresh = section('async function attemptRefresh(expiresAt: string)', 'function scheduleRefresh');
  assert.match(
    refresh,
    /clearPendingInvitationAcceptance\(\);\s*session = next/,
    'session rotation must invalidate a reviewed warning',
  );
  assert.match(
    page,
    /pendingInvitations = mergePendingInvitationPage\([\s\S]*warning\?\.pathVisibility === pendingInvitationReview\.warning\.pathVisibility[\s\S]*warning\?\.hasRetainedActivity === pendingInvitationReview\.warning\.hasRetainedActivity/,
  );
});
