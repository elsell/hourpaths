import type { NotificationDestination } from './push-notifications';

type RouteParameters = Readonly<Record<string, string | string[] | undefined>>;

function routeIdentifier(parameters: RouteParameters, name: string): string | null {
  const value = parameters[name];
  return typeof value === 'string' && value.trim() === value && value.length > 0
    ? value
    : null;
}

export function foregroundNotificationTargetKey(
  pathname: string,
  parameters: RouteParameters,
): string | null {
  const eventID = routeIdentifier(parameters, 'eventID');
  if (pathname.includes('/comments/') && eventID) return `comments:${eventID}`;
  const pathID = routeIdentifier(parameters, 'pathID');
  if (pathname.startsWith('/path/') && pathID) return `path:${pathID}`;
  if (pathname === '/invitations') return 'invitations';
  if (pathname === '/follow-requests') return 'follow-requests';
  const username = routeIdentifier(parameters, 'username');
  if (pathname.startsWith('/profile/') && username) return `profile:${username.toLowerCase()}`;
  return null;
}

export function notificationDestinationTargetKey(
  destination: NotificationDestination,
): string {
  switch (destination.kind) {
    case 'comments':
    case 'interaction-disabled':
      return `comments:${destination.eventID}`;
    case 'invitation':
      return 'invitations';
    case 'follow-request':
      return 'follow-requests';
    case 'profile':
      return `profile:${destination.username.toLowerCase()}`;
    case 'path':
      return `path:${destination.pathID}`;
  }
}
