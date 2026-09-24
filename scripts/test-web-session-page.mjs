import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

test('the page reads browser session storage once into owned state', () => {
  const page = readFileSync(new URL('../apps/web/src/routes/+page.svelte', import.meta.url), 'utf8');
  assert.equal(page.match(/applicationSession\(\)/g)?.length, 1);
  assert.match(page, /const revocation = revokeApplicationSession\(data\.config, session\);[\s\S]*await revocation;/);
});

test('the callback presents typed session persistence failures consistently', () => {
  const callback = readFileSync(new URL('../apps/web/src/routes/callback/+page.svelte', import.meta.url), 'utf8');
  assert.match(callback, /exchangeSessionFailureMessage\(cause\)/);
  assert.match(callback, /applicationDestination\(nextAction\)/);
  assert.doesNotMatch(callback, /location\.replace\('\/'\)/);
  assert.doesNotMatch(callback, /problemMessageKey/);
});

test('the onboarding surface reads private seeds without calling the active profile endpoint', () => {
  const onboarding = readFileSync(new URL('../apps/web/src/routes/onboarding/+page.svelte', import.meta.url), 'utf8');
  assert.match(onboarding, /\.onboarding\(\)/);
  assert.match(onboarding, /type="email"/);
  assert.match(onboarding, /readonly/);
  assert.match(onboarding, /bind:value=\{displayName\}/);
  assert.match(onboarding, /bind:value=\{username\}/);
  assert.match(onboarding, /autocomplete="username"/);
  assert.doesNotMatch(onboarding, /\.profile\(\)/);
});

test('the recovery surface bounds unreadable browser session state', () => {
  const recovery = readFileSync(new URL('../apps/web/src/routes/account-recovery/+page.svelte', import.meta.url), 'utf8');
  assert.match(recovery, /catch \(cause\)/);
  assert.match(recovery, /webSessionFailure\(failure\)/);
  assert.match(recovery, /clearApplicationSession\(\)/);
  assert.match(recovery, /role="alert"/);
});

test('the recovery decline persists onboarding before navigating and leaves sign-out available', () => {
  const recovery = readFileSync(new URL('../apps/web/src/routes/account-recovery/+page.svelte', import.meta.url), 'utf8');
  const decline = /function declineRecovery[\s\S]*?\n  }/.exec(recovery)?.[0];
  assert.ok(decline);
  assert.match(decline, /declineDuplicateEmailRecovery/);
  assert.match(decline, /declineApplicationRecovery/);
  assert.match(decline, /replaceApplicationLocation\('\/onboarding'\)/);
  assert.ok(
    decline.indexOf('declineDuplicateEmailRecovery') < decline.indexOf('declineApplicationRecovery')
      && decline.indexOf('declineApplicationRecovery') < decline.indexOf("replaceApplicationLocation('/onboarding')"),
    'the server decline and replacement credential must be durable before navigation',
  );
  assert.match(recovery, /duplicateEmailRecovery\.decline/);
  assert.match(recovery, /auth\.signOut/);
  assert.match(decline, /catch \(cause\)/);
  assert.match(decline, /webSessionFailure\(failure\)/);
});

test('the recovery route clears expiry before decline and schedules the same boundary', () => {
  const recovery = readFileSync(new URL('../apps/web/src/routes/account-recovery/+page.svelte', import.meta.url), 'utf8');
  assert.match(recovery, /applicationSessionExpired/);
  assert.match(recovery, /applicationSessionOperations\.invalidate\(\)/);
  assert.match(recovery, /clearApplicationSession\(\)/);
  assert.match(recovery, /setTimeout\(expireSession/);
  const decline = /function declineRecovery[\s\S]*?\n  }/.exec(recovery)?.[0];
  assert.ok(decline);
  assert.ok(
    decline.indexOf('applicationSessionExpired') < decline.indexOf('declineApplicationRecovery'),
    'expiry must win before the recovery transition',
  );
});

test('a rejected recovery decline disposes the credential and returns to sign-in', () => {
  const recovery = readFileSync(new URL('../apps/web/src/routes/account-recovery/+page.svelte', import.meta.url), 'utf8');
  const decline = /function declineRecovery[\s\S]*?\n  }/.exec(recovery)?.[0];
  assert.ok(decline);
  const rejected = /if \(!serverDeclined && presentation\.discardCredential\) \{[\s\S]*?\n      \}/.exec(decline)?.[0];
  assert.ok(rejected);
  assert.match(rejected, /expireSession\(\)/);
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
