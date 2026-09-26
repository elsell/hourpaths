import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const view = readFileSync(fileURLToPath(new URL('./ui/home-view.tsx', import.meta.url)), 'utf8');
const headerIOS = readFileSync(fileURLToPath(new URL('./ui/home-header-actions.ios.tsx', import.meta.url)), 'utf8');
const headerFallback = readFileSync(fileURLToPath(new URL('./ui/home-header-actions.tsx', import.meta.url)), 'utf8');
const card = readFileSync(fileURLToPath(new URL('./ui/path-card.tsx', import.meta.url)), 'utf8');
const nativeTrackingButton = readFileSync(fileURLToPath(new URL('./ui/native-tracking-button.ios.tsx', import.meta.url)), 'utf8');
const homeLayout = readFileSync(fileURLToPath(new URL('../app/(tabs)/home/_layout.tsx', import.meta.url)), 'utf8');
const en = JSON.parse(readFileSync(fileURLToPath(new URL('../../../packages/i18n/src/locales/en.json', import.meta.url)), 'utf8')) as Record<string, string>;
const es = JSON.parse(readFileSync(fileURLToPath(new URL('../../../packages/i18n/src/locales/es.json', import.meta.url)), 'utf8')) as Record<string, string>;

test('HomeView renders explicit recovery without bypassing timer-aware account sign-out', () => {
  assert.match(view, /presentation\.kind === 'loading'[\s\S]*<StatusBanner[\s\S]*home\.loading/);
  assert.match(view, /presentation\.kind === 'offline'[\s\S]*home\.offlineExplanation[\s\S]*home\.offlineHeading/);
  assert.match(view, /presentation\.kind === 'error'[\s\S]*home\.errorExplanation[\s\S]*home\.errorHeading/);
  assert.match(view, /label=\{i18n\.t\('common\.retry'\)\}[\s\S]*onPress=\{onRetry\}/);
  assert.doesNotMatch(view, /auth\.signOut|onSignOut|canSignOut/);
});

test('HomeView uses intentional empty hierarchy and never adds a populated bottom Create action', () => {
  assert.match(view, /presentation\.kind === 'filtered-empty'[\s\S]*home\.filteredEmptyHeading[\s\S]*onPress=\{onClearFilter\}/);
  assert.match(view, /presentation\.kind === 'empty'[\s\S]*home\.empty\.heading[\s\S]*onPress=\{onCreate\}/);
  const readyBranch = view.slice(view.indexOf("presentation.kind === 'ready'"));
  assert.doesNotMatch(readyBranch, /onPress=\{onCreate\}/);
});

test('Home condenses secondary controls and reflows Path progress at accessibility sizes', () => {
  assert.equal((headerIOS.match(/<Stack\.Toolbar\.Menu(?:\s|>)/g) ?? []).length, 1);
  assert.match(headerIOS, /activeLabel[\s\S]*archivedLabel[\s\S]*orderLabels\.arrange/);
  assert.match(headerFallback, /const menuAccessibilityLabel =[\s\S]*pathViewAccessibilityLabel[\s\S]*<NativeActionMenu/);
  assert.doesNotMatch(headerFallback, /flexWrap:\s*'wrap'/);
  assert.match(headerFallback, /menuAccessibilityLabel[\s\S]*orderLabels\[order\]/);
  assert.match(card, /progress \? <View style=\{styles\.progressStack\}/);
  assert.match(card, /progressStack:\s*\{[\s\S]*flexDirection:\s*'column'/);
  assert.doesNotMatch(card, /surfaceRaised|borderRadius:\s*mobileTheme\.radii\.lg/);
  assert.match(view, /rows:\s*\{[\s\S]*gap:\s*0/);
  assert.match(card, /identity:\s*\{[\s\S]*flex:\s*1,[\s\S]*minWidth:\s*0/);
  assert.match(homeLayout, /title: i18n\.t\('home\.heading'\)/);
});

test('Home uses a compact system toolbar, menu-owned filters, and flat collision-free rows', () => {
  assert.match(headerIOS, /<Stack\.Toolbar placement="left">[\s\S]*<Stack\.Toolbar\.Menu/);
  assert.match(headerIOS, /<Stack\.Toolbar placement="right">/);
  assert.equal((headerIOS.match(/<Stack\.Toolbar\.Button/g) ?? []).length, 3);
  assert.match(headerIOS, /filterLabels[\s\S]*onFilterChange[\s\S]*homeFilters/);
  assert.match(headerIOS, /separateBackground/);
  assert.match(headerIOS, /Stack\.Toolbar\.Button accessibilityLabel=\{settingsAccessibilityLabel\}/);
  assert.match(card, /progressStack/);
  assert.doesNotMatch(card, /progress:\s*\{[\s\S]*flexDirection:\s*'row'/);
  assert.match(card, /backgroundColor:\s*'transparent'/);
  assert.doesNotMatch(view, /rows:\s*\{[\s\S]*borderRadius/);
  assert.doesNotMatch(homeLayout, /headerStyle|navigationBarColor/);
  assert.match(card, /<View style=\{styles\.progressStack\}>\{progress\}<\/View>/);
  assert.ok(card.indexOf('</Pressable>') < card.indexOf('<View style={styles.progressStack}>{progress}</View>'));
  assert.match(nativeTrackingButton, /Math\.max\(48, 40 \+ 8 \* fontScale\)/);
});

test('Home presentation copy is complete in EN and ES', () => {
  for (const locale of [en, es]) {
    for (const key of [
      'home.loading',
      'home.offlineHeading',
      'home.offlineExplanation',
      'home.errorHeading',
      'home.errorExplanation',
      'home.filteredEmptyHeading',
      'home.filteredEmptyExplanation',
      'home.clearFilter',
    ]) assert.ok(locale[key], `${key} missing`);
    assert.match(locale['home.pathViewLabel'] ?? '', /view|vista/i);
  }
});
