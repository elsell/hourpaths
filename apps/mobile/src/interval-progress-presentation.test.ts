import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const progressIndicator = readFileSync(fileURLToPath(new URL('./ui/progress-indicator.tsx', import.meta.url)), 'utf8');

test('native Home cards and Path details conditionally render current interval progress', () => {
  assert.match(page, /intervalProgressPresentation/);
  assert.match(page, /state(?:\?\.|\.)intervalProgress/);
  assert.match(page, /selectedTimerState(?:\?\.|\.)intervalProgress/);
  assert.match(page, /<IntervalProgressIndicator compact progress=\{currentIntervalProgress\} pathName=\{path\.name\} \/>/);
  assert.match(page, /intervalProgress=\{selectedIntervalProgress \? <IntervalProgressIndicator progress=\{selectedIntervalProgress\} \/> : undefined\}/);
  assert.doesNotMatch(page, /intervalProgress\.accumulatedSeconds\s*>\s*0/);
});

test('native interval progress exposes zero, partial, and over-target state without color-only meaning', () => {
  assert.match(page, /function IntervalProgressIndicator/);
  assert.match(page, /accessibilityLabel=\{[^}]*interval/);
  assert.match(page, /targetValue=\{progress\.targetSeconds\}/);
  assert.match(page, /visualValue=\{progress\.visualSeconds\}/);
  assert.match(progressIndicator, /accessibilityRole="progressbar"/);
  assert.match(progressIndicator, /accessibilityValue=\{\{\s*min:\s*0,\s*\.\.\.accessibilityProgress,\s*text/);
  assert.match(page, /path\.progress\.intervalDetail(?:Complete)?/);
  assert.match(progressIndicator, /<Text[^>]*>\{text\}<\/Text>/);
});

test('native refreshes current interval progress from supported stop, manual create, and edit results', () => {
  const timerMutation = page.slice(page.indexOf('async function toggleTimer'), page.indexOf('function openPathDetail'));
  const manualMutation = page.slice(page.indexOf('async function submitManualActivity'), page.indexOf('function closeManualActivity'));

  assert.match(timerMutation, /applyOwnedTimerState\(ownerID, currentSession, pathID, presentation\.state\)/);
  assert.match(page, /timers: \{ \.\.\.current\.destination\.profile\.timers, \[pathID\]: state \}/);
  assert.match(manualMutation, /\.updateActivity\(/);
  assert.match(manualMutation, /\.createManualActivity\(/);
  assert.match(manualMutation, /intervalProgress:\s*result\.intervalProgress/);
});

test('native immediately presents zero interval progress for a newly created interval goal', () => {
  const creation = page.slice(page.indexOf('async function createPath'), page.indexOf('async function toggleTimer'));

  assert.match(creation, /createdPath\.intervalGoal/);
  assert.match(creation, /\{\s*accumulatedSeconds:\s*0,\s*targetSeconds:\s*createdPath\.intervalGoal\.targetSeconds\s*\}/);
});

test('shared progress track clamps visual and numeric values while preserving uncapped semantic text', () => {
  assert.match(page, /pathName\s*\?\s*i18n\.t\('path\.progress\.intervalLabelForPath',\s*\{\s*path:\s*pathName\s*\}\)\s*:\s*i18n\.t\('path\.progress\.intervalLabel'\)/);
  assert.match(page, /function IntervalProgressIndicator[\s\S]*?<ProgressIndicator[\s\S]*?targetValue=\{progress\.targetSeconds\}[\s\S]*?visualValue=\{progress\.visualSeconds\}/);
  assert.match(progressIndicator, /Math\.min\(100,\s*Math\.max\(0,\s*visualPercent\)\)/);
  assert.match(progressIndicator, /boundedAccessibilityProgress\(targetValue, visualValue\)/);
  assert.match(progressIndicator, /accessibilityValue=\{\{[\s\S]*\.\.\.accessibilityProgress[\s\S]*text/);
  assert.match(progressIndicator, /width:\s*`\$\{clampedVisualPercent\}%`/);
});
