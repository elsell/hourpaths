import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const component = readFileSync(new URL('./social-profile-discovery.svelte', import.meta.url), 'utf8');
const markup = (component.split('</script>')[1] ?? component).split('<style>')[0] ?? component;

test('profile discovery exposes a presentation-only state and action seam', () => {
  for (const prop of [
    'i18n',
    'query',
    'searchState',
    'results',
    'nextCursor',
    'loadingMore',
    'selectedProfile',
    'profileState',
    'relationshipBusy',
    'followRequestsOpen',
    'followRequests',
    'onQueryChange',
    'onSearch',
    'onSelectProfile',
    'onRetrySearch',
    'onLoadMore',
    'onCloseProfile',
    'onRefreshProfile',
    'onRelationshipAction',
    'onOpenFollowRequests',
    'onReviewFollowRequest',
  ]) {
    assert.match(component, new RegExp(`export let ${prop}(?:\\b|:)`));
  }

  assert.doesNotMatch(component, /fetch\s*\(|createSessionApiClient|onMount|localStorage|sessionStorage/);
});

test('search is an accessible native form with every required presentation state', () => {
  assert.match(markup, /<form[^>]*onsubmit=/);
  assert.match(markup, /<input[^>]*type="search"[^>]*aria-label=\{i18n\.t\('social\.searchPlaceholder'\)\}/);
  assert.match(component, /Array\.from\(query\)\.filter\(\(character\) => !\/\\s\/u\.test\(character\)\)\.length < 2/);
  assert.match(markup, /searchState === 'hint'/);
  assert.match(markup, /searchState === 'loading'/);
  assert.match(markup, /searchState === 'empty'/);
  assert.match(markup, /searchState === 'error'/);
  assert.match(markup, /searchState === 'results'/);
  assert.match(markup, /role="status"/);
  assert.match(markup, /role="alert"/);
  assert.match(markup, /i18n\.t\('common\.retry'\)/);
  assert.match(markup, /i18n\.t\(loadingMore \? 'social\.loadingMore' : 'social\.loadMore'\)/);
});

test('results are compact identity rows and selection is a public-profile-only destination', () => {
  assert.match(markup, /<ul class="profile-results"/);
  assert.match(markup, /class="profile-row"/);
  assert.match(markup, /onSelectProfile\(profile\.userId\)/);
  assert.match(markup, /selectedProfile\.displayName/);
  assert.match(markup, /selectedProfile\.username/);
  assert.match(markup, /selectedProfile\.description/);
  assert.match(markup, /selectedProfile\.followerCount/);
  assert.match(markup, /selectedProfile\.followingCount/);
  assert.match(markup, /i18n\.number\(/);
  assert.match(markup, /onCloseProfile\(\)/);
  assert.match(markup, /onRefreshProfile\(\)/);

  assert.doesNotMatch(markup, /(?:profile|selectedProfile)\.(?:email|provider|privacy|paths?|goals?|activity|internalState)/i);
  assert.match(markup, /onRelationshipAction/);
  assert.match(markup, /selectedProfile\.relationship/);
  assert.match(markup, /followRequestsOpen/);
  assert.match(markup, /onReviewFollowRequest\('accept'/);
  assert.match(markup, /onReviewFollowRequest\('reject'/);
});

test('the slice uses only the neutral avatar and all copy is symbolic', () => {
  assert.match(markup, /class="neutral-avatar"/);
  assert.match(markup, /aria-label=\{i18n\.t\('social\.neutralAvatarLabel'\)\}/);
  assert.doesNotMatch(component, /<img\b|\bsrc=|profilePictureUrl/);
  assert.doesNotMatch(markup.replaceAll('=>', '='), />\s*[A-Za-z][^<{]*</);
});
