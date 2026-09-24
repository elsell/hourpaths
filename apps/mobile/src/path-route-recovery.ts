export type PathRouteIntent =
  | Readonly<{ kind: 'path'; pathID: string; routeKey: string }>
  | Readonly<{ kind: 'members'; pathID: string; routeKey: string }>
  | Readonly<{ kind: 'nudge-settings'; pathID: string; routeKey: string }>
  | Readonly<{ kind: 'member'; pathID: string; routeKey: string; userID: string }>
  | Readonly<{ kind: 'history'; pathID: string; routeKey: string }>
  | Readonly<{ activityID: string; kind: 'activity'; pathID: string; routeKey: string }>;

type RouteParameters = Readonly<Record<string, string | readonly string[] | undefined>>;

function singleton(value: string | readonly string[] | undefined): string | null {
  return typeof value === 'string' && value.trim() ? value : null;
}

export function pathRouteIntent(pathname: string, parameters: RouteParameters): PathRouteIntent | null {
  const segments = pathname.split('/').filter(Boolean);
  const pathID = singleton(parameters.pathID);
  if (!pathID || segments[0] !== 'path' || segments[1] !== pathID) return null;
  if (segments.length === 2) return { kind: 'path', pathID, routeKey: `path:${pathID}` };
  if (segments.length === 3 && segments[2] === 'history') {
    return { kind: 'history', pathID, routeKey: `path:${pathID}:history` };
  }
  if (segments.length === 3 && segments[2] === 'members') {
    return { kind: 'members', pathID, routeKey: `path:${pathID}:members` };
  }
  if (segments.length === 3 && segments[2] === 'nudge-settings') {
    return { kind: 'nudge-settings', pathID, routeKey: `path:${pathID}:nudge-settings` };
  }
  const userID = singleton(parameters.userID);
  if (segments.length === 4 && segments[2] === 'members' && userID && segments[3] === userID) {
    return { kind: 'member', pathID, routeKey: `path:${pathID}:member:${userID}`, userID };
  }
  const activityID = singleton(parameters.activityID);
  if (segments.length === 4 && segments[2] === 'history' && activityID && segments[3] === activityID) {
    return { activityID, kind: 'activity', pathID, routeKey: `path:${pathID}:activity:${activityID}` };
  }
  return null;
}

export type PathRouteTarget = Readonly<{
  ownerID: string;
  requestSessionToken: string;
  routeKey: string;
  sessionTokens: readonly string[];
}>;

export function createPathRouteTarget(ownerID: string, sessionToken: string, routeKey: string): PathRouteTarget {
  return { ownerID, requestSessionToken: sessionToken, routeKey, sessionTokens: [sessionToken] };
}

export function ownsPathRouteTarget(
  target: PathRouteTarget | null,
  ownerID: string | null | undefined,
  sessionToken: string | null | undefined,
  routeKey: string,
): boolean {
  return target !== null && target.ownerID === ownerID && target.routeKey === routeKey &&
    (target.requestSessionToken === sessionToken || target.sessionTokens.includes(sessionToken ?? ''));
}

export function rotatePathRouteTarget(
  target: PathRouteTarget,
  ownerID: string,
  sessionToken: string,
): PathRouteTarget {
  if (target.ownerID !== ownerID || target.sessionTokens.includes(sessionToken)) return target;
  return { ...target, sessionTokens: [...target.sessionTokens, sessionToken] };
}

export type PathDetailTarget = Readonly<{
  activityID?: string;
  ownerID: string;
  pathID: string;
  requestSessionToken: string;
  sessionTokens: readonly string[];
}>;

export function createPathDetailTarget(
  ownerID: string,
  sessionToken: string,
  pathID: string,
  activityID?: string,
): PathDetailTarget {
  return { activityID, ownerID, pathID, requestSessionToken: sessionToken, sessionTokens: [sessionToken] };
}

export function ownsPathDetailTarget(
  target: PathDetailTarget | null,
  ownerID: string | null | undefined,
  sessionToken: string | null | undefined,
  pathID: string,
  activityID?: string,
): boolean {
  return target !== null && target.ownerID === ownerID && target.pathID === pathID &&
    target.activityID === activityID &&
    (target.requestSessionToken === sessionToken || target.sessionTokens.includes(sessionToken ?? ''));
}

export function rotatePathDetailTarget(
  target: PathDetailTarget,
  ownerID: string,
  sessionToken: string,
): PathDetailTarget {
  if (target.ownerID !== ownerID || target.sessionTokens.includes(sessionToken)) return target;
  return { ...target, sessionTokens: [...target.sessionTokens, sessionToken] };
}

export type PathMemberTarget = Readonly<{
  ownerID: string;
  pathID: string;
  requestSessionToken: string;
  sessionTokens: readonly string[];
  userID?: string;
}>;

export function createPathMemberTarget(
  ownerID: string,
  sessionToken: string,
  pathID: string,
  userID?: string,
): PathMemberTarget {
  return { ownerID, pathID, requestSessionToken: sessionToken, sessionTokens: [sessionToken], userID };
}

export function ownsPathMemberTarget(
  target: PathMemberTarget | null,
  ownerID: string | null | undefined,
  sessionToken: string | null | undefined,
  pathID: string,
  userID?: string,
): boolean {
  return target !== null && target.ownerID === ownerID && target.pathID === pathID && target.userID === userID &&
    (target.requestSessionToken === sessionToken || target.sessionTokens.includes(sessionToken ?? ''));
}

export function rotatePathMemberTarget(
  target: PathMemberTarget,
  ownerID: string,
  sessionToken: string,
): PathMemberTarget {
  if (target.ownerID !== ownerID || target.sessionTokens.includes(sessionToken)) return target;
  return { ...target, sessionTokens: [...target.sessionTokens, sessionToken] };
}
