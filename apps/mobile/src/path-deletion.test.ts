import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const form = readFileSync(fileURLToPath(new URL('./ui/path-goal-management-form.tsx', import.meta.url)), 'utf8');
const notificationView = readFileSync(fileURLToPath(new URL('./ui/notification-history-view.tsx', import.meta.url)), 'utf8');

test('Manage Path exposes deletion to lifecycle managers in a compact destructive section', () => {
  assert.match(page, /selectedCapabilities\.manageGoals \|\| selectedCapabilities\.renamePath \|\| selectedCapabilities\.manageLifecycle/);
  assert.match(page, /canDelete=\{selectedCapabilities\.manageLifecycle\}/);
  assert.match(form, /SettingsActionRow[\s\S]*pathDelete\.action/);
  assert.match(form, /setScreen\('delete'\)/);
  assert.match(form, /pathDelete\.heading/);
  assert.match(form, /pathDelete\.warning/);
  assert.match(form, /pathDelete\.timerWarning/);
  assert.match(form, /pathDelete\.archiveAlternative/);
  assert.match(form, /variant="danger"/);
});

test('deletion owns retry, blocks dismissal and competing mutations, and lands on Home only after success', () => {
  assert.match(page, /createPathDeletionOperationOwner/);
  assert.match(page, /pathDeletionOperations\.submit\(review, true/);
  assert.match(page, /\.deletePath\(requestedPathID, body, idempotencyKey\)/);
  assert.match(page, /applyPathDeletionResult/);
  assert.match(page, /resetPathDetail\(\)/);
  assert.match(form, /dismissible=\{!managementBusy && !dirty\}/);
  assert.match(form, /deleteBusy/);
  const cancel = page.slice(page.indexOf('function cancelPathDeletion'), page.indexOf('async function confirmPathDeletion'));
  assert.doesNotMatch(cancel, /deletePath|applyPathDeletionResult/);
});

test('standalone Path-deletion notifications are informational and never navigable', () => {
  assert.match(page, /notification\.type === 'path_deleted'/);
  assert.match(page, /notification\.type === 'path_deleted' \|\| notification\.type === 'path_member_removed'\s*\? false/);
  assert.match(notificationView, /notificationPresentationMessageKey/);
});
