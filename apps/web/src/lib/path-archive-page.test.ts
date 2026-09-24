import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('web loads a distinct archived collection and keeps archived Home cards free of tracking controls', () => {
  assert.match(page, /\.archivedPaths\(\)/);
  assert.match(page, /let archivedPaths: SessionPath\[\] = \[\]/);
  assert.match(page, /home\.activePaths/);
  assert.match(page, /home\.archivedPaths/);
  const archivedList = page.slice(page.indexOf('{:else if showingArchived}'), page.indexOf('{:else if paths.length === 0}'));
  assert.match(archivedList, /archivedPaths as path/);
  assert.match(archivedList, /pathArchive\.readOnly/);
  assert.doesNotMatch(archivedList, /toggleTimer|openManualActivity|pathManage\.action/);
});

test('web archive lifecycle requires confirmation and applies only the authoritative result', () => {
  assert.match(page, /reviewPathArchiveChange\(selectedPath\)/);
  assert.match(page, /pathArchive\.warning/);
  assert.match(page, /pathArchive\.unarchiveWarning/);
  assert.match(page, /archiveOperations\.submit\(review, true/);
  assert.match(page, /\.setPathArchiveState\(pathID, body as PathArchiveStateDraft, idempotencyKey\)/);
  assert.match(page, /applyPathArchiveResult\(\{ activePaths: paths, archivedPaths, selectedPath, timerStates: restoredTimerStates \}, result\.path\)/);
  const cancel = page.slice(page.indexOf('function cancelArchiveChange'), page.indexOf('async function confirmArchiveChange'));
  assert.doesNotMatch(cancel, /setPathArchiveState|applyPathArchiveResult/);
});

test('web unarchive restores an authoritative stopped timer projection without restarting it', () => {
  const confirm = page.slice(page.indexOf('async function confirmArchiveChange'), page.indexOf('function openPathManagement'));
  assert.match(confirm, /if \(!body\.archived && effectivePathCapabilities\(path\)\.trackTime\)[\s\S]*\.currentTimer\(pathID\)/);
  assert.match(confirm, /unarchivedTimer[\s\S]*running: false,[\s\S]*timer: undefined/);
  assert.match(confirm, /timerStates: restoredTimerStates/);
});

test('archived details retain history while hiding tracking and activity mutation controls', () => {
  assert.match(page, /\{#if selectedPath\.archivedAt\}<p>\{i18n\.t\('pathArchive\.readOnly'\)\}<\/p>\{\/if\}/);
  assert.match(page, /\{#if selectedCapabilities\.manageGoals \|\| selectedCapabilities\.renamePath \|\| selectedCapabilities\.manageLifecycle \|\| selectedCapabilities\.manageVisibility\}[\s\S]*pathManage\.action[\s\S]*\{\/if\}/);
  assert.match(page, /\{#if selectedCapabilities\.trackTime\}[\s\S]*activity\.add[\s\S]*\{\/if\}/);
  assert.match(page, /pathDetails\.openHistory/);
  assert.match(page, /\{#if selectedCapabilities\.trackTime && selectedActivity\.activity\.participantId === profile\.id\}/);
});
