import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const app = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('native Home wires authoritative organization without mixing supporter tracking controls', () => {
  assert.doesNotMatch(app, /<HomeFilterChips|homeFilterControl/);
  assert.match(app, /<HomeHeaderActions[\s\S]*filter=\{homeFilter\}[\s\S]*onFilterChange=\{setHomeFilter\}/);
  assert.match(app, /filterLabels=\{\{[\s\S]*home\.filter\.all[\s\S]*home\.filter\.supporting/);
  assert.match(app, /organizeHomePaths\(ownedHomeDestination\.profile\.paths, ownedHomeDestination\.profile\.timers, homePreferences, homeFilter\)/);
  assert.match(app, /homeSections\.supporting\.map\(renderHomePath\)/);
  assert.match(app, /home\.section\.supporting/);
  assert.match(app, /homeSections\.pinned\.map\(renderHomePath\)/);
  assert.match(app, /home\.arrange\.pinnedHeading/);
});

test('Home toolbar, row actions, and arrangement sheet persist through the generated API client', () => {
  assert.match(app, /<HomeHeaderActions[\s\S]*onArrange=\{\(\) => setHomeArrangementOpen\(true\)\}/);
  assert.match(app, /onOrderChange=\{\(order\) => void updateHomePreferences/);
  assert.match(app, /actions=\{\[[\s\S]*home\.arrange\.(?:pin|unpin)/);
  assert.match(app, /\.updateHomePreferences\(body, idempotencyKey\)/);
  assert.match(app, /<NativeSheet[\s\S]*scrollable=\{false\}[\s\S]*home\.arrange\.title/);
  assert.match(app, /<HomeArrangementView[\s\S]*onMove=[\s\S]*onPinChange=/);
});

test('Home preference failures remain visible and retryable without optimistic reordering', () => {
  assert.match(app, /createHomePreferenceOperationOwner/);
  assert.match(app, /result\.kind === 'failed'[\s\S]*setHomePreferenceErrorKey/);
  assert.match(app, /homePreferenceErrorKey[\s\S]*StatusBanner/);
  const update = app.slice(app.indexOf('async function updateHomePreferences'), app.indexOf('useEffect(() => {\n    if (accessState'));
  assert.ok(update.indexOf('setHomePreferences({') > update.indexOf("if (result.kind === 'failed')"));
});

test('Home organization stays current after membership and recorded-activity changes', () => {
  assert.match(app, /archivedPaths\]\.map\(\(\{ id \}\) => id\)\.join\('\\u0000'\)/);
  assert.match(app, /async function refreshHomeOrganizationPath/);
  assert.match(app, /\.path\(pathID\)/);
  assert.match(app, /refreshForegroundNotificationTarget[\s\S]*await refreshHomeOrganizationPath\(pathID, currentSession, ownerID\)/);
  assert.equal(app.match(/await refreshHomeOrganizationPath\(pathID, currentSession, ownerID\)/g)?.length, 4);
});
