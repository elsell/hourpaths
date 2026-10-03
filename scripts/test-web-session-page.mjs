import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

test('legacy sign-out awaits locally owned revocation', () => {
  const page = readFileSync(new URL('../apps/web/src/routes/+page.svelte', import.meta.url), 'utf8');
  assert.match(page, /const revocation = revokeApplicationSession\(data\.config, session\);[\s\S]*await revocation;/);
});

test('profile-only retry does not rotate the authoritative browser credential', () => {
  const page = readFileSync(new URL('../apps/web/src/routes/+page.svelte', import.meta.url), 'utf8');
  assert.match(page, /retryOperation/);
  assert.match(page, /retryOperation === 'profile'/);
  assert.match(page, /attemptProfile\(session/);
  const profileRecovery = /async function attemptProfile[\s\S]*?\n  }\n\n  async function attemptRefresh/.exec(page)?.[0];
  assert.ok(profileRecovery);
  assert.doesNotMatch(profileRecovery, /refreshApplicationSession/);
});

test('active Home loads the server Path collection and presents the specified empty state', () => {
  const page = readFileSync(new URL('../apps/web/src/routes/+page.svelte', import.meta.url), 'utf8');
  assert.match(page, /\.paths\(\)/);
  assert.match(page, /home\.empty\.explanation/);
  assert.match(page, /home\.createPath/);
  assert.match(page, /paths\.length === 0/);
});

test('Path creation is controlled, retryable, and immediately trackable', () => {
  const page = readFileSync(new URL('../apps/web/src/routes/+page.svelte', import.meta.url), 'utf8');
  assert.match(page, /createPathSubmissionOwner/);
  assert.match(page, /\.createPath\(body, idempotencyKey\)/);
  assert.match(page, /paths = \[\.\.\.paths, result\.path\]/);
  assert.match(page, /pathCreate\.submitting/);
  assert.match(page, /pathCreate\.retry/);
  assert.match(page, /timerStates = \{[\s\S]*?\.\.\.timerStates,[\s\S]*?\[result\.path\.id\]:/);
  assert.match(page, /pathCreation\.cancel\(\)/);
  assert.doesNotMatch(page, /fetch\(/);
  assert.doesNotMatch(page, /fetch\(/);
});

test('Home restores and controls each per-Path timer through the generated client', () => {
  const page = readFileSync(new URL('../apps/web/src/routes/+page.svelte', import.meta.url), 'utf8');
  assert.match(page, /\.currentTimer\(path\.id\)/);
  assert.match(page, /createTimerOperationOwner/);
  assert.match(page, /\.startTimer\(pathID, idempotencyKey\)/);
  assert.match(page, /\.stopTimer\(pathID, timerID, idempotencyKey\)/);
  assert.match(page, /activeTimerSeconds\(state\.timer\?\.startedAt, now\)/);
  assert.match(page, /const presentation = timerMutationPresentation\(result\.state\)/);
  assert.match(page, /timerStates = \{ \.\.\.timerStates, \[pathID\]: presentation\.state \}/);
  assert.match(page, /if \(presentation\.notice === 'subsecond'\) timerNoticeKey = 'timer\.subsecondNotice'/);
  assert.match(page, /\{#if timerNoticeKey\}<p role="status">\{i18n\.t\(timerNoticeKey\)\}<\/p>\{\/if\}/);
  assert.doesNotMatch(page, /pathCreate\.trackingUnavailable/);
});
