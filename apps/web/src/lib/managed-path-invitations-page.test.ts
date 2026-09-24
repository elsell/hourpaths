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

test('opening Share loads manager-scoped pending invitations with signed pagination', () => {
  const sharing = section('function openPathSharing()', 'function cancelPathSharing()');
  assert.match(sharing, /sharingPath = true/);
  assert.match(sharing, /void loadManagedPathInvitations\('', true, current, ownerID, pathID\)/);

  const loading = section('async function loadManagedPathInvitations(', 'function beginManagedInvitationCancellation');
  assert.match(loading, /\.managedPathInvitations\(pathID, cursor \|\| undefined\)/);
  assert.match(loading, /mergeManagedPendingInvitationPage\(managedInvitations, page, cursor\)/);
  assert.match(loading, /session !== expectedSession/);
  assert.match(loading, /profile\?\.id !== expectedOwnerID/);
  assert.match(loading, /selectedPath\?\.id !== pathID/);
  assert.match(loading, /!effectivePathCapabilities\(currentPath\)\.inviteMembers/);
});

test('Share renders compact pending rows and complete list states without private identity', () => {
  const share = section('{#if sharingPath && selectedCapabilities.inviteMembers}', '{#if ownershipOpen');
  assert.match(share, /pathInvitation\.managed\.heading/);
  assert.match(share, /pathInvitation\.managed\.loading/);
  assert.match(share, /pathInvitation\.managed\.emptyHeading/);
  assert.match(share, /pathInvitation\.managed\.unavailableHeading/);
  assert.match(share, /pathInvitation\.managed\.loadingMore/);
  assert.match(share, /managedInvitations\.nextCursor/);

  const rows = section('{#each managedInvitations.items as managed', '{/each}');
  assert.match(rows, /managed\.recipient\.displayName/);
  assert.match(rows, /managed\.recipient\.username/);
  assert.match(rows, /managed\.inviter\.displayName/);
  assert.match(rows, /managed\.inviter\.username/);
  assert.match(rows, /managed\.invitation\.offeredRole/);
  assert.match(rows, /managed\.invitation\.createdAt/);
  assert.doesNotMatch(rows, /\.email|providerEmail|profileVisibility/);
});

test('cancellation requires an accessible destructive confirmation', () => {
  const share = section('{#if sharingPath && selectedCapabilities.inviteMembers}', '{#if ownershipOpen');
  assert.match(share, /role="alertdialog"/);
  assert.match(share, /aria-labelledby="managed-invitation-cancel-heading"/);
  assert.match(share, /aria-describedby="managed-invitation-cancel-description"/);
  assert.match(share, /pathInvitation\.managed\.cancelConfirmationHeading/);
  assert.match(share, /pathInvitation\.managed\.cancelConfirmationBody/);
  assert.match(share, /pathInvitation\.role\.\$\{managedInvitationCancelReview\.invitation\.offeredRole\}/);
  assert.match(share, /aria-label=\{i18n\.t\('pathInvitation\.managed\.cancelFor'/);
  assert.match(share, /class="destructive-action"/);
  assert.match(share, /confirmManagedInvitationCancellation/);
  assert.match(share, /cancelManagedInvitationCancellation/);
});

test('failed cancellation retains row and retry key; only acknowledged success removes it', () => {
  const cancellation = section(
    'async function confirmManagedInvitationCancellation()',
    'function openPathSharing()',
  );
  assert.match(cancellation, /managedInvitationCancelOwner\.submit\(/);
  assert.match(cancellation, /\.cancelPathInvitation\(pathID, invitationID, idempotencyKey\)/);
  assert.match(cancellation, /if \(result\.kind === 'canceled'\)/);
  assert.match(cancellation, /if \(result\.kind === 'canceled'\)[\s\S]*managedInvitationListOperations\.invalidate\(\)[\s\S]*managedInvitations = \{[\s\S]*items: managedInvitations\.items\.filter/);
  assert.match(cancellation, /if \(result\.kind === 'failed'\)[\s\S]*managedInvitationCancelFailed = true/);

  const failedIndex = cancellation.indexOf("if (result.kind === 'failed')");
  const successIndex = cancellation.indexOf("if (result.kind === 'canceled')");
  assert.ok(successIndex >= 0 && failedIndex >= 0);
  assert.doesNotMatch(
    cancellation.slice(failedIndex),
    /managedInvitationCancelOwner\.cancel\(invitationID\)/,
    'network failure must preserve the owner retry key',
  );
});

test('successful send refreshes managed invitations and closing or changing context invalidates work', () => {
  const sending = section('async function sendReviewedInvitation()', 'function acceptPendingInvitation');
  assert.match(sending, /result\.kind === 'sent'[\s\S]*managedInvitationListOperations\.invalidate\(\)[\s\S]*loadManagedPathInvitations\('', true, current, ownerID, pathID\)/);

  const reset = section('function resetInvitationShare()', 'function resetInvitations()');
  assert.match(reset, /managedInvitationListOperations\.invalidate\(\)/);
  assert.match(reset, /managedInvitationCancelOwner\.cancel\(\)/);
  assert.match(reset, /managedInvitations = \{ items: \[\], nextCursor: '' \}/);
});
