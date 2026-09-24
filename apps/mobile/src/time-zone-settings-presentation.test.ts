import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const source = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');

test('configured time zone has a dedicated compact native Settings destination', () => {
  const settings = source('../app/settings/index.tsx');
  const layout = source('../app/_layout.tsx');
  const route = source('../app/settings/time-zone.tsx');

  assert.match(settings, /settings\.timeZone\.openLabel/);
  assert.match(settings, /router\.push\('\/settings\/time-zone'\)/);
  assert.match(layout, /settings\/time-zone/);
  assert.match(route, /FlatList/);
  assert.match(route, /TextInput/);
  assert.match(route, /filterTimeZones/);
  assert.match(route, /accessibilityRole="search"/);
  assert.match(route, /accessibilityState=\{\{ selected:/);
});

test('time-zone changes require an explicit native warning and stable confirmed intent', () => {
  const route = source('../app/settings/time-zone.tsx');
  const presentation = source('./ui/settings-presentation.tsx');
  const home = source('../app/index.tsx');

  assert.match(route, /createTimeZoneChangeIntentCoordinator/);
  assert.match(route, /Alert\.alert\(/);
  assert.match(route, /settings\.timeZone\.warning/);
  assert.match(route, /style: 'cancel'/);
  assert.match(route, /settings\.timeZone\.confirm/);
  assert.match(route, /ownedPresentation\.updateConfiguredTimeZone\(changeIntent\)/);
  assert.match(route, /const changeIntent = intents\.current\.freeze\(ownedPreference\.timeZone, proposedTimeZone\)/);
  assert.match(route, /preference\?\.timeZone !== changeIntent\.reviewedTimeZone/);
  assert.match(presentation, /getConfiguredTimeZone: \(\) => Promise<TimeZonePreference>/);
  assert.match(presentation, /updateConfiguredTimeZone: \(intent: FrozenTimeZoneChangeIntent\) => Promise<TimeZonePreference>/);
  assert.match(home, /\.configuredTimeZone\(\)/);
  assert.match(home, /\.updateConfiguredTimeZone\(\{/);
  assert.match(home, /timeZonePreferenceFromAPI\(envelope\.data\)/);
  assert.match(home, /getConfiguredTimeZone=\{getConfiguredTimeZone\}/);
  assert.match(home, /updateConfiguredTimeZone=\{updateConfiguredTimeZone\}/);
});

test('time-zone settings expose complete loading, failure, saving, and retry states', () => {
  const route = source('../app/settings/time-zone.tsx');
  assert.match(route, /ActivityIndicator/);
  assert.match(route, /NativeContentUnavailable/);
  assert.match(route, /settings\.timeZone\.retry/);
  assert.match(route, /settings\.timeZone\.saveError/);
  assert.match(route, /accessibilityLiveRegion="polite"/);
  assert.match(route, /requestRevision\.current/);
  assert.match(route, /mutationRevision\.current/);
});
