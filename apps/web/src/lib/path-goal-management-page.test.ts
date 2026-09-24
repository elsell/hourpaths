import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

function sourceBetween(start: string, end: string): string {
  const from = page.indexOf(start);
  assert.notEqual(from, -1, `missing ${start}`);
  const to = page.indexOf(end, from + start.length);
  assert.notEqual(to, -1, `missing ${end} after ${start}`);
  return page.slice(from, to);
}

test('Manage Path keeps editable and reviewed goal state separate and presents the localized comparison warning', () => {
  assert.match(page, /let managingPathGoals = false;/);
  assert.match(page, /let goalForm: GoalFormState \| null = null;/);
  assert.match(page, /let goalReview: GoalConfigurationComparison \| null = null;/);
  assert.match(page, /function openPathManagement\(\)[\s\S]*goalFormState\(selectedPath\)/);

  const review = sourceBetween('function reviewGoalChanges', 'function cancelGoalChanges');
  assert.match(review, /buildGoalDraft\(goalForm\)/);
  assert.match(review, /compareGoalConfigurations\([\s\S]*intervalGoal: selectedPath\.intervalGoal[\s\S]*overallTarget: selectedPath\.overallTarget[\s\S]*goalDraft\.goals/);
  assert.doesNotMatch(review, /\.updatePathGoals\(/);

  assert.match(page, /pathManage\.action/);
  assert.match(page, /pathManage\.heading/);
  assert.match(page, /\{#if goalReview\}[\s\S]*pathManage\.current[\s\S]*pathManage\.proposed/);
  assert.match(page, /\{#if goalReview\}[\s\S]*pathManage\.goalWarning/);
  assert.match(page, /onclick=\{cancelGoalChanges\}[\s\S]*common\.cancel/);
  assert.match(page, /onclick=\{\(\) => void confirmGoalChanges\(\)\}[\s\S]*pathManage\.confirm/);
});

test('cancel closes goal management without issuing a mutation or replacing authoritative Path and timer state', () => {
  const cancel = sourceBetween('function cancelGoalChanges', 'async function confirmGoalChanges');
  assert.match(cancel, /resetGoalManagement\(\)/);
  assert.doesNotMatch(cancel, /\.updatePathGoals\(/);
  assert.doesNotMatch(cancel, /\bpaths\s*=/);
  assert.doesNotMatch(cancel, /\bselectedPath\s*=/);
  assert.doesNotMatch(cancel, /\btimerStates\s*=/);
});

test('confirmation alone owns the generated mutation and atomically applies its authoritative result', () => {
  assert.equal([...page.matchAll(/\.updatePathGoals\(/g)].length, 1);
  assert.match(page, /applyGoalMutationResult/);
  assert.match(page, /const goalManagementOperations = createSessionOperationOwner\(\)/);

  const confirmation = sourceBetween('async function confirmGoalChanges', 'function openPathHistory');
  assert.match(confirmation, /const ownerID = profile\.id;/);
  assert.match(confirmation, /const pathID = selectedPath\.id;/);
  assert.match(confirmation, /const ticket = goalManagementOperations\.issue\(\);/);
  assert.match(confirmation, /if \(!goalUpdateIdempotencyKey\) goalUpdateIdempotencyKey = crypto\.randomUUID\(\);[\s\S]*const idempotencyKey = goalUpdateIdempotencyKey;/);
  assert.match(confirmation, /confirmed: true,[\s\S]*expectedGoals: goalReview\.current,[\s\S]*\.\.\.goalReview\.proposed/);
  assert.match(confirmation, /\.updatePathGoals\(pathID, body, idempotencyKey\)/);
  assert.match(confirmation, /if \(!ticket\.current\(\) \|\| goalUpdateOwnerID !== ownerID \|\| selectedPath\?\.id !== pathID \|\| session !== current\) return;/);
  assert.match(confirmation, /if \(result\.path\.id !== pathID\)[\s\S]*errors\.apiRejected[\s\S]*return;/);
  assert.match(confirmation, /const applied = applyGoalMutationResult\(\{\s*paths,\s*selectedPath,\s*timerStates\s*\}, result\);/);
  assert.match(confirmation, /paths = applied\.paths[\s\S]*selectedPath = applied\.selectedPath[\s\S]*timerStates = applied\.timerStates/);
  assert.match(confirmation, /pathMembers = pathMembers\.map\(\(member\) => \(\{ \.\.\.member, intervalProgress: undefined, overallProgress: undefined \}\)\);/);
  assert.match(confirmation, /void openPathMembers\('', true, false\);/);

  const failure = confirmation.slice(confirmation.indexOf('catch (cause)'));
  assert.doesNotMatch(failure, /crypto\.randomUUID\(\)/);
});

test('goal-management reset invalidates stale owner and Path completions and clears its replay key', () => {
  const reset = sourceBetween('function resetGoalManagement', 'function resetPathDetails');
  assert.match(reset, /goalManagementOperations\.invalidate\(\)/);
  assert.match(reset, /goalUpdateIdempotencyKey = '';/);
  assert.match(reset, /goalUpdateOwnerID = null;/);
  assert.match(reset, /goalReview = null;/);
  assert.match(reset, /goalForm = null;/);
  assert.match(page, /function resetPathDetails\(\)[\s\S]*resetGoalManagement\(\)/);
});

test('goal management is mutually exclusive with same-Path activity mutations', () => {
  const open = sourceBetween('function openPathManagement', 'function reviewGoalChanges');
  assert.match(open, /manualBusy \|\| deleteBusy \|\| timerBusy\[selectedPath\.id\]/);
  assert.match(page, /disabled=\{managingPathGoals \|\| manualBusy \|\| deleteBusy \|\| pathDeletionBusy \|\| timerBusy\[selectedPath\.id\]/);
  assert.match(page, /function openManualActivity[\s\S]*if \([^\n]*managingPathGoals[^\n]*\) return;/);
  assert.match(page, /async function submitManualActivity[\s\S]*if \([^\n]*managingPathGoals[^\n]*\) return;/);
  assert.match(page, /async function confirmDeleteActivity[\s\S]*if \([^\n]*managingPathGoals[^\n]*\) return;/);
});

test('goal confirmation owns the web profile-refresh boundary', () => {
  assert.match(page, /let sessionOperationBusy = false;/);
  const profileLoad = sourceBetween('async function attemptProfile', 'async function attemptRefresh');
  assert.match(profileLoad, /nested = false/);
  assert.match(profileLoad, /if \(goalUpdateBusy \|\| \(sessionOperationBusy && !nested\)\) return;/);
  assert.match(profileLoad, /if \(!ticket\.current\(\) \|\| goalUpdateBusy\) return;/);

  const refresh = sourceBetween('async function attemptRefresh', 'function scheduleRefresh');
  assert.match(refresh, /if \(!session \|\| goalUpdateBusy \|\| sessionOperationBusy\) return;/);
  assert.match(refresh, /await attemptProfile\(next, next\.expiresAt, ticket, true\)/);

  const confirmation = sourceBetween('async function confirmGoalChanges', 'async function openPathHistory');
  assert.match(confirmation, /sessionOperationBusy/);
  assert.doesNotMatch(confirmation, /applicationSessionOperations\.invalidate\(\)/);
  assert.match(confirmation, /if \(refreshTimer\) clearTimeout\(refreshTimer\);/);
  assert.match(confirmation, /session !== current/);
  assert.match(confirmation, /resetGoalManagement\(\);[\s\S]*scheduleOwnedSession\(\);/);
  assert.match(page, /disabled=\{goalUpdateBusy \|\| sessionOperationBusy \|\| !goalReview\.changed\}/);
});
