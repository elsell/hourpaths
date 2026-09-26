import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  activeFollowingItemFromAPI,
  mergeActiveFollowingPage,
  shouldRefreshActiveFollowing,
  type ActiveFollowingItem,
} from './ui/social-active-following-presentation';

function source(path: string) {
  const file = fileURLToPath(new URL(path, import.meta.url));
  return existsSync(file) ? readFileSync(file, 'utf8') : '';
}

const feedView = source('./ui/social-feed-view.tsx');
const routePresentation = source('./ui/social-feed-route-presentation.tsx');
const orchestration = source('../app/index.tsx');
const followingRoute = source('../app/(tabs)/following/index.tsx');
const en = source('../../../packages/i18n/src/locales/en.json');
const es = source('../../../packages/i18n/src/locales/es.json');

function activeParticipant(userId: string, timerID: string): ActiveFollowingItem {
  return {
    participant: {
      displayName: 'Alex Rivera',
      userId,
      username: `user-${userId}`,
    },
    timers: [{
      id: timerID,
      path: { id: `path-${timerID}`, name: timerID },
      startedAt: '2026-07-28T01:00:00Z',
    }],
  };
}

test('active following mapping retains only grouped public current-state fields', () => {
  const mapped = activeFollowingItemFromAPI({
    ...activeParticipant('one', 'timer-1'),
    privateNote: 'must not cross the boundary',
    timers: [{
      ...activeParticipant('one', 'timer-1').timers[0],
      occurrenceTimeZone: 'private implementation detail',
    }],
  } as unknown as Parameters<typeof activeFollowingItemFromAPI>[0]);

  assert.deepEqual(Object.keys(mapped).sort(), ['participant', 'timers']);
  assert.deepEqual(Object.keys(mapped.participant).sort(), ['displayName', 'profilePictureURL', 'userId', 'username']);
  assert.deepEqual(Object.keys(mapped.timers[0] ?? {}).sort(), ['id', 'path', 'startedAt']);
  assert.equal('privateNote' in mapped, false);
  assert.equal('occurrenceTimeZone' in (mapped.timers[0] ?? {}), false);
});

test('active following pagination preserves server groups and removes duplicate people', () => {
  const first = activeParticipant('one', 'timer-1');
  const second = activeParticipant('two', 'timer-2');
  const merged = mergeActiveFollowingPage(
    { items: [first], nextCursor: 'page-2' },
    { items: [first, second], nextCursor: '' },
    'page-2',
  );

  assert.deepEqual(merged.items.map(({ participant }) => participant.userId), ['one', 'two']);
  assert.equal(merged.nextCursor, '');

  const refreshed = mergeActiveFollowingPage(merged, { items: [second], nextCursor: 'new' });
  assert.deepEqual(refreshed.items.map(({ participant }) => participant.userId), ['two']);
});

test('active following refreshes only when its focused route returns to the foreground', () => {
  assert.equal(shouldRefreshActiveFollowing('background', 'active', true), true);
  assert.equal(shouldRefreshActiveFollowing('inactive', 'active', true), true);
  assert.equal(shouldRefreshActiveFollowing('active', 'active', true), false);
  assert.equal(shouldRefreshActiveFollowing('background', 'inactive', true), false);
  assert.equal(shouldRefreshActiveFollowing('background', 'active', false), false);
});

test('Following renders compact grouped live timers above completed events', () => {
  assert.match(feedView, /ActiveFollowingSection/);
  assert.match(feedView, /activeTimerSeconds/);
  assert.match(feedView, /state=\{active\}/);
  assert.match(feedView, /state\.items/);
  assert.ok(feedView.indexOf('ActiveFollowingSection') < feedView.indexOf('state.items.map'));
  assert.match(feedView, /SocialProfileAvatar/);
  assert.match(feedView, /accessibilityLabel/);
  assert.match(feedView, /state\.status === 'error' && state\.items\.length > 0/);
  assert.doesNotMatch(feedView, /Card|shadowOpacity|linear-gradient/i);
});

test('an empty failed Recent Activity feed exposes an accessible full-size retry action', () => {
  const unavailable = feedView.slice(
    feedView.indexOf('function FeedUnavailable'),
    feedView.indexOf('function InteractionNotice'),
  );

  assert.match(unavailable, /onRetry: \(\) => void/);
  assert.match(unavailable, /<NativeContentUnavailable/);
  assert.match(unavailable, /<NativeButton label=\{i18n\.t\('common\.retry'\)\}/);
  assert.match(unavailable, /onPress=\{onRetry\}/);
  assert.match(feedView, /<FeedUnavailable i18n=\{i18n\} onRetry=\{onRetry\} state=\{state\} \/>/);
  assert.match(feedView, /state\.status === 'error' && hasEvents[\s\S]*?<NativeButton[\s\S]*?onPress=\{onRetry\}/);
});

test('published active state owns initial load, refresh, retry, pagination, and stale responses', () => {
  assert.match(routePresentation, /active: ActiveFollowingState/);
  assert.match(routePresentation, /loadActive/);
  assert.match(routePresentation, /loadMoreActive/);
  assert.match(routePresentation, /refreshActive/);
  assert.match(routePresentation, /retryActive/);
  assert.match(orchestration, /socialActiveFollowingOperations\.issue\(\)/);
  assert.match(orchestration, /socialActiveFollowingTarget\.current/);
  assert.match(orchestration, /\.socialActiveFollowing\(cursor \|\| undefined\)/);
  assert.match(orchestration, /loadSocialActiveFollowing/);
  assert.match(followingRoute, /useFocusEffect/);
  assert.match(followingRoute, /presentationRef\.current/);
  assert.match(followingRoute, /current\?\.refreshActive\(\)/);
  assert.match(followingRoute, /useFocusEffect\(useCallback\([\s\S]*?\}, \[\]\)\)/);
  assert.match(followingRoute, /AppState\.addEventListener\('change'/);
  assert.match(followingRoute, /focusedRef\.current/);
  assert.match(followingRoute, /shouldRefreshActiveFollowing/);
});

test('active following copy is localized for compact loading, empty, failure, and grouped states', () => {
  for (const catalog of [en, es]) {
    assert.match(catalog, /"social\.activeHeading"/);
    assert.match(catalog, /"social\.activeTimer"/);
    assert.match(catalog, /"social\.activeGroupAccessibility"/);
    assert.match(catalog, /"social\.activeUnavailable"/);
  }
});
