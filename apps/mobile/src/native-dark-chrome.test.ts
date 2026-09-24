import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

function source(path: string) {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');
}

const appConfig = source('../app.json');
const rootLayout = source('../app/_layout.tsx');
const homeLayout = source('../app/(tabs)/home/_layout.tsx');
const followingLayout = source('../app/(tabs)/following/_layout.tsx');

test('native application chrome stays dark while stack bars inherit system material', () => {
  const parsedAppConfig = JSON.parse(appConfig) as {
    expo?: { ios?: { infoPlist?: { UIViewControllerBasedStatusBarAppearance?: unknown } } };
  };
  assert.match(appConfig, /"userInterfaceStyle":\s*"dark"/);
  assert.match(appConfig, /"backgroundColor":\s*"#141414"/);
  assert.match(appConfig, /"androidStatusBar":\s*\{[\s\S]*?"backgroundColor":\s*"#141414"[\s\S]*?"barStyle":\s*"light-content"/);
  assert.match(appConfig, /"androidNavigationBar":\s*\{[\s\S]*?"backgroundColor":\s*"#141414"[\s\S]*?"barStyle":\s*"light-content"/);
  assert.equal(
    parsedAppConfig.expo?.ios?.infoPlist?.UIViewControllerBasedStatusBarAppearance,
    true,
    'native-stack status-bar presentation requires view-controller ownership on iOS',
  );

  assert.doesNotMatch(rootLayout, /headerStyle:/);
  assert.doesNotMatch(rootLayout, /navigationBarColor:/);
  assert.doesNotMatch(rootLayout, /statusBarStyle:/);

  assert.doesNotMatch(homeLayout, /headerStyle:|navigationBarColor:/);
  assert.match(homeLayout, /contentStyle:\s*\{ backgroundColor: mobileTheme\.colors\.background \}/);
  assert.match(homeLayout, /statusBarStyle:\s*'light'/);

  assert.doesNotMatch(followingLayout, /headerStyle:|navigationBarColor:/);
  assert.match(followingLayout, /contentStyle:\s*\{ backgroundColor: mobileTheme\.colors\.background \}/);
  assert.match(followingLayout, /statusBarStyle:\s*'light'/);
});
