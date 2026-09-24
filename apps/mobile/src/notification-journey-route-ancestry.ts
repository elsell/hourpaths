import type { NotificationJourneyIntent } from './notification-journey-route-recovery';

export type NotificationJourneyNavigationRoute = Readonly<{
  key?: string;
  name: string;
  params?: Readonly<Record<string, string>>;
  state?: unknown;
}>;

export type NotificationJourneyNavigationState = Readonly<{
  index: number;
  key?: string;
  routes: readonly NotificationJourneyNavigationRoute[];
}>;

export function notificationJourneyNavigationState(
  intent: NotificationJourneyIntent,
): NotificationJourneyNavigationState {
  const routes: NotificationJourneyNavigationRoute[] = [{ name: '(tabs)', params: { screen: 'home' } }];
  if (intent.kind === 'notification-settings') {
    routes.push({ name: 'settings/index' }, { name: 'settings/notifications' });
  } else {
    routes.push({ name: 'notifications' });
    if (intent.kind === 'invitations') routes.push({ name: 'invitations' });
  }
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

function matches(actual: NotificationJourneyNavigationRoute, expected: NotificationJourneyNavigationRoute) {
  return actual.name === expected.name;
}

function reuse(
  actual: NotificationJourneyNavigationRoute,
  expected: NotificationJourneyNavigationRoute,
): NotificationJourneyNavigationRoute {
  const state = expected.name === '(tabs)' ? normalizeTabsState(actual.state) : actual.state;
  const expectedParams = expected.params ?? {};
  const paramsMatch = Object.entries(expectedParams).every(([key, value]) => actual.params?.[key] === value);
  if (paramsMatch && state === actual.state) return actual;
  return {
    ...actual,
    params: { ...actual.params, ...expectedParams },
    ...(state === actual.state ? {} : { state }),
  };
}

export function normalizeNotificationJourneyRouteState(
  current: NotificationJourneyNavigationState,
  intent: NotificationJourneyIntent,
): NotificationJourneyNavigationState {
  const expected = notificationJourneyNavigationState(intent);
  const routes = expected.routes.map((route) => {
    const retained = current.routes.find((candidate) => matches(candidate, route));
    return retained ? reuse(retained, route) : route;
  });
  if (current.index === expected.index && current.routes.length === routes.length &&
    current.routes.every((route, index) => route === routes[index])) return current;
  return { ...current, index: expected.index, routes };
}

export function nativeBackNotificationJourneyState(
  current: NotificationJourneyNavigationState,
): NotificationJourneyNavigationState {
  if (current.routes.length <= 1) return current;
  const routes = current.routes.slice(0, -1);
  return { ...current, index: routes.length - 1, routes };
}
