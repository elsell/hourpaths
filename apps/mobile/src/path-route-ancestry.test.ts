import assert from 'node:assert/strict';
import test from 'node:test';
import {
  nativeBackPathRouteState,
  normalizePathRouteState,
  pathRouteNavigationState,
} from './path-route-ancestry';

const activity = {
  activityID: 'activity-1',
  kind: 'activity' as const,
  pathID: 'path-1',
  routeKey: 'path:path-1:activity:activity-1',
};

const member = {
  kind: 'member' as const,
  pathID: 'path-1',
  routeKey: 'path:path-1:member:user-1',
  userID: 'user-1',
};

const nudgeSettings = {
  kind: 'nudge-settings' as const,
  pathID: 'path-1',
  routeKey: 'path:path-1:nudge-settings',
};

test('cold direct activity constructs exact Home, Path, History, Activity ancestry without inferring canGoBack', () => {
  for (const initiallyCanGoBack of [false, true]) {
    const current = initiallyCanGoBack
      ? { index: 2, routes: [{ name: '(tabs)' }, { name: 'settings/index' }, { name: 'path/[pathID]/history/[activityID]', params: { activityID: 'activity-1', pathID: 'path-1' } }] }
      : { index: 0, routes: [{ name: 'path/[pathID]/history/[activityID]', params: { activityID: 'activity-1', pathID: 'path-1' } }] };
    assert.deepEqual(normalizePathRouteState(current, activity), pathRouteNavigationState(activity));
  }
});

test('warm exact ancestry is retained and native back transitions Activity to History to Path to Home', () => {
  const warm = pathRouteNavigationState(activity);
  assert.strictEqual(normalizePathRouteState(warm, activity), warm);
  const history = nativeBackPathRouteState(warm);
  const path = nativeBackPathRouteState(history);
  const home = nativeBackPathRouteState(path);
  assert.deepEqual(history.routes.map(({ name }) => name), ['(tabs)', 'path/[pathID]', 'path/[pathID]/history/index']);
  assert.deepEqual(path.routes.map(({ name }) => name), ['(tabs)', 'path/[pathID]']);
  assert.deepEqual(home.routes.map(({ name }) => name), ['(tabs)']);
});

test('normalization reuses keyed route identity and nested tab state through the native back chain', () => {
  const homeState = { index: 0, routes: [{ key: 'home-tab', name: 'home' }] };
  const tabs = { key: 'tabs-root', name: '(tabs)', params: { screen: 'settings' }, state: homeState };
  const path = { key: 'path-route', name: 'path/[pathID]', params: { pathID: 'path-1' }, state: { retained: 'path' } };
  const history = { key: 'history-route', name: 'path/[pathID]/history/index', params: { pathID: 'path-1' }, state: { retained: 'history' } };
  const detail = { key: 'activity-route', name: 'path/[pathID]/history/[activityID]', params: { activityID: 'activity-1', pathID: 'path-1' }, state: { retained: 'activity' } };
  const misleading = {
    index: 4,
    key: 'root-stack',
    routes: [tabs, { key: 'settings-route', name: 'settings/index' }, history, path, detail],
  };

  const normalized = normalizePathRouteState(misleading, activity);
  assert.deepEqual(normalized.routes.map(({ name }) => name), [
    '(tabs)',
    'path/[pathID]',
    'path/[pathID]/history/index',
    'path/[pathID]/history/[activityID]',
  ]);
  assert.equal(normalized.routes[0]?.key, 'tabs-root');
  assert.equal(normalized.key, 'root-stack');
  assert.strictEqual(normalized.routes[0]?.state, homeState);
  assert.equal(normalized.routes[0]?.params?.screen, 'home');
  assert.equal(normalized.routes[1]?.key, 'path-route');
  assert.equal(normalized.routes[2]?.key, 'history-route');
  assert.equal(normalized.routes[3]?.key, 'activity-route');

  const historyState = nativeBackPathRouteState(normalized);
  const pathState = nativeBackPathRouteState(historyState);
  const home = nativeBackPathRouteState(pathState);
  assert.strictEqual(home.routes[0]?.state, homeState);
  assert.equal(home.routes[0]?.key, 'tabs-root');
});

test('retained tabs select their existing keyed Home route without discarding inactive tab state', () => {
  const homeRoute = { key: 'home-tab', name: 'home', state: { retained: 'home' } };
  const followingRoute = { key: 'following-tab', name: 'following', state: { retained: 'following' } };
  const tabState = { index: 1, key: 'tabs-state', routes: [homeRoute, followingRoute] };
  const tabs = { key: 'tabs-root', name: '(tabs)', params: { screen: 'following' }, state: tabState };
  const normalized = normalizePathRouteState({
    index: 1,
    routes: [tabs, {
      key: 'activity-route',
      name: 'path/[pathID]/history/[activityID]',
      params: { activityID: 'activity-1', pathID: 'path-1' },
    }],
  }, activity);

  const retainedTabs = normalized.routes[0];
  assert.equal(retainedTabs?.key, 'tabs-root');
  assert.equal(retainedTabs?.params?.screen, 'home');
  assert.equal((retainedTabs?.state as typeof tabState).index, 0);
  assert.strictEqual((retainedTabs?.state as typeof tabState).routes[0], homeRoute);
  assert.strictEqual((retainedTabs?.state as typeof tabState).routes[1], followingRoute);
  assert.equal((retainedTabs?.state as typeof tabState).routes[1]?.state.retained, 'following');

  const history = nativeBackPathRouteState(normalized);
  const path = nativeBackPathRouteState(history);
  const home = nativeBackPathRouteState(path);
  const landedTabs = home.routes[0];
  const landedTabState = landedTabs?.state as typeof tabState;
  assert.equal(landedTabs?.key, 'tabs-root');
  assert.equal(landedTabState.routes[landedTabState.index]?.name, 'home');
  assert.equal(landedTabState.routes[landedTabState.index]?.key, 'home-tab');
});

test('cold direct member constructs exact Home, Path, People, Member ancestry and native back chain', () => {
  const normalized = normalizePathRouteState({
    index: 1,
    routes: [
      { key: 'tabs-root', name: '(tabs)', state: { index: 0, routes: [{ key: 'home', name: 'home' }] } },
      { key: 'direct-member', name: 'path/[pathID]/members/[userID]', params: { pathID: 'path-1', userID: 'user-1' } },
    ],
  }, member);

  assert.deepEqual(normalized.routes.map(({ name }) => name), [
    '(tabs)',
    'path/[pathID]',
    'path/[pathID]/members/index',
    'path/[pathID]/members/[userID]',
  ]);
  assert.equal(normalized.routes[0]?.key, 'tabs-root');
  assert.equal(normalized.routes[3]?.key, 'direct-member');
  const people = nativeBackPathRouteState(normalized);
  const path = nativeBackPathRouteState(people);
  const home = nativeBackPathRouteState(path);
  assert.equal(people.routes.at(-1)?.name, 'path/[pathID]/members/index');
  assert.equal(path.routes.at(-1)?.name, 'path/[pathID]');
  assert.equal(home.routes.at(-1)?.name, '(tabs)');
});

test('cold direct nudge settings constructs keyed Home, Path, Settings ancestry and native back chain', () => {
  const tabsState = {
    index: 1,
    key: 'tabs-state',
    routes: [
      { key: 'home-tab', name: 'home', state: { retained: 'home' } },
      { key: 'following-tab', name: 'following', state: { retained: 'following' } },
    ],
  };
  const normalized = normalizePathRouteState({
    index: 1,
    key: 'root-stack',
    routes: [
      { key: 'tabs-root', name: '(tabs)', params: { screen: 'following' }, state: tabsState },
      { key: 'nudge-route', name: 'path/[pathID]/nudge-settings', params: { pathID: 'path-1' } },
    ],
  }, nudgeSettings);

  assert.deepEqual(normalized.routes.map(({ name }) => name), [
    '(tabs)',
    'path/[pathID]',
    'path/[pathID]/nudge-settings',
  ]);
  assert.equal(normalized.routes[0]?.key, 'tabs-root');
  assert.equal(normalized.routes[2]?.key, 'nudge-route');
  assert.equal((normalized.routes[0]?.state as typeof tabsState).index, 0);
  const path = nativeBackPathRouteState(normalized);
  const home = nativeBackPathRouteState(path);
  assert.equal(path.routes.at(-1)?.name, 'path/[pathID]');
  assert.equal(home.routes.at(-1)?.key, 'tabs-root');
  assert.equal(((home.routes[0]?.state as typeof tabsState).routes[1]?.state as { retained: string }).retained, 'following');
});
