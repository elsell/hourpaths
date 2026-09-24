import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('web sign-out uses an accessible native decision dialog with timer-aware choices', () => {
  assert.match(page, /createSignOutTimerResolutionCoordinator/);
  assert.match(page, /\{#if signOutDialogOpen\}[\s\S]*<dialog open/);
  assert.match(page, /aria-labelledby="sign-out-dialog-heading"/);
  assert.match(page, /settings\.account\.activeTimers\.title/);
  assert.match(page, /settings\.account\.activeTimers\.message/);
  assert.match(page, /settings\.account\.activeTimers\.stopAndSignOut/);
  assert.match(page, /settings\.account\.activeTimers\.keepRunningAndSignOut/);
  assert.match(page, /i18n\.t\('common\.cancel'\)/);
});

test('sign-out blocks new timer mutations and drains an acknowledged start before snapshotting', () => {
  assert.match(page, /createAsyncMutationBarrier\(\)/);
  assert.match(page, /await timerMutationBarrier\.blockAndDrain\(\);[\s\S]*Object\.entries\(timerStates\)/);
  assert.match(page, /const mutationLease = timerMutationBarrier\.enter\(\)/);
  assert.match(page, /finally \{[\s\S]*mutationLease\.release\(\)/);
  assert.match(page, /disabled=\{timerBusy\[path\.id\] \|\| timerMutationLocked\}/);
  assert.match(page, /cancelSignOut[\s\S]*timerMutationBarrier\.unblock\(\)/);
});

test('ordinary sign-out confirmation is preserved when no timer is running', () => {
  assert.match(page, /runningEntries\.length === 0/);
  assert.match(page, /settings\.account\.signOutConfirmTitle/);
  assert.match(page, /settings\.account\.signOutConfirmMessage/);
  assert.match(page, /void completeOrdinarySignOut\(\)/);
});

test('stop-and-save retains partial success and only signs out after authorization', () => {
  assert.match(page, /context\.resolution\.stopAndSave/);
  assert.match(page, /timerOperations\.stop/);
  assert.match(page, /timerStates = \{ \.\.\.timerStates, \[timer\.pathId\]: presentation\.state \}/);
  assert.match(page, /if \(!decision\.authorizeSignOut\)/);
  assert.match(page, /settings\.account\.activeTimers\.stopFailed/);
  assert.match(page, /await completeOwnedSignOut\(context\)/);
  assert.doesNotMatch(page, /signOutResolutionContext = null;[\s\S]{0,160}stopFailed/);
});

test('keep-running and stale ownership cannot stop timers or dispose a replacement session', () => {
  assert.match(page, /context\.resolution\.keepRunning\(\)/);
  assert.match(page, /function ownsSignOutResolution/);
  assert.match(page, /session === context\.session/);
  assert.match(page, /profile\?\.id === context\.ownerID/);
  assert.match(page, /signOutTimerResolutions\.invalidate\(\)/);
  assert.match(page, /if \(!ownsSignOutResolution\(context\)\) \{[\s\S]*signOutTimerResolutions\.invalidate\(\);[\s\S]*return/);
});
