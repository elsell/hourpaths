import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import test from 'node:test';

const source = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('notification and invitation loads retain same-owner credential lineage', () => {
  assert.match(source, /activeNotificationTargets = \[notificationTarget, notificationMutationTarget, notificationSettingsTarget\]/);
  assert.match(source, /rotateNotificationSessionTarget\(target\.current, ownerID, credential\)/);
  assert.match(source, /pendingInvitationsTarget\.current = rotateNotificationSessionTarget\(/);
  assert.match(source, /ownsCurrentNotificationOperation\(pendingInvitationsTarget\.current/);
});

test('notification settings mutations are frozen, synchronous, and replacement-safe', () => {
  assert.match(source, /notificationSettingsAdmission\.current\) \{[\s\S]*throw new Error\('nudge_channel_unavailable'\)/);
  assert.match(source, /const admission = Symbol\('notification-settings'\)/);
  assert.match(source, /notificationSettingsAdmission\.current === admission\) notificationSettingsAdmission\.current = null/);
});

test('notification mutations admit synchronously and cannot be overwritten by an older refresh', () => {
  assert.match(source, /notificationMutationAdmission\.current\) return false/);
  assert.match(source, /notificationMutationAdmission\.current = true;[\s\S]*notificationRefreshOperations\.invalidate\(\);[\s\S]*notificationTarget\.current = null/);
  assert.match(source, /notificationMutationOperations\.issue\(\)/);
  assert.match(source, /notificationMutationAdmission\.current = false/);
});

test('only the active request credential can apply a credential failure to the current session', () => {
  assert.match(source, /notificationLifecycleState\.current\.session\?\.token !== requestSession\.token\) return/);
  assert.match(source, /handleFeatureSessionFailure\(cause, requestSession, \{ current \}\)/);
});

test('cold notification journeys resume after authenticated Home restoration', () => {
  assert.match(source, /takeNotificationJourneyBootstrap\(socialPresentationKey\)/);
  assert.match(source, /pending\.kind === 'notifications'[\s\S]*openNotifications\(true\)/);
  assert.match(source, /pending\.kind === 'invitations'[\s\S]*openPendingInvitations\(undefined, true\)/);
  assert.match(source, /<NotificationJourneyRecoverySource[\s\S]*sessionKey=\{socialPresentationKey\}/);
});

test('warm direct notification journeys open their real owned route sources', () => {
  assert.match(source, /currentNotificationJourneyIntent\.kind === 'notifications'[\s\S]*openNotifications\(false\)/);
  assert.match(source, /currentNotificationJourneyIntent\.kind === 'invitations'[\s\S]*openPendingInvitations\(undefined, false\)/);
  assert.match(source, /notificationMutationAdmission\.current = false;[\s\S]*setNotificationMutationGeneration\(\(current\) => current \+ 1\)/);
  assert.match(source, /notificationMutationGeneration,[\s\S]*ownedHomeDestination\?\.profile\.id/);
});
