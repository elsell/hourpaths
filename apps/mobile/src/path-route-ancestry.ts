import type { PathRouteIntent } from './path-route-recovery';

export type PathNavigationRoute = Readonly<{
  key?: string;
  name: string;
  params?: Readonly<Record<string, string>>;
  state?: unknown;
}>;

export type PathNavigationState = Readonly<{
  index: number;
  key?: string;
  routes: readonly PathNavigationRoute[];
}>;

export function pathRouteNavigationState(intent: PathRouteIntent): PathNavigationState {
  const routes: PathNavigationRoute[] = [
    { name: '(tabs)', params: { screen: 'home' } },
    { name: 'path/[pathID]', params: { pathID: intent.pathID } },
  ];
  if (intent.kind === 'history' || intent.kind === 'activity') {
    routes.push({ name: 'path/[pathID]/history/index', params: { pathID: intent.pathID } });
  }
  if (intent.kind === 'members' || intent.kind === 'member') {
    routes.push({ name: 'path/[pathID]/members/index', params: { pathID: intent.pathID } });
  }
  if (intent.kind === 'nudge-settings') {
    routes.push({ name: 'path/[pathID]/nudge-settings', params: { pathID: intent.pathID } });
  }
  if (intent.kind === 'member') {
    routes.push({
      name: 'path/[pathID]/members/[userID]',
      params: { pathID: intent.pathID, userID: intent.userID },
    });
  }
  if (intent.kind === 'activity') {
    routes.push({
      name: 'path/[pathID]/history/[activityID]',
      params: { activityID: intent.activityID, pathID: intent.pathID },
    });
  }
  return { index: routes.length - 1, routes };
}

function sameRoute(actual: PathNavigationRoute, expected: PathNavigationRoute): boolean {
  if (actual.name !== expected.name) return false;
  if (expected.name === '(tabs)') return true;
  return hasExpectedParams(actual, expected);
}

function hasExpectedParams(actual: PathNavigationRoute, expected: PathNavigationRoute): boolean {
  const expectedParams = expected.params ?? {};
  return Object.entries(expectedParams).every(([key, value]) => actual.params?.[key] === value);
}

function normalizeTabsState(state: unknown): unknown {
  if (typeof state !== 'object' || state === null) return state;
  const nested = state as { index?: unknown; routes?: unknown };
  if (!Array.isArray(nested.routes)) return state;
  const homeIndex = nested.routes.findIndex((route) =>
    typeof route === 'object' && route !== null &&
    'name' in route && (route.name === 'home' || route.name === '(tabs)/home'));
  if (homeIndex < 0 || nested.index === homeIndex) return state;
  return { ...state, index: homeIndex };
}

function reuseRoute(actual: PathNavigationRoute, expected: PathNavigationRoute): PathNavigationRoute {
  const expectedParams = expected.params ?? {};
  const state = expected.name === '(tabs)' ? normalizeTabsState(actual.state) : actual.state;
  if (hasExpectedParams(actual, expected) && state === actual.state) return actual;
  return {
    ...actual,
    params: { ...actual.params, ...expectedParams },
    ...(state === actual.state ? {} : { state }),
  };
}

export function normalizePathRouteState(
  current: PathNavigationState,
  intent: PathRouteIntent,
): PathNavigationState {
  const expected = pathRouteNavigationState(intent);
  return current.routes.length === expected.routes.length &&
    current.index === expected.index &&
    current.routes.every((route, index) =>
      sameRoute(route, expected.routes[index]) && reuseRoute(route, expected.routes[index]) === route)
    ? current
    : {
        ...current,
        index: expected.index,
        routes: expected.routes.map((route) => {
          const retained = current.routes.find((candidate) => sameRoute(candidate, route));
          return retained ? reuseRoute(retained, route) : route;
        }),
      };
}

export function nativeBackPathRouteState(current: PathNavigationState): PathNavigationState {
  if (current.routes.length <= 1) return current;
  const routes = current.routes.slice(0, -1);
  return { index: routes.length - 1, routes };
}
