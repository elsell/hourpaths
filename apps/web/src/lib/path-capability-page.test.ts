import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('web restores timers only for server-authorized tracking Paths', () => {
  assert.match(page, /pathsRequiringTimerRestore\(nextPaths\)/);
  assert.match(page, /trackablePaths\.map\(\(path\) =>/);
  assert.match(page, /Object\.fromEntries\(trackablePaths\.map\(/);
});

test('web controls and handlers use effective server-authoritative capabilities', () => {
  assert.match(page, /effectivePathCapabilities\(selectedPath\)\.manageLifecycle/);
  assert.match(page, /const capabilities = effectivePathCapabilities\(selectedPath\);[\s\S]*capabilities\.manageGoals \|\| capabilities\.renamePath \|\| capabilities\.manageLifecycle/);
  assert.match(page, /effectivePathCapabilities\(selectedPath\)\.trackTime/);
  assert.match(page, /\{#if selectedCapabilities\.manageGoals \|\| selectedCapabilities\.renamePath \|\| selectedCapabilities\.manageLifecycle \|\| selectedCapabilities\.manageVisibility\}/);
  assert.match(page, /\{#if selectedCapabilities\.trackTime\}/);
  assert.match(page, /\{#if selectedCapabilities\.manageLifecycle\}/);
  assert.match(page, /\{#if pathCapabilities\.trackTime && state\}/);
});
