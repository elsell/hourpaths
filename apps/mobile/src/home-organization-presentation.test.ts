import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import { createTranslator } from '@hourpaths/i18n';

function source(relative: string): string {
  return readFileSync(fileURLToPath(new URL(relative, import.meta.url)), 'utf8');
}

const filterIOS = source('./ui/home-filter-chips.ios.tsx');
const filterFallback = source('./ui/home-filter-chips.tsx');
const arrangementIOS = source('./ui/home-arrangement-view.ios.tsx');
const arrangementFallback = source('./ui/home-arrangement-view.tsx');

test('Home filters use horizontally scalable native chips and an accessible fallback', () => {
  assert.match(filterIOS, /from '@expo\/ui\/swift-ui'/);
  assert.match(filterIOS, /<ScrollView[\s\S]*axes="horizontal"[\s\S]*showsIndicators=\{false\}/);
  assert.match(filterIOS, /buttonStyle\(selected \? 'borderedProminent' : 'bordered'\)/);
  assert.match(filterIOS, /tint\(mobileTheme\.colors\.accent\)/);
  assert.match(filterIOS, /accessibilityValue\(i18n\.t\(selected \? 'home\.filter\.selected' : 'home\.filter\.notSelected'\)\)/);
  assert.match(filterFallback, /horizontal/);
  assert.match(filterFallback, /accessibilityState=\{\{ selected \}\}/);
  assert.match(filterFallback, /minHeight:\s*44/);
  assert.doesNotMatch(`${filterIOS}\n${filterFallback}`, />All<|>Solo<|>Shared<|>Supporting</);
});

test('Home arrangement uses a native editable List with non-gesture fallback actions', () => {
  assert.match(arrangementIOS, /<List modifiers=\{\[/);
  assert.match(arrangementIOS, /environment\('editMode', 'active'\)/);
  assert.match(arrangementIOS, /<List\.ForEach onMove=/);
  assert.match(arrangementIOS, /preferences\.order === 'manual'/);
  assert.match(arrangementIOS, /systemImage=\{pinned \? 'pin\.slash' : 'pin'\}/);
  assert.match(arrangementIOS, /tint\(mobileTheme\.colors\.accent\)/);
  assert.match(arrangementIOS, /disabled\(busy\)/);
  assert.match(arrangementFallback, /home\.arrange\.moveUp/);
  assert.match(arrangementFallback, /home\.arrange\.moveDown/);
  assert.match(arrangementFallback, /disabled=\{busy \|\| index === 0\}/);
  assert.match(arrangementFallback, /disabled=\{busy \|\| index === paths\.length - 1\}/);
  assert.match(arrangementFallback, /disabled=\{busy\}/);
  assert.doesNotMatch(`${arrangementIOS}\n${arrangementFallback}`, />Pinned<|>Paths<|>Pin<|>Unpin<|>Up<|>Down</);
});

test('Home organization presentation copy is complete in both supported locales', () => {
  for (const locale of ['en', 'es'] as const) {
    const i18n = createTranslator([locale]);
    for (const key of [
      'home.filter.all',
      'home.filter.solo',
      'home.filter.shared',
      'home.filter.supporting',
      'home.arrange.pinnedHeading',
      'home.arrange.pathsHeading',
      'home.arrange.pin',
      'home.arrange.unpin',
      'home.arrange.moveUp',
      'home.arrange.moveDown',
    ] as const) {
      assert.notEqual(i18n.t(key, { pathName: String.fromCodePoint(82, 101, 97, 100, 105, 110, 103) }), key);
    }
  }
});
