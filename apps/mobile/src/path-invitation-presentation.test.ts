import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const invitationRoute = readFileSync(fileURLToPath(new URL('../app/invitations.tsx', import.meta.url)), 'utf8');
const pendingInvitationsView = readFileSync(
  fileURLToPath(new URL('./ui/pending-invitations-view.tsx', import.meta.url)),
  'utf8',
);
const invitationSheet = readFileSync(fileURLToPath(new URL('./ui/path-share-sheet.tsx', import.meta.url)), 'utf8');
const warningSheet = readFileSync(
  fileURLToPath(new URL('./ui/path-invitation-visibility-warning-sheet.tsx', import.meta.url)),
  'utf8',
);
const nativePrimaryButton = readFileSync(
  fileURLToPath(new URL('./ui/native-primary-button.ios.tsx', import.meta.url)),
  'utf8',
);

function section(start: string, end: string): string {
  const startIndex = page.indexOf(start);
  assert.notEqual(startIndex, -1, `missing ${start}`);
  const endIndex = page.indexOf(end, startIndex + start.length);
  assert.notEqual(endIndex, -1, `missing ${end}`);
  return page.slice(startIndex, endIndex);
}

test('mobile dedicated Invitations screen presents recipient-scoped pending invitations with retained pagination', () => {
  assert.match(page, /\.pendingPathInvitations\(cursor \|\| undefined\)/);
  assert.match(page, /mergePendingInvitationPage\(current\.profile\.pendingInvitations, page, cursor\)/);
  assert.match(page, /loadPendingInvitationsThroughTarget\([\s\S]*currentSession,[\s\S]*invitationID,[\s\S]*pendingInvitationsTarget/);
  assert.match(page, /visitedCursors\.has\(cursor\)/);
  assert.match(page, /pageCount < 100/);
  assert.match(page, /if \(!isCurrent\(\)\) throw \{ kind: 'superseded' \}/);
  assert.match(page, /<InvitationRouteSource/);
  assert.match(page, /router\.push\(\{[\s\S]*pathname: '\/invitations'/);
  assert.match(invitationRoute, /useInvitationRoutePresentation/);
  assert.match(invitationRoute, /<NativeRouteScreen grouped>/);
  assert.match(pendingInvitationsView, /pathInvitation\.pendingEmpty/);
  assert.match(pendingInvitationsView, /<NativeContentUnavailable/);
  assert.doesNotMatch(pendingInvitationsView, /<SettingsSection/);
  assert.match(pendingInvitationsView, /accessibilityRole="header"/);
  assert.match(pendingInvitationsView, /<SettingsActionRow/);
  assert.match(pendingInvitationsView, /orderedInvitations[\s\S]*focusedInvitationID/);
  assert.match(invitationRoute, /<NotificationJourneyRecoveryView/);
  assert.doesNotMatch(invitationRoute, /return null/);
  assert.match(page, /prepareInvitationRoute\(\)[\s\S]*pathname: '\/invitations'/);
});

test('mobile Share is capability gated and presents only reviewed public identity', () => {
  assert.match(page, /selectedCapabilities\.inviteMembers \|\| selectedCapabilities\.manageVisibility \|\| selectedCapabilities\.manageMembers/);
  assert.match(page, /\? <PathShareSheet/);
  assert.match(invitationSheet, /pathInvitation\.usernameLabel/);
  assert.match(invitationSheet, /pathInvitation\.usernameHint/);
  assert.match(invitationSheet, /pathInvitation\.reviewedIdentity/);
  assert.doesNotMatch(invitationSheet, /\.email|providerEmail/);
});

test('mobile review, send, accept, and reject use generated routes through retry-safe owners', () => {
  assert.match(page, /invitationReviewOwner\.review\(/);
  assert.match(page, /\.reviewPathInvitationRecipient\(pathID, exactUsername\)/);
  assert.match(page, /invitationSendOwner\.submit\(review, invitationRole, true/);
  assert.match(page, /\.sendPathInvitation\(pathID, body, idempotencyKey\)/);
  assert.match(page, /reviewPendingPathInvitationAcceptance\(pending\)/);
  assert.match(page, /invitationAcceptOwner\.submit\(review, true/);
  assert.match(page, /\.acceptPathInvitation\(id, idempotencyKey, body\)/);
  assert.match(page, /invitationRejectOwner\.submit\(invitationID, true/);
  assert.match(page, /\.rejectPathInvitation\(id, idempotencyKey\)/);
  assert.equal(
    page.match(/pathInvitationOutputData\(await invitationResponse/g)?.length,
    4,
    'every mobile singleton invitation output must be unwrapped before domain validation',
  );
});

test('mobile decline uses native destructive confirmation and cancellation performs no request', () => {
  const rejection = section(
    'function beginPendingInvitationRejection(pending: PendingPathInvitation) {',
    'async function submitPendingInvitationRejection(invitationID: string) {',
  );
	assert.match(rejection, /presentNativeDestructiveConfirmation\(\{/);
  assert.match(rejection, /pathInvitation\.rejectConfirmationHeading/);
  assert.match(rejection, /pathInvitation\.rejectConfirmationBody/);
	assert.match(rejection, /cancelLabel: i18n\.t\('common\.cancel'\)/);
	assert.match(rejection, /confirmLabel: i18n\.t\('pathInvitation\.reject'\)/);
	assert.match(rejection, /onConfirm: \(\) => void submitPendingInvitationRejection\(invitationID\)/);
  assert.doesNotMatch(rejection, /rejectPathInvitation|invitationRejectOwner\.submit/);
  assert.match(pendingInvitationsView, /tone="destructive"/);
  assert.match(pendingInvitationsView, /pathInvitation\.reject/);
});

test('mobile decline is mutually exclusive and converges invitation, notification, and badge state', () => {
  const rejection = section(
    'async function submitPendingInvitationRejection(invitationID: string) {',
    'function openPathManagement(path: SessionPath) {',
  );
  assert.match(rejection, /pendingInvitationBusy\[invitationID\]/);
  assert.match(rejection, /invitationRejectionTargets\.current\.set\(invitationID/);
  assert.match(rejection, /target\?\.session !== currentSession \|\| target\.ownerID !== ownerID/);
  assert.match(rejection, /items: current\.profile\.pendingInvitations\.items\.filter/);
  assert.match(rejection, /notification\.invitationId !== invitationID/);
  assert.match(rejection, /setNotificationUnreadCount\(result\.rejection\.unreadCount\)/);
  assert.match(rejection, /setNativeNotificationBadge\(result\.rejection\.unreadCount\)/);
  assert.match(rejection, /setRejectedInvitationID\(invitationID\)/);
  assert.match(pendingInvitationsView, /accessibilityLiveRegion="polite"/);
});

test('mobile warning review is frozen before confirmation and cancellation performs no request', () => {
  assert.match(page, /setPendingInvitationAcceptanceReview\(review\)/);
  assert.match(page, /if \(review\.kind === 'confirmation-required'\)[\s\S]*return;/);
  assert.match(page, /invitationAcceptOwner\.cancel\(review\.invitationId\)/);
  assert.match(page, /setPendingInvitationAcceptanceReview\(null\)/);
  assert.match(page, /<InvitationRouteSource[\s\S]*<PathInvitationVisibilityWarningSheet/);
});

test('mobile stale warning failures clear the frozen review and reload the current projection', () => {
  const failure = section(
    "if (result.kind === 'failed') {",
    "if (result.kind !== 'accepted') return;",
  );
  assert.match(failure, /errorKey === 'pathInvitation\.warningRequired'/);
  assert.match(failure, /invitationAcceptOwner\.cancel\(invitationID\)/);
  assert.match(failure, /setPendingInvitationAcceptanceReview\(null\)/);
  assert.match(failure, /await refreshPendingInvitationProjection\(currentSession, ownerID, invitationID\)/);
});

test('mobile opaque invitation decisions reload authoritative invitations and notifications only for unavailable state', () => {
  const acceptance = section(
    'async function submitPendingInvitationAcceptance(review: PathInvitationAcceptanceReview) {',
    'function beginPendingInvitationRejection(pending: PendingPathInvitation) {',
  );
  const rejection = section(
    'async function submitPendingInvitationRejection(invitationID: string) {',
    'function openPathManagement(path: SessionPath) {',
  );
  for (const decision of [acceptance, rejection]) {
    assert.match(
      decision,
      /result\.failure\.kind === 'opaque'[\s\S]*await refreshUnavailableInvitationProjections\(currentSession, ownerID, invitationID\)/,
    );
    assert.doesNotMatch(
      decision,
      /result\.failure\.kind === 'network'[\s\S]*refreshUnavailableInvitationProjections/,
    );
  }
  const refresh = section(
    'async function refreshUnavailableInvitationProjections(',
    'async function submitPendingInvitationAcceptance(review: PathInvitationAcceptanceReview) {',
  );
  assert.match(refresh, /await refreshPendingInvitationProjection\(currentSession, ownerID, invitationID\)/);
  assert.match(refresh, /await refreshPushNotificationHistory\(currentSession, ownerID\)/);
});

test('mobile acceptance ownership is isolated from invitation sharing and keyed per invitation', () => {
  const resetShare = section('function resetInvitationShare() {', 'function resetInvitations() {');
  const resetAll = section('function resetInvitations() {', 'function resetNotifications() {');
  const accept = section(
    'async function submitPendingInvitationAcceptance(review: PathInvitationAcceptanceReview) {',
    'function openPathManagement(path: SessionPath) {',
  );
  assert.doesNotMatch(resetShare, /invitationAcceptanceTargets/);
  assert.match(resetAll, /invitationAcceptanceTargets\.current\.clear\(\)/);
  assert.match(resetAll, /invitationRejectionTargets\.current\.clear\(\)/);
  assert.match(accept, /invitationAcceptanceTargets\.current\.set\(invitationID/);
  assert.match(accept, /invitationAcceptanceTargets\.current\.get\(invitationID\)/);
  assert.match(accept, /invitationAcceptanceTargets\.current\.delete\(invitationID\)/);
});

test('mobile pending-invitation pagination ownership is isolated from invitation sharing', () => {
  const resetShare = section('function resetInvitationShare() {', 'function resetInvitations() {');
  const resetAll = section('function resetInvitations() {', 'function resetNotifications() {');
  const paginate = section(
    'async function loadMorePendingInvitations(cursor: string) {',
    'async function openNotifications(navigate = true) {',
  );
  assert.doesNotMatch(resetShare, /pendingInvitationsTarget/);
  assert.match(resetAll, /pendingInvitationsTarget\.current = null/);
  assert.match(paginate, /pendingInvitationsTarget\.current = createNotificationSessionTarget\(ownerID, currentSession, intentKey\)/);
  assert.match(paginate, /ownsCurrentNotificationOperation\(pendingInvitationsTarget\.current, ownerID, intentKey, currentSession\)/);
  assert.match(paginate, /pendingInvitationsTarget\.current = null[\s\S]*setPendingInvitationsBusy\(false\)/);
  assert.doesNotMatch(paginate, /invitationTarget/);
});

test('mobile visibility warning uses a native page sheet with ordered accessible disclosure', () => {
  assert.match(warningSheet, /<NativeSheet[\s\S]*visible>/);
  assert.match(warningSheet, /accessibilityRole="alert"/);
  const disclosure = sectionFrom(
    warningSheet,
    /pathInvitation\.visibilityWarning\.heading/,
    /pathInvitation\.visibilityWarning\.confirm/,
  );
  assert.match(disclosure, /pathInvitation\.visibilityWarning\.audience\./);
  assert.match(disclosure, /pathInvitation\.visibilityWarning\.exposure/);
  assert.match(disclosure, /pathInvitation\.visibilityWarning\.privacyScope/);
  assert.match(disclosure, /hasRetainedActivity[\s\S]*pathInvitation\.visibilityWarning\.retainedActivity/);
  assert.match(warningSheet, /<NativePrimaryButton/);
  assert.match(nativePrimaryButton, /@expo\/ui\/swift-ui/);
  assert.match(nativePrimaryButton, /buttonStyle\('borderedProminent'\)/);
  assert.match(warningSheet, /variant="quiet"/);
  assert.doesNotMatch(warningSheet, /@hourpaths\/api-client|createSessionApiClient/);
});

test('mobile role effects, warning retention, and supporter read-only outcome are explicit', () => {
  assert.match(invitationSheet, /pathInvitation\.role\.participantEffect/);
  assert.match(invitationSheet, /pathInvitation\.role\.supporterEffect/);
  assert.match(invitationSheet, /pathInvitation\.confirmHeading/);
  assert.match(invitationSheet, /pathInvitation\.confirmSend/);
  assert.match(page, /pathInvitation\.warningRequired/);
  assert.match(pendingInvitationsView, /pathInvitation\.supporterReadOnly/);
});

test('mobile invitation handlers recheck owner, session, capability, and cancel stale work', () => {
  assert.match(page, /effectivePathCapabilities\(path\)\.inviteMembers/);
  assert.match(page, /ownsPathAdministrationTarget\(invitationTarget\.current, ownerID, pathID, currentSession\.token\)/);
  assert.match(page, /rotatePathAdministrationTarget\(invitationTarget\.current, activeBeforeAdoption\.destination\.profile\.id, credential\)/);
  assert.match(page, /invitationAcceptanceTargets\.current\.values\(\)/);
  assert.match(page, /target\?\.session === session[\s\S]*target\.ownerID === destination\.profile\.id/);
  assert.match(page, /pending\?\.warning\?\.pathVisibility === review\.warning\.pathVisibility/);
  assert.match(page, /invitationReviewOwner\.cancel\(\)/);
  assert.match(page, /invitationSendOwner\.cancel\(/);
  assert.match(page, /invitationAcceptOwner\.cancel\(\)/);
  assert.match(page, /invitationRejectOwner\.cancel\(\)/);
});

function sectionFrom(source: string, start: RegExp, end: RegExp): string {
  const startMatch = start.exec(source);
  assert.ok(startMatch, `missing ${start}`);
  const rest = source.slice(startMatch.index + startMatch[0].length);
  const endMatch = end.exec(rest);
  assert.ok(endMatch, `missing ${end}`);
  return rest.slice(0, endMatch.index);
}
