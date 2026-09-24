import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  createInteractionSettingsIntentCoordinator,
  interactionSettingsFromAPI,
  interactionSettingsWithChange,
} from './interaction-settings';

const source = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');
const route = source('../app/settings/interactions.tsx');
const settings = source('../app/settings/index.tsx');
const layout = source('../app/_layout.tsx');
const presentation = source('./ui/settings-presentation.tsx');
const home = source('../app/index.tsx');
const list = source('./ui/settings-list.tsx');

test('changing one interaction preference preserves the other', () => {
  assert.deepEqual(
    interactionSettingsWithChange(
      { commentsEnabled: true, reactionsEnabled: false },
      'reactionsEnabled',
      true,
    ),
    { commentsEnabled: true, reactionsEnabled: true },
  );
  assert.deepEqual(
    interactionSettingsWithChange(
      { commentsEnabled: true, reactionsEnabled: true },
      'commentsEnabled',
      false,
    ),
    { commentsEnabled: false, reactionsEnabled: true },
  );
});

test('interaction settings admit only the exact public boolean projection', () => {
  assert.deepEqual(interactionSettingsFromAPI({
    commentsEnabled: false,
    reactionsEnabled: true,
  }), {
    commentsEnabled: false,
    reactionsEnabled: true,
  });
  for (const invalid of [
    null,
    {},
    { commentsEnabled: false, reactionsEnabled: 1 },
    { commentsEnabled: false, privateOwnerId: 'owner', reactionsEnabled: true },
  ]) assert.throws(() => interactionSettingsFromAPI(invalid));
});

test('one frozen intent keeps one stable mutation key until authoritative completion', () => {
  let issued = 0;
  const coordinator = createInteractionSettingsIntentCoordinator(() => `key-${++issued}`);
  const settings = { commentsEnabled: false, reactionsEnabled: true };
  const first = coordinator.freeze(settings);
  const retry = coordinator.freeze({ ...settings });
  assert.equal(retry, first);
  assert.equal(retry.idempotencyKey, 'key-1');

  const changed = coordinator.freeze({ commentsEnabled: true, reactionsEnabled: true });
  assert.equal(changed.idempotencyKey, 'key-2');
  coordinator.complete(changed);
  assert.equal(coordinator.freeze(changed.settings).idempotencyKey, 'key-3');
});

test('interaction controls have a dedicated native Settings destination', () => {
  assert.match(settings, /settings\.interactions\.openLabel/);
  assert.match(settings, /router\.push\('\/settings\/interactions'\)/);
  assert.match(layout, /settings\/interactions/);
  assert.match(route, /<SettingsShell>/);
  assert.match(route, /<SettingsSection[\s\S]*settings\.interactions\.footer/);
  assert.match(route, /<SettingsSwitchRow[\s\S]*settings\.interactions\.comments/);
  assert.match(route, /<SettingsSwitchRow[\s\S]*settings\.interactions\.reactions/);
  assert.match(list, /<Switch/);
  assert.match(list, /accessibilityValue=\{\{ text: valueLabel \}\}/);
});

test('loading, retry, saving, and authoritative rollback stay operation-owned', () => {
  const orchestration = home.slice(
    home.indexOf('async function getInteractionSettings'),
    home.indexOf('async function getNudgeChannelPreference'),
  );
  assert.match(route, /ActivityIndicator/);
  assert.match(route, /NativeContentUnavailable/);
  assert.match(route, /settings\.interactions\.retry/);
  assert.match(route, /accessibilityLiveRegion="polite"[\s\S]*settings\.interactions\.saving/);
  assert.match(route, /const baseline = ownedPreferences/);
  assert.match(route, /setPreferences\(baseline\)/);
  assert.match(route, /setPreferences\(authoritative\)/);
  assert.match(route, /requestRevision\.current/);
  assert.match(route, /mutationRevision\.current/);
  assert.match(presentation, /getInteractionSettings: \(\) => Promise<InteractionSettings>/);
  assert.match(presentation, /updateInteractionSettings: \(settings: InteractionSettings, idempotencyKey: string\)/);
  assert.match(home, /\.getInteractionSettings\(\)/);
  assert.match(home, /\.updateInteractionSettings\(settings, idempotencyKey\)/);
  assert.match(home, /ownsNotificationSessionTarget/);
  assert.match(home, /settingsOperationTargets\.current/);
  assert.equal(orchestration.match(/handleSettingsOperationFailure\(cause, currentSession, currentTarget\)/g)?.length, 2);
  assert.match(orchestration, /participant\.userId === ownerID[\s\S]*commentsEnabled: authoritative\.commentsEnabled[\s\S]*reactionsEnabled: authoritative\.reactionsEnabled/);
  assert.match(orchestration, /refreshNotificationsAfterInteractionSettings\(currentSession, ownerID, currentTarget\)/);
  assert.match(orchestration, /loadNotificationHistoryPage\(currentSession\)[\s\S]*mergeNotificationHistoryPage\([\s\S]*page,[\s\S]*''[\s\S]*setNativeNotificationBadge\(history\.unreadCount\)/);
});

test('interaction settings retain the last authoritative controls through a refresh failure', () => {
  const load = route.slice(
    route.indexOf('async function load('),
    route.indexOf('useEffect(() =>', route.indexOf('async function load(')),
  );
  const retained = route.slice(
    route.indexOf("return <SettingsShell>\n    <SettingsSection footer={i18n.t('settings.interactions.footer')}>"),
  );

  assert.doesNotMatch(load, /setPreferences\(undefined\)/);
  assert.match(load, /setLoadFailed\(false\)[\s\S]*setPreferences\(authoritative\)/);
  assert.match(load, /setPreferences\(authoritative\)[\s\S]*setLoadFailed\(false\)/);
  assert.match(load, /catch[\s\S]*setLoadFailed\(true\)/);
  assert.match(retained, /<SettingsSwitchRow[\s\S]*settings\.interactions\.comments/);
  assert.match(retained, /<SettingsSwitchRow[\s\S]*settings\.interactions\.reactions/);
  assert.match(retained, /loadFailed[\s\S]*<StatusBanner[\s\S]*settings\.interactions\.unavailableDescription[\s\S]*tone="error"/);
  assert.match(retained, /loadFailed[\s\S]*<SettingsActionRow[\s\S]*settings\.interactions\.retry/);
});

test('retry cannot supersede an in-flight privacy mutation or retain an unacknowledged optimistic value', () => {
  const update = route.slice(route.indexOf('async function update('), route.indexOf('if (!ownedPreferences)'));
  const retained = route.slice(route.indexOf("return <SettingsShell>\n    <SettingsSection footer={i18n.t('settings.interactions.footer')}>"));
  assert.match(update, /setPreferences\(authoritative\);[\s\S]*setLoadFailed\(false\)/);
  assert.match(retained, /<SettingsActionRow[\s\S]*disabled=\{saving\}[\s\S]*settings\.interactions\.retry/);
});
