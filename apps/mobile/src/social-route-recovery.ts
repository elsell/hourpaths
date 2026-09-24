export type SocialRouteIntent =
  | Readonly<{ kind: 'following'; routeKey: 'social:following' }>
  | Readonly<{ kind: 'people'; routeKey: 'social:people' }>
  | Readonly<{ kind: 'follow-requests'; routeKey: 'social:follow-requests' }>
  | Readonly<{ kind: 'profile'; routeKey: string; username: string }>
  | Readonly<{ activityID: string; kind: 'activity'; pathID: string; routeKey: string }>
  | Readonly<{ eventID: string; kind: 'comments'; routeKey: string }>
  | Readonly<{ commentID: string; eventID: string; kind: 'comment-hearts'; routeKey: string }>;

let pendingBootstrap: SocialRouteIntent | null = null;

type RouteParameters = Readonly<Record<string, string | readonly string[] | undefined>>;
type SocialSessionCredential = Readonly<{ token: string }>;

function singleton(value: string | readonly string[] | undefined): string | null {
  return typeof value === 'string' && value.trim() ? value : null;
}

export function socialRouteIntent(pathname: string, parameters: RouteParameters): SocialRouteIntent | null {
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length === 1 && segments[0] === 'following') {
    return { kind: 'following', routeKey: 'social:following' };
  }
  if (segments.length === 2 && segments[0] === 'following' && segments[1] === 'people') {
    return { kind: 'people', routeKey: 'social:people' };
  }
  if (segments.length === 1 && segments[0] === 'follow-requests') {
    return { kind: 'follow-requests', routeKey: 'social:follow-requests' };
  }
  const username = singleton(parameters.username);
  if (segments.length === 2 && segments[0] === 'profile' && username && segments[1] === username) {
    return { kind: 'profile', routeKey: `social:profile:${username.toLowerCase()}`, username };
  }
  const pathID = singleton(parameters.pathID);
  const activityID = singleton(parameters.activityID);
  if (segments.length === 4 && segments[0] === 'following' && segments[1] === 'activity' &&
    pathID && activityID && segments[2] === pathID && segments[3] === activityID) {
    return { activityID, kind: 'activity', pathID, routeKey: `social:activity:${pathID}:${activityID}` };
  }
  const eventID = singleton(parameters.eventID);
  const commentID = singleton(parameters.commentID);
  if (segments.length === 5 && segments[0] === 'following' && segments[1] === 'comments' &&
    segments[3] === 'hearts' && eventID && commentID && segments[2] === eventID && segments[4] === commentID) {
    return { commentID, eventID, kind: 'comment-hearts', routeKey: `social:comment-hearts:${eventID}:${commentID}` };
  }
  if (segments.length === 3 && segments[0] === 'following' && segments[1] === 'comments' &&
    eventID && segments[2] === eventID) {
    return { eventID, kind: 'comments', routeKey: `social:comments:${eventID}` };
  }
  return null;
}

export function queueSocialRouteBootstrap(intent: SocialRouteIntent) { pendingBootstrap = intent; }
export function takeSocialRouteBootstrap() { const intent = pendingBootstrap; pendingBootstrap = null; return intent; }
export function scheduleSocialRouteBootstrap(intent: SocialRouteIntent, navigateHome: () => void) {
  const timeout = setTimeout(() => { queueSocialRouteBootstrap(intent); navigateHome(); }, 50);
  return () => clearTimeout(timeout);
}

export function socialRouteHref(intent: SocialRouteIntent): string {
  switch (intent.kind) {
    case 'following': return '/(tabs)/following';
    case 'people': return '/following/people';
    case 'follow-requests': return '/follow-requests';
    case 'profile': return `/profile/${encodeURIComponent(intent.username)}`;
    case 'activity': return `/following/activity/${encodeURIComponent(intent.pathID)}/${encodeURIComponent(intent.activityID)}`;
    case 'comments': return `/following/comments/${encodeURIComponent(intent.eventID)}`;
    case 'comment-hearts': return `/following/comments/${encodeURIComponent(intent.eventID)}/hearts/${encodeURIComponent(intent.commentID)}`;
  }
}

export type SocialSessionTarget<Credential extends SocialSessionCredential> = Readonly<{
  intentKey: string;
  ownerID: string;
  requestSession: Credential;
  sessionTokens: readonly string[];
}>;

export function createSocialSessionTarget<Credential extends SocialSessionCredential>(
  ownerID: string,
  requestSession: Credential,
  intentKey: string,
): SocialSessionTarget<Credential> {
  return { intentKey, ownerID, requestSession, sessionTokens: [requestSession.token] };
}

export function rotateSocialSessionTarget<Credential extends SocialSessionCredential>(
  target: SocialSessionTarget<Credential>,
  ownerID: string,
  session: Credential,
): SocialSessionTarget<Credential> {
  if (target.ownerID !== ownerID || target.sessionTokens.includes(session.token)) return target;
  return { ...target, sessionTokens: [...target.sessionTokens, session.token] };
}

export function ownsSocialSessionTarget<Credential extends SocialSessionCredential>(
  target: SocialSessionTarget<Credential> | null,
  ownerID: string | null | undefined,
  sessionToken: string | null | undefined,
  intentKey: string,
): target is SocialSessionTarget<Credential> {
  return target !== null && target.ownerID === ownerID && target.intentKey === intentKey &&
    target.sessionTokens.includes(sessionToken ?? '');
}

export function currentSocialSessionCredential<Credential extends SocialSessionCredential>(
  target: SocialSessionTarget<Credential> | null,
  ownerID: string | null | undefined,
  intentKey: string,
  current: Credential | null,
): Credential | null {
  return current && ownsSocialSessionTarget(target, ownerID, current.token, intentKey) ? current : null;
}
