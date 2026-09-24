import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const pathRoute = readFileSync(fileURLToPath(new URL('../app/path/[pathID].tsx', import.meta.url)), 'utf8');
const historyRoute = readFileSync(fileURLToPath(new URL('../app/path/[pathID]/history/index.tsx', import.meta.url)), 'utf8');
const activityRoute = readFileSync(fileURLToPath(new URL('../app/path/[pathID]/history/[activityID].tsx', import.meta.url)), 'utf8');
const recovery = readFileSync(fileURLToPath(new URL('./ui/native-route-recovery-view.tsx', import.meta.url)), 'utf8');
const detail = readFileSync(fileURLToPath(new URL('./ui/path-detail-view.tsx', import.meta.url)), 'utf8');
const manualForm = readFileSync(fileURLToPath(new URL('./ui/manual-activity-form.tsx', import.meta.url)), 'utf8');
const unavailable = readFileSync(fileURLToPath(new URL('./ui/native-content-unavailable.tsx', import.meta.url)), 'utf8');
const systemImage = readFileSync(fileURLToPath(new URL('./ui/native-system-image.ios.tsx', import.meta.url)), 'utf8');
const rootLayout = readFileSync(fileURLToPath(new URL('../app/_layout.tsx', import.meta.url)), 'utf8');
const app = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('cold direct Path routes never return a blank body and expose native recovery/back', () => {
  for (const source of [pathRoute, historyRoute, activityRoute]) {
    assert.doesNotMatch(source, /if \(!presentation\) return null/);
    assert.match(source, /<NativeRouteRecoveryView/);
    assert.doesNotMatch(source, /icon="chevron\.left"/);
    assert.match(source, /onGoHome=/);
    assert.match(source, /usePathRouteAncestry\(/);
    assert.doesNotMatch(source, /canGoBack/);
    assert.doesNotMatch(source, /<Stack\.Toolbar placement="left">/);
  }
  assert.match(recovery, /tone="loading"/);
  assert.match(recovery, /<NativeContentUnavailable/);
  assert.match(recovery, /<NativePrimaryButton[\s\S]*common\.retry/);
  assert.match(recovery, /<NativePrimaryButton[\s\S]*pathDetails\.back/);
  assert.doesNotMatch(recovery, /numberOfLines|maxFontSizeMultiplier|allowFontScaling=\{false\}/);
});

test('shell adopts exact route intents through session-owned recovery', () => {
  assert.match(app, /pathRouteIntent\(pathname, routeParameters\)/);
  assert.match(app, /createPathRouteTarget\(/);
  assert.match(app, /rotatePathRouteTarget\(/);
  assert.match(app, /rotatePathDetailTarget\(/);
  assert.match(app, /ownsPathDetailTarget\(/);
  assert.match(app, /ownsPathRouteTarget\(/);
  assert.match(app, /recoverPathRoute\(/);
  assert.match(app, /if \(!path\) \{[\s\S]*await retryPathRoute\(intent\);[\s\S]*return;/);
  assert.match(app, /pathRouteTarget\.current = null/);
  const adoption = app.slice(app.indexOf('async function activate('), app.indexOf('async function synchronizePushPermission'));
  assert.match(adoption, /pathDetailTarget\.current[\s\S]*retainsOwnedHome[\s\S]*rotatePathDetailTarget/);
  assert.match(adoption, /rotatePathDetailTarget\([\s\S]*else \{[\s\S]*resetPathDetail\(\)/);
  for (const operation of ['openActivityHistory', 'inspectActivity', 'loadMoreActivityRevisions', 'refreshPathDetail']) {
    const start = app.indexOf(`${operation}(`);
    assert.notEqual(start, -1, operation);
  }
  assert.match(app, /bindPathDetailTarget\(/);
  assert.match(app, /ownsActivePathDetail\(/);
});

test('rotated request rejection becomes retryable without discarding the current credential', () => {
  const retry = app.slice(app.indexOf('async function retryPathRoute'), app.indexOf('async function openPathMembers'));
  assert.match(retry, /latest\.session !== currentSession[\s\S]*createPathRouteTarget\([\s\S]*setPathRouteRecovery\('offline'\)/);
  assert.match(retry, /latest\.session === currentSession[\s\S]*handleSessionFailure/);
});

test('manual activity uses native sheet actions and risk-scoped dismissal', () => {
  assert.match(manualForm, /title=\{i18n\.t\(editing \? 'activity\.editHeading' : 'activity\.addHeading'\)\}/);
  assert.match(manualForm, /leadingAction=\{\{[\s\S]*common\.cancel/);
  assert.match(manualForm, /trailingAction=\{\{[\s\S]*common\.retry[\s\S]*activity\.saveEdit[\s\S]*activity\.save/);
  assert.match(manualForm, /dismissible=\{!busy\}/);
  assert.doesNotMatch(manualForm, /<SectionHeading|<ActionButton|<Surface/);
  assert.doesNotMatch(manualForm, /activity\.exactStartTime/);
  assert.match(app, /key=\{manualActivity[\s\S]*manualActivity\.id[\s\S]*manualActivity\.version[\s\S]*manualPathID/);
  assert.match(app, /manualDraftDirty/);
  assert.match(app, /presentNativeDestructiveConfirmation\(\{[\s\S]*activity\.discard/);
});

test('system chrome stays native and unavailable fallback renders its icon', () => {
  assert.doesNotMatch(rootLayout, /headerStyle:/);
  assert.doesNotMatch(rootLayout, /navigationBarColor:/);
  assert.match(unavailable, /systemImage/);
  assert.match(unavailable, /<NativeSystemImage/);
  assert.match(systemImage, /Image as SwiftUIImage/);
  assert.match(systemImage, /systemName=\{systemName\}/);
});

test('compact Path detail keeps History visible in body and avoids duplicate Path title', () => {
  assert.match(detail, /<SettingsNavigationRow[\s\S]*pathDetails\.openHistory[\s\S]*onPress=\{onOpenHistory\}/);
  assert.doesNotMatch(detail, /pathName|<ScreenHeader|<PageHeader/);
  assert.doesNotMatch(detail, /numberOfLines|maxFontSizeMultiplier|allowFontScaling=\{false\}/);
});
