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

test('web contains the localized, capability-gated rename form inside Manage Path', () => {
  assert.match(page, /\{#if \(selectedCapabilities\.manageGoals \|\| selectedCapabilities\.renamePath \|\| selectedCapabilities\.manageLifecycle \|\| selectedCapabilities\.manageVisibility\) && managingPathGoals && goalForm\}[\s\S]*\{#if selectedCapabilities\.renamePath && renamingPath\}[\s\S]*pathRename\.heading/);
  assert.doesNotMatch(page, />\{i18n\.t\('pathRename\.action'\)\}<\/button>/);
  assert.match(page, /aria-labelledby="path-rename-heading"/);
  assert.match(page, /maxlength="100"/);
  assert.match(page, /pathRename\.nameLabel/);
  assert.match(page, /pathRename\.explanation/);
  assert.match(page, /pathRename\.save/);
  assert.match(page, /common\.cancel/);
});

test('Manage Path cancel is local and invalidates any in-flight rename completion', () => {
  const cancel = sourceBetween('function cancelGoalChanges', 'async function confirmGoalChanges');
  assert.match(cancel, /resetPathRename\(\)/);
  assert.doesNotMatch(cancel, /\.renamePath\(/);
  assert.doesNotMatch(cancel, /\bpaths\s*=/);
  assert.doesNotMatch(cancel, /\bselectedPath\s*=/);
});

test('rename submit is capability checked and applies only the authoritative projection', () => {
  assert.equal([...page.matchAll(/\.renamePath\(/g)].length, 1);
  const submit = sourceBetween('async function submitPathRename', 'function openPathHistory');
  assert.match(submit, /managingPathGoals/);
  assert.match(submit, /effectivePathCapabilities\(selectedPath\)\.renamePath/);
  assert.match(submit, /reviewPathRename\(selectedPath, pathRenameDraft\)/);
  assert.match(submit, /pathRenameOperations\.submit\(review/);
  assert.match(submit, /\.renamePath\(pathID, body, idempotencyKey\)/);
  assert.match(submit, /applyPathRenameResult\(\{ paths, selectedPath \}, result\.path\)/);
  assert.match(submit, /paths = applied\.paths/);
  assert.match(submit, /selectedPath = applied\.selectedPath/);
  assert.doesNotMatch(submit, /timerStates\s*=/);
});

test('closing Path details clears rename state and rejects stale completion', () => {
  const reset = sourceBetween('function resetPathDetails', 'function scheduleExpiration');
  assert.match(reset, /resetPathRename\(\)/);
  assert.match(page, /function resetPathRename\(\)[\s\S]*pathRenameOperations\.cancel\(\)/);
});
