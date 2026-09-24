import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const sheet = readFileSync(
  fileURLToPath(new URL('./ui/path-invitation-sheet.tsx', import.meta.url)),
  'utf8',
);

test('Share presents managed pending invitations as compact native rows with complete collection states', () => {
  assert.match(sheet, /ManagedPendingPathInvitationState/);
  assert.match(sheet, /invitations\.items\.length === 0/);
  assert.match(sheet, /pathInvitation\.managed\.loading/);
  assert.match(sheet, /pathInvitation\.managed\.unavailableHeading/);
  assert.match(sheet, /pathInvitation\.managed\.emptyHeading/);
  assert.match(sheet, /<SettingsSection/);
  assert.match(sheet, /<SettingsActionRow/);
  assert.match(sheet, /invitations\.nextCursor/);
  assert.match(sheet, /onLoadMoreManagedInvitations/);
  assert.match(sheet, /onRetryManagedInvitations/);
});

test('each managed invitation identifies recipient, role, inviter, and localized sent time', () => {
  assert.match(sheet, /pathInvitation\.managed\.recipient/);
  assert.match(sheet, /pathInvitation\.managed\.role/);
  assert.match(sheet, /pathInvitation\.managed\.inviter/);
  assert.match(sheet, /pathInvitation\.managed\.sentAt/);
  assert.match(sheet, /const sentAt = new Date\(invitation\.invitation\.createdAt\)/);
  assert.match(sheet, /translator\.date\(sentAt/);
  assert.match(sheet, /translator\.time\(sentAt/);
  assert.doesNotMatch(sheet, /\.email|providerEmail/);
});

test('cancel is confirmed natively before its callback and exposes per-row progress and failure', () => {
  assert.match(sheet, /presentNativeDestructiveConfirmation\(\{/);
  assert.match(sheet, /pathInvitation\.managed\.cancelConfirmationHeading/);
  assert.match(sheet, /pathInvitation\.managed\.cancelConfirmationBody/);
  assert.match(sheet, /pathInvitation\.role\.\$\{invitation\.invitation\.offeredRole\}/);
  assert.match(sheet, /pathInvitation\.managed\.cancelFor/);
  assert.match(sheet, /onConfirm: \(\) => onCancelManagedInvitation\(invitation\)/);
  assert.match(sheet, /invitationBusy\[invitation\.invitation\.id\]/);
  assert.match(sheet, /invitationErrors\[invitation\.invitation\.id\]/);
  assert.match(sheet, /tone="destructive"/);
});
