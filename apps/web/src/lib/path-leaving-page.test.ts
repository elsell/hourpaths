import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('web offers a retained-default activity choice only to current track-capable members', () => {
  assert.match(page, /selectedCapabilities\.leavePath/);
  assert.match(page, /reviewPathLeave\(selectedPath, true\)/);
  assert.match(page, /selectedCapabilities\.trackTime[\s\S]*pathLeave\.keepActivity[\s\S]*pathLeave\.deleteActivity/);
  assert.match(page, /checked=\{pathLeaveReview\.retainActivity\}/);
  assert.match(page, /pathLeave\.supporterWarning/);
  assert.doesNotMatch(page, /leavePath:\s*!.*manageLifecycle/);
});

test('web leave dialog is modal, keyboard cancellable, labelled, busy, and restores trigger focus', () => {
  assert.match(page, /<dialog[\s\S]*use:showModal[\s\S]*role="alertdialog"/);
  assert.match(page, /aria-labelledby="path-leave-heading"/);
  assert.match(page, /aria-describedby="path-leave-warning path-leave-choice-description"/);
  assert.match(page, /oncancel=\{\(event\) => \{ event\.preventDefault\(\); cancelPathLeave\(\); \}\}/);
  assert.match(page, /Promise\.resolve\(\)\.then\(\(\) => returnFocus\?\.focus\(\)\)/);
  assert.match(page, /pathLeaveInteractionBlocked = pathLeaveReview !== null \|\| pathLeaveBusy/);
  assert.match(page, /disabled=\{pathLeaveInteractionBlocked\} onclick=\{closePathDetails\}/);
  assert.match(page, /pathLeave\.confirmDelete/);
  assert.match(page, /pathLeave\.leaving/);
  assert.match(page, /pathLeaveOperations\.submit\(review, true/);
  const cancel = page.slice(page.indexOf('function cancelPathLeave'), page.indexOf('async function confirmPathLeave'));
  assert.doesNotMatch(cancel, /\.leavePath\(|applyPathLeaveResult/);
});

test('successful retained and deleted leave reconcile every local access surface and failures remain actionable', () => {
  assert.match(page, /\.leavePath\(pathID, body, idempotencyKey\)/);
  assert.match(page, /applyPathLeaveResult\(\{ activePaths: paths, archivedPaths, selectedPath, timerStates \}, result\)/);
  assert.match(page, /paths = applied\.activePaths[\s\S]*archivedPaths = applied\.archivedPaths[\s\S]*selectedPath = applied\.selectedPath[\s\S]*timerStates = applied\.timerStates/);
  assert.match(page, /errors\.temporarilyUnavailable/);
  assert.match(page, /role="status"[\s\S]*pathLeave\.completedRetained[\s\S]*pathLeave\.completedDeleted/);
});
