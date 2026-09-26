import type { SocialRouteIntent } from './social-route-recovery';

export type SocialInteractionIntent = Extract<SocialRouteIntent,
  { kind: 'activity' | 'comments' | 'comment-hearts' }>;
export type SocialNavigationRoute = Readonly<{
  key?: string;
  name: string;
  params?: Readonly<Record<string, string>>;
  state?: unknown;
}>;
export type SocialNavigationState = Readonly<{
  index: number;
  key?: string;
  routes: readonly SocialNavigationRoute[];
}>;

export function socialInteractionNavigationState(intent: SocialInteractionIntent): SocialNavigationState {
  const routes: SocialNavigationRoute[] = [{ name: '(tabs)', state: { index: 0, routes: [{ name: 'following' }] } }];
  if (intent.kind === 'activity') routes.push({
    name: 'following/activity/[pathID]/[activityID]',
    params: { activityID: intent.activityID, pathID: intent.pathID },
  });
  if (intent.kind === 'comments' || intent.kind === 'comment-hearts') routes.push({
    name: 'following/comments/[eventID]',
    params: { eventID: intent.eventID },
  });
  if (intent.kind === 'comment-hearts') routes.push({
    name: 'following/comments/[eventID]/hearts/[commentID]',
    params: { commentID: intent.commentID, eventID: intent.eventID },
  });
  return { index: routes.length - 1, routes };
}

function matches(actual: SocialNavigationRoute, expected: SocialNavigationRoute) {
  return actual.name === expected.name && Object.entries(expected.params ?? {})
    .every(([key, value]) => actual.params?.[key] === value);
}

export function normalizeSocialInteractionRouteState(
  current: SocialNavigationState,
  intent: SocialInteractionIntent,
): SocialNavigationState {
  const expected = socialInteractionNavigationState(intent);
  const active = current.routes[current.index];
  const parent = current.routes[current.index - 1];
  // A warm push already owns its return destination (for example Notifications).
  // Only synthesize ancestry when cold links lack the required parent screens.
  if (active && matches(active, expected.routes.at(-1)!) &&
    current.routes.slice(0, current.index).some((route) => route.name === '(tabs)') &&
    (intent.kind !== 'comment-hearts' || (parent && matches(parent, expected.routes[1]!)))) return current;
  if (current.index === expected.index && current.routes.length === expected.routes.length &&
    current.routes.every((route, index) => matches(route, expected.routes[index]!))) return current;
  return {
    ...current,
    index: expected.index,
    routes: expected.routes.map((route) => current.routes.find((candidate) => matches(candidate, route)) ?? route),
  };
}

export function nativeBackSocialInteractionState(current: SocialNavigationState): SocialNavigationState {
  if (current.routes.length <= 1) return current;
  const routes = current.routes.slice(0, -1);
  return { ...current, index: routes.length - 1, routes };
}
