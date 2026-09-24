import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import { manualShareDraftChanged } from './ui/path-share-draft';

const source = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');

test('Share is one native overview with visibility, people, invitations, and a keyboard-safe Invite flow', () => {
  const sheet = source('./ui/path-share-sheet.tsx');
  const page = source('../app/index.tsx');
  assert.match(sheet, /export function PathShareSheet/);
  assert.match(sheet, /effectiveVisibility/);
  assert.match(sheet, /people\.map/);
  assert.match(sheet, /pendingInvitations/);
  assert.match(sheet, /NativeChoicePicker/);
  assert.match(sheet, /onSubmitEditing/);
  assert.match(sheet, /manualShareDraftChanged/);
  assert.match(sheet, /reduceMotionChanged/);
  assert.match(sheet, /trailingAction/);
  assert.match(sheet, /ManagedInvitations/);
  assert.match(sheet, /pathInvitation\.reviewedIdentity/);
  assert.match(page, /\? <PathShareSheet/);
  assert.match(page, /people=\{pathMembers\.map/);
  assert.match(page, /onVisibilityChange=\{selectedCapabilities\.manageVisibility/);
  assert.doesNotMatch(page, /\? <PathInvitationSheet/);
});

test('native Share pickers fill their settings row instead of collapsing text vertically', () => {
  const picker = source('./ui/native-choice-picker.ios.tsx');
  assert.doesNotMatch(picker, /<Host matchContents/);
  assert.match(picker, /<Host style=\{styles\.host\}>/);
  assert.match(picker, /container:\s*\{[\s\S]*alignSelf:\s*'stretch'[\s\S]*width:\s*'100%'/);
  assert.match(picker, /host:\s*\{[\s\S]*minHeight:\s*48[\s\S]*width:\s*'100%'/);
});

test('Share dismissal becomes safe again after an exact normalized draft revert', () => {
  const baseline = { role: 'participant' as const, username: 'person' };
  assert.equal(manualShareDraftChanged(baseline, baseline), false);
  assert.equal(manualShareDraftChanged(baseline, { ...baseline, username: 'changed' }), true);
  assert.equal(manualShareDraftChanged(baseline, { ...baseline, username: ' person ' }), false);
  assert.equal(manualShareDraftChanged(baseline, { ...baseline, role: 'supporter' }), true);
});

test('Share exposes authoritative People states and only actionable server-authorized rows navigate', () => {
  const sheet = source('./ui/path-share-sheet.tsx');
  const page = source('../app/index.tsx');
  assert.match(sheet, /props\.peopleLoading && props\.people\.length === 0/);
  assert.match(sheet, /props\.peopleError && props\.people\.length === 0/);
  assert.match(sheet, /person\.canManage \? <SettingsNavigationRow/);
  assert.match(sheet, /const roleLabel = props\.i18n\.t\(person\.role === 'administrator'[\s\S]*`pathMembers\.\$\{person\.role\}`\)/);
  assert.match(sheet, /pathMembers\.identityWithRole/);
  assert.match(sheet, /props\.peopleNextCursor/);
  assert.match(sheet, /props\.canInvite \? <ManagedInvitations/);
  assert.match(page, /canManage: member\.canChangeRole \|\| member\.canGrantAdministrator \|\| member\.canRemove/);
  assert.match(page, /onOpenPerson=\{\(userID\) =>/);
  assert.match(page, /peopleLoading=\{pathMembersLoading && pathMembers\.length === 0\}/);
  assert.match(page, /canInvite=\{selectedCapabilities\.inviteMembers\}/);
});

test('Manage delegates archive and ownership and no longer renders visibility controls', () => {
  const form = source('./ui/path-goal-management-form.tsx');
  const page = source('../app/index.tsx');
  assert.match(form, /onOpenArchive/);
  assert.match(form, /onOpenOwnershipTransfer/);
  assert.doesNotMatch(form, /NativeSegmentedControl|screen === 'visibility'/);
  assert.match(page, /onOpenArchive=\{selectedCapabilities\.manageLifecycle/);
  assert.match(page, /onOpenOwnershipTransfer=\{selectedCapabilities\.transferOwnership/);
  assert.match(page, /<PathArchiveConfirmationSheet/);
  assert.doesNotMatch(page, /pathArchiveReview \? <NativeSheet/);
});

test('Manage and Share prevent swipe dismissal for actual dirty drafts and admitted mutations', () => {
  const management = source('./ui/path-goal-management-form.tsx');
  const share = source('./ui/path-share-sheet.tsx');
  assert.match(management, /const dirty = goalCanReview \|\| renamePresentation\.canSave \|\| Boolean\(review\)/);
  assert.match(management, /dismissible=\{!managementBusy && !dirty\}/);
  assert.match(management, /onCancel\(dirty\)/);
  assert.match(share, /dismissible=\{!props\.busy && !dirty\}/);
  assert.match(share, /props\.onCancel\(dirty\)/);
  assert.match(management, /onOpenArchive\(dirty\)/);
  assert.match(management, /onOpenOwnershipTransfer\(dirty\)/);
  const page = source('../app/index.tsx');
  assert.match(page, /function handoffPathManagement\(dirty: boolean, kind: 'archive' \| 'ownership-transfer'\)/);
  assert.match(page, /pathManagementHandoffTarget\.current = target/);
  assert.match(page, /currentPathAdministrationCredential\([\s\S]*target\.pathID,[\s\S]*latest\.session/);
  assert.match(page, /effectivePathCapabilities\(authoritativePath\)\.manageLifecycle/);
  assert.match(page, /effectivePathCapabilities\(authoritativePath\)\.transferOwnership/);
  assert.match(page, /handoffPathManagement\(dirty, 'archive'\)/);
  assert.match(page, /handoffPathManagement\(dirty, 'ownership-transfer'\)/);
});

test('delayed native confirmations revalidate owned session lineage, Path presence, and current capability', () => {
  const page = source('../app/index.tsx');
  assert.match(page, /rotatePathAdministrationTarget\(pathManagementHandoffTarget\.current,[\s\S]*credential\)/);
  assert.match(page, /rotatePathAdministrationTarget\(pathVisibilityConfirmationTarget\.current,[\s\S]*credential\)/);
  assert.match(page, /pathManagementHandoffTarget\.current !== target \|\| !currentSession \|\| !authoritativePath \|\| !stillPermitted/);
  assert.match(page, /pathVisibilityConfirmationTarget\.current !== admittedTarget/);
  assert.match(page, /active\.destination\.profile\.paths\.find\(\(candidate\) => candidate\.id === review\.pathId\)/);
  assert.match(page, /!effectivePathCapabilities\(path\)\.manageVisibility/);
  assert.match(page, /pathManagementHandoffTarget\.current = null/);
  assert.match(page, /pathVisibilityConfirmationTarget\.current = null/);
});

test('archive confirmation owns native stable actions and motion-safe dismissal', () => {
  const sheet = source('./ui/path-archive-confirmation-sheet.tsx');
  assert.match(sheet, /leadingAction/);
  assert.match(sheet, /trailingAction/);
  assert.match(sheet, /dismissible=\{!busy\}/);
  assert.match(sheet, /reduceMotionChanged/);
  assert.doesNotMatch(sheet, /ActionButton/);
});
