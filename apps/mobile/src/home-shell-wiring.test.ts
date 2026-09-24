import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('authenticated Home recovery is explicit, retryable, and session owned', () => {
  assert.match(page, /type HomeRecoveryStatus = 'loading' \| 'offline' \| 'error'/);
  assert.match(page, /recovery\?\.sessionToken === current\.token/);
  assert.match(page, /const retainsOwnedHome =[\s\S]*activeBeforeAdoption\.session\?\.token === sourceSessionToken[\s\S]*homeProjectionSessionToken\.current === sourceSessionToken/);
  assert.match(page, /credential\.nextAction === 'home' && !retainsOwnedHome[\s\S]*homeProjectionSessionToken\.current = null;[\s\S]*setDestination\(null\)/);
  assert.match(page, /const ownedHomeDestination = destination\?\.kind === 'home' && session &&[\s\S]*homeProjectionSessionToken\.current === session\.token/);
  assert.match(page, /async function retryAuthenticatedHome\(\)/);
  assert.match(page, /const ticket = sessionOperations\.issue\(\);[\s\S]*recoverMobileSession\(\{[\s\S]*mode: 'profile'/);
  assert.match(page, /current: ticket\.current/);
  assert.match(page, /onRetry=\{\(\) => void retryAuthenticatedHome\(\)\}/);
  assert.doesNotMatch(page, /ready && !destination && accessState === 'authenticated_offline'[\s\S]*offlineStatusDismissed/);
});

test('Home uses one intentional presentation hierarchy without a redundant populated Create action', () => {
  assert.match(page, /homePresentation\(\{/);
  assert.match(page, /<HomeView/);
  assert.match(page, /sections=\{homeViewSections\}/);
  assert.doesNotMatch(page, /filterControl=\{|<HomeFilterChips/);
  assert.match(page, /<HomeHeaderActions[\s\S]*filter=\{homeFilter\}[\s\S]*onFilterChange=\{setHomeFilter\}/);
  assert.match(page, /notice=\{/);
  assert.doesNotMatch(
    page,
    /!archivedPathsOpen && !creatingPath && destination\.profile\.paths\.length > 0 \? <ActionButton label=\{i18n\.t\('home\.createPath'\)\}/,
  );
});
