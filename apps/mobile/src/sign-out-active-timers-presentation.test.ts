import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const sourceDirectory = dirname(fileURLToPath(import.meta.url));
const source = (relativePath: string) => readFileSync(resolve(sourceDirectory, relativePath), 'utf8');
const accountRoute = source('../app/settings/account.tsx');
const page = source('../app/index.tsx');
const settingsPresentation = source('./ui/settings-presentation.tsx');

test('Account settings uses the native three-choice sign-out decision when timers are running', () => {
  assert.match(settingsPresentation, /runningTimerCount:\s*number/);
  assert.match(settingsPresentation, /signOut:\s*\(resolution:\s*SignOutTimerChoice\)/);
  assert.match(accountRoute, /activePresentation\.runningTimerCount > 0/);
  assert.match(accountRoute, /Alert\.alert\([\s\S]*settings\.account\.activeTimers\.title/);
  assert.match(accountRoute, /settings\.account\.activeTimers\.stopAndSignOut/);
  assert.match(accountRoute, /settings\.account\.activeTimers\.keepRunningAndSignOut/);
  assert.match(accountRoute, /style:\s*'cancel'/);
  assert.doesNotMatch(accountRoute, /Modal|NativeSheet/);
});

test('Account settings leaves the session visible unless timer resolution authorizes sign-out', () => {
  assert.match(accountRoute, /const result = await activePresentation\.signOut\(resolution\)/);
  assert.match(accountRoute, /if \(result\.kind === 'signed_out'\) router\.dismissAll\(\)/);
  assert.match(accountRoute, /setFailureKey\('settings\.account\.activeTimers\.stopFailed'\)/);
  assert.match(accountRoute, /result\.kind === 'choice_required'/);
  assert.match(accountRoute, /signOut\('confirmed_no_timers'\)/);
  assert.match(accountRoute, /<SettingsSection footer=\{ownedFailureKey \? i18n\.t\(ownedFailureKey\) : undefined\}>/);
});

test('Home derives running timer count and delegates safe resolution at the current session boundary', () => {
  assert.match(page, /createSignOutTimerResolutionCoordinator/);
  assert.match(page, /runningTimerCount=\{Object\.values\(ownedHomeDestination\.profile\.timers\)\.filter\(\(state\) => state\.running\)\.length\}/);
  assert.match(page, /signOut=\{resolveTimerSignOut\}/);
  assert.match(page, /notificationLifecycleState\.current/);
  assert.match(page, /current\.session === currentSession/);
  assert.match(page, /authorizeSignOut/);
  assert.match(page, /timerOperations\.stop/);
  assert.match(page, /applyOwnedTimerState\(ownerID, currentSession, timer\.pathId, presentation\.state\)/);
  assert.match(page, /sessionOperations\.invalidate\(\);[\s\S]*await deregisterPushSession\(disposedSession\)/);
  assert.match(page, /active\.session !== disposedSession/);
  assert.match(page, /resetNotifications\(\);[\s\S]*void setNativeNotificationBadge\(0\)/);
});

test('sign-out drains an admitted timer start before taking its authoritative snapshot', () => {
  assert.match(page, /createAsyncMutationBarrier\(\)/);
  assert.match(page, /await timerMutationBarrier\.blockAndDrain\(\);[\s\S]*notificationLifecycleState\.current/);
  assert.match(page, /const mutationLease = timerMutationBarrier\.enter\(\)/);
  assert.match(page, /commitTimerProjectionBeforeRender\([\s\S]*notificationLifecycleState\.current[\s\S]*setDestination/);
  assert.match(page, /finally \{[\s\S]*mutationLease\.release\(\)/);
  assert.match(page, /if \(!signOutCompleted\) timerMutationBarrier\.unblock\(\)/);
});
