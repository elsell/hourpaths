import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const component = readFileSync(new URL('./social-profile-discovery.svelte', import.meta.url), 'utf8');
const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');
const markup = (component.split('</script>')[1] ?? component).split('<style>')[0] ?? component;

test('public profiles expose a destructive block review rather than an immediate mutation', () => {
  for (const prop of [
    'blockReview',
    'blockBusy',
    'blockError',
    'onReviewBlock',
    'onCancelBlock',
    'onConfirmBlock',
  ]) {
    assert.match(component, new RegExp(`export let ${prop}(?:\\b|:)`));
  }

  assert.match(markup, /onReviewBlock\(selectedProfile\.username\)/);
  assert.match(markup, /role="alertdialog"/);
  assert.match(markup, /blockReview\.sharedPaths/);
  assert.match(markup, /i18n\.t\('blocking\.sharedPathsWarning'/);
  assert.match(markup, /i18n\.t\('blocking\.leavePathsSeparately'/);
  assert.match(markup, /class="destructive-action"/);
  assert.match(markup, /onConfirmBlock\(\)/);
});

test('blocked accounts are a dedicated paginated destination with retry and unblock review', () => {
  for (const prop of [
    'blockedAccountsOpen',
    'blockedAccountsState',
    'blockedAccounts',
    'blockedAccountsNextCursor',
    'unblockReview',
    'onOpenBlockedAccounts',
    'onCloseBlockedAccounts',
    'onRetryBlockedAccounts',
    'onLoadMoreBlockedAccounts',
    'onReviewUnblock',
    'onCancelUnblock',
    'onConfirmUnblock',
  ]) {
    assert.match(component, new RegExp(`export let ${prop}(?:\\b|:)`));
  }

  assert.match(markup, /blockedAccountsOpen/);
  assert.match(markup, /i18n\.t\('blocking\.settingsHeading'\)/);
  assert.match(markup, /blockedAccountsState === 'error'/);
  assert.match(markup, /blockedAccountsState === 'empty'/);
  assert.match(markup, /onRetryBlockedAccounts\(\)/);
  assert.match(markup, /onLoadMoreBlockedAccounts\(\)/);
  assert.match(markup, /onReviewUnblock\(account\.username\)/);
  assert.match(markup, /role="alertdialog"/);
  assert.match(markup, /onConfirmUnblock\(\)/);
});

test('web orchestration uses session-owned block operations and removes a blocked profile', () => {
  assert.match(page, /createSessionOperationOwner\(\)/);
  assert.match(page, /async function reviewSocialBlock/);
  assert.match(page, /async function confirmSocialBlock/);
  assert.match(page, /async function loadBlockedAccounts/);
  assert.match(page, /async function confirmSocialUnblock/);
  assert.match(page, /crypto\.randomUUID\(\)/);
  assert.match(page, /closeSocialProfile\(\)/);
  assert.match(page, /socialSearch = \{ \.\.\.socialSearch, items: socialSearch\.items\.filter/);
});
