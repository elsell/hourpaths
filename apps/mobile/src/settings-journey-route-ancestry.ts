import type { SettingsJourneyIntent } from './settings-journey-route-recovery';

export type SettingsJourneyNavigationRoute = Readonly<{
  key?: string;
  name: string;
  params?: Readonly<Record<string, string>>;
  state?: unknown;
}>;

export type SettingsJourneyNavigationState = Readonly<{
  index: number;
  key?: string;
  routes: readonly SettingsJourneyNavigationRoute[];
}>;

export function settingsJourneyNavigationState(intent: SettingsJourneyIntent): SettingsJourneyNavigationState {
  const routes: SettingsJourneyNavigationRoute[] = [
    { name: '(tabs)', params: { screen: 'home' } },
    { name: 'settings/index' },
  ];
  if (intent.kind !== 'settings') routes.push({ name: `settings/${intent.kind}` });
  return { index: routes.length - 1, routes };
}

function normalizeTabsState(state: unknown): unknown {
  if (typeof state !== 'object' || state === null) return state;
  const nested = state as { index?: unknown; routes?: unknown };
  if (!Array.isArray(nested.routes)) return state;
  const homeIndex = nested.routes.findIndex((route) =>
    typeof route === 'object' && route !== null && 'name' in route &&
    (route.name === 'home' || route.name === '(tabs)/home'));
  if (homeIndex < 0 || nested.index === homeIndex) return state;
  return { ...state, index: homeIndex };
}

function reuse(
  actual: SettingsJourneyNavigationRoute,
  expected: SettingsJourneyNavigationRoute,
): SettingsJourneyNavigationRoute {
  const state = expected.name === '(tabs)' ? normalizeTabsState(actual.state) : actual.state;
  const expectedParams = expected.params ?? {};
  const paramsMatch = Object.entries(expectedParams).every(([key, value]) => actual.params?.[key] === value);
  if (paramsMatch && state === actual.state) return actual;
  return { ...actual, params: { ...actual.params, ...expectedParams }, ...(state === actual.state ? {} : { state }) };
}

export function normalizeSettingsJourneyRouteState(
  current: SettingsJourneyNavigationState,
  intent: SettingsJourneyIntent,
): SettingsJourneyNavigationState {
  const expected = settingsJourneyNavigationState(intent);
  const routes = expected.routes.map((route) => {
    const retained = current.routes.find((candidate) => candidate.name === route.name);
    return retained ? reuse(retained, route) : route;
  });
  if (current.index === expected.index && current.routes.length === routes.length &&
    current.routes.every((route, index) => route === routes[index])) return current;
  return { ...current, index: expected.index, routes };
}

export function nativeBackSettingsJourneyState(
  current: SettingsJourneyNavigationState,
): SettingsJourneyNavigationState {
  if (current.routes.length <= 1) return current;
  const routes = current.routes.slice(0, -1);
  return { ...current, index: routes.length - 1, routes };
}
