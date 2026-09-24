import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const managementForm = readFileSync(
  fileURLToPath(new URL('./ui/path-goal-management-form.tsx', import.meta.url)),
  'utf8',
);
const goalFields = readFileSync(
  fileURLToPath(new URL('./ui/path-create-form.tsx', import.meta.url)),
  'utf8',
);

function sourceSection(start: string, end: string): string {
  const startIndex = page.indexOf(start);
  assert.notEqual(startIndex, -1, `missing source section ${start}`);
  const endIndex = page.indexOf(end, startIndex + start.length);
  assert.notEqual(endIndex, -1, `missing source section terminator ${end}`);
  return page.slice(startIndex, endIndex);
}

test('Path detail opens Manage Path with the stored goals and reviews old and proposed settings beside the localized warning', () => {
  assert.match(page, /pathGoalFormFromPath/);
  assert.match(page, /buildPathGoalUpdateDraft/);
  assert.match(page, /function openPathManagement\(path: SessionPath\)/);
  assert.match(page, /setGoalManagementForm\(pathGoalFormFromPath\(path\)\)/);

  const pathDetail = sourceSection('{selectedPath ? <NativeRouteSource', '{selectedCapabilities.manageLifecycle && pathArchiveReview ?');
  assert.match(pathDetail, /pathManage\.action/);
  assert.match(pathDetail, /openPathManagement\(selectedPath\)/);
  assert.match(pathDetail, /selectedCapabilities\.manageGoals/);

  const management = sourceSection('<PathGoalManagementForm', '{currentPathRouteIntent && !selectedPath ?');
  assert.match(management, /<PathGoalManagementForm/);
  assert.match(management, /form=\{goalManagementForm\}/);
  assert.match(management, /goalCanReview=\{goalManagementCanReview\}/);
  assert.match(management, /review=\{goalManagementReview \?\? undefined\}/);
  assert.match(managementForm, /pathManage\.mobileHeading/);
  assert.match(managementForm, /<PathGoalFields[\s\S]*compact/);
  assert.match(managementForm, /setScreen\('goals'\)/);
  assert.match(managementForm, /<SettingsNavigationRow/);
  assert.match(goalFields, /pathCreate\.intervalEnabled/);
  assert.match(goalFields, /pathCreate\.overallEnabled/);
  assert.match(managementForm, /pathManage\.review/);
  assert.match(managementForm, /pathManage\.current/);
  assert.match(managementForm, /pathManage\.proposed/);
  assert.match(managementForm, /pathManage\.changed/);
  assert.match(managementForm, /accessibilityRole="alert"[^>]*style=\{styles\.warning\}/);
  assert.match(managementForm, /pathManage\.goalWarning/);
  assert.match(managementForm, /pathManage\.confirm/);
  assert.match(managementForm, /common\.done/);
  assert.match(managementForm, /common\.back/);
  assert.match(managementForm, /dismissible=\{!managementBusy && !dirty\}/);
  assert.match(managementForm, /disabled:\s*managementBusy \|\| !goalCanReview/);

  const review = sourceSection('function reviewPathGoalChanges()', 'function cancelPathGoalReview()');
  assert.match(review, /buildPathGoalUpdateDraft\(goalManagementForm\)/);
  assert.match(review, /current:\s*goalManagementCurrent/);
  assert.match(review, /proposed:\s*prepared\.draft/);
  assert.doesNotMatch(review, /\.updatePathGoals\(/);
});

test('Manage Path is presented by the visible Path-details route instead of behind it', () => {
  const pathDetail = sourceSection(
    '{selectedPath ? <NativeRouteSource',
    '{selectedPath && activityHistoryOpen ? <NativeChildRouteSource',
  );

  assert.match(pathDetail, /<PathGoalManagementForm/);
  assert.ok(
    pathDetail.indexOf('<PathGoalManagementForm') < pathDetail.indexOf('</></NativeRouteSource>'),
    'Manage Path must remain inside the native Path-details presentation hierarchy',
  );
});

test('canceling review or dismissing Manage Path preserves destination and timers without issuing an update', () => {
  const cancel = sourceSection('function cancelPathGoalReview()', 'async function confirmPathGoalChanges()');
  assert.match(cancel, /setGoalManagementReview\(null\)/);
  assert.doesNotMatch(cancel, /\.updatePathGoals\(/);
  assert.doesNotMatch(cancel, /setDestination\(/);
  assert.doesNotMatch(cancel, /timers:/);

  const dismiss = sourceSection('function closePathManagement(dirty = false)', 'function reviewPathDeletionIntent()');
  assert.match(dismiss, /resetGoalManagement\(\)/);
  assert.doesNotMatch(dismiss, /\.updatePathGoals\(/);
  assert.doesNotMatch(dismiss, /setDestination\(/);
  assert.doesNotMatch(dismiss, /timers:/);
});

test('only explicit confirmation calls the generated goal endpoint and retains one idempotency key across retries', () => {
  const confirm = sourceSection('async function confirmPathGoalChanges()', 'function closePathManagement(dirty = false)');
  assert.match(confirm, /!goalManagementReview\.changed/);
  assert.match(confirm, /validateSessionCredential<PathGoalMutationResult>/);
  assert.match(confirm, /generatedResponse\(/);
  assert.match(
    confirm,
    /\.updatePathGoals\(pathID,\s*\{\s*expectedGoals:\s*review\.current,\s*\.\.\.review\.proposed,\s*confirmed:\s*true\s*\},\s*idempotencyKey\)/,
  );
  assert.match(confirm, /idempotencyKey\s*=\s*review\.idempotencyKey/);
  assert.doesNotMatch(confirm, /Crypto\.randomUUID\(\)/);

  const management = sourceSection('<PathGoalManagementForm', '{currentPathRouteIntent && !selectedPath ?');
  assert.match(management, /busy=\{goalManagementBusy\}/);
  assert.match(management, /review=\{goalManagementReview \?\? undefined\}/);
  assert.match(managementForm, /disabled:\s*busy \|\| !review\?\.changed/);

  const review = sourceSection('function reviewPathGoalChanges()', 'function cancelPathGoalReview()');
  assert.match(review, /idempotencyKey:\s*Crypto\.randomUUID\(\)/);
});

test('confirmed authoritative goals and progress update Home and selected detail atomically while stale completions are ignored', () => {
  assert.match(page, /const goalManagementOperations = createSessionOperationOwner\(\)/);
  assert.match(page, /const goalManagementTarget = useRef<PathAdministrationTarget<Session> \| null>\(null\)/);

  const confirm = sourceSection('async function confirmPathGoalChanges()', 'function closePathManagement(dirty = false)');
  assert.match(confirm, /const ticket = goalManagementOperations\.issue\(\)/);
  assert.match(confirm, /goalManagementTarget\.current = createPathAdministrationTarget\(ownerID, pathID, currentSession\)/);
  assert.match(confirm, /!ticket\.current\(\)/);
  assert.match(confirm, /ownsPathAdministrationTarget\(goalManagementTarget\.current, ownerID, pathID, currentSession\.token\)/);
  assert.match(confirm, /setDestination\(\(current\) => current\?\.kind === 'home' && current\.profile\.id === ownerID/);
  assert.match(confirm, /paths:\s*current\.profile\.paths\.map\(\(path\) => path\.id === pathID \? \{ \.\.\.result\.path, home: path\.home \} : path\)/);
  assert.match(confirm, /\[pathID\]:\s*\{[\s\S]*?\.\.\.\(current\.profile\.timers\[pathID\]/);
  assert.match(confirm, /accumulatedSeconds:\s*result\.accumulatedSeconds/);
  assert.match(confirm, /intervalProgress:\s*result\.intervalProgress/);
  assert.match(confirm, /setPathMembers\(\(current\) => current\.map\(\(member\) => \(\{ \.\.\.member, intervalProgress: undefined, overallProgress: undefined \}\)\)\);/);
  assert.match(confirm, /void openPathMembers\(pathID, undefined, false\);/);

  const reset = sourceSection('function resetGoalManagement()', 'function resetManualActivity()');
  assert.match(reset, /goalManagementOperations\.invalidate\(\)/);
  assert.match(reset, /goalManagementTarget\.current = null/);
  assert.match(page, /async function clearSession[\s\S]*resetGoalManagement\(\)/);
  assert.match(page, /goalManagementTarget\.current && !goalManagementTarget\.current\.sessionTokens\.includes\(credential\.token\)[\s\S]*rotatePathAdministrationTarget\(goalManagementTarget\.current[\s\S]*resetGoalManagement\(\)/);
});

test('goal management serializes same-Path timer and activity mutations', () => {
  const open = sourceSection('function openPathManagement(path: SessionPath)', 'function updateGoalManagementForm');
  assert.match(open, /manualBusy \|\| timerBusy\[path\.id\]/);

  const timer = sourceSection('async function toggleTimer(pathID: string)', 'function openPathDetail');
  assert.match(timer, /goalManagementPathID === pathID/);
  assert.match(page, /selectedCapabilities\.manageGoals \|\| selectedCapabilities\.renamePath \|\| selectedCapabilities\.manageLifecycle[\s\S]*disabled:\s*pathLeaveBusy \|\| pathRenamePathID === selectedPath\.id \|\| goalManagementBusy \|\| pathDeletionBusy \|\| manualBusy \|\| Boolean\(timerBusy\[selectedPath\.id\]\)[\s\S]*label:\s*i18n\.t\('pathManage\.action'\)/);

  const manual = sourceSection('async function openManualActivity(pathID: string)', 'function changeManualDuration');
  assert.match(manual, /goalManagementPathID === pathID/);
  const submit = sourceSection('async function submitManualActivity()', 'function closeManualActivity');
  assert.match(submit, /goalManagementPathID === manualPathID/);
});
