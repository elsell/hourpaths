import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('native Share owns manager list and cancellation across exact session and Path boundaries', () => {
  assert.match(page, /createPathInvitationCancelOwner/);
  assert.match(page, /mergeManagedPendingInvitationPage/);
  assert.match(page, /\.managedPathInvitations\(pathID, cursor \|\| undefined\)/);
  assert.match(page, /\.cancelPathInvitation\(pathID, invitationID, idempotencyKey\)/);
  assert.match(page, /const ticket = managedInvitationListOperations\.issue\(\)/);
  assert.match(page, /!ticket\.current\(\)/);
  assert.match(page, /ownsPathAdministrationTarget\(invitationTarget\.current, ownerID, pathID, currentSession\.token\)/);
  assert.match(page, /pendingInvitations=\{managedInvitationState\}/);
  assert.match(page, /onCancelManagedInvitation=.*cancelManagedInvitation/);
  assert.match(page, /onLoadMoreManagedInvitations=/);
  assert.match(page, /onRetryManagedInvitations=/);
});

test('sending refreshes manager truth and cancellation changes rows only after acknowledgement', () => {
  assert.match(page, /result\.kind === 'sent'[\s\S]*loadManagedInvitations\(pathID, '', currentSession, ownerID, true\)/);
  assert.match(page, /result\.kind === 'canceled'[\s\S]*managedInvitationListOperations\.invalidate\(\)[\s\S]*candidate\.invitation\.id !== invitationID[\s\S]*loadManagedInvitations\(pathID, '', currentSession, ownerID, true\)/);
  assert.match(page, /result\.kind === 'failed'[\s\S]*pathInvitationFailureMessageKey\(result\.failure\)/);
});

test('closing or replacing the invitation owner clears managed list work', () => {
  assert.match(page, /invitationCancelOwner\.cancel\(\)/);
  assert.match(page, /setManagedInvitationState\(\{ items: \[\], nextCursor: '' \}\)/);
  assert.match(page, /setManagedInvitationBusy\(\{\}\)/);
  assert.match(page, /setManagedInvitationErrors\(\{\}\)/);
});
