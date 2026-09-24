import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('web Home cards and Path details conditionally render current interval progress', () => {
  assert.match(page, /intervalProgressPresentation/);
  assert.match(page, /state(?:\?\.|\.)intervalProgress/);
  assert.match(page, /selectedTimerState(?:\?\.|\.)intervalProgress/);
  assert.match(page, /\{#if [^}]*IntervalProgress\}[\s\S]*<progress/);
  assert.doesNotMatch(page, /intervalProgress\.accumulatedSeconds\s*>\s*0/);
});

test('web interval progress exposes zero, partial, and over-target state without color-only meaning', () => {
  assert.match(page, /<progress[^>]*max=\{[^}]*targetSeconds\}[^>]*value=\{[^}]*visualSeconds\}[^>]*aria-label=\{/);
  assert.match(page, /path\.progress\.interval(?:Complete)?/);
  assert.match(page, /accumulatedSeconds/);
  assert.match(page, /targetSeconds/);
  assert.match(page, /<p[^>]*>\{[^}]*intervalProgressMessage\([^)]*\)\}<\/p>/);
});

test('web refreshes current interval progress from every authoritative activity result', () => {
  const timerMutation = page.slice(page.indexOf('async function toggleTimer'), page.indexOf('function manualLocalNow'));
  const manualMutation = page.slice(page.indexOf('async function submitManualActivity'), page.indexOf('function closeManualActivity'));
  const deletion = page.slice(page.indexOf('async function confirmDeleteActivity'), page.indexOf('async function inspectActivity'));

  assert.match(timerMutation, /\[pathID\]: presentation\.state/);
  assert.match(manualMutation, /\.updateActivity\(/);
  assert.match(manualMutation, /\.createManualActivity\(/);
  assert.match(manualMutation, /intervalProgress:\s*result\.intervalProgress/);
  assert.match(deletion, /\.deleteActivity\(/);
  assert.match(deletion, /intervalProgress:\s*result\.intervalProgress/);
});

test('web immediately presents zero interval progress for a newly created interval goal', () => {
  const creation = page.slice(page.indexOf('async function submitPath'), page.indexOf('function recurrenceLabel'));

  assert.match(creation, /result\.path\.intervalGoal/);
  assert.match(creation, /\{\s*accumulatedSeconds:\s*0,\s*targetSeconds:\s*result\.path\.intervalGoal\.targetSeconds\s*\}/);
});
