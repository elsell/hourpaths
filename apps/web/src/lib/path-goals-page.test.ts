import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('Path creation submits the independently optional goal draft through the generated client', () => {
  assert.match(page, /buildGoalDraft\(\{/);
  assert.match(page, /const draft: PathCreateDraft = \{ name: pathName, \.\.\.goalDraft\.goals \}/);
  assert.match(page, /\.createPath\(body, idempotencyKey\)/);
  assert.match(page, /bind:checked=\{intervalGoalEnabled\}/);
  assert.match(page, /bind:checked=\{overallTargetEnabled\}/);
});

test('Path creation exposes exactly the five supported recurrences and calendar overrides', () => {
  const creationStart = page.indexOf('<section aria-labelledby="create-path-heading">');
  const creationEnd = page.indexOf('</section>', creationStart);
  const creation = page.slice(creationStart, creationEnd);
  const options = [...creation.matchAll(/<option value="(hourly|daily|weekly|monthly|yearly)">/g)]
    .map((match) => match[1]);
  assert.deepEqual(options, ['hourly', 'daily', 'weekly', 'monthly', 'yearly']);
  assert.match(creation, /bind:checked=\{customAlignment\}/);
  assert.match(creation, /intervalRecurrence === 'weekly'/);
  assert.match(creation, /intervalRecurrence === 'yearly'/);
});

test('Home cards omit missing goals and present every configured goal', () => {
  assert.match(page, /\{#if path\.intervalGoal\}/);
  assert.match(page, /path\.goal\.intervalSummary/);
  assert.match(page, /\{#if path\.overallTarget\}/);
  assert.match(page, /overallProgress\(state\.accumulatedSeconds, path\.overallTarget\)/);
});

test('Home and Path details present authoritative accumulated and uncapped overall progress accessibly', () => {
  assert.match(page, /overallProgress/);
  assert.match(page, /path\.progress\.accumulated/);
  assert.match(page, /path\.progress\.overall(?:Complete)?/);
  assert.match(page, /<progress[^>]*max=\{[^}]*targetSeconds\}[^>]*value=\{[^}]*visualSeconds\}[^>]*aria-label=\{/);
  assert.match(page, /\{#if path\.overallTarget\}[\s\S]*overallProgress\(state\.accumulatedSeconds, path\.overallTarget\)/);
  assert.match(page, /\{#if selectedPath\.overallTarget\}[\s\S]*overallProgress\(timerStates\[selectedPath\.id\]!?\.accumulatedSeconds, selectedPath\.overallTarget\)/);
});

test('Home renders the control selected from the authoritative timer mutation state', () => {
  assert.match(page, /\[pathID\]: presentation\.state/);
  assert.match(page, /timerMutationPresentation\(state\)\.controlMessage/);
  assert.match(page, /\{#if state\.running\}<span aria-label=\{i18n\.t\('timer\.elapsedValue', \{ duration: formatTimerDuration\(activeTimerSeconds\(state\.timer\?\.startedAt, now\)\) \}\)\}>/);
  assert.match(page, /activeTimerSeconds\(state\.timer\?\.startedAt, now\)/);
});

test('manual private state is session-owned and late responses cannot repopulate it', () => {
  assert.match(page, /const manualOperations = createSessionOperationOwner\(\)/);
  assert.match(page, /function resetManualActivity\(\)/);
  assert.match(page, /manualPathID = null;[\s\S]*manualForm = null;[\s\S]*manualNote = '';[\s\S]*manualBusy = false;[\s\S]*manualActivity = null;[\s\S]*manualIdempotencyKey = '';/);
  assert.match(page, /const ticket = manualOperations\.issue\(\);[\s\S]*if \(!ticket\.current\(\) \|\| manualOwnerID !== ownerID\) return;/);
  assert.match(page, /function closeManualActivity\(\) \{\s*resetManualActivity\(\);/);
  assert.match(page, /async function signOut\(\)[\s\S]*resetManualActivity\(\);/);
  assert.match(page, /if \(profile && profile\.id !== home\.profile\.id\) \{ resetManualActivity\(\); resetPathDetails\(\); resetInvitations\(\); \}/);
  const signOut = page.slice(page.indexOf('async function signOut()'), page.indexOf('function openPathCreation()'));
  assert.ok(signOut.indexOf('resetManualActivity()') >= 0);
  assert.ok(signOut.indexOf('applicationSessionOperations.invalidate()') < signOut.indexOf('resetManualActivity()'));
  assert.ok(signOut.indexOf('resetManualActivity()') < signOut.indexOf('const revocation = revokeApplicationSession'));
  assert.ok(signOut.indexOf('timerNoticeKey = null') < signOut.indexOf('await revocation'));
});

test('Home opens a selected Path shell and keeps manual entry out of Home cards', () => {
  assert.match(page, /function openPathDetails\(path: SessionPath\)/);
  assert.match(page, /onclick=\{\(\) => void openPathDetails\(path\)\}/);
  assert.match(page, /\{#if selectedPath\}[\s\S]*activity\.add/);
  const homeList = page.slice(page.indexOf('<ul class="path-list">'), page.indexOf('</ul>', page.indexOf('<ul class="path-list">')));
  assert.doesNotMatch(homeList, /openManualActivity|activity\.add/);
});

test('Path details require an explicit action before rendering session history', () => {
  const openDetails = page.slice(page.indexOf('function openPathDetails'), page.indexOf('function invitationFailureKey'));
  assert.doesNotMatch(openDetails, /\.activities\(/);
  assert.match(page, /onclick=\{\(\) => void openPathHistory\(\)\}>\{i18n\.t\('pathDetails\.openHistory'\)\}/);
  assert.match(page, /\{#if historyOpen\}\s*<h3>\{i18n\.t\('pathDetails\.history'\)\}/);
});

test('Path history uses generated reads, owner-only editing, and refreshes after mutation', () => {
  assert.match(page, /\.activities\(pathID, (?:cursor|undefined)(?:, [^)]+)?\)/);
  assert.match(page, /\.activity\(pathID, activityID\)/);
  assert.match(page, /\.activityRevisions\(pathID, activityID, (?:cursor|undefined)\)/);
  assert.match(page, /selectedActivity\.activity\.participantId === profile\.id/);
  assert.match(page, /manualForm = activityEditForm\(activity\.activity\)/);
  assert.match(page, /await refreshPathActivity\(pathID, result\.activity\.id, ticket, ownerID\)/);
});

test('activity owners receive an explicit, cancellable delete confirmation', () => {
  assert.match(page, /selectedActivity\.activity\.participantId === profile\.id[\s\S]*beginDeleteSelectedActivity/);
  assert.match(page, /deleteConfirmActivityID === selectedActivity\.activity\.id[\s\S]*pathDetails\.deleteConfirmation/);
  assert.match(page, /onclick=\{cancelDeleteActivity\}[\s\S]*common\.cancel/);
  assert.match(page, /onclick=\{\(\) => void confirmDeleteActivity\(\)\}[\s\S]*pathDetails\.confirmDelete/);
});

test('confirmed deletion uses the generated client and applies only its authoritative result', () => {
  const deletion = page.slice(page.indexOf('async function confirmDeleteActivity'), page.indexOf('async function inspectActivity'));
  assert.match(deletion, /\.deleteActivity\(pathID, activityID, idempotencyKey\)/);
  assert.match(deletion, /if \(!deleteIdempotencyKey\) deleteIdempotencyKey = crypto\.randomUUID\(\);[\s\S]*const idempotencyKey = deleteIdempotencyKey;/);
  assert.match(deletion, /removeActivity\(activityHistoryItems, activityID\)/);
  assert.match(deletion, /activityDays = groupActivitiesNewestFirst\(activityHistoryItems\)/);
  assert.match(deletion, /accumulatedSeconds: result\.accumulatedSeconds/);
  assert.match(deletion, /resetManualActivity\(\)/);
  assert.match(deletion, /selectedActivity = null;[\s\S]*activityRevisions = \[\]/);
});

test('activity deletion retries retain their key and stale completions cannot cross an owner or resource', () => {
  const deletion = page.slice(page.indexOf('async function confirmDeleteActivity'), page.indexOf('async function inspectActivity'));
  assert.match(page, /const deleteOperations = createSessionOperationOwner\(\)/);
  assert.match(page, /function resetDeleteActivity\(\)[\s\S]*deleteOperations\.invalidate\(\)/);
  assert.match(page, /if \(!ticket\.current\(\) \|\| deleteOwnerID !== ownerID \|\| selectedPath\?\.id !== pathID \|\| selectedActivity\?\.activity\.id !== activityID \|\| deleteConfirmActivityID !== activityID\) return;/);
  assert.match(page, /catch \(cause\)[\s\S]*if \(ticket\.current\(\)[\s\S]*handlePathDetailFailure\(cause\)/);
  assert.doesNotMatch(deletion.slice(deletion.indexOf('catch (cause)')), /crypto\.randomUUID\(\)/);
  assert.match(page, /function resetPathDetails\(\)[\s\S]*resetDeleteActivity\(\)/);
  assert.match(page, /async function inspectActivity\(activityID: string\)[\s\S]*resetDeleteActivity\(\)/);
});

test('Path history identifies participants, marks projected edits, and exposes full retained revisions', () => {
  assert.match(page, /data-activity-id=\{item\.activity\.id\}/);
  assert.match(page, /item\.version > 1 \? 'pathDetails\.historyRowEdited' : 'pathDetails\.historyRow'/);
  assert.match(page, /participant: item\.activity\.participantId/);
  assert.match(page, /new Date\(revision\.startedAt\)[^\n]*revision\.occurrenceTimeZone/);
  assert.match(page, /new Date\(revision\.endedAt\)[^\n]*revision\.occurrenceTimeZone/);
  assert.match(page, /i18n\.number\(revision\.durationSeconds\)/);
  assert.match(page, /selectedActivity\.activity\.participantId === profile\.id && revision\.note/);
  assert.match(page, /pathDetails\.priorNote/);
});

test('Path-detail private state has explicit operation ownership and reset boundaries', () => {
  assert.match(page, /const pathDetailOperations = createSessionOperationOwner\(\)/);
  assert.match(page, /function resetPathDetails\(\)[\s\S]*pathDetailOperations\.invalidate\(\)/);
  assert.match(page, /if \(!ticket\.current\(\) \|\| detailOwnerID !== ownerID \|\| selectedPath\?\.id !== pathID\) return;/);
  assert.match(page, /async function signOut\(\)[\s\S]*resetPathDetails\(\);/);
});

test('manual failures preserve semantic session messages and refresh cannot turn a saved mutation into a failed save', () => {
  const manualFailure = page.slice(page.indexOf('function handleManualFailure'), page.indexOf('function handlePathDetailFailure'));
  assert.match(manualFailure, /webSessionFailure\(failure\)/);
  assert.match(manualFailure, /presentation\.retryable \? 'errors\.temporarilyUnavailable' : presentation\.message/);
  assert.match(manualFailure, /if \(!presentation\.discardCredential\)/);
  const submission = page.slice(page.indexOf('async function submitManualActivity'), page.indexOf('function closeManualActivity'));
  assert.match(submission, /manualActivity = \{ id: result\.activity\.id, version: result\.version \};[\s\S]*try \{\s*await refreshPathActivity/);
  assert.match(submission, /catch \(cause\) \{\s*if \(ticket\.current\(\)\) handlePathDetailFailure\(cause\);\s*\}/);
});

test('Path and revision histories request owned cursor pages and expose localized continuation controls', () => {
  assert.match(page, /\.activities\(pathID, cursor\)/);
  assert.match(page, /\.activityRevisions\(pathID, activityID, cursor\)/);
  assert.match(page, /mergeActivityHistory\(activityHistoryItems, page\.items, replace\)/);
  assert.match(page, /mergeRevisionHistory\(activityRevisions, page\.items, replace\)/);
  assert.match(page, /i18n\.t\('common\.loadMore'\)/);
  assert.match(page, /i18n\.t\('common\.retry'\)/);
});

test('failed continuation pages retain rows and cursors while stale completions cannot cross owner or resource', () => {
  assert.match(page, /activityRetryCursor = cursor \?\? null/);
  assert.match(page, /revisionRetryCursor = cursor \?\? null/);
  assert.match(page, /if \(!ticket\.current\(\) \|\| detailOwnerID !== ownerID \|\| selectedPath\?\.id !== pathID\) return;/);
  assert.match(page, /selectedActivity\?\.activity\.id !== activityID/);
  assert.match(page, /activityNextCursor = page\.nextCursor \?\? null/);
  assert.match(page, /revisionNextCursor = page\.nextCursor \?\? null/);
  assert.match(page, /activityRevisions\.length === 0\}[\s\S]*!revisionPageFailed[\s\S]*pathDetails\.revisionsEmpty/);
});
