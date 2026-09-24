import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  nativeBackSettingsJourneyState,
  normalizeSettingsJourneyRouteState,
} from './settings-journey-route-ancestry';
import {
  queueSettingsJourneyBootstrap,
  settingsJourneyIntentFromPathname,
  takeSettingsJourneyBootstrap,
} from './settings-journey-route-recovery';
import { ownsBlockedAccountLoad } from './blocked-account-operation';
import { shouldHandleSettingsOperationFailure } from './settings-operation-ownership';
import { ownsSettingsRouteState, settingsRouteOwnerDecision } from './settings-route-state';

const source = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');
const routes = [
  source('../app/settings/index.tsx'),
  source('../app/settings/account.tsx'),
  source('../app/settings/time-zone.tsx'),
  source('../app/settings/interactions.tsx'),
  source('../app/settings/blocked-accounts.tsx'),
];
const account = routes[1]!;
const timeZone = routes[2]!;
const interactions = routes[3]!;
const blocked = routes[4]!;
const recoveryPresentation = source('./ui/settings-journey-route-presentation.tsx');
const recoveryView = source('./ui/settings-journey-recovery-view.tsx');
const blockedView = source('./ui/blocked-account-list-view.tsx');
const en = JSON.parse(source('../../../packages/i18n/src/locales/en.json')) as Record<string, string>;
const es = JSON.parse(source('../../../packages/i18n/src/locales/es.json')) as Record<string, string>;

test('canonical Settings intents admit only exact owned destinations', () => {
  assert.equal(settingsJourneyIntentFromPathname('/settings')?.kind, 'settings');
  assert.equal(settingsJourneyIntentFromPathname('/settings/account')?.kind, 'account');
  assert.equal(settingsJourneyIntentFromPathname('/settings/time-zone')?.kind, 'time-zone');
  assert.equal(settingsJourneyIntentFromPathname('/settings/interactions')?.kind, 'interactions');
  assert.equal(settingsJourneyIntentFromPathname('/settings/blocked-accounts')?.kind, 'blocked-accounts');
  assert.equal(settingsJourneyIntentFromPathname('/settings/notifications'), null);
  assert.equal(settingsJourneyIntentFromPathname('/settings/account/extra'), null);
});

test('owned Settings bootstrap cannot cross replacement while a true cold bootstrap remains admissible', () => {
  const intent = { kind: 'account', routeKey: 'settings:account' } as const;
  queueSettingsJourneyBootstrap(intent, 'account-a:1');
  assert.equal(takeSettingsJourneyBootstrap('account-b:2'), null);
  assert.equal(takeSettingsJourneyBootstrap('account-a:1'), null);
  queueSettingsJourneyBootstrap(intent);
  assert.deepEqual(takeSettingsJourneyBootstrap('account-b:2'), intent);
});

test('Settings route ownership distinguishes true cold entry from account replacement', () => {
  assert.equal(settingsRouteOwnerDecision(undefined, undefined), 'unowned');
  assert.equal(settingsRouteOwnerDecision(undefined, 'account-a:1'), 'claim');
  assert.equal(settingsRouteOwnerDecision('account-a:1', undefined), 'retain');
  assert.equal(settingsRouteOwnerDecision('account-a:1', 'account-a:1'), 'retain');
  assert.equal(settingsRouteOwnerDecision('account-a:1', 'account-b:2'), 'replacement');
  for (const route of routes) {
    assert.match(route, /publishedPresentation\?\.sessionKey \?\? publishedRecovery\?\.sessionKey/);
    assert.match(route, /ownerReplaced[\s\S]*router\.replace\('\/\(tabs\)\/home'\)/);
  }
});

test('only the latest rotated credential may drive Settings credential recovery', () => {
  assert.equal(shouldHandleSettingsOperationFailure('token-a', 'token-c'), false);
  assert.equal(shouldHandleSettingsOperationFailure('token-b', 'token-c'), false);
  assert.equal(shouldHandleSettingsOperationFailure('token-c', 'token-c'), true);
  assert.equal(shouldHandleSettingsOperationFailure('token-c', undefined), false);
});

test('blocked-account mutation invalidates older loads and blocks refresh until release', () => {
  const refreshBeforeUnblock = 4;
  const mutationRevision = refreshBeforeUnblock + 1;
  assert.equal(ownsBlockedAccountLoad(refreshBeforeUnblock, mutationRevision, true), false);
  assert.equal(ownsBlockedAccountLoad(mutationRevision, mutationRevision, true), false);
  assert.equal(ownsBlockedAccountLoad(mutationRevision + 1, mutationRevision + 1, false), true);
  assert.match(blocked, /if \(!presentation \|\| unblockAdmission\.current\) return/);
  assert.match(blocked, /unblockAdmission\.current = reviewedIntent;[\s\S]*\+\+operation\.current/);
});

test('cold Settings child ancestry reuses keyed tabs and Settings while selecting nested Home', () => {
  const tabs = {
    key: 'tabs-key',
    name: '(tabs)',
    state: {
      index: 1,
      routes: [
        { key: 'home-key', name: 'home', state: { retained: 'home' } },
        { key: 'following-key', name: 'following', state: { retained: 'following' } },
      ],
    },
  };
  const settings = { key: 'settings-key', name: 'settings/index', state: { retained: true } };
  const detail = { key: 'blocked-key', name: 'settings/blocked-accounts' };
  const normalized = normalizeSettingsJourneyRouteState({
    index: 0,
    key: 'root-key',
    routes: [detail, tabs, settings],
  }, { kind: 'blocked-accounts', routeKey: 'settings:blocked-accounts' });

  assert.equal(normalized.key, 'root-key');
  assert.deepEqual(normalized.routes.map(({ name }) => name), [
    '(tabs)', 'settings/index', 'settings/blocked-accounts',
  ]);
  assert.equal(normalized.routes[0]?.key, 'tabs-key');
  assert.equal(normalized.routes[1]?.key, 'settings-key');
  assert.equal(normalized.routes[2]?.key, 'blocked-key');
  const nested = normalized.routes[0]?.state as typeof tabs.state;
  assert.equal(nested.index, 0);
  assert.equal(nested.routes[0]?.key, 'home-key');
  assert.equal(nested.routes[1]?.key, 'following-key');
  assert.equal(nativeBackSettingsJourneyState(normalized).routes.at(-1)?.name, 'settings/index');
  assert.equal(
    nativeBackSettingsJourneyState(nativeBackSettingsJourneyState(normalized)).routes.at(-1)?.name,
    '(tabs)',
  );
});

test('every Settings route keeps native ancestry and visible localized recovery instead of redirecting or blanking', () => {
  for (const route of routes) {
    assert.match(route, /SettingsJourneyRecoveryView/);
    assert.match(route, /useSettingsJourneyRouteAncestry/);
    assert.doesNotMatch(route, /return null/);
    assert.doesNotMatch(route, /router\.replace\('\/'\)/);
  }
  assert.match(recoveryPresentation, /SettingsJourneyRecoverySource/);
  assert.match(recoveryPresentation, /sessionKey/);
  assert.match(recoveryView, /accessibilityRole="progressbar"/);
  assert.match(recoveryView, /common\.retry/);
  assert.match(recoveryView, /settings\.recoveryHome/);
  for (const catalog of [en, es]) for (const key of [
    'settings.recoveryLoading',
    'settings.recoveryOfflineHeading',
    'settings.recoveryOfflineDescription',
    'settings.recoveryUnavailableHeading',
    'settings.recoveryUnavailableDescription',
    'settings.recoveryHome',
    'settings.account.signingOut',
  ]) assert.ok(catalog[key]?.trim(), `${key} must be localized`);
});

test('owner-private Settings state fails closed across replacement and routes bind all late results to sessionKey', () => {
  assert.equal(ownsSettingsRouteState('account-a:1', 'account-a:1'), true);
  assert.equal(ownsSettingsRouteState('account-a:1', 'account-b:2'), false);
  for (const route of [account, timeZone, interactions, blocked]) {
    assert.match(route, /presentation\.sessionKey/);
    assert.match(route, /ownsSettingsRouteState/);
    assert.match(route, /activeOwnerKey\.current/);
  }
  assert.match(timeZone, /setPreference\(undefined\)/);
  assert.match(interactions, /setPreferences\(undefined\)/);
  assert.match(blocked, /page\.current = emptyPage/);
  assert.match(timeZone, /mutationAdmission\.current/);
  assert.match(timeZone, /onPress: \(\) => void save\(changeIntent, ownedPresentation, ownedSessionKey\)/);
  assert.match(interactions, /if \(!presentation \|\| !ownedPreferences \|\| mutationAdmission\.current\) return/);
  assert.match(blocked, /const reviewedIntent: UnblockIntent/);
  assert.match(blocked, /reviewedIntent\.idempotencyKey/);
  assert.match(blocked, /unblockAdmission\.current !== reviewedIntent/);
});

test('Account keeps its native destructive label stable and announces separate busy progress', () => {
  assert.match(account, /label=\{i18n\.t\('auth\.signOut'\)\}/);
  assert.match(account, /accessibilityLiveRegion="polite"/);
  assert.match(account, /settings\.account\.signingOut/);
  assert.doesNotMatch(account, /i18n\.t\(working \?/);
});

test('Time Zone has keyboard completion and a real native selected icon', () => {
  assert.match(timeZone, /InputAccessoryView/);
  assert.match(timeZone, /inputAccessoryViewID=/);
  assert.match(timeZone, /Keyboard\.dismiss/);
  assert.match(timeZone, /<NativeSystemImage/);
  assert.doesNotMatch(timeZone, />✓</);
});

test('blocked identities reflow at accessibility sizes without truncating names', () => {
  assert.match(blockedView, /needsCompactVerticalLayout/);
  assert.doesNotMatch(blockedView, /numberOfLines=/);
  assert.match(blockedView, /rowStacked/);
});
